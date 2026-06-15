package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"marketengine/internal/backtest"
	"marketengine/internal/storage"
)

func main() {
	var (
		dbURL    = flag.String("db", "postgres://regime:regime@localhost:5432/regime?sslmode=disable", "DB URL")
		asset    = flag.String("asset", "BTC", "asset to analyse")
		fromS    = flag.String("from", "2023-01-01", "analysis start (YYYY-MM-DD)")
		toS      = flag.String("to", "", "analysis end (YYYY-MM-DD); default = today")
		thrTrig  = flag.Float64("event-transition-threshold", 0.6, "transition_risk threshold for event lead-time firing")
		classTR  = flag.Float64("class-trans", 0.50, "classifier: transition_risk threshold for TRANSITION label")
		classOn  = flag.Float64("class-on", 0.15, "classifier: regime_indicator floor for RISK_ON label")
		classOff = flag.Float64("class-off", -0.15, "classifier: regime_indicator ceiling for RISK_OFF label")
		smoothN  = flag.Int("smooth", 0, "EMA span (days) to smooth regime_indicator + transition_risk before classifying; 0 = disabled")

		useHyst   = flag.Bool("hysteresis", false, "use the stateful hysteresis classifier instead of the stateless one")
		hystTrEn  = flag.Float64("class-trans-enter", backtest.DefaultHysteresis().TransitionEnter, "hysteresis: transition_risk enter threshold")
		hystTrEx  = flag.Float64("class-trans-exit", backtest.DefaultHysteresis().TransitionExit, "hysteresis: transition_risk exit threshold")
		hystOnEn  = flag.Float64("class-on-enter", backtest.DefaultHysteresis().RiskOnEnter, "hysteresis: regime_indicator enter threshold for risk_on")
		hystOnEx  = flag.Float64("class-on-exit", backtest.DefaultHysteresis().RiskOnExit, "hysteresis: regime_indicator exit threshold for risk_on")
		hystOffEn = flag.Float64("class-off-enter", backtest.DefaultHysteresis().RiskOffEnter, "hysteresis: regime_indicator enter threshold for risk_off")
		hystOffEx = flag.Float64("class-off-exit", backtest.DefaultHysteresis().RiskOffExit, "hysteresis: regime_indicator exit threshold for risk_off")
	)
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	from, err := time.Parse("2006-01-02", *fromS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse -from: %v\n", err)
		os.Exit(2)
	}
	to := time.Now().UTC().Truncate(24 * time.Hour)
	if *toS != "" {
		to, err = time.Parse("2006-01-02", *toS)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse -to: %v\n", err)
			os.Exit(2)
		}
	}

	ctx := context.Background()
	pool, err := storage.Open(ctx, *dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	days, err := loadRegimeDays(ctx, pool, *asset, from, to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load regime states: %v\n", err)
		os.Exit(1)
	}
	if len(days) == 0 {
		fmt.Fprintf(os.Stderr, "no regime_states rows in range\n")
		os.Exit(1)
	}

	coinID := coinIDFor(*asset)
	pricesRaw, err := loadPrices(ctx, pool, coinID, from.AddDate(0, 0, -10), to.AddDate(0, 0, 200))
	if err != nil {
		fmt.Fprintf(os.Stderr, "load prices: %v\n", err)
		os.Exit(1)
	}
	prices := forwardFillDaily(pricesRaw, from, to.AddDate(0, 0, 200))

	if *smoothN > 1 {
		applyEMA(days, *smoothN)
	}

	cfg := backtest.ClassifierConfig{
		TransitionRiskHigh: *classTR,
		RiskOnFloor:        *classOn,
		RiskOffCeiling:     *classOff,
	}
	hystCfg := backtest.HysteresisConfig{
		TransitionEnter: *hystTrEn,
		TransitionExit:  *hystTrEx,
		RiskOnEnter:     *hystOnEn,
		RiskOnExit:      *hystOnEx,
		RiskOffEnter:    *hystOffEn,
		RiskOffExit:     *hystOffEx,
	}
	if *useHyst {
		if err := hystCfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
		prev := backtest.LabelTransition
		for i := range days {
			days[i].Label = backtest.ClassifyHysteresis(hystCfg, prev, days[i].RegimeIndicator, days[i].TransitionRisk)
			prev = days[i].Label
		}
	} else {
		for i := range days {
			days[i].Label = backtest.Classify(cfg, days[i].RegimeIndicator, days[i].TransitionRisk)
		}
	}
	byDate := make(map[time.Time]backtest.RegimeDay, len(days))
	for _, d := range days {
		byDate[d.ValueDate] = d
	}

	fwd := backtest.ForwardReturnTable(days, prices, []int{30, 90, 180})
	persist := backtest.PersistenceMetrics(days)
	events := backtest.EventLeadTimes(byDate, backtest.CalibrationEvents(), *thrTrig)
	strat := backtest.ToyStrategy(days, prices)

	printReport(os.Stdout, *asset, from, to, cfg, hystCfg, *useHyst, len(days), len(prices), fwd, persist, events, strat, *thrTrig)
}

