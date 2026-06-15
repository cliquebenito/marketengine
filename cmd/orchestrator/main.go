package main

import (
	"context"
	"encoding/json"
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
	pgleverage "marketengine/internal/infra/postgres/leverage"
	pgliquidity "marketengine/internal/infra/postgres/liquidity"
	pgmarketstress "marketengine/internal/infra/postgres/marketstress"
	pgoutbox "marketengine/internal/infra/postgres/outbox"
	pgregime "marketengine/internal/infra/postgres/regime"
	pgvolatility "marketengine/internal/infra/postgres/volatility"
	provbinance_lev "marketengine/internal/infra/provider/binance"
	provbinancespot "marketengine/internal/infra/provider/binance_spot"
	provderibit_lev "marketengine/internal/infra/provider/deribit"
	pgderibitvol "marketengine/internal/infra/provider/deribit_volatility"
	"marketengine/internal/leverage"
	"marketengine/internal/liquidity"
	"marketengine/internal/marketstress"
	"marketengine/internal/providers/binance"
	"marketengine/internal/providers/deribit"
	"marketengine/internal/regime"
	"marketengine/internal/repo/webapi/bybit"
	provbybit_lev "marketengine/internal/repo/webapi/bybit"
	"marketengine/internal/repo/webapi/coinbase"
	provcoinbase_ms "marketengine/internal/repo/webapi/coinbase"
	pgcoingecko "marketengine/internal/repo/webapi/coingecko"
	"marketengine/internal/repo/webapi/coinglass"
	pgcoinmetrics "marketengine/internal/repo/webapi/coinmetrics"
	pgdefillama "marketengine/internal/repo/webapi/defillama"
	"marketengine/internal/repo/webapi/kraken"
	provkraken_ms "marketengine/internal/repo/webapi/kraken"
	"marketengine/internal/repo/webapi/okx"
	provokx_lev "marketengine/internal/repo/webapi/okx"
	"marketengine/internal/storage"
	"marketengine/internal/volatility"
	"marketengine/pkg/clock"
)

var GitSHA = "dev"

var defaultAssets = []string{"BTC", "ETH"}

func loadLiquidityFeatureVersions(path string) (map[string]string, error) {
	if path == "" {
		path = "configs/liquidity.yaml"
	}
	l, err := config.Load[config.LiquidityConfig](path)
	if err != nil {
		return nil, fmt.Errorf("load liquidity config %s: %w", path, err)
	}
	m := make(map[string]string, len(l.Parsed.Features))
	for _, f := range l.Parsed.Features {
		m[f.Name] = f.Version
	}
	return m, nil
}

func coinglassAPIKey(cfg config.OrchestratorConfig) string {
	if v := os.Getenv("COINGLASS_API_KEY"); v != "" {
		return v
	}
	return cfg.Providers.CoinGlass.APIKey
}

func main() {
	var (
		configPath   = flag.String("config", "configs/orchestrator.yaml", "YAML config")
		valueDateS   = flag.String("value-date", "", "override value_date (UTC, YYYY-MM-DD)")
		backfillFrom = flag.String("from", "", "backfill start (YYYY-MM-DD)")
		backfillTo   = flag.String("to", "", "backfill end (YYYY-MM-DD)")
		daemonMode   = flag.Bool("daemon", false, "run continuously, ticking every schedule.tick_every")
	)
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(*configPath, *valueDateS, *backfillFrom, *backfillTo, *daemonMode); err != nil {
		slog.Error("orchestrator exit", "err", err)
		os.Exit(1)
	}
}

