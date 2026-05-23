package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
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
	ExpectedSHA256   string
	TrustedModuleDir string
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
	if err := ensureTrustedModulePath(cfg.ModulePath, cfg.TrustedModuleDir); err != nil {
		return nil, err
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
	if err := verifyModuleSHA256(source, cfg.ExpectedSHA256); err != nil {
		_ = runtime.Close(ctx)
		return nil, err
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
	cfg.ModulePath = strings.TrimSpace(cfg.ModulePath)
	cfg.ExpectedSHA256 = normalizeSHA256Hex(cfg.ExpectedSHA256)
	cfg.TrustedModuleDir = strings.TrimSpace(cfg.TrustedModuleDir)
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

func verifyModuleSHA256(source []byte, expectedHex string) error {
	if expectedHex == "" {
		return nil
	}
	actual := sha256.Sum256(source)
	actualHex := hex.EncodeToString(actual[:])
	if actualHex != expectedHex {
		return fmt.Errorf("wasm scoring module integrity check failed: expected sha256 %s, got %s", expectedHex, actualHex)
	}
	return nil
}

func normalizeSHA256Hex(v string) string {
	trimmed := strings.TrimSpace(v)
	trimmed = strings.TrimPrefix(strings.ToLower(trimmed), "sha256:")
	return trimmed
}

func ensureTrustedModulePath(modulePath, trustedModuleDir string) error {
	if trustedModuleDir == "" {
		return nil
	}
	trustedAbs, err := filepath.Abs(trustedModuleDir)
	if err != nil {
		return fmt.Errorf("resolve trusted wasm module directory: %w", err)
	}
	trustedResolved, err := filepath.EvalSymlinks(trustedAbs)
	if err != nil {
		return fmt.Errorf("resolve trusted wasm module directory symlink path: %w", err)
	}
	moduleAbs, err := filepath.Abs(modulePath)
	if err != nil {
		return fmt.Errorf("resolve wasm scoring module path: %w", err)
	}
	moduleResolved, err := filepath.EvalSymlinks(moduleAbs)
	if err != nil {
		return fmt.Errorf("resolve wasm scoring module symlink path: %w", err)
	}
	rel, err := filepath.Rel(trustedResolved, moduleResolved)
	if err != nil {
		return fmt.Errorf("compare wasm module path against trusted directory: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("wasm scoring module path %q is outside trusted directory %q", modulePath, trustedModuleDir)
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
