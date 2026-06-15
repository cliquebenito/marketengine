package backtest

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/regime"
)

type Service struct {
	runs      RunRepo
	states    RegimeStateRepo
	metrics   MetricsRepo
	scores    DomainScoreReader
	indicator IndicatorRawReader
	prices    PriceReader
	clock     Clock
	cfg       Config
	engineCfg regime.Config
	assets    []domain.Asset

	configYAML string
}

func NewService(
	runs RunRepo,
	states RegimeStateRepo,
	metrics MetricsRepo,
	scores DomainScoreReader,
	indicator IndicatorRawReader,
	prices PriceReader,
	clk Clock,
	cfg Config,
	engineCfg regime.Config,
	configYAML string,
	assets []domain.Asset,
) *Service {
	cfg.Defaults()
	engineCfg.Defaults()
	if len(assets) == 0 {
		assets = domain.AssetsTradeable()
	}
	return &Service{
		runs:       runs,
		states:     states,
		metrics:    metrics,
		scores:     scores,
		indicator:  indicator,
		prices:     prices,
		clock:      clk,
		cfg:        cfg,
		engineCfg:  engineCfg,
		assets:     assets,
		configYAML: configYAML,
	}
}

func (s *Service) Replay(ctx context.Context, periodStart, periodEnd time.Time, parent *RunID, mode string) (RunID, error) {
	periodStart, periodEnd = domain.UTCDay(periodStart), domain.UTCDay(periodEnd)
	if periodStart.After(periodEnd) {
		return "", fmt.Errorf("replay: periodStart (%s) after periodEnd (%s)",
			periodStart.Format("2006-01-02"), periodEnd.Format("2006-01-02"))
	}
	if mode == "" {
		mode = "replay"
	}

	snapshot, err := s.collectSnapshot(ctx, periodStart, periodEnd)
	if err != nil {
		return "", fmt.Errorf("replay snapshot: %w", err)
	}
	dataHash := SnapshotHash(snapshot)

	run := BacktestRun{
		Mode:             mode,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		ModelVersion:     s.engineCfg.ModelVersion,
		ConfigVersion:    s.engineCfg.ConfigVersion,
		ConfigYAML:       s.configYAML,
		CodeSHA:          s.cfg.CodeSHA,
		DataSnapshotHash: dataHash,
		SLAOffsetMinutes: s.cfg.SLAOffsetMinutes,
		ParentRunID:      parent,
		HarnessVersion:   s.cfg.HarnessVersion,
		Status:           "running",
		StartedAt:        s.clock.Now(),
	}
	runID, err := s.runs.Save(ctx, run)
	if err != nil {
		return "", fmt.Errorf("save run: %w", err)
	}

	saved := 0
	skipped := 0
	for d := periodStart; !d.After(periodEnd); d = d.AddDate(0, 0, 1) {
		for _, a := range s.assets {
			st, err := s.computeOne(ctx, a, d)
			if err != nil {
				slog.Warn("replay skip",
					"asset", a, "date", d.Format("2006-01-02"),
					"run_id", runID, "err", err)
				skipped++
				continue
			}
			if err := s.states.Save(ctx, runID, st); err != nil {
				slog.Warn("replay save state failed",
					"asset", a, "date", d.Format("2006-01-02"),
					"run_id", runID, "err", err)
				skipped++
				continue
			}
			saved++
		}
	}

	completed := s.clock.Now()
	if uerr := s.runs.UpdateStatus(ctx, runID, "completed", &completed); uerr != nil {
		slog.Warn("replay update status", "run_id", runID, "err", uerr)
	}
	slog.Info("replay done",
		"run_id", runID, "saved", saved, "skipped", skipped,
		"period_start", periodStart, "period_end", periodEnd)
	return runID, nil
}

