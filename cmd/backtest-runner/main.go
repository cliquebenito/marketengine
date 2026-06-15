package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"marketengine/internal/backtest"
	"marketengine/internal/config"
	"marketengine/internal/domain"
	pgbacktest "marketengine/internal/infra/postgres/backtest"
	"marketengine/internal/regime"
	"marketengine/internal/storage"
	"marketengine/pkg/clock"
)

var GitSHA = "dev"

func main() {
	var (
		configPath  = flag.String("config", "configs/regime-engine.yaml", "path to YAML config")
		mode        = flag.String("mode", "replay", "replay | sensitivity")
		from        = flag.String("from", "", "period start (YYYY-MM-DD)")
		to          = flag.String("to", "", "period end (YYYY-MM-DD)")
		sweepArg    = flag.String("sweep", "", "for mode=sensitivity: parameter=v1,v2,v3 (e.g. transition.baseline=0.4,0.5,0.6)")
		metricsOnly = flag.Bool("metrics-only", false, "skip replay; recompute metrics for -run")
		runIDFlag   = flag.String("run", "", "existing run_id (required with -metrics-only)")
		slaMinutes  = flag.Int("sla-minutes", 60, "PIT cutoff = value_date + N minutes; use very large value (e.g. 525600 = 1 year) for post-hoc replay where source data was ingested AFTER the period")
	)
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(*configPath, *mode, *from, *to, *sweepArg, *metricsOnly, *runIDFlag, *slaMinutes); err != nil {
		slog.Error("backtest-runner exit", "err", err)
		os.Exit(1)
	}
}

func run(configPath, mode, fromS, toS, sweepArg string, metricsOnly bool, runIDFlag string, slaMinutes int) error {
	loaded, err := config.Load[config.RegimeEngineConfig](configPath)
	if err != nil {
		return err
	}
	cfg := loaded.Parsed

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := storage.Open(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	runRepo := pgbacktest.NewRunRepo(pool)
	stateRepo := pgbacktest.NewRegimeStateRepo(pool)
	metricsRepo := pgbacktest.NewMetricsRepo(pool)
	scoreReader := pgbacktest.NewDomainScoreReader(pool)
	indicatorReader := pgbacktest.NewIndicatorRawReader(pool)
	priceReader := pgbacktest.NewPriceReader(pool)

	engineCfg := buildEngineConfig(cfg, loaded.ConfigVersion)
	bcfg := backtest.Config{SLAOffsetMinutes: slaMinutes, CodeSHA: GitSHA}
	bcfg.Defaults()
	assets := assetsFromConfig(cfg)

	svc := backtest.NewService(
		runRepo, stateRepo, metricsRepo, scoreReader, indicatorReader, priceReader,
		clock.Real{}, bcfg, engineCfg, string(loaded.Raw), assets,
	)

	if metricsOnly {
		if runIDFlag == "" {
			return fmt.Errorf("-run required with -metrics-only")
		}
		slog.Info("recomputing metrics", "run_id", runIDFlag)
		return svc.ComputeMetrics(ctx, backtest.RunID(runIDFlag))
	}

	if fromS == "" || toS == "" {
		return fmt.Errorf("-from and -to required")
	}
	periodStart, err := time.Parse("2006-01-02", fromS)
	if err != nil {
		return fmt.Errorf("parse -from: %w", err)
	}
	periodEnd, err := time.Parse("2006-01-02", toS)
	if err != nil {
		return fmt.Errorf("parse -to: %w", err)
	}

	switch mode {
	case "replay":
		runID, err := svc.Replay(ctx, periodStart.UTC(), periodEnd.UTC(), nil, "replay")
		if err != nil {
			return fmt.Errorf("replay: %w", err)
		}
		slog.Info("replay completed", "run_id", string(runID))
		if err := svc.ComputeMetrics(ctx, runID); err != nil {
			return fmt.Errorf("compute metrics: %w", err)
		}
		slog.Info("metrics computed", "run_id", string(runID))
		return nil

	case "sensitivity":
		if sweepArg == "" {
			return fmt.Errorf("-sweep required with -mode sensitivity")
		}
		param, values, err := parseSweep(sweepArg)
		if err != nil {
			return fmt.Errorf("parse -sweep: %w", err)
		}
		slog.Info("sensitivity sweep", "parameter", param, "values", values)
		ids, rows, err := svc.Sweep(ctx, periodStart.UTC(), periodEnd.UTC(), param, values)
		if err != nil {
			return fmt.Errorf("sweep: %w", err)
		}
		fmt.Println(backtest.SensitivitySweep(rows))
		fmt.Printf("\nrun_ids:\n")
		for i, id := range ids {
			fmt.Printf("  %s = %g  →  %s\n", param, values[i], string(id))
		}
		return nil

	default:
		return fmt.Errorf("unknown -mode %q (want replay|sensitivity)", mode)
	}
}

func parseSweep(s string) (string, []float64, error) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("expected parameter=v1,v2,v3")
	}
	param := strings.TrimSpace(parts[0])
	rawVals := strings.Split(parts[1], ",")
	values := make([]float64, 0, len(rawVals))
	for _, rv := range rawVals {
		v, err := strconv.ParseFloat(strings.TrimSpace(rv), 64)
		if err != nil {
			return "", nil, fmt.Errorf("invalid value %q: %w", rv, err)
		}
		values = append(values, v)
	}
	if len(values) == 0 {
		return "", nil, fmt.Errorf("no values to sweep")
	}
	return param, values, nil
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