func run(configPath, valueDateOverride, bfFrom, bfTo string, daemonMode bool) error {
	loaded, err := config.Load[config.OrchestratorConfig](configPath)
	if err != nil {
		return err
	}
	cfg := loaded.Parsed
	slog.Info("orchestrator config loaded",
		"path", configPath, "config_version", loaded.ConfigVersion)

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

	steps := []struct {
		name string
		fn   func(context.Context, *storage.Pool, config.OrchestratorConfig, string, time.Time, time.Time) error
	}{
		{"liquidity", runLiquidity},
		{"leverage", runLeverage},
		{"capital_flows", runCapitalFlows},
		{"market_stress", runMarketStress},
		{"volatility", runVolatility},
		{"regime_engine", runRegimeEngine},
	}

	runOnce := func(mode string, from, to time.Time) error {
		slog.Info("orchestrator run", "mode", mode,
			"from", from.Format("2006-01-02"), "to", to.Format("2006-01-02"))
		var failed []string
		for _, s := range steps {
			started := time.Now()
			err := s.fn(ctx, pool, cfg, loaded.ConfigVersion, from, to)
			dur := time.Since(started)
			rec := map[string]any{
				"step":        s.name,
				"mode":        mode,
				"started_at":  started.UTC().Format(time.RFC3339),
				"duration_ms": dur.Milliseconds(),
			}
			if err != nil {
				rec["err"] = err.Error()
				b, _ := json.Marshal(rec)
				slog.Error("step failed", "record", string(b))
				failed = append(failed, s.name)
				continue
			}
			b, _ := json.Marshal(rec)
			slog.Info("step ok", "record", string(b))
		}
		if len(failed) > 0 {
			return fmt.Errorf("steps failed: %v", failed)
		}
		return nil
	}

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
		return runOnce("backfill", from, to)
	}

	if daemonMode {
		tick, err := time.ParseDuration(cfg.Schedule.TickEvery)
		if err != nil || tick <= 0 {
			tick = 4 * time.Hour
		}
		slog.Info("orchestrator daemon", "tick_every", tick)

		vd, _ := resolveValueDate(valueDateOverride)
		if err := runOnce("daemon", vd, vd); err != nil {
			slog.Error("initial tick failed (continuing)", "err", err)
		}
		t := time.NewTicker(tick)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("orchestrator shutdown")
				return nil
			case <-t.C:
				vd, _ := resolveValueDate("")
				if err := runOnce("daemon", vd, vd); err != nil {
					slog.Error("tick failed (continuing)", "err", err)
				}
			}
		}
	}

	vd, err := resolveValueDate(valueDateOverride)
	if err != nil {
		return err
	}
	return runOnce("one-shot", vd, vd)
}