func (s *Service) computeOne(ctx context.Context, asset domain.Asset, valueDate time.Time) (domain.RegimeState, error) {
	cutoff := valueDate.Add(time.Duration(s.cfg.SLAOffsetMinutes) * time.Minute)

	scores, err := s.scores.GetLatestAll(ctx, asset, valueDate, cutoff)
	if err != nil {
		return domain.RegimeState{}, fmt.Errorf("read domain scores: %w", err)
	}
	if regime.CoverageFraction(scores) < s.engineCfg.MinCoverage {
		return domain.RegimeState{}, fmt.Errorf("%w: %.0f%% (need %.0f%%)",
			domain.ErrInsufficientCoverage,
			regime.CoverageFraction(scores)*100, s.engineCfg.MinCoverage*100)
	}

	present := regime.PresentSet(scores)
	effW := regime.RedistributeWeights(s.engineCfg.Weights, present)

	histories := make(map[domain.DomainCode][]float64, len(scores))
	for d := range scores {
		h, herr := s.scores.GetHistory(ctx, asset, d, s.engineCfg.NormLookbackDays)
		if herr != nil {
			return domain.RegimeState{}, fmt.Errorf("domain history %s: %w", d, herr)
		}
		histories[d] = h
	}
	normalized := regime.NormalizeScores(scores, histories, s.engineCfg.NormMinSamples)

	rawIndicator := regime.WeightedSumRaw(scores, effW)
	contributions, normIndicator, _ := regime.WeightedSumNormalized(normalized, effW)

	trInputs, terr := s.gatherTransitionRiskInputs(ctx, asset, valueDate, cutoff, scores, normalized, present)
	if terr != nil {
		return domain.RegimeState{}, fmt.Errorf("transition risk inputs: %w", terr)
	}
	transRisk := regime.ComputeTransitionRisk(trInputs, s.engineCfg)

	indicator := normIndicator
	if s.engineCfg.SmoothingSpanDays > 1 {
		indicator = regime.EmaSmooth(normIndicator, nil, s.engineCfg.SmoothingSpanDays)
	}
	riskOn, riskOff := regime.FinalProbabilities(indicator, transRisk, s.engineCfg.SigmoidK)

	topDrivers := regime.BuildTopDrivers(contributions)

	coverageFlag := make(map[domain.DomainCode]bool, len(domain.AllDomains()))
	for _, d := range domain.AllDomains() {
		coverageFlag[d] = present[d]
	}

	return domain.RegimeState{
		Asset:               asset,
		ValueDate:           valueDate,
		RegimeIndicator:     indicator,
		RegimeIndicatorRaw:  rawIndicator,
		RiskOnProbability:   riskOn,
		RiskOffProbability:  riskOff,
		TransitionRisk:      transRisk,
		ModelVersion:        s.engineCfg.ModelVersion,
		ConfigVersion:       s.engineCfg.ConfigVersion,
		CodeSHA:             s.engineCfg.CodeSHA,
		DomainContributions: contributions,
		TopDrivers:          topDrivers,
		EffectiveWeights:    effW,
		FeatureCoverageFlag: coverageFlag,
		InteractionFlags:    []string{},
	}, nil
}

func (s *Service) gatherTransitionRiskInputs(
	ctx context.Context,
	asset domain.Asset,
	valueDate, cutoff time.Time,
	scores map[domain.DomainCode]float64,
	normalized map[domain.DomainCode]float64,
	present map[domain.DomainCode]bool,
) (regime.TransitionRiskInputs, error) {
	in := regime.TransitionRiskInputs{NormalizedScores: normalized}
	refDate := valueDate.AddDate(0, 0, -s.engineCfg.RocWindowDays)
	for _, d := range domain.AllDomains() {
		if !present[d] {
			continue
		}
		past, found, err := s.scores.GetByDate(ctx, asset, d, refDate, cutoff)
		if err != nil {
			return in, fmt.Errorf("domain score on %s: %w", refDate.Format("2006-01-02"), err)
		}
		if !found {
			continue
		}
		today := scores[d]
		in.RocPerDomain = append(in.RocPerDomain, math.Abs(today-past)/float64(s.engineCfg.RocWindowDays))
	}

	histFrom := valueDate.AddDate(0, 0, -90-s.engineCfg.RocWindowDays)
	histTo := valueDate.AddDate(0, 0, -1)
	rawSeries, err := s.indicator.GetIndicatorRawSeries(ctx, asset, histFrom, histTo, cutoff)
	if err != nil {
		return in, fmt.Errorf("indicator raw series: %w", err)
	}
	in.HistoricalRocs, in.HistoricalDivs = regime.ComputeHistoricalRocDiv(rawSeries, valueDate, s.engineCfg.RocWindowDays, 90)

	threeDaysAgo := valueDate.AddDate(0, 0, -3)
	allSameDir := true
	checkedAny := false
	for _, d := range domain.AllDomains() {
		if !present[d] {
			continue
		}
		past, found, err := s.scores.GetByDate(ctx, asset, d, threeDaysAgo, cutoff)
		if err != nil {
			return in, fmt.Errorf("momentum lookup %s: %w", d, err)
		}
		if !found {
			allSameDir = false
			break
		}
		todayNorm := normalized[d]
		if regime.SignF(todayNorm) != regime.SignF(past) {
			allSameDir = false
			break
		}
		checkedAny = true
	}
	in.MomentumChecked = checkedAny
	in.MomentumSameDirection = checkedAny && allSameDir
	return in, nil
}

