// Package server wires transport, auth, router, and tool registry together.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/emage/cwso/orchestrator/internal/mergeengine"
	"github.com/emage/cwso/orchestrator/internal/sandbox"
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
	runner    sandbox.RunnerInterface
	caps      *dispatch.CapabilityRegistry
	emitter   *dispatch.DecisionEmitter
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

	var baselineRunner sandbox.RunnerInterface
	if cfg.SandboxRunner == "docker" {
		dockerRunner, runnerErr := sandbox.NewDockerRunner(sandbox.DockerRunnerConfig{
			Host:            cfg.SandboxDockerHost,
			DefaultImage:    cfg.SandboxImage,
			NetworkMode:     cfg.SandboxNetwork,
			CPUQuotaMicros:  cfg.SandboxCPUQuota,
			MemoryBytes:     cfg.SandboxMemory,
			PIDsLimit:       cfg.SandboxPIDs,
			StopTimeout:     time.Duration(cfg.SandboxStopSecs) * time.Second,
			CleanupRetries:  3,
			ReadOnlyRootFS:  true,
			NoNewPrivileges: true,
			DropAllCaps:     true,
		})
		if runnerErr != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("init docker runner: %w", runnerErr)
		}
		baselineRunner = dockerRunner
		log.Info().Str("runner", baselineRunner.Name()).Msg("sandbox baseline enabled")
	}
	if cfg.SandboxRunner == "gvisor" {
		gvisorRunner, runnerErr := sandbox.NewGVisorRunner(sandbox.GVisorRunnerConfig{
			Host:           cfg.SandboxDockerHost,
			DefaultImage:   cfg.SandboxImage,
			Runtime:        cfg.SandboxRuntime,
			NetworkMode:    cfg.SandboxNetwork,
			CPUQuotaMicros: cfg.SandboxCPUQuota,
			MemoryBytes:    cfg.SandboxMemory,
			PIDsLimit:      cfg.SandboxPIDs,
			StopTimeout:    time.Duration(cfg.SandboxStopSecs) * time.Second,
			CleanupRetries: 3,
			CreateTimeout:  10 * time.Second,
			ListTimeout:    3 * time.Second,
		})
		if runnerErr != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("init gvisor runner: %w", runnerErr)
		}
		baselineRunner = gvisorRunner
		log.Info().Str("runner", baselineRunner.Name()).Msg("sandbox baseline enabled")
	}
	if cfg.SandboxRunner == "firecracker" {
		firecrackerRunner, runnerErr := sandbox.NewFirecrackerRunner(sandbox.FirecrackerRunnerConfig{
			BinaryPath:          cfg.SandboxFCBin,
			ExecHelperPath:      cfg.SandboxFCHelper,
			KVMDevicePath:       cfg.SandboxKVMDevice,
			VhostNetDevicePath:  cfg.SandboxVhostNet,
			SnapshotTemplateDir: cfg.SandboxSnapshot,
			VMStateDir:          cfg.SandboxVMState,
			DefaultCommand:      []string{"/bin/sh", "-lc", "echo ${CWSO_OBJECTIVE_PROMPT:-cwso-job}"},
			CreateTimeout:       10 * time.Second,
			StopTimeout:         time.Duration(cfg.SandboxStopSecs) * time.Second,
			CleanupTimeout:      5 * time.Second,
			RequireVhostNet:     cfg.SandboxRequireVh,
			MemoryBytes:         cfg.SandboxMemory,
			CPUQuotaMicros:      cfg.SandboxCPUQuota,
		})
		if runnerErr != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("init firecracker runner: %w", runnerErr)
		}
		baselineRunner = firecrackerRunner
		log.Info().Str("runner", baselineRunner.Name()).Msg("sandbox baseline enabled")
	}
	if cfg.SandboxRunner == "router" {
		dockerRunner, dErr := sandbox.NewDockerRunner(sandbox.DockerRunnerConfig{
			Host:            cfg.SandboxDockerHost,
			DefaultImage:    cfg.SandboxImage,
			NetworkMode:     cfg.SandboxNetwork,
			CPUQuotaMicros:  cfg.SandboxCPUQuota,
			MemoryBytes:     cfg.SandboxMemory,
			PIDsLimit:       cfg.SandboxPIDs,
			StopTimeout:     time.Duration(cfg.SandboxStopSecs) * time.Second,
			CleanupRetries:  3,
			ReadOnlyRootFS:  true,
			NoNewPrivileges: true,
			DropAllCaps:     true,
		})
		if dErr != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("init tier-router docker runner: %w", dErr)
		}
		gvisorRunner, gErr := sandbox.NewGVisorRunner(sandbox.GVisorRunnerConfig{
			Host:           cfg.SandboxDockerHost,
			DefaultImage:   cfg.SandboxImage,
			Runtime:        cfg.SandboxRuntime,
			NetworkMode:    cfg.SandboxNetwork,
			CPUQuotaMicros: cfg.SandboxCPUQuota,
			MemoryBytes:    cfg.SandboxMemory,
			PIDsLimit:      cfg.SandboxPIDs,
			StopTimeout:    time.Duration(cfg.SandboxStopSecs) * time.Second,
			CleanupRetries: 3,
			CreateTimeout:  10 * time.Second,
			ListTimeout:    3 * time.Second,
		})
		if gErr != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("init tier-router gvisor runner: %w", gErr)
		}
		routerCfg := sandbox.TierRouterConfig{
			DockerRunner:       dockerRunner,
			GVisorRunner:       gvisorRunner,
			DegradedMode:       cfg.SandboxDegradedMode,
			AllowDockerTrusted: cfg.SandboxAllowDockerTrusted,
		}
		if !cfg.SandboxDegradedMode {
			fcRunner, fcErr := sandbox.NewFirecrackerRunner(sandbox.FirecrackerRunnerConfig{
				BinaryPath:          cfg.SandboxFCBin,
				ExecHelperPath:      cfg.SandboxFCHelper,
				KVMDevicePath:       cfg.SandboxKVMDevice,
				VhostNetDevicePath:  cfg.SandboxVhostNet,
				SnapshotTemplateDir: cfg.SandboxSnapshot,
				VMStateDir:          cfg.SandboxVMState,
				DefaultCommand:      []string{"/bin/sh", "-lc", "echo ${CWSO_OBJECTIVE_PROMPT:-cwso-job}"},
				CreateTimeout:       10 * time.Second,
				StopTimeout:         time.Duration(cfg.SandboxStopSecs) * time.Second,
				CleanupTimeout:      5 * time.Second,
				RequireVhostNet:     cfg.SandboxRequireVh,
				MemoryBytes:         cfg.SandboxMemory,
				CPUQuotaMicros:      cfg.SandboxCPUQuota,
			})
			if fcErr != nil {
				// Treat firecracker init failure as automatic degraded mode rather
				// than a hard startup failure; the router will fall back to gVisor.
				log.Warn().Err(fcErr).Msg("firecracker runner unavailable; tier-router entering degraded mode")
				routerCfg.DegradedMode = true
			} else {
				routerCfg.FirecrackerRunner = fcRunner
			}
		}
		tierRouter, rErr := sandbox.NewTierRouter(routerCfg)
		if rErr != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("init tier-router: %w", rErr)
		}
		baselineRunner = tierRouter
		log.Info().
			Any("degraded_mode", routerCfg.DegradedMode).
			Any("allow_docker_trusted", routerCfg.AllowDockerTrusted).
			Msg("sandbox tier-router enabled")
	}

	var capRegistry *dispatch.CapabilityRegistry
	var telemetryEmitter *dispatch.DecisionEmitter
	if cfg.HHDCapabilityRegistry {
		capRegistry = dispatch.NewCapabilityRegistry(time.Duration(cfg.HHDSnapshotTTLSeconds) * time.Second)
		if err := capRegistry.Upsert(dispatch.ProviderCapability{
			ProviderID:            "cpu-baseline",
			ContractVersion:       "dispatch.provider/v1.0",
			HealthState:           dispatch.HealthHealthy,
			LatencyClass:          "baseline",
			CostClass:             "low",
			QueueDepth:            0,
			SupportedWorkloadTags: []string{"default"},
			ReliabilityClass:      "standard",
			FeatureFlags:          []string{},
		}); err != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("seed capability registry: %w", err)
		}
	}
	if cfg.HHDDecisionTelemetry {
		redactionCfg := dispatch.TelemetryRedactionConfig{
			Enabled:          cfg.HHDTelemetryRedaction,
			RequestIDMode:    cfg.HHDTelemetryRequestIDMode,
			AnomalyNotesMode: cfg.HHDTelemetryAnomalyNotes,
			RequestIDSalt:    cfg.HHDTelemetryRedactionSalt,
		}
		var anomalyMonitor *dispatch.DecisionAnomalyMonitor
		if cfg.HHDEventMonitorEnabled {
			anomalyMonitor = dispatch.NewDecisionAnomalyMonitor(publisher, dispatch.DecisionAnomalyMonitorConfig{
				PreferEBPF:         cfg.HHDEventMonitorEBPF,
				LatencyThresholdMS: cfg.HHDEventMonitorLatencyMS,
				Redaction:          redactionCfg,
			})
		}
		telemetryEmitter = dispatch.NewDecisionEmitterWithAnomalyMonitorAndRedaction(publisher, anomalyMonitor, redactionCfg)
	}

	s := &Server{
		cfg: cfg, log: log, registry: tools.NewRegistry(), bus: bus, memory: broker,
		publisher: publisher, jobs: jobMgr, runner: baselineRunner, caps: capRegistry, emitter: telemetryEmitter,
	}
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
	if cfg.MergeEngineSocket != "" {
		if err := s.registerMergeTools(cfg.MergeEngineSocket); err != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("register merge tools: %w", err)
		}
		log.Info().Str("socket", cfg.MergeEngineSocket).Msg("merge tools enabled")
	}
	return s, nil
}

