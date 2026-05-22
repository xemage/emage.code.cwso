package dispatch

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const defaultWasmScoringAdjustFunction = "adjust_score"

// ScoreAdjustmentInput is the policy candidate data exposed to scoring plugins.
type ScoreAdjustmentInput struct {
	ProviderID   string
	CurrentScore float64
	WorkloadTags []string
}

// ScoreAdjuster adjusts a candidate score for pluggable policy behavior.
type ScoreAdjuster interface {
	AdjustScore(ctx context.Context, input ScoreAdjustmentInput) (float64, error)
}

// WasmScoringConfig controls Wasm scoring plugin runtime behavior.
type WasmScoringConfig struct {
	Enabled          bool
	ModulePath       string
	AdjustFunction   string
	CallTimeout      time.Duration
	MemoryLimitPages uint32
	AllowedHostCalls []string
	AdjustmentMin    float64
	AdjustmentMax    float64
}

// DefaultWasmScoringConfig returns conservative defaults for safe plugin usage.
func DefaultWasmScoringConfig() WasmScoringConfig {
	return WasmScoringConfig{
		Enabled:          false,
		AdjustFunction:   defaultWasmScoringAdjustFunction,
		CallTimeout:      20 * time.Millisecond,
		MemoryLimitPages: 64,
		AllowedHostCalls: nil,
		AdjustmentMin:    0,
		AdjustmentMax:    1,
	}
}

type wasmScoreAdjuster struct {
	timeout       time.Duration
	adjustFn      api.Function
	adjustmentMin float64
	adjustmentMax float64
}

// NewWasmScoreAdjuster creates a Wasm-backed scorer. When disabled, this
// returns nil with no error to preserve baseline behavior.
func NewWasmScoreAdjuster(ctx context.Context, rawCfg WasmScoringConfig) (ScoreAdjuster, error) {
	cfg := normalizeWasmScoringConfig(rawCfg)
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.ModulePath) == "" {
		return nil, errors.New("wasm scoring module path is required when enabled")
	}

	runtimeConfig := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithMemoryLimitPages(cfg.MemoryLimitPages)
	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

	if err := instantiateHostAllowlist(ctx, runtime, cfg.AllowedHostCalls); err != nil {
		_ = runtime.Close(ctx)
		return nil, err
	}

	source, err := os.ReadFile(cfg.ModulePath)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("read wasm scoring module: %w", err)
	}

	compiled, err := runtime.CompileModule(ctx, source)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("compile wasm scoring module: %w", err)
	}

	moduleCfg := wazero.NewModuleConfig().WithStartFunctions()
	module, err := runtime.InstantiateModule(ctx, compiled, moduleCfg)
	if err != nil {
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("instantiate wasm scoring module: %w", err)
	}

	adjustFn := module.ExportedFunction(cfg.AdjustFunction)
	if adjustFn == nil {
		_ = module.Close(ctx)
		_ = runtime.Close(ctx)
		return nil, fmt.Errorf("wasm scoring function %q not exported", cfg.AdjustFunction)
	}

	return &wasmScoreAdjuster{
		timeout:       cfg.CallTimeout,
		adjustFn:      adjustFn,
		adjustmentMin: cfg.AdjustmentMin,
		adjustmentMax: cfg.AdjustmentMax,
	}, nil
}

func (a *wasmScoreAdjuster) AdjustScore(ctx context.Context, input ScoreAdjustmentInput) (float64, error) {
	if a == nil || a.adjustFn == nil {
		return 0, errors.New("wasm scorer is not initialized")
	}

	callCtx := ctx
	if a.timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, a.timeout)
		defer cancel()
	}

	scaled := int64(math.Round(input.CurrentScore * 1000))
	result, err := a.adjustFn.Call(callCtx, providerHash(input.ProviderID), uint64(scaled))
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, errors.New("wasm scorer returned no results")
	}

	adjusted := float64(int64(result[0])) / 1000
	return clampFloat64(adjusted, a.adjustmentMin, a.adjustmentMax), nil
}

func normalizeWasmScoringConfig(cfg WasmScoringConfig) WasmScoringConfig {
	def := DefaultWasmScoringConfig()
	if strings.TrimSpace(cfg.AdjustFunction) == "" {
		cfg.AdjustFunction = def.AdjustFunction
	}
	if cfg.CallTimeout <= 0 {
		cfg.CallTimeout = def.CallTimeout
	}
	if cfg.MemoryLimitPages == 0 {
		cfg.MemoryLimitPages = def.MemoryLimitPages
	}
	if cfg.AdjustmentMax <= cfg.AdjustmentMin {
		cfg.AdjustmentMin = def.AdjustmentMin
		cfg.AdjustmentMax = def.AdjustmentMax
	}
	cfg.AllowedHostCalls = dedupeAndSort(cfg.AllowedHostCalls)
	return cfg
}

func instantiateHostAllowlist(ctx context.Context, runtime wazero.Runtime, allowlist []string) error {
	if len(allowlist) == 0 {
		return nil
	}

	builder := runtime.NewHostModuleBuilder("cwso_host")
	hasHostFn := false
	for _, callName := range allowlist {
		switch callName {
		case "time.now_unix_ms":
			builder.NewFunctionBuilder().WithFunc(func() uint64 {
				return uint64(time.Now().UTC().UnixMilli())
			}).Export("time.now_unix_ms")
			hasHostFn = true
		default:
			return fmt.Errorf("host call %q is not allowlisted by runtime", callName)
		}
	}
	if !hasHostFn {
		return nil
	}
	_, err := builder.Instantiate(ctx)
	if err != nil {
		return fmt.Errorf("instantiate host allowlist module: %w", err)
	}
	return nil
}

func providerHash(providerID string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.TrimSpace(providerID)))
	return h.Sum64()
}

func clampFloat64(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