func (s *Service) collectSnapshot(ctx context.Context, from, to time.Time) (map[SnapshotKey]map[domain.DomainCode]float64, error) {
	out := make(map[SnapshotKey]map[domain.DomainCode]float64)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		for _, a := range s.assets {
			cutoff := d.Add(time.Duration(s.cfg.SLAOffsetMinutes) * time.Minute)
			scores, err := s.scores.GetLatestAll(ctx, a, d, cutoff)
			if err != nil {
				return nil, err
			}
			if len(scores) == 0 {
				continue
			}
			out[SnapshotKey{Asset: a, ValueDate: d}] = scores
		}
	}
	return out, nil
}

func (s *Service) ComputeMetrics(ctx context.Context, runID RunID) error {
	run, err := s.runs.Get(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run %s: %w", runID, err)
	}
	rows, err := s.states.GetByRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get states %s: %w", runID, err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("no replay states for run %s", runID)
	}

	byAsset := map[domain.Asset][]domain.RegimeState{}
	for _, r := range rows {
		byAsset[r.Asset] = append(byAsset[r.Asset], r)
	}

	for asset, states := range byAsset {
		days := s.toRegimeDays(states)
		prices, err := s.loadPrices(ctx, asset, run.PeriodStart, run.PeriodEnd.AddDate(0, 0, 200))
		if err != nil {
			return fmt.Errorf("load prices %s: %w", asset, err)
		}

		fwd := ForwardReturnTable(days, prices, []int{30, 90, 180})
		for _, c := range fwd {
			scope := fmt.Sprintf("%s|%s|h=%d", asset, c.Label, c.Horizon)
			if err := s.metrics.Save(ctx, runID, Metric{
				Name: "forward_return_mean", Scope: scope, Value: c.Mean,
				Metadata: map[string]any{"n": c.N, "median": c.Median, "hit_rate": c.HitRate},
			}); err != nil {
				return err
			}
		}

		ci := BootstrapForwardReturnCI(days, prices, []int{30, 90, 180}, 1000)
		for _, c := range ci {
			scope := fmt.Sprintf("%s|%s|h=%d", asset, c.Label, c.Horizon)
			if err := s.metrics.Save(ctx, runID, Metric{
				Name: "forward_return_mean_ci_low", Scope: scope, Value: c.MeanCILow,
				Metadata: map[string]any{"n_resamples": c.NResamples},
			}); err != nil {
				return err
			}
			if err := s.metrics.Save(ctx, runID, Metric{
				Name: "forward_return_mean_ci_high", Scope: scope, Value: c.MeanCIHigh,
				Metadata: map[string]any{"n_resamples": c.NResamples},
			}); err != nil {
				return err
			}
		}

		persist := PersistenceMetrics(days)
		if err := s.metrics.Save(ctx, runID, Metric{
			Name: "flip_flop_rate", Scope: string(asset), Value: persist.FlipFlopRate,
			Metadata: map[string]any{"transitions": persist.Transitions, "transitions_per_year": persist.TransitionsPerYear},
		}); err != nil {
			return err
		}
		for lbl, dur := range persist.MeanDurationDays {
			if err := s.metrics.Save(ctx, runID, Metric{
				Name: "mean_duration_days", Scope: fmt.Sprintf("%s|%s", asset, lbl), Value: dur,
			}); err != nil {
				return err
			}
		}

		strat := ToyStrategy(days, prices)
		if err := s.metrics.Save(ctx, runID, Metric{
			Name: "strategy_calmar", Scope: string(asset), Value: strat.Calmar,
			Metadata: map[string]any{
				"sharpe": strat.Sharpe, "ann_return": strat.AnnualizedReturn,
				"max_dd": strat.MaxDrawdown, "buy_hold_calmar": strat.BuyAndHoldCalmar,
			},
		}); err != nil {
			return err
		}
		if err := s.metrics.Save(ctx, runID, Metric{
			Name: "strategy_sharpe", Scope: string(asset), Value: strat.Sharpe,
		}); err != nil {
			return err
		}

		byDate := make(map[time.Time]RegimeDay, len(days))
		for _, d := range days {
			byDate[d.ValueDate] = d
		}
		events := EventLeadTimes(byDate, CalibrationEvents(), 0.6)
		for _, e := range events {
			if err := s.metrics.Save(ctx, runID, Metric{
				Name:  "event_first_risk_off_offset",
				Scope: fmt.Sprintf("%s|%s", asset, e.Event.Name),
				Value: float64(e.FirstRiskOffOffset),
				Metadata: map[string]any{
					"first_trans_offset": e.FirstTransOffset,
					"data_present":       e.DataPresent,
				},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) Sweep(ctx context.Context, periodStart, periodEnd time.Time, parameter string, values []float64) ([]RunID, []SensitivityRow, error) {
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("sweep: empty values")
	}

	seed := BacktestRun{
		Mode:             "sensitivity",
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		ModelVersion:     s.engineCfg.ModelVersion,
		ConfigVersion:    s.engineCfg.ConfigVersion,
		ConfigYAML:       s.configYAML,
		CodeSHA:          s.cfg.CodeSHA,
		DataSnapshotHash: "sha256:sweep-seed",
		SLAOffsetMinutes: s.cfg.SLAOffsetMinutes,
		HarnessVersion:   s.cfg.HarnessVersion,
		Status:           "running",
		StartedAt:        s.clock.Now(),
		Metadata: map[string]any{
			"parameter": parameter, "values": values,
		},
	}
	seedID, err := s.runs.Save(ctx, seed)
	if err != nil {
		return nil, nil, fmt.Errorf("save sweep seed: %w", err)
	}

	out := make([]RunID, 0, len(values))
	rows := make([]SensitivityRow, 0, len(values))
	for _, v := range values {
		original := s.engineCfg
		switch parameter {
		case "transition.baseline":
			s.engineCfg.TransitionBaseline = v
		case "smoothing.span_days":
			s.engineCfg.SmoothingSpanDays = int(v)
		default:
			s.engineCfg = original
			return out, rows, fmt.Errorf("sweep: unsupported parameter %q", parameter)
		}
		runID, err := s.Replay(ctx, periodStart, periodEnd, &seedID, "sensitivity")
		s.engineCfg = original
		if err != nil {
			return out, rows, fmt.Errorf("sweep value %v: %w", v, err)
		}
		out = append(out, runID)

		if err := s.ComputeMetrics(ctx, runID); err != nil {
			slog.Warn("sweep compute metrics", "run_id", runID, "err", err)
		}

		ms, err := s.metrics.GetByRun(ctx, runID)
		if err != nil {
			return out, rows, fmt.Errorf("sweep metrics %s: %w", runID, err)
		}
		row := SensitivityRow{Parameter: parameter, Value: v, Metrics: map[string]float64{}}
		for _, m := range ms {

			if m.Name == "flip_flop_rate" || m.Name == "strategy_calmar" || m.Name == "strategy_sharpe" {
				row.Metrics[fmt.Sprintf("%s|%s", m.Scope, m.Name)] = m.Value
			}
		}
		rows = append(rows, row)
	}

	completed := s.clock.Now()
	if uerr := s.runs.UpdateStatus(ctx, seedID, "completed", &completed); uerr != nil {
		slog.Warn("sweep update status", "run_id", seedID, "err", uerr)
	}
	return out, rows, nil
}

func (s *Service) toRegimeDays(states []domain.RegimeState) []RegimeDay {
	out := make([]RegimeDay, 0, len(states))
	prev := LabelTransition
	for _, st := range states {
		lbl := ClassifyHysteresis(DefaultHysteresis(), prev, st.RegimeIndicator, st.TransitionRisk)
		out = append(out, RegimeDay{
			ValueDate:       st.ValueDate,
			RegimeIndicator: st.RegimeIndicator,
			RiskOnProb:      st.RiskOnProbability,
			RiskOffProb:     st.RiskOffProbability,
			TransitionRisk:  st.TransitionRisk,
			Label:           lbl,
		})
		prev = lbl
	}
	return out
}

func (s *Service) loadPrices(ctx context.Context, asset domain.Asset, from, to time.Time) (map[time.Time]float64, error) {
	pts, err := s.prices.GetPriceHistory(ctx, asset, from, to)
	if err != nil {
		return nil, err
	}
	out := make(map[time.Time]float64, len(pts))
	for _, p := range pts {
		out[domain.UTCDay(p.Date)] = p.Price
	}
	return out, nil
}
