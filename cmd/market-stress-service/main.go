package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"marketengine/internal/config"
	"marketengine/internal/domain"
	pgmarketstress "marketengine/internal/infra/postgres/marketstress"
	pgoutbox "marketengine/internal/infra/postgres/outbox"
	provbinance "marketengine/internal/infra/provider/binance_spot"
	"marketengine/internal/marketstress"
	"marketengine/internal/providers/binance"
	"marketengine/internal/repo/webapi/coinbase"
	provcoinbase "marketengine/internal/repo/webapi/coinbase"
	"marketengine/internal/repo/webapi/coinglass"
	"marketengine/internal/repo/webapi/kraken"
	provkraken "marketengine/internal/repo/webapi/kraken"
	"marketengine/internal/storage"
	"marketengine/pkg/clock"
)

var GitSHA = "dev"

func main() {
	var (
		configPath   = flag.String("config", "configs/market-stress.yaml", "YAML config")
		backfillFrom = flag.String("from", "", "backfill start (YYYY-MM-DD)")
		backfillTo   = flag.String("to", "", "backfill end (YYYY-MM-DD)")
		valueDateS   = flag.String("value-date", "", "override value_date (YYYY-MM-DD)")
		klinesOnly   = flag.Bool("klines-only", false, "backfill только Binance klines, без прочих провайдеров и без feature compute")
	)
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(*configPath, *backfillFrom, *backfillTo, *valueDateS, *klinesOnly); err != nil {
		slog.Error("exit", "err", err)
		os.Exit(1)
	}
}

type marketStressConfig struct {
	Domain       string   `yaml:"domain"`
	ModelVersion string   `yaml:"model_version"`
	Assets       []string `yaml:"assets"`
	Providers    struct {
		Binance struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"binance"`
		Kraken struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"kraken"`
		Coinbase struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"coinbase"`
		CoinGlass struct {
			APIKey     string `yaml:"api_key"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"coinglass"`
	} `yaml:"providers"`
	Features struct {
		IntermediateVersion  string `yaml:"intermediate_version"`
		FinalVersion         string `yaml:"final_version"`
		LeverageBasisVersion string `yaml:"leverage_basis_version"`
	} `yaml:"features"`
	Scoring struct {
		TanhScale float64            `yaml:"tanh_scale"`
		Weights   map[string]float64 `yaml:"weights"`
	} `yaml:"scoring"`
	Database config.DatabaseConfig `yaml:"database"`
	Schedule config.ScheduleConfig `yaml:"schedule"`
}

func run(configPath, backfillFrom, backfillTo, valueDateOverride string, klinesOnly bool) error {
	loaded, err := config.Load[marketStressConfig](configPath)
	if err != nil {
		return err
	}
	cfg := loaded.Parsed
	slog.Info("config loaded", "model_version", cfg.ModelVersion, "config_version", loaded.ConfigVersion)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := storage.Open(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	_, _ = pool.Exec(ctx, `
INSERT INTO model_configs (config_version, scope, model_version, yaml_body, yaml_parsed)
VALUES ($1, $2, $3, $4, $5::jsonb) ON CONFLICT (config_version) DO NOTHING`,
		loaded.ConfigVersion, cfg.Domain, cfg.ModelVersion, string(loaded.Raw),
		fmt.Sprintf(`{"domain":"%s","model_version":"%s"}`, cfg.Domain, cfg.ModelVersion))

	featRepo := pgmarketstress.NewFeatureRepo(pool)
	scoreRepo := pgmarketstress.NewScoreRepo(pool)
	rawRepo := pgmarketstress.NewRawRepo(pool)
	leverageReader := pgmarketstress.NewLeverageFeatureReader(pool)
	pub := pgoutbox.New(pool)

	p := cfg.Providers
	bin := provbinance.NewAdapter(binance.New(p.Binance.BaseURL, time.Duration(p.Binance.TimeoutSec)*time.Second))
	kr := provkraken.NewAdapter(kraken.New(p.Kraken.BaseURL, time.Duration(p.Kraken.TimeoutSec)*time.Second))
	cb := provcoinbase.NewAdapter(coinbase.New(p.Coinbase.BaseURL, time.Duration(p.Coinbase.TimeoutSec)*time.Second))

	var cg marketstress.CoinglassProvider
	var cgMicro marketstress.CoinglassMicroProvider
	if p.CoinGlass.APIKey != "" {
		cgTimeout := time.Duration(p.CoinGlass.TimeoutSec) * time.Second
		if cgTimeout == 0 {
			cgTimeout = 30 * time.Second
		}
		cgClient := coinglass.New(p.CoinGlass.APIKey, cgTimeout)
		cg = coinglass.NewMarketStressPremiumAdapter(cgClient)
		cgMicro = coinglass.NewMarketStressMicroAdapter(cgClient)
	}

	svcCfg := marketstress.Config{
		ModelVersion:         cfg.ModelVersion,
		ConfigVersion:        loaded.ConfigVersion,
		CodeSHA:              GitSHA,
		IntermediateVersion:  cfg.Features.IntermediateVersion,
		FinalVersion:         cfg.Features.FinalVersion,
		LeverageBasisVersion: cfg.Features.LeverageBasisVersion,
		TanhScale:            cfg.Scoring.TanhScale,
		WeightCorrelation:    cfg.Scoring.Weights["correlation"],
		WeightPeg:            cfg.Scoring.Weights["peg"],
		WeightCoinbase:       cfg.Scoring.Weights["coinbase"],
		WeightBasisInversion: cfg.Scoring.Weights["basis_inversion"],
		WeightMicrostructure: cfg.Scoring.Weights["microstructure"],
	}

	assets := make([]domain.Asset, 0, len(cfg.Assets))
	for _, a := range cfg.Assets {
		assets = append(assets, domain.Asset(a))
	}

	svc := marketstress.NewService(featRepo, scoreRepo, rawRepo,
		bin, kr, cb, cg, cgMicro, leverageReader, pub, clock.Real{}, svcCfg, assets)

	if backfillFrom != "" {
		if backfillTo == "" {
			return fmt.Errorf("-to required with -from")
		}
		from, err := time.Parse("2006-01-02", backfillFrom)
		if err != nil {
			return fmt.Errorf("parse -from: %w", err)
		}
		to, err := time.Parse("2006-01-02", backfillTo)
		if err != nil {
			return fmt.Errorf("parse -to: %w", err)
		}
		if klinesOnly {
			slog.Info("backfill klines-only", "from", from, "to", to)
			return svc.BackfillKlines(ctx, from.UTC(), to.UTC())
		}
		slog.Info("backfill", "from", from, "to", to)
		return svc.RunBackfill(ctx, from.UTC(), to.UTC())
	}

	clk := clock.Real{}
	vd := clk.Now().Truncate(24 * time.Hour)
	if valueDateOverride != "" {
		vd, _ = time.Parse("2006-01-02", valueDateOverride)
		vd = vd.UTC()
	}
	slog.Info("one-shot", "value_date", vd)
	return svc.RunOnce(ctx, vd)
}
