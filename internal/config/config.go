package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Loaded[T any] struct {
	Raw           []byte
	ConfigVersion string
	Parsed        T
}

func Load[T any](path string) (Loaded[T], error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Loaded[T]{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var parsed T
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return Loaded[T]{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return Loaded[T]{
		Raw:           raw,
		ConfigVersion: "sha256:" + hex.EncodeToString(sum[:]),
		Parsed:        parsed,
	}, nil
}

type LiquidityConfig struct {
	Domain       string          `yaml:"domain"`
	ModelVersion string          `yaml:"model_version"`
	Assets       []string        `yaml:"assets"`
	Providers    ProvidersConfig `yaml:"providers"`
	Features     []FeatureConfig `yaml:"features"`
	Scoring      ScoringConfig   `yaml:"scoring"`
	Database     DatabaseConfig  `yaml:"database"`
	Schedule     ScheduleConfig  `yaml:"schedule"`
}

type ProvidersConfig struct {
	DefiLlama    DefiLlamaConfig   `yaml:"defillama"`
	DefiLlamaTVL DefiLlamaConfig   `yaml:"defillama_tvl"`
	CoinMetrics  CoinMetricsConfig `yaml:"coinmetrics"`
}

type DefiLlamaConfig struct {
	BaseURL    string `yaml:"base_url"`
	TimeoutSec int    `yaml:"timeout_sec"`
}

type CoinMetricsConfig struct {
	BaseURL    string `yaml:"base_url"`
	TimeoutSec int    `yaml:"timeout_sec"`
}

type FeatureConfig struct {
	Name    string         `yaml:"name"`
	Version string         `yaml:"version"`
	Params  map[string]any `yaml:"params"`
}

type ScoringConfig struct {
	ScoreVersion string            `yaml:"score_version"`
	Components   []ComponentConfig `yaml:"components"`
}

type ComponentConfig struct {
	Name     string   `yaml:"name"`
	Features []string `yaml:"features"`
	Weight   float64  `yaml:"weight"`
	Invert   bool     `yaml:"invert"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

type ScheduleConfig struct {
	TickEvery string `yaml:"tick_every"`
}

type RegimeEngineConfig struct {
	ModelVersion string                  `yaml:"model_version"`
	Assets       []string                `yaml:"assets"`
	Database     DatabaseConfig          `yaml:"database"`
	Schedule     ScheduleConfig          `yaml:"schedule"`
	Aggregation  RegimeAggregationConfig `yaml:"aggregation"`
	Probability  RegimeProbabilityConfig `yaml:"probability_mapping"`
	Transition   RegimeTransitionConfig  `yaml:"transition_risk"`
	Coverage     RegimeCoverageConfig    `yaml:"feature_coverage_policy"`
	Smoothing    RegimeSmoothingConfig   `yaml:"smoothing"`
}

type RegimeSmoothingConfig struct {
	SpanDays int `yaml:"span_days"`
}

type RegimeAggregationConfig struct {
	Weights map[string]float64 `yaml:"weights"`
}

type RegimeProbabilityConfig struct {
	SigmoidSlopeK float64 `yaml:"sigmoid_slope_k"`
}

type RegimeTransitionConfig struct {
	RocWindowDays    int     `yaml:"roc_window_days"`
	RocWeight        float64 `yaml:"roc_weight"`
	DivergenceWeight float64 `yaml:"divergence_weight"`

	Baseline float64 `yaml:"baseline"`
}

type RegimeCoverageConfig struct {
	MinCoverage float64 `yaml:"min_coverage"`
}

type OrchestratorConfig struct {
	Database  DatabaseConfig        `yaml:"database"`
	Providers OrchestratorProviders `yaml:"providers"`
	Domains   OrchestratorDomains   `yaml:"domains"`
	Paths     OrchestratorPaths     `yaml:"paths"`
	Schedule  ScheduleConfig        `yaml:"schedule"`
}

type OrchestratorPaths struct {
	LiquidityConfig string `yaml:"liquidity_config"`
}

type OrchestratorProviders struct {
	DefiLlama    OrchProvider `yaml:"defillama"`
	DefiLlamaTVL OrchProvider `yaml:"defillama_tvl"`
	CoinMetrics  OrchProvider `yaml:"coinmetrics"`
	Binance      OrchProvider `yaml:"binance"`
	BinanceSpot  OrchProvider `yaml:"binance_spot"`
	Bybit        OrchProvider `yaml:"bybit"`
	OKX          OrchProvider `yaml:"okx"`
	Deribit      OrchProvider `yaml:"deribit"`
	CoinGlass    OrchProvider `yaml:"coinglass"`
	Kraken       OrchProvider `yaml:"kraken"`
	Coinbase     OrchProvider `yaml:"coinbase"`
}

type OrchProvider struct {
	BaseURL    string `yaml:"base_url"`
	TimeoutSec int    `yaml:"timeout_sec"`
	APIKey     string `yaml:"api_key"`
}

type OrchestratorDomains struct {
	Liquidity    OrchDomainBasic  `yaml:"liquidity"`
	Leverage     OrchDomainFeats  `yaml:"leverage"`
	MarketStress OrchDomainStress `yaml:"market_stress"`
	CapitalFlows OrchDomainFeats  `yaml:"capital_flows"`
	Volatility   OrchDomainFeats  `yaml:"volatility"`
	RegimeEngine OrchDomainEngine `yaml:"regime_engine"`
}

type OrchDomainBasic struct {
	ModelVersion string `yaml:"model_version"`
}

type OrchDomainFeats struct {
	ModelVersion        string `yaml:"model_version"`
	IntermediateVersion string `yaml:"intermediate_version"`
	FinalVersion        string `yaml:"final_version"`
}

type OrchDomainStress struct {
	ModelVersion         string `yaml:"model_version"`
	IntermediateVersion  string `yaml:"intermediate_version"`
	FinalVersion         string `yaml:"final_version"`
	LeverageBasisVersion string `yaml:"leverage_basis_version"`
}

type OrchDomainEngine struct {
	ModelVersion  string                 `yaml:"model_version"`
	Weights       map[string]float64     `yaml:"weights"`
	SigmoidSlopeK float64                `yaml:"sigmoid_slope_k"`
	Transition    RegimeTransitionConfig `yaml:"transition"`
	Normalization OrchNormalization      `yaml:"normalization"`
	Smoothing     RegimeSmoothingConfig  `yaml:"smoothing"`
}

type OrchNormalization struct {
	LookbackDays int `yaml:"lookback_days"`
	MinSamples   int `yaml:"min_samples"`
}

type GatewayConfig struct {
	Listen   string         `yaml:"listen"`
	Database DatabaseConfig `yaml:"database"`
}
