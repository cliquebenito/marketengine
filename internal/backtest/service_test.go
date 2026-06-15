package backtest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/regime"
)

type fakeRunRepo struct {
	saved   map[RunID]BacktestRun
	order   []RunID
	saveErr error
}

func newFakeRunRepo() *fakeRunRepo {
	return &fakeRunRepo{saved: map[RunID]BacktestRun{}}
}

func (r *fakeRunRepo) Save(_ context.Context, run BacktestRun) (RunID, error) {
	if r.saveErr != nil {
		return "", r.saveErr
	}
	id := RunID(fmt.Sprintf("run-%d", len(r.saved)+1))
	run.ID = id
	r.saved[id] = run
	r.order = append(r.order, id)
	return id, nil
}
func (r *fakeRunRepo) Get(_ context.Context, id RunID) (BacktestRun, error) {
	v, ok := r.saved[id]
	if !ok {
		return BacktestRun{}, domain.ErrNotFound
	}
	return v, nil
}
func (r *fakeRunRepo) UpdateStatus(_ context.Context, id RunID, status string, completedAt *time.Time) error {
	v, ok := r.saved[id]
	if !ok {
		return domain.ErrNotFound
	}
	v.Status = status
	v.CompletedAt = completedAt
	r.saved[id] = v
	return nil
}

type fakeStateRepo struct {
	byRun map[RunID][]domain.RegimeState
}

func newFakeStateRepo() *fakeStateRepo {
	return &fakeStateRepo{byRun: map[RunID][]domain.RegimeState{}}
}
func (r *fakeStateRepo) Save(_ context.Context, runID RunID, st domain.RegimeState) error {
	r.byRun[runID] = append(r.byRun[runID], st)
	return nil
}
func (r *fakeStateRepo) GetByRun(_ context.Context, runID RunID) ([]domain.RegimeState, error) {
	out := append([]domain.RegimeState(nil), r.byRun[runID]...)
	sort.Slice(out, func(i, j int) bool { return out[i].ValueDate.Before(out[j].ValueDate) })
	return out, nil
}

type fakeMetricsRepo struct {
	byRun map[RunID][]Metric
}

func newFakeMetricsRepo() *fakeMetricsRepo { return &fakeMetricsRepo{byRun: map[RunID][]Metric{}} }
func (r *fakeMetricsRepo) Save(_ context.Context, runID RunID, m Metric) error {
	r.byRun[runID] = append(r.byRun[runID], m)
	return nil
}
func (r *fakeMetricsRepo) GetByRun(_ context.Context, runID RunID) ([]Metric, error) {
	return append([]Metric(nil), r.byRun[runID]...), nil
}

type fakeScoreReader struct {
	latest  map[string]map[domain.DomainCode]float64
	byDate  map[string]float64
	history map[string][]float64
}

func newFakeScoreReader() *fakeScoreReader {
	return &fakeScoreReader{
		latest:  map[string]map[domain.DomainCode]float64{},
		byDate:  map[string]float64{},
		history: map[string][]float64{},
	}
}

func skLatest(a domain.Asset, d time.Time) string {
	return string(a) + "|" + d.Format("2006-01-02")
}
func skByDate(a domain.Asset, dom domain.DomainCode, d time.Time) string {
	return string(a) + "|" + string(dom) + "|" + d.Format("2006-01-02")
}

