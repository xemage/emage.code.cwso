// Package server wires transport, auth, router, and tool registry together.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/emage/cwso/orchestrator/internal/config"
	"github.com/emage/cwso/orchestrator/internal/dashboard"
	"github.com/emage/cwso/orchestrator/internal/dispatch"
	"github.com/emage/cwso/orchestrator/internal/eventbus"
	"github.com/emage/cwso/orchestrator/internal/hal"
	"github.com/emage/cwso/orchestrator/internal/jobs"
	"github.com/emage/cwso/orchestrator/internal/logging"
	"github.com/emage/cwso/orchestrator/internal/mcp"
	"github.com/emage/cwso/orchestrator/internal/memorybroker"
	"github.com/emage/cwso/orchestrator/internal/mergeengine"
	"github.com/emage/cwso/orchestrator/internal/rollout"
	"github.com/emage/cwso/orchestrator/internal/sandbox"
	"github.com/emage/cwso/orchestrator/internal/shadow"
	"github.com/emage/cwso/orchestrator/internal/sparse"
	"github.com/emage/cwso/orchestrator/internal/tools"
	"github.com/emage/cwso/orchestrator/internal/transport"
)

const (
	defaultMemoryBrokerCapacity    = 4096
	defaultMemoryBrokerIngressSize = 2048
)

