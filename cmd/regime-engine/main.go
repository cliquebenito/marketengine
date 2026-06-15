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
	pgregime "marketengine/internal/infra/postgres/regime"
	"marketengine/internal/regime"
	"marketengine/internal/storage"
	"marketengine/pkg/clock"

	"github.com/jackc/pgx/v5"
)

var GitSHA = "dev"

func main() {
	var (
		configPath   = flag.String("config", "configs/regime-engine.yaml", "path to YAML config")
		daemon       = flag.Bool("daemon", false, "run as daemon (ticker); default is one-shot")
		valueDateS   = flag.String("value-date", "", "override value_date (UTC, YYYY-MM-DD)")
		backfillFrom = flag.String("from", "", "backfill start date (YYYY-MM-DD)")
		backfillTo   = flag.String("to", "", "backfill end date (YYYY-MM-DD)")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(*configPath, *daemon, *valueDateS, *backfillFrom, *backfillTo); err != nil {
		slog.Error("exit", "err", err)
		os.Exit(1)
	}
}

func run(configPath string, daemonMode bool, valueDateOverride, bfFrom, bfTo string) error {
	loaded, err := config.Load[config.RegimeEngineConfig](configPath)
	if err != nil {
		return err
	}
	cfg := loaded.Parsed
	slog.Info("config loaded", "path", configPath, "config_version", loaded.ConfigVersion,
		"model_version", cfg.ModelVersion)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := storage.Open(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	if err := registerConfig(ctx, pool, loaded, cfg); err != nil {
		return fmt.Errorf("register config: %w", err)
	}

	engineCfg := buildEngineConfig(cfg, loaded.ConfigVersion)
	clk := clock.Real{}

	repo := pgregime.NewRegimeRepo(pool)
	reader := pgregime.NewDomainScoreReader(pool)
	pub := pgoutbox.New(pool)

	assets := assetsFromConfig(cfg)
	svc := regime.NewService(repo, reader, pub, clk, engineCfg, assets)

	if bfFrom != "" {
		if bfTo == "" {
			return fmt.Errorf("-to required when -from is set")
		}
		from, err := time.Parse("2006-01-02", bfFrom)
		if err != nil {
			return fmt.Errorf("parse -from: %w", err)
		}
		to, err := time.Parse("2006-01-02", bfTo)
		if err != nil {
			return fmt.Errorf("parse -to: %w", err)
		}
		slog.Info("backfill", "from", from, "to", to)
		return svc.RunBackfill(ctx, from.UTC(), to.UTC())
	}

	valueDate, err := resolveValueDate(clk, valueDateOverride)
	if err != nil {
		return err
	}

	if !daemonMode {
		slog.Info("one-shot", "value_date", valueDate)
		return svc.RunOnce(ctx, valueDate)
	}

	tick, err := time.ParseDuration(cfg.Schedule.TickEvery)
	if err != nil {
		return fmt.Errorf("parse tick_every: %w", err)
	}
	slog.Info("daemon", "tick_every", tick)
	if err := svc.RunOnce(ctx, valueDate); err != nil {
		slog.Error("initial tick failed", "err", err)
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			vd, _ := resolveValueDate(clk, "")
			if err := svc.RunOnce(ctx, vd); err != nil {
				slog.Error("tick failed", "err", err)
			}
		}
	}
}

func assetsFromConfig(cfg config.RegimeEngineConfig) []domain.Asset {
	if len(cfg.Assets) == 0 {
		return nil
	}
	out := make([]domain.Asset, 0, len(cfg.Assets))
	for _, a := range cfg.Assets {
		out = append(out, domain.Asset(a))
	}
	return out
}

func buildEngineConfig(cfg config.RegimeEngineConfig, configVersion string) regime.Config {
	ec := regime.DefaultConfig()
	ec.ModelVersion = cfg.ModelVersion
	ec.ConfigVersion = configVersion
	ec.CodeSHA = GitSHA

	if len(cfg.Aggregation.Weights) > 0 {
		w := make(map[domain.DomainCode]float64, len(cfg.Aggregation.Weights))
		for k, v := range cfg.Aggregation.Weights {
			w[domain.DomainCode(k)] = v
		}
		ec.Weights = w
	}
	if cfg.Probability.SigmoidSlopeK > 0 {
		ec.SigmoidK = cfg.Probability.SigmoidSlopeK
	}
	if cfg.Transition.RocWindowDays > 0 {
		ec.RocWindowDays = cfg.Transition.RocWindowDays
	}
	if cfg.Transition.RocWeight > 0 {
		ec.RocWeight = cfg.Transition.RocWeight
	}
	if cfg.Transition.DivergenceWeight > 0 {
		ec.DivergenceWeight = cfg.Transition.DivergenceWeight
	}
	if cfg.Transition.Baseline > 0 {
		ec.TransitionBaseline = cfg.Transition.Baseline
	}
	if cfg.Coverage.MinCoverage > 0 {
		ec.MinCoverage = cfg.Coverage.MinCoverage
	}

	ec.SmoothingSpanDays = cfg.Smoothing.SpanDays
	return ec
}

func resolveValueDate(clk clock.Clock, override string) (time.Time, error) {
	if override != "" {
		t, err := time.Parse("2006-01-02", override)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse value-date: %w", err)
		}
		return t.UTC(), nil
	}
	return clk.Now().UTC().Truncate(24 * time.Hour), nil
}

func registerConfig(ctx context.Context, pool *storage.Pool,
	loaded config.Loaded[config.RegimeEngineConfig], cfg config.RegimeEngineConfig,
) error {
	parsed := map[string]any{
		"scope":         "engine",
		"model_version": cfg.ModelVersion,
		"assets":        cfg.Assets,
	}
	return pool.InTx(ctx, func(tx pgx.Tx) error {
		return storage.UpsertModelConfig(ctx, tx, loaded.ConfigVersion,
			"engine", cfg.ModelVersion, loaded.Raw, parsed)
	})
}
