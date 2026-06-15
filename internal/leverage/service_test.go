package leverage

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/leverage/features"
)

type fakeFeatureRepo struct {
	saved   []domain.Feature
	latest  map[string]float64
	series  map[string][]float64
	saveErr error
}

func newFakeFeatureRepo() *fakeFeatureRepo {
	return &fakeFeatureRepo{
		latest: map[string]float64{},
		series: map[string][]float64{},
	}
}

func featKey(k domain.FeatureKey, asset domain.Asset, date time.Time) string {
	return k.Name + "|" + k.Version + "|" + string(asset) + "|" + date.Format("2006-01-02")
}

func (f *fakeFeatureRepo) Save(ctx context.Context, x domain.Feature) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, x)
	f.latest[featKey(x.Key, x.Asset, x.ValueDate)] = x.Value
	return nil
}

func (f *fakeFeatureRepo) GetLatest(ctx context.Context, k domain.FeatureKey, a domain.Asset, d, _ time.Time) (float64, error) {
	v, ok := f.latest[featKey(k, a, d)]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (f *fakeFeatureRepo) GetSeries(ctx context.Context, k domain.FeatureKey, a domain.Asset, _, _, _ time.Time) ([]float64, error) {
	return f.series[k.Name+"|"+k.Version+"|"+string(a)], nil
}

type fakeScoreRepo struct{ saved []domain.DomainScore }

func (s *fakeScoreRepo) Save(ctx context.Context, x domain.DomainScore) error {
	s.saved = append(s.saved, x)
	return nil
}

type fakePublisher struct{ events []domain.Event }

func (p *fakePublisher) Publish(ctx context.Context, ev domain.Event) error {
	p.events = append(p.events, ev)
	return nil
}

type fakeRawRepo struct {
	oi        map[domain.Asset]float64
	oiAvail   map[domain.Asset]bool
	funding   map[domain.Asset]float64
	fundingOK map[domain.Asset]bool
	liq       map[domain.Asset]float64
	liqOK     map[domain.Asset]bool
	cgBasis   map[string]float64
	cgBasisOK map[string]bool
	deribit   map[domain.Asset]float64
	deribitOK map[domain.Asset]bool
	mcap      map[string]float64
}

func newFakeRawRepo() *fakeRawRepo {
	return &fakeRawRepo{
		oi:        map[domain.Asset]float64{},
		oiAvail:   map[domain.Asset]bool{},
		funding:   map[domain.Asset]float64{},
		fundingOK: map[domain.Asset]bool{},
		liq:       map[domain.Asset]float64{},
		liqOK:     map[domain.Asset]bool{},
		cgBasis:   map[string]float64{},
		cgBasisOK: map[string]bool{},
		deribit:   map[domain.Asset]float64{},
		deribitOK: map[domain.Asset]bool{},
		mcap:      map[string]float64{},
	}
}

func (r *fakeRawRepo) SaveExchangeOI(_ context.Context, _ []ExchangeOIRow) error { return nil }
func (r *fakeRawRepo) SaveExchangeFunding(_ context.Context, _ []ExchangeFundingRow) error {
	return nil
}
func (r *fakeRawRepo) SaveCoinglassFuturesBasis(_ context.Context, _ []CoinglassFuturesBasisRow) error {
	return nil
}
func (r *fakeRawRepo) SaveDeribitBasis(_ context.Context, _ DeribitBasisRow) error { return nil }
func (r *fakeRawRepo) SaveExchangeLiquidations(_ context.Context, _ []ExchangeLiquidationsRow) error {
	return nil
}
func (r *fakeRawRepo) AggregatedOIAsOf(_ context.Context, a domain.Asset, _, _ time.Time) (float64, bool, error) {
	return r.oi[a], r.oiAvail[a], nil
}
func (r *fakeRawRepo) CoinglassAggregatedOIAsOf(_ context.Context, _ domain.Asset, _, _ time.Time) (float64, bool, error) {

	return 0, false, nil
}
func (r *fakeRawRepo) DailyAvgFundingAsOf(_ context.Context, a domain.Asset, _, _ time.Time) (float64, bool, error) {
	return r.funding[a], r.fundingOK[a], nil
}
func (r *fakeRawRepo) DailyTotalLiquidationsAsOf(_ context.Context, a domain.Asset, _, _ time.Time) (float64, bool, error) {
	return r.liq[a], r.liqOK[a], nil
}
func (r *fakeRawRepo) CoinglassAggregatedLiquidationsAsOf(_ context.Context, _ domain.Asset, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetCoinglassFuturesBasisAsOf(_ context.Context, sym, exch string, _, _ time.Time) (float64, bool, error) {
	k := sym + "|" + exch
	return r.cgBasis[k], r.cgBasisOK[k], nil
}
func (r *fakeRawRepo) GetDeribitBasisAsOf(_ context.Context, a domain.Asset, _, _ time.Time) (float64, bool, error) {
	return r.deribit[a], r.deribitOK[a], nil
}
func (r *fakeRawRepo) GetMarketCapAsOf(_ context.Context, coinID string, _, _ time.Time) (float64, error) {
	if v, ok := r.mcap[coinID]; ok {
		return v, nil
	}
	return 0, domain.ErrNotFound
}

func (r *fakeRawRepo) SaveCoinglassLSRatio(_ context.Context, _ LSRatioKind, _ []CoinglassLSRatioRow) error {
	return nil
}
func (r *fakeRawRepo) SaveCoinglassTakerVolume(_ context.Context, _ []CoinglassTakerVolumeRow) error {
	return nil
}
func (r *fakeRawRepo) SaveCoinglassBorrowRate(_ context.Context, _ []CoinglassBorrowRateRow) error {
	return nil
}
func (r *fakeRawRepo) CoinglassLSRatioAvgAsOf(_ context.Context, _ LSRatioKind, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) CoinglassTakerVolumeAsOf(_ context.Context, _ string, _, _ time.Time) (float64, float64, bool, error) {
	return 0, 0, false, nil
}

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

func TestComputeScore_HappyPath_RiskOnSignals(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),

		OIPercentile: 0.30, OIPercentileAvailable: true,
		FundingZ: 1.0, FundingZAvailable: true,
		BasisZ: 0.8, BasisZAvailable: true,

		LiqZ: -0.5, LiqZAvailable: true,

		OIChangeZ: -0.5, OIChangeZAvailable: true,

		PositionSkewZ: 0.2, PositionSkewZAvailable: true,
		CrowdDivergenceZ: 0.5, CrowdDivergenceZAvailable: true,
		TakerAggressionZ: 0.6, TakerAggressionZAvailable: true,
	}
	cfg := Config{ModelVersion: "leverage_v0.4.0", ConfigVersion: "sha256:test", CodeSHA: "abc"}
	s := computeScore(in, cfg)
	if s.Score <= 0 {
		t.Fatalf("expected positive (risk-on) score, got %v", s.Score)
	}
	if s.Score > 1 || s.Score < -1 {
		t.Fatalf("score out of range: %v", s.Score)
	}
	if s.Domain != domain.DomainLeverage {
		t.Fatalf("wrong domain: %v", s.Domain)
	}
	if s.DataQuality["partial_coverage"] != false {
		t.Fatalf("expected partial_coverage=false, got %v", s.DataQuality["partial_coverage"])
	}
}