func runLiquidity(ctx context.Context, pool *storage.Pool,
	cfg config.OrchestratorConfig, configVersion string, from, to time.Time,
) error {
	fv, err := loadLiquidityFeatureVersions(cfg.Paths.LiquidityConfig)
	if err != nil {
		return err
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

	svcCfg := liquidity.Config{
		ModelVersion:                cfg.Domains.Liquidity.ModelVersion,
		ConfigVersion:               configVersion,
		CodeSHA:                     GitSHA,
		SupplyFeatureVersion:        fv["stablecoin_supply_total"],
		GrowthFeatureVersion:        fv["stablecoin_growth_30d"],
		ZScoreFeatureVersion:        fv["stablecoin_growth_zscore_90d"],
		Netflow7dFeatureVersion:     fv["exchange_netflow_7d"],
		NetflowZScoreFeatureVersion: fv["exchange_netflow_zscore_180d"],
		TVLFeatureVersion:           fv["defi_tvl_usd"],
		TVLGrowthFeatureVersion:     fv["defi_tvl_growth_30d"],
		TVLZScoreFeatureVersion:     fv["defi_tvl_growth_zscore_180d"],
		SupplyMajorFeatureVersion:   fv["stablecoin_supply_usdt_usdc_dai"],
		SSRFeatureVersion:           fv["ssr"],
		SSRPercentileFeatureVersion: fv["ssr_percentile_rank_365d"],
	}
	assets := make([]domain.Asset, 0, len(defaultAssets))
	for _, a := range defaultAssets {
		assets = append(assets, domain.Asset(a))
	}
	svc := liquidity.NewService(featRepo, scoreRepo, rawRepo, dl, dlTVL, cm, cg, pub, clock.Real{}, svcCfg, assets)
	if from.Equal(to) {
		return svc.RunOnce(ctx, from.UTC())
	}
	return svc.RunBackfill(ctx, from.UTC(), to.UTC())
}

func runLeverage(ctx context.Context, pool *storage.Pool,
	cfg config.OrchestratorConfig, configVersion string, from, to time.Time,
) error {
	p := cfg.Providers
	binClient := binance.New(p.Binance.BaseURL, time.Duration(p.Binance.TimeoutSec)*time.Second)
	bbClient := bybit.New(p.Bybit.BaseURL, time.Duration(p.Bybit.TimeoutSec)*time.Second)
	oxClient := okx.New(p.OKX.BaseURL, time.Duration(p.OKX.TimeoutSec)*time.Second)
	drClient := deribit.New(p.Deribit.BaseURL, time.Duration(p.Deribit.TimeoutSec)*time.Second)
	cgTimeout := time.Duration(p.CoinGlass.TimeoutSec) * time.Second
	if cgTimeout == 0 {
		cgTimeout = 30 * time.Second
	}
	cgClient := coinglass.New(coinglassAPIKey(cfg), cgTimeout)

	oiProviders := map[string]leverage.OIProvider{
		"binance": provbinance_lev.NewOIAdapter(binClient),
		"bybit":   provbybit_lev.NewOIAdapter(bbClient),
		"okx":     provokx_lev.NewOIAdapter(oxClient),
	}
	fundingProviders := map[string]leverage.FundingProvider{
		"binance": provbinance_lev.NewFundingAdapter(binClient),
		"bybit":   provbybit_lev.NewFundingAdapter(bbClient),
		"okx":     provokx_lev.NewFundingAdapter(oxClient),
	}
	basisProv := provderibit_lev.NewBasisAdapter(drClient)
	cgOI := coinglass.NewLeverageOIAdapter(cgClient)
	cgBasis := coinglass.NewLeverageBasisAdapter(cgClient)
	cgLiq := coinglass.NewLeverageLiqAdapter(cgClient)
	cgCrowd := coinglass.NewLeverageCrowdAdapter(cgClient)

	featRepo := pgleverage.NewFeatureRepo(pool)
	scoreRepo := pgleverage.NewScoreRepo(pool)
	rawRepo := pgleverage.NewRawRepo(pool)
	pub := pgoutbox.New(pool)

	assets := make([]domain.Asset, 0, len(defaultAssets))
	for _, a := range defaultAssets {
		assets = append(assets, domain.Asset(a))
	}

	svcCfg := leverage.Config{
		ModelVersion:        cfg.Domains.Leverage.ModelVersion,
		ConfigVersion:       configVersion,
		CodeSHA:             GitSHA,
		IntermediateVersion: cfg.Domains.Leverage.IntermediateVersion,
		FinalVersion:        cfg.Domains.Leverage.FinalVersion,
	}
	svc := leverage.NewService(featRepo, scoreRepo, rawRepo,
		oiProviders, fundingProviders, basisProv,
		cgOI, cgBasis, cgLiq, cgCrowd,
		pub, clock.Real{}, svcCfg, assets)

	if from.Equal(to) {
		return svc.RunOnce(ctx, from.UTC())
	}
	return svc.RunBackfill(ctx, from.UTC(), to.UTC())
}

func runCapitalFlows(ctx context.Context, pool *storage.Pool,
	cfg config.OrchestratorConfig, configVersion string, from, to time.Time,
) error {
	cgTimeout := time.Duration(cfg.Providers.CoinGlass.TimeoutSec) * time.Second
	if cgTimeout == 0 {
		cgTimeout = 30 * time.Second
	}
	cgClient := coinglass.New(coinglassAPIKey(cfg), cgTimeout)
	cgAdapter := coinglass.NewCapitalFlowsAdapter(cgClient)

	featRepo := pgcapitalflows.NewFeatureRepo(pool)
	scoreRepo := pgcapitalflows.NewScoreRepo(pool)
	rawRepo := pgcapitalflows.NewRawRepo(pool)
	pub := pgoutbox.New(pool)

	svcCfg := capitalflows.Config{
		ModelVersion:        cfg.Domains.CapitalFlows.ModelVersion,
		ConfigVersion:       configVersion,
		CodeSHA:             GitSHA,
		IntermediateVersion: cfg.Domains.CapitalFlows.IntermediateVersion,
		FinalVersion:        cfg.Domains.CapitalFlows.FinalVersion,
	}
	svc := capitalflows.NewService(featRepo, scoreRepo, rawRepo,
		cgAdapter, cgAdapter, cgAdapter, cgAdapter, cgAdapter, pub, clock.Real{}, svcCfg,
		[]domain.Asset{domain.AssetGlobal})

	if from.Equal(to) {
		return svc.RunOnce(ctx, from.UTC())
	}
	return svc.RunBackfill(ctx, from.UTC(), to.UTC())
}

func runMarketStress(ctx context.Context, pool *storage.Pool,
	cfg config.OrchestratorConfig, configVersion string, from, to time.Time,
) error {
	p := cfg.Providers

	binAdapter := provbinancespot.NewAdapter(
		binance.New(p.BinanceSpot.BaseURL, time.Duration(p.BinanceSpot.TimeoutSec)*time.Second))
	krAdapter := provkraken_ms.NewAdapter(
		kraken.New(p.Kraken.BaseURL, time.Duration(p.Kraken.TimeoutSec)*time.Second))
	cbAdapter := provcoinbase_ms.NewAdapter(
		coinbase.New(p.Coinbase.BaseURL, time.Duration(p.Coinbase.TimeoutSec)*time.Second))

	var cgAdapter marketstress.CoinglassProvider
	var cgMicroAdapter marketstress.CoinglassMicroProvider
	if key := coinglassAPIKey(cfg); key != "" {
		cgTimeout := time.Duration(p.CoinGlass.TimeoutSec) * time.Second
		if cgTimeout == 0 {
			cgTimeout = 30 * time.Second
		}
		msCG := coinglass.New(key, cgTimeout)
		cgAdapter = coinglass.NewMarketStressPremiumAdapter(msCG)
		cgMicroAdapter = coinglass.NewMarketStressMicroAdapter(msCG)
	}

	featRepo := pgmarketstress.NewFeatureRepo(pool)
	scoreRepo := pgmarketstress.NewScoreRepo(pool)
	rawRepo := pgmarketstress.NewRawRepo(pool)
	leverageReader := pgmarketstress.NewLeverageFeatureReader(pool)
	pub := pgoutbox.New(pool)

	svcCfg := marketstress.Config{
		ModelVersion:         cfg.Domains.MarketStress.ModelVersion,
		ConfigVersion:        configVersion,
		CodeSHA:              GitSHA,
		IntermediateVersion:  cfg.Domains.MarketStress.IntermediateVersion,
		FinalVersion:         cfg.Domains.MarketStress.FinalVersion,
		LeverageBasisVersion: cfg.Domains.MarketStress.LeverageBasisVersion,
	}

	assets := make([]domain.Asset, 0, len(defaultAssets))
	for _, a := range defaultAssets {
		assets = append(assets, domain.Asset(a))
	}

	svc := marketstress.NewService(featRepo, scoreRepo, rawRepo,
		binAdapter, krAdapter, cbAdapter, cgAdapter, cgMicroAdapter, leverageReader,
		pub, clock.Real{}, svcCfg, assets)

	if from.Equal(to) {
		return svc.RunOnce(ctx, from.UTC())
	}
	return svc.RunBackfill(ctx, from.UTC(), to.UTC())
}

func runVolatility(ctx context.Context, pool *storage.Pool,
	cfg config.OrchestratorConfig, configVersion string, from, to time.Time,
) error {
	p := cfg.Providers
	dr := pgderibitvol.New(p.Deribit.BaseURL, time.Duration(p.Deribit.TimeoutSec)*time.Second)
	dvolProv := pgderibitvol.NewDvolAdapter(dr)
	optProv := pgderibitvol.NewOptionsAdapter(dr)
	chainProv := pgderibitvol.NewChainAdapter(dr)
	var cgOptions volatility.CoinglassOptionsProvider
	if k := p.CoinGlass.APIKey; k != "" {
		t := time.Duration(p.CoinGlass.TimeoutSec) * time.Second
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
		ModelVersion:        cfg.Domains.Volatility.ModelVersion,
		ConfigVersion:       configVersion,
		CodeSHA:             GitSHA,
		IntermediateVersion: cfg.Domains.Volatility.IntermediateVersion,
		FinalVersion:        cfg.Domains.Volatility.FinalVersion,
	}
	assets := make([]domain.Asset, 0, len(defaultAssets))
	for _, a := range defaultAssets {
		assets = append(assets, domain.Asset(a))
	}
	svc := volatility.NewService(featRepo, scoreRepo, rawRepo, dvolProv, optProv, cgOptions, chainProv, pub, clock.Real{}, svcCfg, assets)
	if from.Equal(to) {
		return svc.RunOnce(ctx, from.UTC())
	}
	return svc.RunBackfill(ctx, from.UTC(), to.UTC())
}

func runRegimeEngine(ctx context.Context, pool *storage.Pool,
	cfg config.OrchestratorConfig, configVersion string, from, to time.Time,
) error {
	ec := regime.DefaultConfig()
	ec.ModelVersion = cfg.Domains.RegimeEngine.ModelVersion
	ec.ConfigVersion = configVersion
	ec.CodeSHA = GitSHA
	if len(cfg.Domains.RegimeEngine.Weights) > 0 {
		w := make(map[domain.DomainCode]float64, len(cfg.Domains.RegimeEngine.Weights))
		for k, v := range cfg.Domains.RegimeEngine.Weights {
			w[domain.DomainCode(k)] = v
		}
		ec.Weights = w
	}
	if cfg.Domains.RegimeEngine.SigmoidSlopeK > 0 {
		ec.SigmoidK = cfg.Domains.RegimeEngine.SigmoidSlopeK
	}
	if cfg.Domains.RegimeEngine.Transition.RocWindowDays > 0 {
		ec.RocWindowDays = cfg.Domains.RegimeEngine.Transition.RocWindowDays
	}
	if cfg.Domains.RegimeEngine.Transition.RocWeight > 0 {
		ec.RocWeight = cfg.Domains.RegimeEngine.Transition.RocWeight
	}
	if cfg.Domains.RegimeEngine.Transition.DivergenceWeight > 0 {
		ec.DivergenceWeight = cfg.Domains.RegimeEngine.Transition.DivergenceWeight
	}
	if cfg.Domains.RegimeEngine.Transition.Baseline > 0 {
		ec.TransitionBaseline = cfg.Domains.RegimeEngine.Transition.Baseline
	}
	if cfg.Domains.RegimeEngine.Normalization.LookbackDays > 0 {
		ec.NormLookbackDays = cfg.Domains.RegimeEngine.Normalization.LookbackDays
	}
	if cfg.Domains.RegimeEngine.Normalization.MinSamples > 0 {
		ec.NormMinSamples = cfg.Domains.RegimeEngine.Normalization.MinSamples
	}

	ec.SmoothingSpanDays = cfg.Domains.RegimeEngine.Smoothing.SpanDays

	repo := pgregime.NewRegimeRepo(pool)
	reader := pgregime.NewDomainScoreReader(pool)
	pub := pgoutbox.New(pool)
	assets := make([]domain.Asset, 0, len(defaultAssets))
	for _, a := range defaultAssets {
		assets = append(assets, domain.Asset(a))
	}
	svc := regime.NewService(repo, reader, pub, clock.Real{}, ec, assets)
	if from.Equal(to) {
		return svc.RunOnce(ctx, from.UTC())
	}
	return svc.RunBackfill(ctx, from.UTC(), to.UTC())
}

func registerConfig(ctx context.Context, pool *storage.Pool,
	loaded config.Loaded[config.OrchestratorConfig], cfg config.OrchestratorConfig,
) error {
	parsed := map[string]any{
		"scope":               "orchestrator",
		"liquidity_model":     cfg.Domains.Liquidity.ModelVersion,
		"leverage_model":      cfg.Domains.Leverage.ModelVersion,
		"market_stress_model": cfg.Domains.MarketStress.ModelVersion,
		"capital_flows_model": cfg.Domains.CapitalFlows.ModelVersion,
		"volatility_model":    cfg.Domains.Volatility.ModelVersion,
		"regime_engine_model": cfg.Domains.RegimeEngine.ModelVersion,
	}
	body, _ := json.Marshal(parsed)
	_, err := pool.Exec(ctx, `
INSERT INTO model_configs (config_version, scope, model_version, yaml_body, yaml_parsed)
VALUES ($1, 'orchestrator', $2, $3, $4::jsonb)
ON CONFLICT (config_version) DO NOTHING`,
		loaded.ConfigVersion, cfg.Domains.RegimeEngine.ModelVersion,
		string(loaded.Raw), string(body))
	return err
}

func resolveValueDate(override string) (time.Time, error) {
	if override != "" {
		t, err := time.Parse("2006-01-02", override)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse value-date: %w", err)
		}
		return t.UTC(), nil
	}
	return time.Now().UTC().Truncate(24 * time.Hour), nil
}