// Server is the top-level orchestrator handle.
type Server struct {
	cfg           *config.Config
	log           *logging.Logger
	registry      *tools.Registry
	bus           *eventbus.Bus
	memory        *memorybroker.Broker
	publisher     *memorybroker.TeePublisher
	jobs          *jobs.Manager
	runner        sandbox.RunnerInterface
	caps          *dispatch.CapabilityRegistry
	emitter       *dispatch.DecisionEmitter
	capSyncer     *dispatch.CapabilitySyncer
	spikeSubs     *dispatch.SpikeSubscriptionRegistry
	sparseAgents  *dispatch.SparseAgentRegistry
	astSink       dispatch.WriteEventSink
	rolloutSvc    *rollout.Service
	clientMetrics *dashboard.ClientMetrics
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
		// Shadow mode (no live HAL): seed the static provider catalog so the policy engine
		// has something to route against. With a live HAL socket the catalog is populated
		// by the CapabilitySyncer instead (see initCapabilitySync below).
		if cfg.HHDHardwareAwareDispatch && cfg.HALSocket == "" {
			for _, prov := range shadowHardwareProviders() {
				if err := capRegistry.Upsert(prov); err != nil {
					jobMgr.Close()
					broker.Close()
					return nil, fmt.Errorf("seed shadow provider %q: %w", prov.ProviderID, err)
				}
			}
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

	var spikeSubs *dispatch.SpikeSubscriptionRegistry
	if cfg.ASTSpikeResourcesEnabled {
		spikeSubs = dispatch.NewSpikeSubscriptionRegistry()
	}

	var sparseAgents *dispatch.SparseAgentRegistry
	if cfg.SparseAgentsEnabled && cfg.SparseSocket != "" {
		sparseAgents = dispatch.NewSparseAgentRegistry()
	}

	astSink := buildASTWriteSink(cfg, publisher, log)

	var rolloutSvc *rollout.Service
	if cfg.RolloutAPIEnabled {
		var rolloutClient *rollout.Client
		if cfg.RolloutSocket != "" {
			rolloutClient = rollout.NewClient(cfg.RolloutSocket)
		}
		var prefixRouter *rollout.PrefixRouter
		if cfg.RolloutKVPrefixRouterEnabled {
			promptHash := cfg.RolloutSystemPromptHash
			if promptHash == "" {
				promptHash = rollout.HashSystemPrompt(cfg.RolloutSystemPrompt)
			}
			var resolver rollout.WorkspaceResolver
			if cfg.ShadowSocket != "" {
				resolver = rollout.NewShadowWorkspaceResolver(shadow.NewClient(cfg.ShadowSocket))
			}
			prefixRouter = rollout.NewPrefixRouter(rollout.PrefixRouterConfig{
				Enabled:          true,
				SystemPromptHash: promptHash,
				Resolver:         resolver,
				Client:           rolloutClient,
			})
			log.Info().Msg("rollout KV prefix router enabled")
		}
		rolloutSvc = rollout.NewService(broker, rolloutClient, prefixRouter)
		if cfg.RolloutGatewayStagingEnabled {
			gwCfg := rollout.GatewayConfigFrom(cfg, rolloutClient)
			gateway, err := rollout.NewGateway(gwCfg, rolloutSvc)
			if err != nil {
				jobMgr.Close()
				broker.Close()
				return nil, fmt.Errorf("rollout gateway: %w", err)
			}
			rolloutSvc.AttachGateway(gateway)
			log.Info().Msg("rollout gateway staging enabled (INIT/READY/RUNNING/POSTRUN pools)")
		}
		if cfg.RolloutEvaluatorRegistryEnabled {
			registry := rollout.NewRegistry(rollout.RegistryConfigFrom(cfg, broker, rolloutClient))
			rolloutSvc.SetEvaluatorRegistry(registry)
			log.Info().Msg("rollout evaluator registry enabled")
		}
		if cfg.RolloutTrajectoryBuilderEnabled {
			rolloutSvc.SetTrajectoryBuilder(rollout.BuilderConfigFrom(cfg))
			log.Info().Str("strategy", cfg.RolloutTrajectoryBuilderStrategy).Msg("rollout trajectory builder v2 enabled")
		}
		log.Info().Msg("rollout Polar REST API enabled (/rollout/*)")
	}

	s := &Server{
		cfg: cfg, log: log, registry: tools.NewRegistry(), bus: bus, memory: broker,
		publisher: publisher, jobs: jobMgr, runner: baselineRunner, caps: capRegistry, emitter: telemetryEmitter,
		spikeSubs: spikeSubs, sparseAgents: sparseAgents, astSink: astSink, rolloutSvc: rolloutSvc,
		clientMetrics: &dashboard.ClientMetrics{},
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
		if err := s.registerMergeTools(cfg); err != nil {
			jobMgr.Close()
			broker.Close()
			return nil, fmt.Errorf("register merge tools: %w", err)
		}
		log.Info().Str("socket", cfg.MergeEngineSocket).Msg("merge tools enabled")
	}
	s.initCapabilitySync()
	return s, nil
}

// initCapabilitySync wires the live capability syncer when hardware-aware dispatch runs
// against a real HAL. It performs one synchronous sync so the first snapshot reflects live
// adapter health; if the HAL is unreachable at boot it falls back to the static catalog so
// routing still works (the CPU baseline remains terminal-safe regardless). The background
// refresh is started later by Run, bound to the server lifecycle.
func (s *Server) initCapabilitySync() {
	if !s.cfg.HHDHardwareAwareDispatch || s.cfg.HALSocket == "" || s.caps == nil {
		return
	}
	client := hal.NewClient(s.cfg.HALSocket)
	fetcher := halCapabilityFetcher{client: client}
	syncer := dispatch.NewCapabilitySyncer(fetcher, s.caps, time.Duration(s.cfg.HALCapabilitySyncSeconds)*time.Second)
	syncer.OnError = func(err error) {
		s.log.Warn().Err(err).Msg("capability sync: HAL refresh failed (using last-known/stale catalog)")
	}
	syncer.OnSync = func(n int) {
		s.log.Debug().Int("providers", n).Msg("capability sync: refreshed from live HAL")
	}
	if n, err := syncer.SyncOnce(); err != nil {
		s.log.Warn().Err(err).Msg("capability sync: initial HAL sync failed; seeding static catalog fallback")
		for _, prov := range shadowHardwareProviders() {
			if upErr := s.caps.Upsert(prov); upErr != nil {
				s.log.Warn().Err(upErr).Str("provider", prov.ProviderID).Msg("capability sync: static seed failed")
			}
		}
	} else {
		s.log.Info().Int("providers", n).Msg("capability sync: live HAL catalog loaded")
	}
	s.capSyncer = syncer
}

// halCapabilityFetcher adapts the HAL client's capability call to the dispatch
// CapabilityFetcher contract, mapping the wire records to the policy-facing struct.
type halCapabilityFetcher struct {
	client *hal.Client
}

func (f halCapabilityFetcher) FetchCapabilities() ([]dispatch.ProviderCapability, error) {
	live, err := f.client.Capabilities()
	if err != nil {
		return nil, err
	}
	out := make([]dispatch.ProviderCapability, 0, len(live))
	for _, c := range live {
		out = append(out, dispatch.ProviderCapability{
			ProviderID:            c.ProviderID,
			ContractVersion:       c.ContractVersion,
			HealthState:           c.HealthState,
			LatencyClass:          c.LatencyClass,
			CostClass:             c.CostClass,
			QueueDepth:            c.QueueDepth,
			SupportedWorkloadTags: c.SupportedWorkloadTags,
			ReliabilityClass:      c.ReliabilityClass,
			FeatureFlags:          c.FeatureFlags,
		})
	}
	return out, nil
}

// shadowHardwareProviders returns the Phase 6 shadow-mode provider catalog used
// by hardware-aware dispatch. These represent specialized backends (LPU, dense
// GPU, SSM long-context) advertised to the deterministic policy engine. Until
// live HAL adapters land (Phase 6 T082-T084) selection is exercised end-to-end
// but jobs execute as context-respecting no-ops; the CPU baseline remains the
// terminal-safe fallback.
func shadowHardwareProviders() []dispatch.ProviderCapability {
	return []dispatch.ProviderCapability{
		{
			ProviderID:            "lpu-realtime",
			ContractVersion:       "dispatch.provider/v1.0",
			HealthState:           dispatch.HealthHealthy,
			LatencyClass:          "ultra",
			CostClass:             "medium",
			QueueDepth:            0,
			SupportedWorkloadTags: []string{"realtime"},
			ReliabilityClass:      "gold",
			FeatureFlags:          []string{},
		},
		{
			ProviderID:            "gpu-accelerated",
			ContractVersion:       "dispatch.provider/v1.0",
			HealthState:           dispatch.HealthHealthy,
			LatencyClass:          "fast",
			CostClass:             "high",
			QueueDepth:            0,
			SupportedWorkloadTags: []string{"inference-heavy", "deterministic-edit"},
			ReliabilityClass:      "gold",
			FeatureFlags:          []string{"hhd.sparse_quantized_assist"},
		},
		{
			ProviderID:            "ssm-longctx",
			ContractVersion:       "dispatch.provider/v1.0",
			HealthState:           dispatch.HealthHealthy,
			LatencyClass:          "baseline",
			CostClass:             "medium",
			QueueDepth:            0,
			SupportedWorkloadTags: []string{"long-context"},
			ReliabilityClass:      "gold",
			FeatureFlags:          []string{"hhd.ssm_sequence_assist"},
		},
	}
}

func (s *Server) registerMergeTools(cfg *config.Config) error {
	client := mergeengine.NewClient(cfg.MergeEngineSocket)
	rewards := rollout.NewRewardEmitter(cfg.RolloutRewardEnabled, s.publisher)
	tool := tools.NewMergeConcurrentResultsWithRewards(client, rewards)
	if err := s.registry.Register(tool); err != nil {
		return err
	}
	if cfg.RolloutRewardEnabled {
		s.log.Info().Msg("merge programmatic rewards enabled (rollout/reward topic)")
	}
	return nil
}

// buildASTWriteSink constructs the AST write-spike monitor + semantic filter and fans them
// into a single sink that the write_shadow_file feeder drives (T118). Returns nil when the
// monitor is disabled, so no feeder is attached. Both stages publish to the broker spike
// topics consumed by the T117 cwso://spikes resources.
func buildASTWriteSink(cfg *config.Config, publisher *memorybroker.TeePublisher, log *logging.Logger) dispatch.WriteEventSink {
	if !cfg.ASTSpikeMonitorEnabled {
		return nil
	}
	redaction := dispatch.TelemetryRedactionConfig{
		Enabled:          cfg.HHDTelemetryRedaction,
		RequestIDMode:    cfg.HHDTelemetryRequestIDMode,
		AnomalyNotesMode: cfg.HHDTelemetryAnomalyNotes,
		RequestIDSalt:    cfg.HHDTelemetryRedactionSalt,
	}
	monitor := dispatch.NewASTWriteSpikeMonitor(publisher, dispatch.ASTWriteSpikeMonitorConfig{
		PreferEBPF:  cfg.ASTSpikePreferEBPF,
		WindowMS:    cfg.ASTSpikeWindowMS,
		Threshold:   cfg.ASTSpikeThreshold,
		DebounceMS:  cfg.ASTSpikeDebounceMS,
		MaxHotPaths: cfg.ASTSpikeMaxHotPaths,
		Redaction:   redaction,
	})
	filter := dispatch.NewASTSpikeFilter(publisher, dispatch.ASTSpikeFilterConfig{
		PreferEBPF:        cfg.ASTSpikePreferEBPF,
		SemanticThreshold: dispatch.SpikeKind(cfg.ASTSpikeSemanticThreshold),
		ConflictWindowMS:  cfg.ASTSpikeConflictWindowMS,
		SignatureTTLMS:    cfg.ASTSpikeSignatureTTLMS,
		MaxConflictPeers:  cfg.ASTSpikeMaxConflictPeers,
		Redaction:         redaction,
	})
	log.Info().Msg("ast spike monitor enabled: write_shadow_file feeds volume monitor + semantic filter")
	return dispatch.NewWriteEventFanout(monitor, filter)
}

func (s *Server) registerShadowTools(socket string) error {
	client := shadow.NewClient(socket)
	writeTool := tools.Tool(tools.NewWriteShadowFile(client))
	if s.astSink != nil {
		writeTool = tools.NewWriteShadowFileWithObserver(client, s.astSink)
	}
	for _, t := range []tools.Tool{
		tools.NewCreateShadowWorkspace(client),
		tools.NewDropShadowWorkspace(client),
		tools.NewReadShadowFile(client),
		writeTool,
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
	baseTools := []tools.Tool{
		&tools.ReadFileSync{Workspace: s.cfg.Workspace},
		&tools.WriteFileSync{Workspace: s.cfg.Workspace},
		&tools.ListDir{Workspace: s.cfg.Workspace},
		dispatchTool,
	}
	if s.spikeSubs != nil {
		baseTools = append(baseTools, tools.NewSubscribeASTSpikes(s.spikeSubs))
		s.log.Info().Msg("ast spike resources enabled: subscribe_ast_spikes tool + cwso://spikes resources")
	}
	if s.sparseAgents != nil && s.cfg.SparseSocket != "" {
		sparseTool := tools.NewCreateEphemeralSparseAgent(
			sparse.NewClient(s.cfg.SparseSocket),
			s.sparseAgents,
			s.publisher,
			s.cfg.SparseHostRAMCapMB,
		)
		if s.cfg.SparseQualityGuardrailEnabled && s.caps != nil && s.cfg.HHDPolicyEngineV2 {
			guardrail := &tools.SparseAgentGuardrail{
				Enabled:   true,
				MinScore:  s.cfg.HHDQualityGuardrailMinScore,
				Policy:    policyCfg,
				Snapshots: s.caps,
				Jobs:      s.jobs,
				Timeout:   time.Duration(s.cfg.JobTimeoutSeconds) * time.Second,
			}
			if s.cfg.HALSocket != "" {
				guardrail.HAL = hal.NewClient(s.cfg.HALSocket)
			}
			sparseTool = sparseTool.WithSparseQualityGuardrail(guardrail)
			s.log.Info().
				Str("min_score", fmt.Sprintf("%g", s.cfg.HHDQualityGuardrailMinScore)).
				Msg("sparse quality guardrail enabled: quality_floor breach escalates to dense GPU")
		}
		baseTools = append(baseTools, sparseTool)
		s.log.Info().Str("socket", s.cfg.SparseSocket).Msg("sparse micro-agents enabled: create_ephemeral_sparse_agent + cwso://agents telemetry")
	}
	if s.cfg.HHDHardwareAwareDispatch && s.caps != nil {
		if s.cfg.HALSocket != "" {
			s.log.Info().Str("socket", s.cfg.HALSocket).Msg("hardware-aware dispatch: live HAL execution enabled")
			baseTools = append(baseTools, tools.NewDispatchHardwareAwareJobWithHAL(
				s.jobs,
				s.cfg.JobTimeoutSeconds,
				s.emitter,
				s.caps,
				policyCfg,
				hal.NewClient(s.cfg.HALSocket),
			))
		} else {
			s.log.Info().Msg("hardware-aware dispatch: shadow mode (no HAL socket configured)")
			baseTools = append(baseTools, tools.NewDispatchHardwareAwareJob(
				s.jobs,
				s.cfg.JobTimeoutSeconds,
				s.emitter,
				s.caps,
				policyCfg,
			))
		}
	}
	for _, t := range baseTools {
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

	if s.capSyncer != nil {
		s.capSyncer.Start(ctx)
	}

	switch s.cfg.Transport {
	case "stdio":
		return transport.RunStdio(ctx, s.log, s.Handle)
	case "http":
		var httpOpts []transport.HTTPOption
		if resolver := s.subscriptionResolver(); resolver != nil {
			httpOpts = append(httpOpts, transport.WithSubscriptionResolver(resolver))
		}
		if s.rolloutSvc != nil {
			httpOpts = append(httpOpts, transport.WithRolloutAPI(rollout.NewHTTPHandler(s.rolloutSvc)))
		}
		httpOpts = append(httpOpts, transport.WithRequestMetrics(s.clientMetrics))
		if dh := s.buildDashboardHandler(); dh != nil {
			httpOpts = append(httpOpts, transport.WithDashboardHandler(dh))
		}
		return transport.RunHTTP(ctx, s.cfg, transport.HTTPHandlerConfig{
			Log:             s.log,
			Bus:             s.bus,
			Broker:          s.memory,
			SamplePublisher: s.publisher,
			Handler:         s.Handle,
		}, httpOpts...)
	default:
		return fmt.Errorf("unsupported transport: %s", s.cfg.Transport)
	}
}

// buildDashboardHandler constructs the dashboard handler. Returns nil if token is unset.
func (s *Server) buildDashboardHandler() *dashboard.Handler {
	sidecars := map[string]string{
		"git_shadow":   s.cfg.ShadowSocket,
		"merge_engine": s.cfg.MergeEngineSocket,
		"hal":          s.cfg.HALSocket,
		"rollout":      s.cfg.RolloutSocket,
		"sparse":       s.cfg.SparseSocket,
	}

	flags := map[string]bool{
		"hhd_capability_registry": s.cfg.HHDCapabilityRegistry,
		"hhd_decision_telemetry":  s.cfg.HHDDecisionTelemetry,
		"ast_spike_monitor":       s.cfg.ASTSpikeMonitorEnabled,
		"sparse_agents":           s.cfg.SparseAgentsEnabled,
		"rollout_api":             s.cfg.RolloutAPIEnabled,
		"rollout_reward":          s.cfg.RolloutRewardEnabled,
	}

	warnings := s.configWarnings()

	jobStats := func() dashboard.JobsSnapshotRaw {
		snap := s.jobs.Stats()
		return dashboard.JobsSnapshotRaw{
			Workers:        snap.Workers,
			QueueCapacity:  snap.QueueCapacity,
			QueueDepth:     snap.QueueDepth,
			Active:         snap.Active,
			TotalCompleted: snap.TotalCompleted,
			TotalFailed:    snap.TotalFailed,
		}
	}

	rolloutSnap := func() dashboard.RolloutSnapshot {
		if s.rolloutSvc == nil || !s.cfg.RolloutAPIEnabled {
			return dashboard.RolloutSnapshot{Enabled: false}
		}
		fleet := s.rolloutSvc.FleetStatus(context.Background())
		return dashboard.RolloutSnapshot{
			Enabled:     true,
			ActiveTasks: fleet.RunningSessions + fleet.PendingSessions,
		}
	}

	h := dashboard.New(dashboard.Config{
		Token:     s.cfg.DashboardToken,
		Sidecars:  sidecars,
		Metrics:   s.clientMetrics,
		ClientMet: s.clientMetrics,
		// Log wires structured auth-failure logging for dashboard routes
		// (F-C061-01); without it, dashboard auth failures are only
		// counted, never logged, and an operator has no log-based signal
		// of a brute-force attempt in progress.
		Log: s.log,
		ConfigSnap: dashboard.ConfigSnapshot{
			Transport:     s.cfg.Transport,
			SandboxRunner: s.cfg.SandboxRunner,
			FeatureFlags:  flags,
			Warnings:      warnings,
		},
		JobStats: jobStats,
		Rollout:  rolloutSnap,
	})
	// Zero the raw token from config after hashing to limit its heap lifetime (F5).
	s.cfg.DashboardToken = ""
	return h
}

// configWarnings returns non-fatal operator warnings derived from the current config.
func (s *Server) configWarnings() []string {
	var w []string
	if s.cfg.Transport == "http" && s.cfg.JWTSecret == "" {
		w = append(w, "CWSO_JWT_SECRET not set")
	}
	if s.cfg.ShadowSocket == "" {
		w = append(w, "shadow tools disabled (CWSO_GIT_SHADOW_SOCKET not set)")
	}
	if s.cfg.MergeEngineSocket == "" {
		w = append(w, "merge tools disabled (CWSO_MERGE_ENGINE_SOCKET not set)")
	}
	return w
}

// Handle dispatches a single JSON-RPC request to the appropriate handler
// and returns the serialized response (nil bytes for notifications).
func (s *Server) Handle(ctx context.Context, sess *transport.Session, raw []byte) ([]byte, error) {
	req, err := mcp.ParseRequest(raw)
	if err != nil {
		// Select the spec-correct JSON-RPC error code for the failure branch
		// (RequestError.Code) instead of collapsing every ParseRequest
		// failure to Parse error. See mcp.RequestError / mcp.ParseRequest.
		code := mcp.ErrParse
		var reqErr *mcp.RequestError
		if errors.As(err, &reqErr) {
			code = reqErr.Code
		}
		s.log.Warn().Err(err).Int("code", code).Msg("parse error")
		return marshal(mcp.ErrorResponse(nil, mcp.NewError(code, "parse error: "+err.Error())))
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
	case "resources/list":
		resp = s.handleResourcesList(req)
	case "resources/templates/list":
		resp = s.handleResourceTemplatesList(req)
	case "resources/read":
		resp = s.handleResourcesRead(req)
	case "resources/subscribe":
		resp = s.handleResourcesSubscribe(req)
	case "resources/unsubscribe":
		resp = s.handleResourcesUnsubscribe(req)
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
	caps := map[string]any{
		"tools": map[string]any{"listChanged": false},
	}
	if s.spikeSubs != nil || s.sparseAgents != nil {
		// listChanged is false, not true: notifications/resources/list_changed
		// is never published anywhere in this codebase (confirmed by the C030
		// gap analysis and the conformance suite). Advertising listChanged:true
		// while never publishing it was a genuine spec-conformance defect
		// (capability/behavior mismatch) flagged by ADR-013 as a required fix
		// for C032 — resolved here by truthfully advertising the capability
		// this server can actually deliver, rather than by implementing the
		// publish path (out of scope for this task; see ADR-013 alternatives).
		// subscribe:true remains accurate: resources/subscribe is implemented
		// and accepted (its own, separate, documented gap — the subscription
		// push itself uses a non-spec SSE mechanism, not
		// notifications/resources/updated — is unaffected by this fix).
		caps["resources"] = map[string]any{"subscribe": true, "listChanged": false}
	}
	result := mcp.InitializeResult{
		ProtocolVersion: mcp.SupportedProtocolVersion,
		Capabilities:    caps,
		ServerInfo:      mcp.ServerInfo{Name: "cwso-orchestrator", Version: "0.1.0-dev"},
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

// defaultSpikeSnapshotLimit bounds how many recent matching events resources/read replays.
const defaultSpikeSnapshotLimit = 100

func (s *Server) resourcesEnabled(req *mcp.Request) (*mcp.Response, bool) {
	if s.spikeSubs == nil && s.sparseAgents == nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrMethodNotFound, "method not found: "+req.Method)), false
	}
	return nil, true
}

func (s *Server) handleResourcesList(req *mcp.Request) *mcp.Response {
	if errResp, ok := s.resourcesEnabled(req); !ok {
		return errResp
	}
	// Non-nil, explicitly-empty slice — mirrors handleResourceTemplatesList's
	// make([]mcp.ResourceTemplate, 0, 2) pattern below. A nil slice marshals
	// to JSON `null`; this marshals to `[]`, which is what the MCP spec
	// (and schema-strict clients such as wong2/mcp-cli's Zod-validated SDK)
	// require for an empty resources/list result. See
	// docs/artifacts/mcp-client-compatibility-v1.md Cross-cutting Finding A.
	resources := make([]mcp.Resource, 0, 2)
	if s.spikeSubs != nil {
		for _, sub := range s.spikeSubs.List() {
			resources = append(resources, mcp.Resource{
				URI:         sub.URI(),
				Name:        "AST spike stream " + sub.ID,
				Description: fmt.Sprintf("Semantic AST spike events (threshold=%s) for path=%q", sub.SemanticThreshold, sub.Path),
				MimeType:    "application/json",
			})
		}
	}
	if s.sparseAgents != nil {
		for _, agent := range s.sparseAgents.List() {
			resources = append(resources, mcp.Resource{
				URI:         agent.StreamURI,
				Name:        "Sparse agent telemetry " + agent.ID,
				Description: fmt.Sprintf("Telemetry stream for sparse agent %s (skill_domain=%q)", agent.ID, agent.SkillDomain),
				MimeType:    "application/json",
			})
		}
	}
	return mcp.OK(req.ID, mcp.ResourcesListResult{Resources: resources})
}

func (s *Server) handleResourceTemplatesList(req *mcp.Request) *mcp.Response {
	if errResp, ok := s.resourcesEnabled(req); !ok {
		return errResp
	}
	templates := make([]mcp.ResourceTemplate, 0, 2)
	if s.spikeSubs != nil {
		templates = append(templates, mcp.ResourceTemplate{
			URITemplate: dispatch.SpikeResourcePrefix + "{subscription_id}",
			Name:        "ast_spike_stream",
			Description: "Semantic AST write-spike stream created via subscribe_ast_spikes. Read for a recent snapshot or open over SSE (GET /mcp?subscription=<id>) for the live stream.",
			MimeType:    "application/json",
		})
	}
	if s.sparseAgents != nil {
		templates = append(templates, mcp.ResourceTemplate{
			URITemplate: dispatch.AgentResourcePrefix + "{wasm_agent_id}/telemetry",
			Name:        "sparse_agent_telemetry",
			Description: "Sparse Wasm micro-agent telemetry created via create_ephemeral_sparse_agent. Read for a snapshot or open over SSE (GET /mcp?subscription=<wasm_agent_id>).",
			MimeType:    "application/json",
		})
	}
	return mcp.OK(req.ID, mcp.ResourceTemplatesListResult{ResourceTemplates: templates})
}

func (s *Server) handleResourcesRead(req *mcp.Request) *mcp.Response {
	if errResp, ok := s.resourcesEnabled(req); !ok {
		return errResp
	}
	var p mcp.ResourceURIParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInvalidParams, err.Error()))
	}
	if agentID, ok := dispatch.ParseAgentTelemetryResourceID(p.URI); ok {
		return s.readAgentTelemetryResource(req, agentID, p.URI)
	}
	return s.readSpikeResource(req, p.URI)
}

