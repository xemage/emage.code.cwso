// Package server wires transport, auth, router, and tool registry together.
package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/emage/cwso/orchestrator/internal/shadow"
	"github.com/emage/cwso/orchestrator/internal/tools"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

const (
	defaultMemoryBrokerCapacity    = 4096
	defaultMemoryBrokerIngressSize = 2048
)

// Server is the top-level orchestrator handle.
type Server struct {
	cfg       *config.Config
	log       *logging.Logger
	registry  *tools.Registry
	bus       *eventbus.Bus
	memory    *memorybroker.Broker
	publisher *memorybroker.TeePublisher
	jobs      *jobs.Manager
}

// New constructs and initializes a Server with all Phase 1 tools registered.
func New(cfg *config.Config, log *logging.Logger) (*Server, error) {
	bus := eventbus.New()
	broker := memorybroker.New(
		memorybroker.WithCapacity(defaultMemoryBrokerCapacity),
		memorybroker.WithIngressQueueSize(defaultMemoryBrokerIngressSize),
	)
	publisher := memorybroker.NewTeePublisher(bus, broker)
	jobMgr, err := jobs.NewManager(jobs.Config{
		Workers:   cfg.JobWorkers,
		QueueSize: cfg.JobQueueSize,
	}, publisher)
	if err != nil {
		broker.Close()
		return nil, fmt.Errorf("init job manager: %w", err)
	}

	s := &Server{cfg: cfg, log: log, registry: tools.NewRegistry(), bus: bus, memory: broker, publisher: publisher, jobs: jobMgr}
	if err := s.registerBaselineTools(); err != nil {
		jobMgr.Close()
		broker.Close()
		return nil, fmt.Errorf("register tools: %w", err)
	}
	if cfg.ShadowSocket != "" {
		if err := s.registerShadowTools(cfg.ShadowSocket); err != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("register shadow tools: %w", err)
		}
		log.Info().Str("socket", cfg.ShadowSocket).Msg("shadow tools enabled")
	}
	return s, nil
}

func (s *Server) registerShadowTools(socket string) error {
	client := shadow.NewClient(socket)
	for _, t := range []tools.Tool{
		tools.NewCreateShadowWorkspace(client),
		tools.NewDropShadowWorkspace(client),
		tools.NewReadShadowFile(client),
		tools.NewWriteShadowFile(client),
		tools.NewCommitShadow(client),
		tools.NewQueryAST(client),
	} {
		if err := s.registry.Register(t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) registerBaselineTools() error {
	for _, t := range []tools.Tool{
		&tools.ReadFileSync{Workspace: s.cfg.Workspace},
		&tools.WriteFileSync{Workspace: s.cfg.Workspace},
		&tools.ListDir{Workspace: s.cfg.Workspace},
		tools.NewDispatchConcurrentJobs(s.jobs, s.cfg.JobTimeoutSeconds, s.cfg.JobQueueSize),
	} {
		if err := s.registry.Register(t); err != nil {
			return err
		}
	}
	return nil
}

// Run blocks until ctx is cancelled or transport returns.
func (s *Server) Run(ctx context.Context) error {
	defer s.jobs.Close()
	defer s.memory.Close()

	switch s.cfg.Transport {
	case "stdio":
		return transport.RunStdio(ctx, s.log, s.Handle)
	case "http":
		return transport.RunHTTP(ctx, s.cfg, s.log, s.bus, s.publisher, s.Handle)
	default:
		return fmt.Errorf("unsupported transport: %s", s.cfg.Transport)
	}
}

// Handle dispatches a single JSON-RPC request to the appropriate handler
// and returns the serialized response (nil bytes for notifications).
func (s *Server) Handle(ctx context.Context, sess *transport.Session, raw []byte) ([]byte, error) {
	req, err := mcp.ParseRequest(raw)
	if err != nil {
		s.log.Warn().Err(err).Msg("parse error")
		return marshal(mcp.ErrorResponse(nil, mcp.NewError(mcp.ErrParse, "parse error: "+err.Error())))
	}

	s.log.Debug().Str("method", req.Method).Msg("handling")

	var resp *mcp.Response
	switch req.Method {
	case "initialize":
		resp = s.handleInitialize(req)
	case "tools/list":
		resp = s.handleToolsList(req)
	case "tools/call":
		resp = s.handleToolsCall(ctx, sess, req)
	case "ping":
		resp = mcp.OK(req.ID, map[string]any{"pong": true})
	case "notifications/initialized":
		return nil, nil // notification — no response
	default:
		if req.IsNotification() {
			return nil, nil
		}
		resp = mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrMethodNotFound, "method not found: "+req.Method))
	}
	return marshal(resp)
}

func marshal(r *mcp.Response) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	return json.Marshal(r)
}

func (s *Server) handleInitialize(req *mcp.Request) *mcp.Response {
	var p mcp.InitializeParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInvalidParams, err.Error()))
		}
	}
	result := mcp.InitializeResult{
		ProtocolVersion: mcp.SupportedProtocolVersion,
		Capabilities: map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		ServerInfo: mcp.ServerInfo{Name: "cwso-orchestrator", Version: "0.1.0-dev"},
	}
	return mcp.OK(req.ID, result)
}

func (s *Server) handleToolsList(req *mcp.Request) *mcp.Response {
	return mcp.OK(req.ID, mcp.ToolsListResult{Tools: s.registry.List()})
}

func (s *Server) handleToolsCall(ctx context.Context, sess *transport.Session, req *mcp.Request) *mcp.Response {
	var p mcp.ToolCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInvalidParams, err.Error()))
	}
	if p.Name == "" {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInvalidParams, "tool name is required"))
	}

	role := tools.RoleOrchestrator
	if sess != nil && sess.Role != "" {
		role = tools.Role(sess.Role)
	}
	tool, ok := s.registry.Authorized(p.Name, role)
	if tool == nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrToolNotFound, "tool not found: "+p.Name))
	}
	if !ok {
		s.log.Warn().Str("tool", p.Name).Str("role", string(role)).Msg("permission denied")
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrPermissionDenied,
			fmt.Sprintf("role %q may not invoke tool %q", role, p.Name)))
	}

	res, err := tool.Execute(ctx, p.Arguments)
	if err != nil {
		s.log.Error().Err(err).Str("tool", p.Name).Msg("tool execution failed")
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrToolExecution, err.Error()))
	}
	if res == nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInternal, "tool returned nil"))
	}
	return mcp.OK(req.ID, res)
}

// Registry exposes the tool registry for tests.
func (s *Server) Registry() *tools.Registry { return s.registry }

// Jobs exposes the async jobs manager for internal integrations and tests.
func (s *Server) Jobs() *jobs.Manager { return s.jobs }

// Memory exposes the event-sourced broker for internal telemetry integrations and tests.
func (s *Server) Memory() *memorybroker.Broker { return s.memory }