func (r *fakeScoreReader) GetLatestAll(_ context.Context, a domain.Asset, d, _ time.Time) (map[domain.DomainCode]float64, error) {
	m, ok := r.latest[skLatest(a, d)]
	if !ok {
		return map[domain.DomainCode]float64{}, nil
	}
	out := make(map[domain.DomainCode]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out, nil
}
func (r *fakeScoreReader) GetByDate(_ context.Context, a domain.Asset, dom domain.DomainCode, d, _ time.Time) (float64, bool, error) {
	v, ok := r.byDate[skByDate(a, dom, d)]
	return v, ok, nil
}
func (r *fakeScoreReader) GetHistory(_ context.Context, a domain.Asset, dom domain.DomainCode, _ int) ([]float64, error) {
	return r.history[string(a)+"|"+string(dom)], nil
}

type fakeIndicatorReader struct {
	series map[domain.Asset][]domain.IndicatorPoint
}

func newFakeIndicatorReader() *fakeIndicatorReader {
	return &fakeIndicatorReader{series: map[domain.Asset][]domain.IndicatorPoint{}}
}
func (r *fakeIndicatorReader) GetIndicatorRawSeries(_ context.Context, a domain.Asset, _, _, _ time.Time) ([]domain.IndicatorPoint, error) {
	return r.series[a], nil
}

type fakePriceReader struct {
	prices map[domain.Asset][]PricePointAt
}

func newFakePriceReader() *fakePriceReader {
	return &fakePriceReader{prices: map[domain.Asset][]PricePointAt{}}
}
func (r *fakePriceReader) GetPriceHistory(_ context.Context, a domain.Asset, _, _ time.Time) ([]PricePointAt, error) {
	return r.prices[a], nil
}

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

func newSvc(t *testing.T) (*Service, *fakeRunRepo, *fakeStateRepo, *fakeMetricsRepo, *fakeScoreReader, *fakeIndicatorReader, *fakePriceReader) {
	t.Helper()
	runs := newFakeRunRepo()
	states := newFakeStateRepo()
	metrics := newFakeMetricsRepo()
	scores := newFakeScoreReader()
	ind := newFakeIndicatorReader()
	prices := newFakePriceReader()
	cfg := DefaultConfig()
	cfg.CodeSHA = "test-sha"
	engineCfg := regime.DefaultConfig()
	engineCfg.ModelVersion = "engine_v1.2.0_heuristic"
	engineCfg.ConfigVersion = "sha256:test"
	engineCfg.CodeSHA = "test-sha"
	engineCfg.SmoothingSpanDays = 0
	svc := NewService(runs, states, metrics, scores, ind, prices,
		fakeClock{t: time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)},
		cfg, engineCfg, "model_version: engine_v1.2.0_heuristic\n",
		[]domain.Asset{domain.AssetBTC},
	)
	return svc, runs, states, metrics, scores, ind, prices
}

func loadAllFiveDays(scores *fakeScoreReader, asset domain.Asset, days []time.Time) {
	for _, d := range days {
		scores.latest[skLatest(asset, d)] = map[domain.DomainCode]float64{
			domain.DomainLiquidity:    0.10,
			domain.DomainLeverage:     -0.05,
			domain.DomainMarketStress: 0.20,
			domain.DomainCapitalFlows: 0.15,
			domain.DomainVolatility:   -0.10,
		}
	}
}

func TestService_Replay_HappyPath_InsertsRunAndStates(t *testing.T) {
	svc, runs, states, _, scores, _, _ := newSvc(t)
	from := domain.MustParseDay("2026-04-10")
	to := domain.MustParseDay("2026-04-12")
	loadAllFiveDays(scores, domain.AssetBTC, []time.Time{from, from.AddDate(0, 0, 1), to})

	runID, err := svc.Replay(context.Background(), from, to, nil, "replay")
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if _, ok := runs.saved[runID]; !ok {
		t.Fatal("expected run row inserted")
	}
	got := states.byRun[runID]
	if len(got) != 3 {
		t.Fatalf("expected 3 state rows (1 asset * 3 days), got %d", len(got))
	}
	if got[0].ModelVersion != "engine_v1.2.0_heuristic" {
		t.Errorf("model_version: %v", got[0].ModelVersion)
	}
}

func TestService_Replay_PITCutoff_UsesSLAOffset(t *testing.T) {
	svc, _, _, _, scores, _, _ := newSvc(t)
	day := domain.MustParseDay("2026-04-10")
	loadAllFiveDays(scores, domain.AssetBTC, []time.Time{day})

	var capturedCutoff time.Time
	captured := &cutoffCapturingReader{inner: scores, captured: &capturedCutoff}
	svc.scores = captured

	if _, err := svc.Replay(context.Background(), day, day, nil, "replay"); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	wantCutoff := day.Add(time.Duration(svc.cfg.SLAOffsetMinutes) * time.Minute)
	if !capturedCutoff.Equal(wantCutoff) {
		t.Fatalf("cutoff: got %v, want %v (day+%dm)", capturedCutoff, wantCutoff, svc.cfg.SLAOffsetMinutes)
	}
}