func (s *Server) readSpikeResource(req *mcp.Request, uri string) *mcp.Response {
	if s.spikeSubs == nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown resource uri: "+uri))
	}
	id, ok := dispatch.ParseSpikeResourceID(uri)
	if !ok {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown resource uri: "+uri))
	}
	sub, ok := s.spikeSubs.Get(id)
	if !ok {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown subscription: "+id))
	}

	type snapshotItem struct {
		Topic    string          `json:"topic"`
		Sequence uint64          `json:"sequence"`
		At       string          `json:"at"`
		Event    json.RawMessage `json:"event"`
	}
	items := make([]snapshotItem, 0)
	if s.memory != nil {
		records := s.memory.Query(memorybroker.QueryOptions{
			Topics: dispatch.ASTSpikeTopics(),
			Limit:  defaultSpikeSnapshotLimit,
		})
		for _, rec := range records {
			if !sub.Allow(rec.Topic, rec.Payload) {
				continue
			}
			items = append(items, snapshotItem{
				Topic:    rec.Topic,
				Sequence: rec.Sequence,
				At:       rec.At.UTC().Format(time.RFC3339Nano),
				Event:    rec.Payload,
			})
		}
	}
	body, err := json.Marshal(map[string]any{
		"subscription_id":    sub.ID,
		"semantic_threshold": string(sub.SemanticThreshold),
		"path":               sub.Path,
		"workspace_scope":    sub.WorkspaceScope,
		"events":             items,
	})
	if err != nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInternal, err.Error()))
	}
	return mcp.OK(req.ID, mcp.ResourceReadResult{Contents: []mcp.ResourceContents{{
		URI:      sub.URI(),
		MimeType: "application/json",
		Text:     string(body),
	}}})
}