func coinIDFor(asset string) string {
	switch asset {
	case "BTC":
		return "bitcoin"
	case "ETH":
		return "ethereum"
	}
	return ""
}

func loadRegimeDays(ctx context.Context, pool *storage.Pool, asset string, from, to time.Time) ([]backtest.RegimeDay, error) {
	rows, err := pool.Query(ctx, `
SELECT DISTINCT ON (value_date)
  value_date, regime_indicator, risk_on_probability, risk_off_probability, transition_risk
FROM regime_states
WHERE asset = $1::asset_code
  AND value_date BETWEEN $2 AND $3
ORDER BY value_date ASC, ingested_at DESC`, asset, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []backtest.RegimeDay
	for rows.Next() {
		var d backtest.RegimeDay
		if err := rows.Scan(&d.ValueDate, &d.RegimeIndicator, &d.RiskOnProb,
			&d.RiskOffProb, &d.TransitionRisk); err != nil {
			return nil, err
		}

		d.ValueDate = d.ValueDate.UTC().Truncate(24 * time.Hour)
		out = append(out, d)
	}
	return out, rows.Err()
}

func loadPrices(ctx context.Context, pool *storage.Pool, coinID string, from, to time.Time) (map[time.Time]float64, error) {
	rows, err := pool.Query(ctx, `
SELECT DISTINCT ON (value_date)
  value_date, price_usd
FROM raw_coingecko_market_cap
WHERE coin_id = $1
  AND value_date BETWEEN $2 AND $3
  AND price_usd > 0
ORDER BY value_date ASC, ingested_at DESC`, coinID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[time.Time]float64)
	for rows.Next() {
		var d time.Time
		var p float64
		if err := rows.Scan(&d, &p); err != nil {
			return nil, err
		}
		d = d.UTC().Truncate(24 * time.Hour)
		out[d] = p
	}
	return out, rows.Err()
}

func printReport(w *os.File, asset string, from, to time.Time, cfg backtest.ClassifierConfig,
	hystCfg backtest.HysteresisConfig, useHyst bool,
	nDays, nPrices int,
	fwd []backtest.ForwardReturnCell, persist backtest.PersistenceStats,
	events []backtest.EventLead, strat backtest.StrategyStats, evThreshold float64,
) {
	fmt.Fprintf(w, "=========================================================\n")
	fmt.Fprintf(w, "Regime Engine backtest metrics — asset=%s\n", asset)
	fmt.Fprintf(w, "  Period: %s → %s (%d days)\n", from.Format("2006-01-02"), to.Format("2006-01-02"), nDays)
	fmt.Fprintf(w, "  Price coverage: %d observations\n", nPrices)
	if useHyst {
		fmt.Fprintf(w, "  Classifier: HYSTERESIS (enter/exit)\n")
		fmt.Fprintf(w, "    transition_risk: enter>=%.2f  exit<%.2f\n", hystCfg.TransitionEnter, hystCfg.TransitionExit)
		fmt.Fprintf(w, "    risk_on       : enter>%.2f   exit<=%.2f\n", hystCfg.RiskOnEnter, hystCfg.RiskOnExit)
		fmt.Fprintf(w, "    risk_off      : enter<%.2f  exit>=%.2f\n", hystCfg.RiskOffEnter, hystCfg.RiskOffExit)
	} else {
		fmt.Fprintf(w, "  Classifier: transition_risk>=%.2f → TRANSITION; else regime_indicator>%.2f→RISK_ON, <%.2f→RISK_OFF\n",
			cfg.TransitionRiskHigh, cfg.RiskOnFloor, cfg.RiskOffCeiling)
	}
	fmt.Fprintf(w, "  Classifier thresholds: INVENTED (see classify.go; to be calibrated).\n\n")

	counts := map[backtest.Label]int{}
	for _, n := range []backtest.Label{backtest.LabelRiskOn, backtest.LabelRiskOff, backtest.LabelTransition} {
		_ = n
	}

	for _, lbl := range []backtest.Label{backtest.LabelRiskOn, backtest.LabelRiskOff, backtest.LabelTransition} {
		counts[lbl] = 0
		_ = lbl
	}

	fmt.Fprintf(w, "── §3.1 Forward returns by label ─────────────────────────\n")
	fmt.Fprintf(w, "%-12s %6s %6s %10s %10s %8s\n", "label", "h(d)", "n", "mean%", "median%", "hit%")
	for _, c := range fwd {
		fmt.Fprintf(w, "%-12s %6d %6d %9.2f%% %9.2f%% %7.1f%%\n",
			c.Label, c.Horizon, c.N, c.Mean*100, c.Median*100, c.HitRate*100)
	}
	fmt.Fprintf(w, "\n")

	var onMean90, offMean90 float64
	var foundOn, foundOff bool
	for _, c := range fwd {
		if c.Horizon == 90 {
			if c.Label == backtest.LabelRiskOn {
				onMean90, foundOn = c.Mean, true
			} else if c.Label == backtest.LabelRiskOff {
				offMean90, foundOff = c.Mean, true
			}
		}
	}
	if foundOn && foundOff {
		spread := (onMean90 - offMean90) * 100
		verdict := "FAIL"
		if spread >= 15 {
			verdict = "PASS"
		}
		fmt.Fprintf(w, "  Spread(risk_on − risk_off) 90d mean = %+.2f pp  [target ≥ 15 pp: %s]\n\n", spread, verdict)
	}

	fmt.Fprintf(w, "── §3.2 Regime persistence ──────────────────────────────\n")
	fmt.Fprintf(w, "  Total days: %d | transitions: %d | transitions/year: %.1f\n",
		persist.TotalDays, persist.Transitions, persist.TransitionsPerYear)
	fmt.Fprintf(w, "  Flip-flop rate (reversed within 7d): %.1f%%  [target ≤ 15%%: %s]\n",
		persist.FlipFlopRate*100, passFail(persist.FlipFlopRate <= 0.15))
	for _, lbl := range []backtest.Label{backtest.LabelRiskOn, backtest.LabelRiskOff, backtest.LabelTransition} {
		if d, ok := persist.MeanDurationDays[lbl]; ok {
			verdict := ""
			if lbl == backtest.LabelRiskOn || lbl == backtest.LabelRiskOff {
				verdict = fmt.Sprintf("  [target ≥ 14d: %s]", passFail(d >= 14))
			}
			fmt.Fprintf(w, "  mean duration %-12s: %5.1f d (median %4.1f)%s\n",
				lbl, d, persist.MedianDurationDays[lbl], verdict)
		}
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "── §3.6 Calibration events lead-time ────────────────────\n")
	fmt.Fprintf(w, "  (trans. threshold = %.2f; offsets in days before peak; negative = signal fired after peak; --- = never in [-14d, +3d])\n",
		evThreshold)
	fmt.Fprintf(w, "%-30s %-12s %7s %7s %9s %9s %9s\n", "event", "peak_date", "lead_ro", "lead_tr", "ri@peak", "tr@peak", "covered")
	var riSamples []int
	var trSamples []int
	for _, e := range events {
		riStr, trStr := "---", "---"
		if e.FirstRiskOffOffset != -999 {
			riStr = fmt.Sprintf("%+d", e.FirstRiskOffOffset)
			riSamples = append(riSamples, e.FirstRiskOffOffset)
		}
		if e.FirstTransOffset != -999 {
			trStr = fmt.Sprintf("%+d", e.FirstTransOffset)
			trSamples = append(trSamples, e.FirstTransOffset)
		}
		ri, tr := "   --", "   --"
		if e.DataPresent {
			ri = fmt.Sprintf("%+6.2f", e.RegimeAtPeak)
			tr = fmt.Sprintf("%6.2f", e.TransitionAtPeak)
		}
		covered := "no"
		if e.DataPresent {
			covered = "yes"
		}
		fmt.Fprintf(w, "%-30s %-12s %7s %7s %9s %9s %9s\n",
			e.Event.Name, e.Event.PeakDate.Format("2006-01-02"), riStr, trStr, ri, tr, covered)
	}

	printDistribution(w, "regime_indicator<0 lead  ", riSamples)
	printDistribution(w, "transition_risk>thresh lead", trSamples)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "── §3.5 Toy regime-based allocator ──────────────────────\n")
	fmt.Fprintf(w, "  Strategy:  risk_on=100%% BTC, risk_off=0%%, transition=50%%  (5bps rebalance cost)\n")
	fmt.Fprintf(w, "             ann.ret=%6.2f%%  ann.vol=%5.2f%%  Sharpe=%5.2f  maxDD=%5.2f%%  Calmar=%5.2f\n",
		strat.AnnualizedReturn*100, strat.AnnualizedVol*100, strat.Sharpe, strat.MaxDrawdown*100, strat.Calmar)
	fmt.Fprintf(w, "  Buy-hold:  ann.ret=%6.2f%%                                       maxDD=%5.2f%%  Calmar=%5.2f\n",
		strat.BuyAndHoldReturn*100, strat.BuyAndHoldMaxDD*100, strat.BuyAndHoldCalmar)
	if strat.BuyAndHoldCalmar > 0 {
		calmarRatio := strat.Calmar / strat.BuyAndHoldCalmar
		ddRatio := strat.MaxDrawdown / strat.BuyAndHoldMaxDD
		fmt.Fprintf(w, "  Calmar(strat)/Calmar(B&H) = %.2fx  [target ≥ 1.5x: %s]\n",
			calmarRatio, passFail(calmarRatio >= 1.5))
		fmt.Fprintf(w, "  MaxDD(strat)/MaxDD(B&H)   = %.2fx  [target ≤ 0.6x: %s]\n",
			ddRatio, passFail(ddRatio <= 0.6))
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "=========================================================\n")
}