func TestService_Replay_ByteEquivalentToEngine(t *testing.T) {
	svc, _, states, _, scores, _, _ := newSvc(t)
	day := domain.MustParseDay("2026-04-10")
	scoreMap := map[domain.DomainCode]float64{
		domain.DomainLiquidity:    0.20,
		domain.DomainLeverage:     -0.10,
		domain.DomainMarketStress: 0.30,
		domain.DomainCapitalFlows: 0.05,
		domain.DomainVolatility:   -0.20,
	}
	scores.latest[skLatest(domain.AssetBTC, day)] = scoreMap

	runID, err := svc.Replay(context.Background(), day, day, nil, "replay")
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	got := states.byRun[runID]
	if len(got) != 1 {
		t.Fatalf("expected 1 state, got %d", len(got))
	}

	present := regime.PresentSet(scoreMap)
	effW := regime.RedistributeWeights(svc.engineCfg.Weights, present)
	rawIndicator := regime.WeightedSumRaw(scoreMap, effW)
	normalized := regime.NormalizeScores(scoreMap, map[domain.DomainCode][]float64{}, svc.engineCfg.NormMinSamples)
	_, normIndicator, _ := regime.WeightedSumNormalized(normalized, effW)

	if math.Abs(got[0].RegimeIndicatorRaw-rawIndicator) > 1e-12 {
		t.Errorf("regime_indicator_raw: got %v, want %v", got[0].RegimeIndicatorRaw, rawIndicator)
	}
	if math.Abs(got[0].RegimeIndicator-normIndicator) > 1e-12 {
		t.Errorf("regime_indicator: got %v, want %v", got[0].RegimeIndicator, normIndicator)
	}
}

func TestService_ComputeMetrics_ForwardReturnMean(t *testing.T) {
	svc, _, states, metrics, _, _, prices := newSvc(t)
	runID, _ := svc.runs.Save(context.Background(), BacktestRun{
		Mode: "replay", PeriodStart: domain.MustParseDay("2026-01-01"),
		PeriodEnd: domain.MustParseDay("2026-01-31"),
	})

	for i := 0; i < 31; i++ {
		d := domain.MustParseDay("2026-01-01").AddDate(0, 0, i)
		states.byRun[runID] = append(states.byRun[runID], domain.RegimeState{
			Asset: domain.AssetBTC, ValueDate: d,
			RegimeIndicator: 0.5, TransitionRisk: 0.10,
		})
	}
	for i := 0; i < 31+91; i++ {
		d := domain.MustParseDay("2026-01-01").AddDate(0, 0, i)
		var p float64
		if i < 31 {
			p = 100
		} else {
			p = 200
		}
		prices.prices[domain.AssetBTC] = append(prices.prices[domain.AssetBTC], PricePointAt{Date: d, Price: p})
	}

	if err := svc.ComputeMetrics(context.Background(), runID); err != nil {
		t.Fatalf("ComputeMetrics: %v", err)
	}
	var foundMean float64
	for _, m := range metrics.byRun[runID] {
		if m.Name == "forward_return_mean" && m.Scope == "BTC|risk_on|h=90" {
			foundMean = m.Value
		}
	}
	if math.Abs(foundMean-math.Log(2)) > 1e-6 {
		t.Fatalf("forward_return_mean(risk_on,90): got %v, want ln(2)=%v", foundMean, math.Log(2))
	}
}