func (s *Server) readAgentTelemetryResource(req *mcp.Request, agentID, uri string) *mcp.Response {
	if s.sparseAgents == nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown resource uri: "+uri))
	}
	if _, ok := s.sparseAgents.Get(agentID); !ok {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown agent: "+agentID))
	}
	filter := &dispatch.AgentTelemetryFilter{AgentID: agentID}
	type snapshotItem struct {
		Topic    string          `json:"topic"`
		Sequence uint64          `json:"sequence"`
		At       string          `json:"at"`
		Event    json.RawMessage `json:"event"`
	}
	items := make([]snapshotItem, 0)
	if s.memory != nil {
		records := s.memory.Query(memorybroker.QueryOptions{
			Topics: []string{dispatch.TopicAgentTelemetry},
			Limit:  defaultSpikeSnapshotLimit,
		})
		for _, rec := range records {
			if !filter.Allow(rec.Topic, rec.Payload) {
				continue
			}
			items = append(items, snapshotItem{
				Topic:    rec.Topic,
				Sequence: rec.Sequence,
				At:       rec.At.UTC().Format(time.RFC3339Nano),
				Event:    rec.Payload,
			})
		}
	}
	body, err := json.Marshal(map[string]any{
		"wasm_agent_id": agentID,
		"events":        items,
	})
	if err != nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInternal, err.Error()))
	}
	return mcp.OK(req.ID, mcp.ResourceReadResult{Contents: []mcp.ResourceContents{{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(body),
	}}})
}

