package regime

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"marketengine/internal/domain"
)

type fakeRepo struct {
	saved              []domain.RegimeState
	latestByAsset      map[domain.Asset]domain.RegimeState
	byDate             map[string]domain.RegimeState
	historyByAsset     map[domain.Asset][]domain.RegimeState
	indicatorRawSeries map[domain.Asset][]domain.IndicatorPoint
	smoothingTrailing  map[domain.Asset][]SmoothingTrailingPoint
	saveErr            error
	getByDateErr       error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		latestByAsset:      map[domain.Asset]domain.RegimeState{},
		byDate:             map[string]domain.RegimeState{},
		historyByAsset:     map[domain.Asset][]domain.RegimeState{},
		indicatorRawSeries: map[domain.Asset][]domain.IndicatorPoint{},
		smoothingTrailing:  map[domain.Asset][]SmoothingTrailingPoint{},
	}
}

func dateKey(a domain.Asset, d time.Time) string {
	return string(a) + "|" + d.Format("2006-01-02")
}

func (r *fakeRepo) Save(_ context.Context, s domain.RegimeState) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = append(r.saved, s)
	r.latestByAsset[s.Asset] = s
	r.byDate[dateKey(s.Asset, s.ValueDate)] = s
	return nil
}
func (r *fakeRepo) GetLatest(_ context.Context, a domain.Asset) (domain.RegimeState, error) {
	v, ok := r.latestByAsset[a]
	if !ok {
		return domain.RegimeState{}, domain.ErrNotFound
	}
	return v, nil
}
func (r *fakeRepo) GetByDate(_ context.Context, a domain.Asset, d time.Time) (domain.RegimeState, error) {
	if r.getByDateErr != nil {
		return domain.RegimeState{}, r.getByDateErr
	}
	v, ok := r.byDate[dateKey(a, d)]
	if !ok {
		return domain.RegimeState{}, domain.ErrNotFound
	}
	return v, nil
}
func (r *fakeRepo) GetHistory(_ context.Context, a domain.Asset, _, _ time.Time) ([]domain.RegimeState, error) {
	return r.historyByAsset[a], nil
}
func (r *fakeRepo) GetIndicatorRawSeries(_ context.Context, a domain.Asset, _, _ time.Time) ([]domain.IndicatorPoint, error) {
	return r.indicatorRawSeries[a], nil
}
func (r *fakeRepo) GetSmoothingTrailing(_ context.Context, a domain.Asset, _, _, _ time.Time) ([]SmoothingTrailingPoint, error) {
	return r.smoothingTrailing[a], nil
}

type fakeReader struct {
	latest  map[domain.Asset]map[domain.DomainCode]float64
	byDate  map[string]float64
	history map[string][]float64
	err     error
}

func newFakeReader() *fakeReader {
	return &fakeReader{
		latest:  map[domain.Asset]map[domain.DomainCode]float64{},
		byDate:  map[string]float64{},
		history: map[string][]float64{},
	}
}

func readerKey(a domain.Asset, d domain.DomainCode, t time.Time) string {
	return string(a) + "|" + string(d) + "|" + t.Format("2006-01-02")
}

func (r *fakeReader) GetLatestAll(_ context.Context, a domain.Asset, _, _ time.Time) (map[domain.DomainCode]float64, error) {
	if r.err != nil {
		return nil, r.err
	}
	if m, ok := r.latest[a]; ok {
		out := make(map[domain.DomainCode]float64, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out, nil
	}
	return map[domain.DomainCode]float64{}, nil
}
func (r *fakeReader) GetByDate(_ context.Context, a domain.Asset, d domain.DomainCode, t, _ time.Time) (float64, bool, error) {
	if r.err != nil {
		return 0, false, r.err
	}
	v, ok := r.byDate[readerKey(a, d, t)]
	return v, ok, nil
}
func (r *fakeReader) GetHistory(_ context.Context, a domain.Asset, d domain.DomainCode, _ int) ([]float64, error) {
	return r.history[string(a)+"|"+string(d)], nil
}

type fakePublisher struct{ events []domain.Event }

func (p *fakePublisher) Publish(_ context.Context, ev domain.Event) error {
	p.events = append(p.events, ev)
	return nil
}

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

func newSvc(t *testing.T, repo *fakeRepo, reader *fakeReader, pub *fakePublisher, cfg Config) *Service {
	t.Helper()
	cfg.Defaults()
	if cfg.ModelVersion == "" {
		cfg.ModelVersion = "engine_v1.2.0_heuristic"
	}
	return NewService(repo, reader, pub, fakeClock{t: time.Now().UTC()}, cfg, []domain.Asset{domain.AssetBTC})
}

func allFiveScores(v float64) map[domain.DomainCode]float64 {
	out := make(map[domain.DomainCode]float64, 5)
	for _, d := range domain.AllDomains() {
		out[d] = v
	}
	return out
}

func TestService_Compute_HappyPath_AllFiveDomainsPresent(t *testing.T) {
	repo := newFakeRepo()
	reader := newFakeReader()
	pub := &fakePublisher{}
	day := domain.MustParseDay("2026-04-22")

	reader.latest[domain.AssetBTC] = map[domain.DomainCode]float64{
		domain.DomainLiquidity:    0.4,
		domain.DomainLeverage:     -0.2,
		domain.DomainMarketStress: 0.1,
		domain.DomainCapitalFlows: 0.5,
		domain.DomainVolatility:   -0.3,
	}
	svc := newSvc(t, repo, reader, pub, Config{})

	if err := svc.Compute(context.Background(), domain.AssetBTC, day); err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 saved row, got %d", len(repo.saved))
	}
	got := repo.saved[0]
	if got.Asset != domain.AssetBTC {
		t.Errorf("asset mismatch: %v", got.Asset)
	}
	if !got.ValueDate.Equal(day) {
		t.Errorf("date mismatch: %v", got.ValueDate)
	}
	if len(got.DomainContributions) != 5 {
		t.Errorf("contributions: got %d, want 5", len(got.DomainContributions))
	}
	if got.ModelVersion != "engine_v1.2.0_heuristic" {
		t.Errorf("model_version: %v", got.ModelVersion)
	}
	if len(pub.events) != 1 || pub.events[0].Topic != "regime.state.completed.v1" {
		t.Errorf("publish: %+v", pub.events)
	}
	for _, dom := range domain.AllDomains() {
		if !got.FeatureCoverageFlag[dom] {
			t.Errorf("coverage flag %s: false", dom)
		}
	}
}

