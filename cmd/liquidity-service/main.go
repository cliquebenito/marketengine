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

	"github.com/jackc/pgx/v5"

	"marketengine/internal/config"
	"marketengine/internal/domain"
	pgliquidity "marketengine/internal/infra/postgres/liquidity"
	pgoutbox "marketengine/internal/infra/postgres/outbox"
	"marketengine/internal/liquidity"
	pgcoingecko "marketengine/internal/repo/webapi/coingecko"
	pgcoinmetrics "marketengine/internal/repo/webapi/coinmetrics"
	pgdefillama "marketengine/internal/repo/webapi/defillama"
	"marketengine/internal/storage"
	"marketengine/pkg/clock"
)

var GitSHA = "dev"

func main() {
	var (
		configPath = flag.String("config", "configs/liquidity.yaml", "path to YAML config")
		daemon     = flag.Bool("daemon", false, "run as daemon (ticker); default is one-shot")
		valueDateS = flag.String("value-date", "", "override value_date (UTC, YYYY-MM-DD); default = today")
		bfFrom     = flag.String("from", "", "backfill start date (UTC, YYYY-MM-DD)")
		bfTo       = flag.String("to", "", "backfill end date (UTC, YYYY-MM-DD)")
	)
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(*configPath, *daemon, *valueDateS, *bfFrom, *bfTo); err != nil {
		slog.Error("exit", "err", err)
		os.Exit(1)
	}
}

func run(configPath string, daemon bool, valueDateOverride, bfFrom, bfTo string) error {
	loaded, err := config.Load[config.LiquidityConfig](configPath)
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

	featRepo := pgliquidity.NewFeatureRepo(pool)
	scoreRepo := pgliquidity.NewScoreRepo(pool)
	rawRepo := pgliquidity.NewRawRepo(pool)
	pub := pgoutbox.New(pool)

	dl := pgdefillama.NewStablecoinAdapter(
		pgdefillama.New(cfg.Providers.DefiLlama.BaseURL, time.Duration(cfg.Providers.DefiLlama.TimeoutSec)*time.Second))
	dlTVL := pgdefillama.NewChainTVLAdapter(
		pgdefillama.New(cfg.Providers.DefiLlamaTVL.BaseURL, time.Duration(cfg.Providers.DefiLlamaTVL.TimeoutSec)*time.Second))
	cm := pgcoinmetrics.NewAdapter(
		pgcoinmetrics.New(cfg.Providers.CoinMetrics.BaseURL, time.Duration(cfg.Providers.CoinMetrics.TimeoutSec)*time.Second))
	cg := pgcoingecko.NewAdapter(pgcoingecko.New(60 * time.Second))

	svcCfg := scoringConfigFromYAML(cfg, loaded.ConfigVersion, GitSHA)
	clk := clock.Real{}

	assets := domain.AssetsTradeable()
	if len(cfg.Assets) > 0 {
		assets = make([]domain.Asset, 0, len(cfg.Assets))
		for _, a := range cfg.Assets {
			assets = append(assets, domain.Asset(a))
		}
	}

	svc := liquidity.NewService(featRepo, scoreRepo, rawRepo, dl, dlTVL, cm, cg, pub, clk, svcCfg, assets)

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
	if !daemon {
		slog.Info("one-shot tick", "value_date", valueDate)
		return svc.RunOnce(ctx, valueDate)
	}

	tick, err := time.ParseDuration(cfg.Schedule.TickEvery)
	if err != nil {
		return fmt.Errorf("parse tick_every: %w", err)
	}
	slog.Info("daemon tick loop", "tick_every", tick)
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

func scoringConfigFromYAML(cfg config.LiquidityConfig, configVersion, codeSHA string) liquidity.Config {
	findVersion := func(name string) string {
		for _, f := range cfg.Features {
			if f.Name == name {
				return f.Version
			}
		}
		return ""
	}
	return liquidity.Config{
		ModelVersion:                cfg.ModelVersion,
		ConfigVersion:               configVersion,
		CodeSHA:                     codeSHA,
		SupplyFeatureVersion:        findVersion("stablecoin_supply_total"),
		GrowthFeatureVersion:        findVersion("stablecoin_growth_30d"),
		ZScoreFeatureVersion:        findVersion("stablecoin_growth_zscore_90d"),
		Netflow7dFeatureVersion:     findVersion("exchange_netflow_7d"),
		NetflowZScoreFeatureVersion: findVersion("exchange_netflow_zscore_180d"),
		TVLFeatureVersion:           findVersion("defi_tvl_usd"),
		TVLGrowthFeatureVersion:     findVersion("defi_tvl_growth_30d"),
		TVLZScoreFeatureVersion:     findVersion("defi_tvl_growth_zscore_180d"),
		SupplyMajorFeatureVersion:   findVersion("stablecoin_supply_usdt_usdc_dai"),
		SSRFeatureVersion:           findVersion("ssr"),
		SSRPercentileFeatureVersion: findVersion("ssr_percentile_rank_365d"),
	}
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

func registerConfig(ctx context.Context, pool *storage.Pool, loaded config.Loaded[config.LiquidityConfig], cfg config.LiquidityConfig) error {
	parsed := map[string]any{
		"domain":        cfg.Domain,
		"model_version": cfg.ModelVersion,
		"assets":        cfg.Assets,
	}
	return pool.InTx(ctx, func(tx pgx.Tx) error {
		return storage.UpsertModelConfig(ctx, tx, loaded.ConfigVersion,
			cfg.Domain, cfg.ModelVersion, loaded.Raw, parsed)
	})
}