func TestComputeScore_ScoreClampedToOne(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),

		OIPercentile: 0, OIPercentileAvailable: true,
		FundingZ: 100, FundingZAvailable: true,
		BasisZ: 100, BasisZAvailable: true,
		LiqZ: -100, LiqZAvailable: true,
		OIChangeZ: -100, OIChangeZAvailable: true,
	}

	cfg := Config{
		ModelVersion:   "x_v1.0.0",
		WeightSize:     2.0,
		WeightFunding:  2.0,
		WeightBasis:    2.0,
		WeightLiq:      2.0,
		WeightMomentum: 2.0,
	}
	s := computeScore(in, cfg)
	if math.Abs(s.Score-1.0) > 1e-9 {
		t.Fatalf("expected score==1 when oversaturated by inflated weights, got %v", s.Score)
	}
}

func TestComputeScore_AllMissing_ScoreZeroPartialCoverageTrue(t *testing.T) {
	in := ScoreInputs{Asset: domain.AssetBTC, ValueDate: domain.MustParseDay("2026-04-22")}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	if math.Abs(s.Score) > 1e-9 {
		t.Fatalf("expected zero score with no inputs, got %v", s.Score)
	}
	if s.DataQuality["partial_coverage"] != true {
		t.Fatalf("expected partial_coverage=true")
	}
}

func TestComputeScore_NaNFunding_FlagsUnavailable(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),
		FundingZ:  math.NaN(), FundingZAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	if c := s.Components["component_funding"]; c != 0 {
		t.Fatalf("expected NaN z to skip funding component, got %v", c)
	}
}