func TestService_Compute_InsufficientCoverage_ReturnsErr(t *testing.T) {
	repo := newFakeRepo()
	reader := newFakeReader()
	day := domain.MustParseDay("2026-04-22")

	reader.latest[domain.AssetBTC] = map[domain.DomainCode]float64{
		domain.DomainLiquidity: 0.4,
		domain.DomainLeverage:  -0.2,
	}
	svc := newSvc(t, repo, reader, &fakePublisher{}, Config{})

	err := svc.Compute(context.Background(), domain.AssetBTC, day)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrInsufficientCoverage) {
		t.Fatalf("expected ErrInsufficientCoverage, got %v", err)
	}
	if len(repo.saved) != 0 {
		t.Fatalf("nothing should be saved on coverage failure, got %d", len(repo.saved))
	}
}

func TestService_Compute_WeightRedistribution_EffectiveWeightsSumToOne(t *testing.T) {
	repo := newFakeRepo()
	reader := newFakeReader()
	day := domain.MustParseDay("2026-04-22")

	reader.latest[domain.AssetBTC] = map[domain.DomainCode]float64{
		domain.DomainLiquidity:    0.2,
		domain.DomainLeverage:     0.1,
		domain.DomainMarketStress: -0.1,
		domain.DomainCapitalFlows: 0.0,
	}
	svc := newSvc(t, repo, reader, &fakePublisher{}, Config{})

	if err := svc.Compute(context.Background(), domain.AssetBTC, day); err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 saved row, got %d", len(repo.saved))
	}
	got := repo.saved[0]
	if _, ok := got.EffectiveWeights[domain.DomainVolatility]; ok {
		t.Errorf("missing domain should not appear in effective weights")
	}
	var sum float64
	for _, w := range got.EffectiveWeights {
		sum += w
	}
	if math.Abs(sum-1.0) > 1e-9 {
		t.Fatalf("effective weights sum: got %.12f, want 1.0", sum)
	}
}

func TestService_GetLatest_ReturnsSavedRow(t *testing.T) {
	repo := newFakeRepo()
	reader := newFakeReader()
	svc := newSvc(t, repo, reader, &fakePublisher{}, Config{})

	want := domain.RegimeState{
		Asset: domain.AssetBTC, ValueDate: domain.MustParseDay("2026-04-22"),
		RegimeIndicator: 0.42, ModelVersion: "engine_v1.2.0_heuristic",
	}
	repo.latestByAsset[domain.AssetBTC] = want

	got, err := svc.GetLatest(context.Background(), domain.AssetBTC)
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}
	if got.RegimeIndicator != 0.42 {
		t.Errorf("indicator: got %v, want 0.42", got.RegimeIndicator)
	}
}