func forwardFillDaily(sparse map[time.Time]float64, from, to time.Time) map[time.Time]float64 {
	if len(sparse) == 0 {
		return sparse
	}
	dates := make([]time.Time, 0, len(sparse))
	for d := range sparse {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

	out := make(map[time.Time]float64, int(to.Sub(from).Hours()/24)+1)
	var lastPrice float64
	var lastDate time.Time
	pi := 0
	const maxGapDays = 7
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {

		for pi < len(dates) && !dates[pi].After(d) {
			lastPrice = sparse[dates[pi]]
			lastDate = dates[pi]
			pi++
		}
		if lastPrice > 0 && int(d.Sub(lastDate).Hours()/24) <= maxGapDays {
			out[d] = lastPrice
		}
	}
	return out
}

func applyEMA(days []backtest.RegimeDay, span int) {
	if len(days) == 0 || span <= 1 {
		return
	}
	alpha := 2.0 / (float64(span) + 1.0)
	ri := days[0].RegimeIndicator
	tr := days[0].TransitionRisk
	for i := range days {
		ri = alpha*days[i].RegimeIndicator + (1-alpha)*ri
		tr = alpha*days[i].TransitionRisk + (1-alpha)*tr
		days[i].RegimeIndicator = ri
		days[i].TransitionRisk = tr
	}
}

func passFail(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func printDistribution(w *os.File, label string, vs []int) {
	if len(vs) == 0 {
		fmt.Fprintf(w, "  %s: no samples\n", label)
		return
	}
	sorted := append([]int(nil), vs...)
	sort.Ints(sorted)
	med := sorted[len(sorted)/2]
	p10 := sorted[int(float64(len(sorted))*0.1)]
	p90 := sorted[int(float64(len(sorted))*0.9)]
	if p90 >= len(sorted) {
		p90 = sorted[len(sorted)-1]
	}
	fmt.Fprintf(w, "  %s: n=%d  p10=%+d  median=%+d  p90=%+d\n",
		label, len(vs), p10, med, p90)
}