func (s *Server) registerMergeTools(socket string) error {
	client := mergeengine.NewClient(socket)
	if err := s.registry.Register(tools.NewMergeConcurrentResults(client)); err != nil {
		return err
	}
	return nil
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
	var scoreAdjuster dispatch.ScoreAdjuster
	if s.cfg.HHDWasmScoringEnabled {
		adjuster, err := dispatch.NewWasmScoreAdjuster(context.Background(), dispatch.WasmScoringConfig{
			Enabled:          s.cfg.HHDWasmScoringEnabled,
			ModulePath:       s.cfg.HHDWasmScoringModulePath,
			ExpectedSHA256:   s.cfg.HHDWasmScoringModuleSHA256,
			TrustedModuleDir: s.cfg.HHDWasmScoringTrustedDir,
			CallTimeout:      time.Duration(s.cfg.HHDWasmScoringTimeoutMS) * time.Millisecond,
			MemoryLimitPages: s.cfg.HHDWasmScoringMemoryPages,
			AllowedHostCalls: s.cfg.HHDWasmScoringHostCalls,
		})
		if err != nil {
			s.log.Warn().Err(err).Msg("wasm scoring plugin disabled; falling back to baseline policy scoring")
		} else {
			scoreAdjuster = adjuster
		}
	}

	policyCfg := dispatch.PolicyV2Config{
		Enabled:            s.cfg.HHDPolicyEngineV2,
		PolicyVersion:      dispatch.DefaultPolicyVersionV2,
		BaselineProviderID: dispatch.DefaultBaselineProviderID,
		MinConfidence:      s.cfg.HHDPolicyMinConfidence,
		MaxQueueDepth:      s.cfg.HHDPolicyMaxQueueDepth,
		ScoreAdjuster:      scoreAdjuster,
		SparseQuantized: dispatch.SparseQuantizedAssistConfig{
			Enabled:                  s.cfg.HHDSparseQuantizedEnabled,
			ProviderFeatureFlag:      "hhd.sparse_quantized_assist",
			CostLatencyTradeoff:      s.cfg.HHDSparseQuantizedTradeoff,
			QualityGuardrailMinScore: s.cfg.HHDQualityGuardrailMinScore,
		},
		SSM: dispatch.SSMAssistConfig{
			Enabled:             s.cfg.HHDSSMAssistEnabled,
			ProviderFeatureFlag: "hhd.ssm_sequence_assist",
			ThroughputBias:      s.cfg.HHDSSMThroughputBias,
			MinSequenceLength:   s.cfg.HHDSSMMinSequenceLength,
			MaxSequenceLength:   s.cfg.HHDSSMMaxSequenceLength,
			SequenceSensitivity: s.cfg.HHDSSMSequenceSensitivity,
		},
		Weights: dispatch.PolicyWeights{
			Health:      s.cfg.HHDWeightHealth,
			Reliability: s.cfg.HHDWeightReliability,
			Cost:        s.cfg.HHDWeightCost,
			Latency:     s.cfg.HHDWeightLatency,
			QueueDepth:  s.cfg.HHDWeightQueueDepth,
			Workload:    s.cfg.HHDWeightWorkload,
		},
	}
	dispatchTool := tools.NewDispatchConcurrentJobsWithDispatchPolicy(
		s.jobs,
		s.cfg.JobTimeoutSeconds,
		s.cfg.JobQueueSize,
		nil,
		"",
		s.emitter,
		s.caps,
		policyCfg,
	)
	if s.runner != nil {
		dispatchTool = tools.NewDispatchConcurrentJobsWithDispatchPolicy(
			s.jobs,
			s.cfg.JobTimeoutSeconds,
			s.cfg.JobQueueSize,
			s.runner,
			s.cfg.Workspace,
			s.emitter,
			s.caps,
			policyCfg,
		)
	}
	for _, t := range []tools.Tool{
		&tools.ReadFileSync{Workspace: s.cfg.Workspace},
		&tools.WriteFileSync{Workspace: s.cfg.Workspace},
		&tools.ListDir{Workspace: s.cfg.Workspace},
		dispatchTool,
	} {
		if err := s.registry.Register(t); err != nil {
			return err
		}
	}
	if s.emitter != nil && s.caps != nil {
		_ = s.emitter.EmitCapabilitySnapshot(s.caps.Snapshot())
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
		return transport.RunHTTP(ctx, s.cfg, s.log, s.bus, s.memory, s.publisher, s.Handle)
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