func TestService_GatherScoreInputs_AllPresent(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{
		IntermediateVersion: "intermediates_v1",
		FinalVersion:        "final_v1",
	}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, newFakeRawRepo(),
		nil, nil, nil, nil, nil, nil, nil,
		&fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)

	day := domain.MustParseDay("2026-04-22")
	feats.latest[featKey(domain.FeatureKey{Name: features.OIMcapPercentile365dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = 0.42
	feats.latest[featKey(domain.FeatureKey{Name: features.FundingRateZScore90dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = 1.1
	feats.latest[featKey(domain.FeatureKey{Name: features.Basis3mZScore90dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = 0.6
	feats.latest[featKey(domain.FeatureKey{Name: features.LiquidationsStress60dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = -0.3
	feats.latest[featKey(domain.FeatureKey{Name: features.OIChangeZScore30d180dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = -0.4

	in, err := s.gatherScoreInputs(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if !in.OIPercentileAvailable || in.OIPercentile != 0.42 {
		t.Errorf("OIPercentile: got %v %v", in.OIPercentile, in.OIPercentileAvailable)
	}
	if !in.FundingZAvailable || in.FundingZ != 1.1 {
		t.Errorf("FundingZ: got %v %v", in.FundingZ, in.FundingZAvailable)
	}
	if !in.BasisZAvailable || in.BasisZ != 0.6 {
		t.Errorf("BasisZ: got %v %v", in.BasisZ, in.BasisZAvailable)
	}
	if !in.LiqZAvailable || in.LiqZ != -0.3 {
		t.Errorf("LiqZ: got %v %v", in.LiqZ, in.LiqZAvailable)
	}
	if !in.OIChangeZAvailable || in.OIChangeZ != -0.4 {
		t.Errorf("OIChangeZ: got %v %v", in.OIChangeZ, in.OIChangeZAvailable)
	}

	if len(in.FeatureCodesUsed) != 8 {
		t.Errorf("expected 8 feature codes, got %v", in.FeatureCodesUsed)
	}
}

func TestService_GatherScoreInputs_MissingFeaturesGracefullyDegrade(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{IntermediateVersion: "v1", FinalVersion: "v1"}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, newFakeRawRepo(),
		nil, nil, nil, nil, nil, nil, nil,
		&fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)

	day := domain.MustParseDay("2026-04-22")
	in, err := s.gatherScoreInputs(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if in.OIPercentileAvailable || in.FundingZAvailable ||
		in.BasisZAvailable || in.LiqZAvailable || in.OIChangeZAvailable {
		t.Fatal("expected all unavailable")
	}
}

func TestService_GatherScoreInputs_UnexpectedRepoErrorBubblesUp(t *testing.T) {
	feats := &erroringFeatureRepo{err: errors.New("connection lost")}
	cfg := Config{IntermediateVersion: "v1", FinalVersion: "v1"}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, newFakeRawRepo(),
		nil, nil, nil, nil, nil, nil, nil,
		&fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)
	_, err := s.gatherScoreInputs(context.Background(), domain.AssetBTC, time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected error to propagate, got nil")
	}
}

type erroringFeatureRepo struct{ err error }

func (e *erroringFeatureRepo) Save(_ context.Context, _ domain.Feature) error { return nil }
func (e *erroringFeatureRepo) GetLatest(_ context.Context, _ domain.FeatureKey, _ domain.Asset, _, _ time.Time) (float64, error) {
	return 0, e.err
}
func (e *erroringFeatureRepo) GetSeries(_ context.Context, _ domain.FeatureKey, _ domain.Asset, _, _, _ time.Time) ([]float64, error) {
	return nil, e.err
}

func TestPctChange_BasicAndZero(t *testing.T) {
	if v, ok := features.PctChange(110, 100); !ok || math.Abs(v-0.1) > 1e-12 {
		t.Errorf("PctChange(110,100): got %v %v", v, ok)
	}
	if _, ok := features.PctChange(110, 0); ok {
		t.Errorf("PctChange divide-by-zero should return ok=false")
	}
	if _, ok := features.PctChange(math.NaN(), 100); ok {
		t.Errorf("PctChange NaN should return ok=false")
	}
}

func TestPercentileRank_Basic(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	if v, ok := features.PercentileRank(3, xs); !ok || math.Abs(v-0.6) > 1e-12 {
		t.Errorf("PercentileRank(3, 1..5): got %v %v (want 0.6)", v, ok)
	}
	if _, ok := features.PercentileRank(0, []float64{}); ok {
		t.Errorf("PercentileRank empty should return ok=false")
	}
}

func TestAnnualizedBasisFromFunding_KnownValue(t *testing.T) {

	got := features.AnnualizedBasisFromFunding(0.0001)
	if math.Abs(got-10.95) > 1e-9 {
		t.Errorf("AnnualizedBasisFromFunding(0.0001)=%v, want 10.95", got)
	}
}

func TestComputeOIPercentile_RatioSeriesPath(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{IntermediateVersion: "v1", FinalVersion: "fv1"}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, newFakeRawRepo(),
		nil, nil, nil, nil, nil, nil, nil,
		&fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)

	ratios := make([]float64, 50)
	for i := range ratios {
		ratios[i] = float64(i) / 49.0
	}
	oiMcapKey := domain.FeatureKey{Name: features.OIMcapRatioName, Version: "v1"}
	feats.series[oiMcapKey.Name+"|"+oiMcapKey.Version+"|BTC"] = ratios
	day := domain.MustParseDay("2026-04-22")
	pct, ok, err := s.computeOIPercentile(context.Background(), domain.AssetBTC, day, time.Now(),
		oiMcapKey, domain.FeatureKey{Name: features.OIUsdRawName, Version: "v1"}, 0, false)
	if err != nil {
		t.Fatalf("computeOIPercentile err: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true with 50 obs >= minObs=30")
	}
	if pct < 0.99 {
		t.Errorf("expected percentile ~1.0, got %v", pct)
	}
}