func TestService_ComputeMetrics_PersistenceFlipFlopRate(t *testing.T) {
	svc, _, states, metrics, _, _, _ := newSvc(t)
	runID, _ := svc.runs.Save(context.Background(), BacktestRun{
		Mode: "replay", PeriodStart: domain.MustParseDay("2026-01-01"),
		PeriodEnd: domain.MustParseDay("2026-01-10"),
	})
	for i := 0; i < 10; i++ {
		d := domain.MustParseDay("2026-01-01").AddDate(0, 0, i)
		states.byRun[runID] = append(states.byRun[runID], domain.RegimeState{
			Asset: domain.AssetBTC, ValueDate: d,
		})
	}
	if err := svc.ComputeMetrics(context.Background(), runID); err != nil {
		t.Fatalf("ComputeMetrics: %v", err)
	}
	gotMetric := false
	for _, m := range metrics.byRun[runID] {
		if m.Name == "flip_flop_rate" && m.Scope == "BTC" {
			gotMetric = true
		}
	}
	if !gotMetric {
		t.Fatal("expected flip_flop_rate metric written")
	}
}

func TestService_ComputeMetrics_ToyStrategyCalmar(t *testing.T) {
	svc, _, states, metrics, _, _, prices := newSvc(t)
	runID, _ := svc.runs.Save(context.Background(), BacktestRun{
		Mode: "replay", PeriodStart: domain.MustParseDay("2026-01-01"),
		PeriodEnd: domain.MustParseDay("2026-03-01"),
	})

	for i := 0; i < 60; i++ {
		d := domain.MustParseDay("2026-01-01").AddDate(0, 0, i)
		states.byRun[runID] = append(states.byRun[runID], domain.RegimeState{
			Asset: domain.AssetBTC, ValueDate: d,
			RegimeIndicator: 0.5, TransitionRisk: 0.05,
		})
		prices.prices[domain.AssetBTC] = append(prices.prices[domain.AssetBTC], PricePointAt{
			Date: d, Price: 100 + float64(i),
		})
	}
	if err := svc.ComputeMetrics(context.Background(), runID); err != nil {
		t.Fatalf("ComputeMetrics: %v", err)
	}
	calmar := math.NaN()
	for _, m := range metrics.byRun[runID] {
		if m.Name == "strategy_calmar" {
			calmar = m.Value
		}
	}

	if !(calmar == 0 || calmar > 0) {
		t.Fatalf("expected non-negative Calmar, got %v", calmar)
	}
}

func TestService_Sweep_BaselineThreeValues(t *testing.T) {
	svc, _, _, _, scores, _, prices := newSvc(t)
	from := domain.MustParseDay("2026-04-10")
	to := domain.MustParseDay("2026-04-12")
	loadAllFiveDays(scores, domain.AssetBTC, []time.Time{from, from.AddDate(0, 0, 1), to})
	prices.prices[domain.AssetBTC] = []PricePointAt{
		{Date: from, Price: 100}, {Date: from.AddDate(0, 0, 1), Price: 101},
		{Date: to, Price: 102},
	}
	runIDs, rows, err := svc.Sweep(context.Background(), from, to, "transition.baseline", []float64{0.4, 0.6, 0.8})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(runIDs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runIDs))
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 sensitivity rows, got %d", len(rows))
	}
}

func TestMetrics_BootstrapForwardReturnCI_BoundsAroundMean(t *testing.T) {
	days := []RegimeDay{}
	prices := map[time.Time]float64{}
	for i := 0; i < 60; i++ {
		d := domain.MustParseDay("2026-01-01").AddDate(0, 0, i)
		days = append(days, RegimeDay{ValueDate: d, Label: LabelRiskOn})
		prices[d] = 100
		prices[d.AddDate(0, 0, 90)] = 110
	}
	cells := BootstrapForwardReturnCI(days, prices, []int{90}, 1000)
	for _, c := range cells {
		if c.Label != LabelRiskOn || c.Horizon != 90 {
			continue
		}
		if c.Mean < c.MeanCILow || c.Mean > c.MeanCIHigh {
			t.Errorf("mean (%v) must lie in [%v, %v]", c.Mean, c.MeanCILow, c.MeanCIHigh)
		}
		if c.NResamples != 1000 {
			t.Errorf("n_resamples: %v", c.NResamples)
		}
	}
}

