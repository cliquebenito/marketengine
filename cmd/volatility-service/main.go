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
	pgoutbox "marketengine/internal/infra/postgres/outbox"
	pgvolatility "marketengine/internal/infra/postgres/volatility"
	pgderibitvol "marketengine/internal/infra/provider/deribit_volatility"
	"marketengine/internal/repo/webapi/coinglass"
	"marketengine/internal/storage"
	"marketengine/internal/volatility"
	"marketengine/pkg/clock"
)

var GitSHA = "dev"

func main() {
	var (
		configPath   = flag.String("config", "configs/volatility.yaml", "YAML config")
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

type volatilityConfig struct {
	Domain       string   `yaml:"domain"`
	ModelVersion string   `yaml:"model_version"`
	Assets       []string `yaml:"assets"`
	Providers    struct {
		Deribit struct {
			BaseURL    string `yaml:"base_url"`
			TimeoutSec int    `yaml:"timeout_sec"`
		} `yaml:"deribit"`
		CoinGlass struct {
			APIKey     string `yaml:"api_key"`
			TimeoutSec int    `yaml:"timeout_sec"`
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
	loaded, err := config.Load[volatilityConfig](configPath)
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

	dr := pgderibitvol.New(cfg.Providers.Deribit.BaseURL, time.Duration(cfg.Providers.Deribit.TimeoutSec)*time.Second)
	dvolProv := pgderibitvol.NewDvolAdapter(dr)
	optProv := pgderibitvol.NewOptionsAdapter(dr)
	chainProv := pgderibitvol.NewChainAdapter(dr)

	var cgOptions volatility.CoinglassOptionsProvider
	if k := cfg.Providers.CoinGlass.APIKey; k != "" {
		t := time.Duration(cfg.Providers.CoinGlass.TimeoutSec) * time.Second
		if t == 0 {
			t = 30 * time.Second
		}
		cgOptions = coinglass.NewVolatilityOptionsAdapter(coinglass.New(k, t))
	}

	featRepo := pgvolatility.NewFeatureRepo(pool)
	scoreRepo := pgvolatility.NewScoreRepo(pool)
	rawRepo := pgvolatility.NewRawRepo(pool)
	pub := pgoutbox.New(pool)

	svcCfg := volatility.Config{
		ModelVersion:        cfg.ModelVersion,
		ConfigVersion:       loaded.ConfigVersion,
		CodeSHA:             GitSHA,
		IntermediateVersion: cfg.Features.IntermediateVersion,
		FinalVersion:        cfg.Features.FinalVersion,
	}

	assets := make([]domain.Asset, 0, len(cfg.Assets))
	for _, a := range cfg.Assets {
		assets = append(assets, domain.Asset(a))
	}
	svc := volatility.NewService(featRepo, scoreRepo, rawRepo, dvolProv, optProv, cgOptions, chainProv, pub, clock.Real{}, svcCfg, assets)

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
