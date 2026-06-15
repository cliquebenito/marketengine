package marketstress

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/marketstress/features"
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

func (f *fakeFeatureRepo) GetLatest(_ context.Context, k domain.FeatureKey, a domain.Asset, d, _ time.Time) (float64, error) {
	v, ok := f.latest[featKey(k, a, d)]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (f *fakeFeatureRepo) GetSeries(_ context.Context, k domain.FeatureKey, a domain.Asset, _, _, _ time.Time) ([]float64, error) {
	return f.series[k.Name+"|"+k.Version+"|"+string(a)], nil
}

type fakeScoreRepo struct{ saved []domain.DomainScore }

func (s *fakeScoreRepo) Save(_ context.Context, x domain.DomainScore) error {
	s.saved = append(s.saved, x)
	return nil
}

type fakePublisher struct{ events []domain.Event }

func (p *fakePublisher) Publish(_ context.Context, ev domain.Event) error {
	p.events = append(p.events, ev)
	return nil
}

type fakeRawRepo struct {
	binanceSeries map[string][]float64
	binanceClose  map[string]float64
	krakenClose   map[string]float64
	coinbaseClose map[string]float64
	coinglassRate *float64
	missingErr    error
}

func newFakeRawRepo() *fakeRawRepo {
	return &fakeRawRepo{
		binanceSeries: map[string][]float64{},
		binanceClose:  map[string]float64{},
		krakenClose:   map[string]float64{},
		coinbaseClose: map[string]float64{},
	}
}

func (r *fakeRawRepo) SaveBinanceKlines(_ context.Context, _ []BinanceKlineRow) error     { return nil }
func (r *fakeRawRepo) SaveKrakenOHLC(_ context.Context, _ []KrakenOHLCRow) error          { return nil }
func (r *fakeRawRepo) SaveCoinbaseCandles(_ context.Context, _ []CoinbaseCandleRow) error { return nil }
func (r *fakeRawRepo) SaveCoinglassCoinbasePremium(_ context.Context, _ []CoinglassCoinbasePremiumRow) error {
	return nil
}
func (r *fakeRawRepo) GetBinanceKlineCloseSeries(_ context.Context, sym string, _, _, _ time.Time) ([]float64, error) {
	return r.binanceSeries[sym], nil
}
func (r *fakeRawRepo) GetBinanceKlineCloseAsOf(_ context.Context, sym string, _, _ time.Time) (float64, error) {
	if v, ok := r.binanceClose[sym]; ok {
		return v, nil
	}
	if r.missingErr != nil {
		return 0, r.missingErr
	}
	return 0, domain.ErrNotFound
}
func (r *fakeRawRepo) GetKrakenCloseAsOf(_ context.Context, pair string, _, _ time.Time) (float64, error) {
	if v, ok := r.krakenClose[pair]; ok {
		return v, nil
	}
	return 0, domain.ErrNotFound
}
func (r *fakeRawRepo) GetCoinbaseCloseAsOf(_ context.Context, productID string, _, _ time.Time) (float64, error) {
	if v, ok := r.coinbaseClose[productID]; ok {
		return v, nil
	}
	return 0, domain.ErrNotFound
}
func (r *fakeRawRepo) GetCoinglassCoinbasePremiumRateAsOf(_ context.Context, _, _ time.Time) (float64, error) {
	if r.coinglassRate != nil {
		return *r.coinglassRate, nil
	}
	return 0, domain.ErrNotFound
}

func (r *fakeRawRepo) SaveCoinglassOrderbookAggregated(_ context.Context, _ []CoinglassOrderbookRow) error {
	return nil
}
func (r *fakeRawRepo) SaveCoinglassFuturesSpotVolRatio(_ context.Context, _ []CoinglassFuturesSpotVolRatioRow) error {
	return nil
}
func (r *fakeRawRepo) GetOrderbookImbalanceAsOf(_ context.Context, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetFuturesSpotRatioAsOf(_ context.Context, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}

type fakeLeverageReader struct {
	basis map[string]float64
	err   error
}

func leverageKey(asset domain.Asset, date time.Time) string {
	return string(asset) + "|" + date.Format("2006-01-02")
}

func (f *fakeLeverageReader) GetBasis3mDailyAnyVersion(_ context.Context, asset domain.Asset, valueDate, _ time.Time) (float64, error) {
	if f.err != nil {
		return 0, f.err
	}
	v, ok := f.basis[leverageKey(asset, valueDate)]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

func newServiceForTests(feats FeatureRepo, raws RawRepo, lev LeverageFeatureReader, cfg Config) *Service {
	cfg.Defaults()
	return NewService(feats, &fakeScoreRepo{}, raws,
		nil, nil, nil, nil, nil, lev,
		&fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)
}

func TestComputeScore_HappyPath_RiskOnSignals(t *testing.T) {

	in := ScoreInputs{
		Asset:        domain.AssetBTC,
		ValueDate:    domain.MustParseDay("2026-04-22"),
		ZCorrelation: -1.0, ZCorrelationAvailable: true,
		ZPeg: -0.6, ZPegAvailable: true,
		ZCoinbase: -0.4, ZCoinbaseAvailable: true,
		ZBasis: -1.2, ZBasisAvailable: true,
	}
	cfg := Config{ModelVersion: "market_stress_v0.1.0", ConfigVersion: "sha256:test", CodeSHA: "abc"}
	s := computeScore(in, cfg)
	if s.Score <= 0 {
		t.Fatalf("expected positive (risk-on) score, got %v", s.Score)
	}
	if s.Score > 1 || s.Score < -1 {
		t.Fatalf("score out of range: %v", s.Score)
	}
	if s.Domain != domain.DomainMarketStress {
		t.Fatalf("wrong domain: %v", s.Domain)
	}
	if s.DataQuality["partial_coverage"] != false {
		t.Fatalf("expected partial_coverage=false, got %v", s.DataQuality["partial_coverage"])
	}
}

func TestComputeScore_ScoreClampedToOne(t *testing.T) {

	in := ScoreInputs{
		Asset:        domain.AssetBTC,
		ValueDate:    domain.MustParseDay("2026-04-22"),
		ZCorrelation: -100, ZCorrelationAvailable: true,
		ZPeg: -100, ZPegAvailable: true,
		ZCoinbase: -100, ZCoinbaseAvailable: true,
		ZBasis: -100, ZBasisAvailable: true,

		ZBookImbalance: 100, ZBookImbalanceAvailable: true,
		ZFuturesSpotRatio: -100, ZFuturesSpotRatioAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	if math.Abs(s.Score-1.0) > 1e-9 {
		t.Fatalf("expected score==1 when all signals saturate negative, got %v", s.Score)
	}
}

func TestComputeScore_ScoreClampedToMinusOne(t *testing.T) {
	in := ScoreInputs{
		Asset:        domain.AssetBTC,
		ValueDate:    domain.MustParseDay("2026-04-22"),
		ZCorrelation: 100, ZCorrelationAvailable: true,
		ZPeg: 100, ZPegAvailable: true,
		ZCoinbase: 100, ZCoinbaseAvailable: true,
		ZBasis: 100, ZBasisAvailable: true,

		ZBookImbalance: -100, ZBookImbalanceAvailable: true,
		ZFuturesSpotRatio: 100, ZFuturesSpotRatioAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	if math.Abs(s.Score+1.0) > 1e-9 {
		t.Fatalf("expected score==-1 when all signals saturate positive (high stress), got %v", s.Score)
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

func TestComputeScore_NaNAxisFlagsZeroComponent(t *testing.T) {
	in := ScoreInputs{
		Asset:        domain.AssetBTC,
		ValueDate:    domain.MustParseDay("2026-04-22"),
		ZCorrelation: math.NaN(), ZCorrelationAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	c := s.Components["component_correlation"]
	if c != 0 {
		t.Fatalf("expected NaN z to skip correlation component, got %v", c)
	}
}

func TestService_GatherScoreInputs_AllPresent(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{FinalVersion: "mstress_final_v1.0.0"}
	cfg.Defaults()
	s := newServiceForTests(feats, newFakeRawRepo(), &fakeLeverageReader{}, cfg)

	day := domain.MustParseDay("2026-04-22")
	feats.latest[featKey(domain.FeatureKey{Name: features.BtcAltCorrelationZScore180dName, Version: cfg.FinalVersion}, domain.AssetGlobal, day)] = -0.5
	feats.latest[featKey(domain.FeatureKey{Name: features.StablecoinPegStressScoreName, Version: cfg.FinalVersion}, domain.AssetGlobal, day)] = 0.2
	feats.latest[featKey(domain.FeatureKey{Name: features.CoinbasePremiumAbsZScore90dName, Version: cfg.FinalVersion}, domain.AssetGlobal, day)] = -0.1
	feats.latest[featKey(domain.FeatureKey{Name: features.BasisInversionDepthZScore180dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = 0.7

	in, err := s.gatherScoreInputs(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if !in.ZCorrelationAvailable || in.ZCorrelation != -0.5 {
		t.Errorf("ZCorrelation: got %v %v", in.ZCorrelation, in.ZCorrelationAvailable)
	}
	if !in.ZPegAvailable || in.ZPeg != 0.2 {
		t.Errorf("ZPeg: got %v %v", in.ZPeg, in.ZPegAvailable)
	}
	if !in.ZCoinbaseAvailable || in.ZCoinbase != -0.1 {
		t.Errorf("ZCoinbase: got %v %v", in.ZCoinbase, in.ZCoinbaseAvailable)
	}
	if !in.ZBasisAvailable || in.ZBasis != 0.7 {
		t.Errorf("ZBasis: got %v %v", in.ZBasis, in.ZBasisAvailable)
	}

	if len(in.FeatureCodesUsed) != 6 {
		t.Errorf("expected 6 feature codes, got %v", in.FeatureCodesUsed)
	}
}

func TestService_GatherScoreInputs_MissingFeaturesGracefullyDegrade(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{FinalVersion: "v1"}
	cfg.Defaults()
	s := newServiceForTests(feats, newFakeRawRepo(), &fakeLeverageReader{}, cfg)

	in, err := s.gatherScoreInputs(context.Background(), domain.AssetBTC, domain.MustParseDay("2026-04-22"), time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if in.ZCorrelationAvailable || in.ZPegAvailable || in.ZCoinbaseAvailable || in.ZBasisAvailable {
		t.Fatal("expected all axes unavailable")
	}
}

func TestService_GatherScoreInputs_UnexpectedRepoErrorBubblesUp(t *testing.T) {
	feats := &erroringFeatureRepo{err: errors.New("connection lost")}
	cfg := Config{FinalVersion: "v1"}
	cfg.Defaults()
	s := newServiceForTests(feats, newFakeRawRepo(), &fakeLeverageReader{}, cfg)

	_, err := s.gatherScoreInputs(context.Background(), domain.AssetBTC, time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected error to propagate, got nil")
	}
}

func TestService_ComputeBasisInversionDepth_UsesLeveragePort(t *testing.T) {
	day := domain.MustParseDay("2026-04-22")
	cfg := Config{IntermediateVersion: "i", FinalVersion: "f", LeverageBasisVersion: "leverage_int_v1.0.0"}
	cfg.Defaults()

	lev := &fakeLeverageReader{basis: map[string]float64{
		leverageKey(domain.AssetBTC, day): -0.05,
	}}
	s := newServiceForTests(newFakeFeatureRepo(), newFakeRawRepo(), lev, cfg)

	v, ok, err := s.computeBasisInversionDepth(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok || math.Abs(v-0.05) > 1e-9 {
		t.Fatalf("expected depth=0.05, got v=%v ok=%v", v, ok)
	}

	lev.basis[leverageKey(domain.AssetBTC, day)] = 0.10
	v, ok, _ = s.computeBasisInversionDepth(context.Background(), domain.AssetBTC, day, time.Now())
	if !ok || v != 0 {
		t.Fatalf("expected depth=0 for contango, got v=%v ok=%v", v, ok)
	}

	delete(lev.basis, leverageKey(domain.AssetBTC, day))
	_, ok, err = s.computeBasisInversionDepth(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil || ok {
		t.Fatalf("expected (false, nil) for missing leverage row, got ok=%v err=%v", ok, err)
	}
}

func TestService_ComputeCoinbasePremium_PrefersCoinglassThenFallback(t *testing.T) {
	cfg := Config{IntermediateVersion: "i", FinalVersion: "f"}
	cfg.Defaults()

	day := domain.MustParseDay("2026-04-22")
	r := newFakeRawRepo()
	rate := -0.0042
	r.coinglassRate = &rate
	s := newServiceForTests(newFakeFeatureRepo(), r, &fakeLeverageReader{}, cfg)

	v, ok, err := s.computeCoinbasePremiumAbs(context.Background(), day, time.Now())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok || math.Abs(v-0.0042) > 1e-12 {
		t.Fatalf("expected abs(coinglass)=0.0042, got v=%v ok=%v", v, ok)
	}

	r.coinglassRate = nil
	r.coinbaseClose["BTC-USD"] = 70_350.0
	r.binanceClose["BTCUSDT"] = 70_000.0
	v, ok, err = s.computeCoinbasePremiumAbs(context.Background(), day, time.Now())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok || math.Abs(v-0.005) > 1e-9 {
		t.Fatalf("expected fallback premium=0.005, got v=%v ok=%v", v, ok)
	}

	r2 := newFakeRawRepo()
	s2 := newServiceForTests(newFakeFeatureRepo(), r2, &fakeLeverageReader{}, cfg)
	_, ok, err = s2.computeCoinbasePremiumAbs(context.Background(), day, time.Now())
	if err != nil || ok {
		t.Fatalf("expected (false, nil) when both sources missing, got ok=%v err=%v", ok, err)
	}
}

func TestFeatures_AvgBtcAltCorrelation_AlignsAndAverages(t *testing.T) {

	btc := []float64{0.01, -0.02, 0.03, -0.01, 0.02, 0.00, 0.01, 0.02, -0.01, 0.03}
	alt := []float64{0.011, -0.018, 0.029, -0.012, 0.022, 0.001, 0.012, 0.019, -0.009, 0.031}
	v, ok := features.AvgBtcAltCorrelation(btc, [][]float64{alt})
	if !ok {
		t.Fatal("expected ok=true with matching length series")
	}
	if v < 0.95 {
		t.Fatalf("expected near-perfect correlation (>0.95), got %v", v)
	}

	if _, ok := features.AvgBtcAltCorrelation(btc, nil); ok {
		t.Fatal("expected ok=false with no alts")
	}

	if _, ok := features.AvgBtcAltCorrelation(btc, [][]float64{{0.1}}); ok {
		t.Fatal("expected ok=false when only contributor has fewer than min observations")
	}
}

func TestFeatures_PegDeviationScaled(t *testing.T) {

	v, ok := features.PegDeviationScaled(1.005, true, 0.997, true)
	if !ok {
		t.Fatal("expected ok=true with both prices")
	}
	want := math.Log1p(0.005 * 10000)
	if math.Abs(v-want) > 1e-9 {
		t.Fatalf("expected %v, got %v", want, v)
	}

	if _, ok := features.PegDeviationScaled(0, false, 0, false); ok {
		t.Fatal("expected ok=false with no inputs")
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