func TestService_Sweep_ChildRunsLinkToParent(t *testing.T) {
	svc, runs, _, _, scores, _, prices := newSvc(t)
	from := domain.MustParseDay("2026-04-10")
	to := domain.MustParseDay("2026-04-11")
	loadAllFiveDays(scores, domain.AssetBTC, []time.Time{from, to})
	prices.prices[domain.AssetBTC] = []PricePointAt{{Date: from, Price: 100}, {Date: to, Price: 101}}
	runIDs, _, err := svc.Sweep(context.Background(), from, to, "smoothing.span_days", []float64{0, 5})
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	seedID := runs.order[0]
	for _, rid := range runIDs {
		got := runs.saved[rid]
		if got.ParentRunID == nil || *got.ParentRunID != seedID {
			t.Errorf("run %s: parent_run_id = %v, want seed %s", rid, got.ParentRunID, seedID)
		}
	}
}

func TestService_Replay_ReproducibilityHashEquality(t *testing.T) {
	hashes := make([]string, 0, 2)
	for i := 0; i < 2; i++ {

		svc, runs, _, _, scores, _, _ := newSvc(t)
		day := domain.MustParseDay("2026-04-10")
		scores.latest[skLatest(domain.AssetBTC, day)] = map[domain.DomainCode]float64{
			domain.DomainLiquidity:    0.20,
			domain.DomainLeverage:     -0.10,
			domain.DomainMarketStress: 0.30,
			domain.DomainCapitalFlows: 0.05,
			domain.DomainVolatility:   -0.20,
		}
		runID, err := svc.Replay(context.Background(), day, day, nil, "replay")
		if err != nil {
			t.Fatalf("iter %d Replay: %v", i, err)
		}
		hashes = append(hashes, runs.saved[runID].DataSnapshotHash)
	}
	if hashes[0] != hashes[1] {
		t.Fatalf("hash mismatch across identical inputs: %s vs %s", hashes[0], hashes[1])
	}
}

func TestService_Replay_InsufficientCoverage_SkipsDay(t *testing.T) {
	svc, _, states, _, scores, _, _ := newSvc(t)
	day := domain.MustParseDay("2026-04-10")

	scores.latest[skLatest(domain.AssetBTC, day)] = map[domain.DomainCode]float64{
		domain.DomainLiquidity: 0.20,
		domain.DomainLeverage:  -0.10,
	}
	runID, err := svc.Replay(context.Background(), day, day, nil, "replay")
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(states.byRun[runID]) != 0 {
		t.Fatalf("expected 0 state rows on coverage failure, got %d", len(states.byRun[runID]))
	}
}

func TestPipeline_SnapshotHash_Deterministic(t *testing.T) {
	day := domain.MustParseDay("2026-04-10")
	in := map[SnapshotKey]map[domain.DomainCode]float64{
		{Asset: domain.AssetBTC, ValueDate: day}: {
			domain.DomainLiquidity: 0.10,
			domain.DomainLeverage:  -0.05,
		},
	}
	a := SnapshotHash(in)
	b := SnapshotHash(in)
	if a != b {
		t.Fatalf("non-deterministic: %s vs %s", a, b)
	}
	in2 := map[SnapshotKey]map[domain.DomainCode]float64{
		{Asset: domain.AssetBTC, ValueDate: day}: {
			domain.DomainLiquidity: 0.10001,
			domain.DomainLeverage:  -0.05,
		},
	}
	if SnapshotHash(in2) == a {
		t.Fatal("hash collision under perturbation")
	}
}

type cutoffCapturingReader struct {
	inner    DomainScoreReader
	captured *time.Time
}

func (r *cutoffCapturingReader) GetLatestAll(ctx context.Context, a domain.Asset, d, c time.Time) (map[domain.DomainCode]float64, error) {
	*r.captured = c
	return r.inner.GetLatestAll(ctx, a, d, c)
}
func (r *cutoffCapturingReader) GetByDate(ctx context.Context, a domain.Asset, dom domain.DomainCode, d, c time.Time) (float64, bool, error) {
	return r.inner.GetByDate(ctx, a, dom, d, c)
}
func (r *cutoffCapturingReader) GetHistory(ctx context.Context, a domain.Asset, dom domain.DomainCode, n int) ([]float64, error) {
	return r.inner.GetHistory(ctx, a, dom, n)
}

var _ = errors.Is