func TestService_GetByDate_NotFound_PropagatesDomainErrNotFound(t *testing.T) {
	repo := newFakeRepo()
	reader := newFakeReader()
	svc := newSvc(t, repo, reader, &fakePublisher{}, Config{})

	_, err := svc.GetByDate(context.Background(), domain.AssetBTC, domain.MustParseDay("2026-04-22"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPipeline_TransitionRisk_BaselineSubtractionClipsBelowZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TransitionBaseline = 0.50

	in := TransitionRiskInputs{
		NormalizedScores: map[domain.DomainCode]float64{
			domain.DomainLiquidity: 0.0,
			domain.DomainLeverage:  0.0,
		},

		MomentumChecked:       false,
		MomentumSameDirection: false,
	}
	got := computeTransitionRisk(in, cfg)
	if got != 0 {
		t.Fatalf("expected tr=0 below baseline, got %v", got)
	}

	in2 := TransitionRiskInputs{
		NormalizedScores: map[domain.DomainCode]float64{
			domain.DomainLiquidity:    -3.0,
			domain.DomainLeverage:     3.0,
			domain.DomainMarketStress: -3.0,
			domain.DomainCapitalFlows: 3.0,
			domain.DomainVolatility:   -3.0,
		},
		RocPerDomain: []float64{10.0, 10.0, 10.0, 10.0, 10.0},
	}
	got2 := computeTransitionRisk(in2, cfg)

	if got2 < 0.9 || got2 > 1.0001 {
		t.Fatalf("expected tr in [0.9, 1.0] with saturated z, got %v", got2)
	}
}

func TestPipeline_Smoothing_Span21VsSpan0Identity(t *testing.T) {
	prev := []float64{0.10, 0.12, 0.13, 0.15, 0.18, 0.20, 0.22, 0.25, 0.28, 0.30}
	raw := 0.50

	id := emaSmooth(raw, prev, 0)
	if id != raw {
		t.Fatalf("span=0 must be identity: got %v, want %v", id, raw)
	}

	id1 := emaSmooth(raw, prev, 1)
	if id1 != raw {
		t.Fatalf("span=1 must be identity: got %v, want %v", id1, raw)
	}

	smoothed := emaSmooth(raw, prev, 21)
	if smoothed >= raw {
		t.Fatalf("span=21 should pull raw toward prev mean; got %v >= raw=%v", smoothed, raw)
	}
	if smoothed <= 0.10 {
		t.Fatalf("span=21 should still be above min(prev); got %v", smoothed)
	}
}

func TestPipeline_TopDrivers_SortedAndShareSumsToOne(t *testing.T) {
	contribs := map[domain.DomainCode]float64{
		domain.DomainLiquidity:    0.10,
		domain.DomainLeverage:     -0.40,
		domain.DomainMarketStress: 0.05,
		domain.DomainCapitalFlows: 0.20,
		domain.DomainVolatility:   -0.25,
	}
	drivers := buildTopDrivers(contribs)
	if len(drivers) != 5 {
		t.Fatalf("expected 5 drivers, got %d", len(drivers))
	}

	expected := []domain.DomainCode{
		domain.DomainLeverage,
		domain.DomainVolatility,
		domain.DomainCapitalFlows,
		domain.DomainLiquidity,
		domain.DomainMarketStress,
	}
	for i, e := range expected {
		if drivers[i].Domain != e {
			t.Errorf("position %d: got %v, want %v", i, drivers[i].Domain, e)
		}
	}
	var shareSum float64
	for _, d := range drivers {
		shareSum += d.Share
	}
	if math.Abs(shareSum-1.0) > 1e-9 {
		t.Fatalf("share sum: got %v, want 1.0", shareSum)
	}
}

func TestPipeline_RedistributeWeights_PreservesRatios(t *testing.T) {
	nominal := map[domain.DomainCode]float64{
		domain.DomainLiquidity:    0.25,
		domain.DomainLeverage:     0.20,
		domain.DomainMarketStress: 0.20,
		domain.DomainCapitalFlows: 0.20,
		domain.DomainVolatility:   0.15,
	}
	present := map[domain.DomainCode]bool{
		domain.DomainLiquidity:    true,
		domain.DomainLeverage:     true,
		domain.DomainMarketStress: true,
		domain.DomainCapitalFlows: true,
		domain.DomainVolatility:   false,
	}
	eff := redistributeWeights(nominal, present)
	if _, ok := eff[domain.DomainVolatility]; ok {
		t.Errorf("absent domain should not be in effective weights")
	}
	var sum float64
	for _, w := range eff {
		sum += w
	}
	if math.Abs(sum-1.0) > 1e-12 {
		t.Errorf("sum != 1.0: %v", sum)
	}

	ratio := eff[domain.DomainLiquidity] / eff[domain.DomainLeverage]
	if math.Abs(ratio-1.25) > 1e-12 {
		t.Errorf("ratio liquidity/leverage: got %v, want 1.25", ratio)
	}
}

func TestPipeline_FinalProbabilities_DiscountsByTransitionRisk(t *testing.T) {
	indicator := 0.0
	on, off := finalProbabilities(indicator, 0.0, 1.5)
	if math.Abs(on-0.5) > 1e-12 || math.Abs(off-0.5) > 1e-12 {
		t.Fatalf("tr=0, indicator=0: got on=%v off=%v, want 0.5/0.5", on, off)
	}
	on2, off2 := finalProbabilities(indicator, 0.4, 1.5)

	if math.Abs(on2-0.3) > 1e-12 || math.Abs(off2-0.3) > 1e-12 {
		t.Fatalf("tr=0.4, indicator=0: got on=%v off=%v, want 0.3/0.3", on2, off2)
	}
}
