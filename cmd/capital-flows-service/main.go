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

	"marketengine/internal/capitalflows"
	"marketengine/internal/config"
	"marketengine/internal/domain"
	pgcapitalflows "marketengine/internal/infra/postgres/capitalflows"
	pgoutbox "marketengine/internal/infra/postgres/outbox"
	"marketengine/internal/repo/webapi/coinglass"
	"marketengine/internal/storage"
	"marketengine/pkg/clock"
)

var GitSHA = "dev"

func main() {
	var (
		configPath   = flag.String("config", "configs/capital-flows.yaml", "YAML config")
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

type capitalFlowsConfig struct {
	Domain       string `yaml:"domain"`
	ModelVersion string `yaml:"model_version"`
	Providers    struct {
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
	loaded, err := config.Load[capitalFlowsConfig](configPath)
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

	cgTimeout := time.Duration(cfg.Providers.CoinGlass.TimeoutSec) * time.Second
	if cgTimeout == 0 {
		cgTimeout = 30 * time.Second
	}
	cgClient := coinglass.New(cfg.Providers.CoinGlass.APIKey, cgTimeout)
	cgAdapter := coinglass.NewCapitalFlowsAdapter(cgClient)

	featRepo := pgcapitalflows.NewFeatureRepo(pool)
	scoreRepo := pgcapitalflows.NewScoreRepo(pool)
	rawRepo := pgcapitalflows.NewRawRepo(pool)
	pub := pgoutbox.New(pool)

	svcCfg := capitalflows.Config{
		ModelVersion:        cfg.ModelVersion,
		ConfigVersion:       loaded.ConfigVersion,
		CodeSHA:             GitSHA,
		IntermediateVersion: cfg.Features.IntermediateVersion,
		FinalVersion:        cfg.Features.FinalVersion,
	}
	svc := capitalflows.NewService(featRepo, scoreRepo, rawRepo,
		cgAdapter, cgAdapter, cgAdapter, cgAdapter, cgAdapter, pub, clock.Real{}, svcCfg,
		[]domain.Asset{domain.AssetGlobal})

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
