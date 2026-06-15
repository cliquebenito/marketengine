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
	pgleverage "marketengine/internal/infra/postgres/leverage"
	pgoutbox "marketengine/internal/infra/postgres/outbox"
	pgbinance "marketengine/internal/infra/provider/binance"
	pgderibit "marketengine/internal/infra/provider/deribit"
	"marketengine/internal/leverage"
	pgbybit "marketengine/internal/repo/webapi/bybit"
	pgcoinglass "marketengine/internal/repo/webapi/coinglass"
	pgokx "marketengine/internal/repo/webapi/okx"
	"marketengine/internal/storage"
	"marketengine/pkg/clock"
)

var GitSHA = "dev"

func main() {
	var (
		configPath   = flag.String("config", "configs/leverage.yaml", "YAML config")
		backfillFrom = flag.String("from", "", "backfill start (YYYY-MM-DD)")
		backfillTo   = flag.String("to", "", "backfill end (YYYY-MM-DD)")
		valueDateS   = flag.String("value-date", "", "override value_date (YYYY-MM-DD)")
	)
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(*configPath, *backfillFrom, *backfillTo, *valueDateS); err != nil {
		slog.Error("exit", "err", err)
		os.Exit(1)
	}
}

type leverageConfig struct {
	Domain       string   `yaml:"domain"`
	ModelVersion string   `yaml:"model_version"`
	Assets       []string `yaml:"assets"`
	Providers    struct {
		Binance struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"binance"`
		Bybit struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"bybit"`
		OKX struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"okx"`
		Deribit struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"deribit"`
		CoinGlass struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
			APIKey     string `yaml:"api_key"`
		} `yaml:"coinglass"`
	} `yaml:"providers"`
	Features struct {
		IntermediateVersion string `yaml:"intermediate_version"`
		FinalVersion        string `yaml:"final_version"`
	} `yaml:"features"`
	Database config.DatabaseConfig `yaml:"database"`
	Schedule config.ScheduleConfig `yaml:"schedule"`
}

func run(configPath, backfillFrom, backfillTo, valueDateOverride string) error {
	loaded, err := config.Load[leverageConfig](configPath)
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

	p := cfg.Providers
	binClient := pgbinance.New(p.Binance.BaseURL, time.Duration(p.Binance.TimeoutSec)*time.Second)
	bbClient := pgbybit.New(p.Bybit.BaseURL, time.Duration(p.Bybit.TimeoutSec)*time.Second)
	oxClient := pgokx.New(p.OKX.BaseURL, time.Duration(p.OKX.TimeoutSec)*time.Second)
	drClient := pgderibit.New(p.Deribit.BaseURL, time.Duration(p.Deribit.TimeoutSec)*time.Second)

	cgTimeout := time.Duration(p.CoinGlass.TimeoutSec) * time.Second
	if cgTimeout == 0 {
		cgTimeout = 30 * time.Second
	}
	cgClient := pgcoinglass.New(p.CoinGlass.APIKey, cgTimeout)

	oiProviders := map[string]leverage.OIProvider{
		"binance": pgbinance.NewOIAdapter(binClient),
		"bybit":   pgbybit.NewOIAdapter(bbClient),
		"okx":     pgokx.NewOIAdapter(oxClient),
	}
	fundingProviders := map[string]leverage.FundingProvider{
		"binance": pgbinance.NewFundingAdapter(binClient),
		"bybit":   pgbybit.NewFundingAdapter(bbClient),
		"okx":     pgokx.NewFundingAdapter(oxClient),
	}
	basisProv := pgderibit.NewBasisAdapter(drClient)
	cgOI := pgcoinglass.NewLeverageOIAdapter(cgClient)
	cgBasis := pgcoinglass.NewLeverageBasisAdapter(cgClient)
	cgLiq := pgcoinglass.NewLeverageLiqAdapter(cgClient)
	cgCrowd := pgcoinglass.NewLeverageCrowdAdapter(cgClient)

	featRepo := pgleverage.NewFeatureRepo(pool)
	scoreRepo := pgleverage.NewScoreRepo(pool)
	rawRepo := pgleverage.NewRawRepo(pool)
	pub := pgoutbox.New(pool)

	assets := make([]domain.Asset, 0, len(cfg.Assets))
	for _, a := range cfg.Assets {
		assets = append(assets, domain.Asset(a))
	}
	if len(assets) == 0 {
		assets = domain.AssetsTradeable()
	}

	svcCfg := leverage.Config{
		ModelVersion:        cfg.ModelVersion,
		ConfigVersion:       loaded.ConfigVersion,
		CodeSHA:             GitSHA,
		IntermediateVersion: cfg.Features.IntermediateVersion,
		FinalVersion:        cfg.Features.FinalVersion,
	}
	svc := leverage.NewService(featRepo, scoreRepo, rawRepo,
		oiProviders, fundingProviders, basisProv,
		cgOI, cgBasis, cgLiq, cgCrowd,
		pub, clock.Real{}, svcCfg, assets)

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