func (s *Server) handleResourcesSubscribe(req *mcp.Request) *mcp.Response {
	if errResp, ok := s.resourcesEnabled(req); !ok {
		return errResp
	}
	var p mcp.ResourceURIParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInvalidParams, err.Error()))
	}
	if agentID, ok := dispatch.ParseAgentTelemetryResourceID(p.URI); ok {
		if s.sparseAgents == nil {
			return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown resource uri: "+p.URI))
		}
		if _, ok := s.sparseAgents.Get(agentID); !ok {
			return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown agent: "+agentID))
		}
		return mcp.OK(req.ID, map[string]any{})
	}
	id, ok := dispatch.ParseSpikeResourceID(p.URI)
	if !ok || s.spikeSubs == nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown resource uri: "+p.URI))
	}
	if _, ok := s.spikeSubs.Get(id); !ok {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown subscription: "+id))
	}
	return mcp.OK(req.ID, map[string]any{})
}

func (s *Server) handleResourcesUnsubscribe(req *mcp.Request) *mcp.Response {
	if errResp, ok := s.resourcesEnabled(req); !ok {
		return errResp
	}
	var p mcp.ResourceURIParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrInvalidParams, err.Error()))
	}
	if agentID, ok := dispatch.ParseAgentTelemetryResourceID(p.URI); ok {
		if s.sparseAgents == nil {
			return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown resource uri: "+p.URI))
		}
		if _, ok := s.sparseAgents.Get(agentID); !ok {
			return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown agent: "+agentID))
		}
		return mcp.OK(req.ID, map[string]any{})
	}
	id, ok := dispatch.ParseSpikeResourceID(p.URI)
	if !ok || s.spikeSubs == nil {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown resource uri: "+p.URI))
	}
	if !s.spikeSubs.Remove(id) {
		return mcp.ErrorResponse(req.ID, mcp.NewError(mcp.ErrResourceNotFound, "unknown subscription: "+id))
	}
	return mcp.OK(req.ID, map[string]any{})
}

// subscriptionResolver adapts spike subscriptions and sparse-agent telemetry filters to
// the transport's SubscriptionResolver contract for subscription-scoped SSE streams.
func (s *Server) subscriptionResolver() transport.SubscriptionResolver {
	if s.spikeSubs == nil && s.sparseAgents == nil {
		return nil
	}
	return func(id string) (transport.RecordFilter, bool) {
		if s.spikeSubs != nil {
			if sub, ok := s.spikeSubs.Get(id); ok {
				return sub, true
			}
		}
		if s.sparseAgents != nil {
			if _, ok := s.sparseAgents.Get(id); ok {
				return &dispatch.AgentTelemetryFilter{AgentID: id}, true
			}
		}
		return nil, false
	}
}

// Registry exposes the tool registry for tests.
func (s *Server) Registry() *tools.Registry { return s.registry }

// Jobs exposes the async jobs manager for internal integrations and tests.
func (s *Server) Jobs() *jobs.Manager { return s.jobs }

// Memory exposes the event-sourced broker for internal telemetry integrations and tests.
func (s *Server) Memory() *memorybroker.Broker { return s.memory }
