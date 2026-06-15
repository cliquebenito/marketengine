package capitalflows

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"marketengine/internal/capitalflows/features"
	"marketengine/internal/domain"
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
	etfSum    float64
	etfOK     bool
	lthSupply map[domain.Asset]float64
}

func (r *fakeRawRepo) SaveETFFlows(_ context.Context, _ []ETFFlowRow) error    { return nil }
func (r *fakeRawRepo) SaveLTHSupply(_ context.Context, _ []LTHSupplyRow) error { return nil }
func (r *fakeRawRepo) SaveBTCMarketCap(_ context.Context, _ []MarketCapRow) error {
	return nil
}
func (r *fakeRawRepo) CombinedETFFlowAsOf(_ context.Context, _, _ time.Time) (float64, bool, error) {
	return r.etfSum, r.etfOK, nil
}
func (r *fakeRawRepo) GetLTHSupplyAsOf(_ context.Context, a domain.Asset, _, _ time.Time) (float64, error) {
	if v, ok := r.lthSupply[a]; ok {
		return v, nil
	}
	return 0, domain.ErrNotFound
}

func (r *fakeRawRepo) SaveStablecoinMcap(_ context.Context, _ []StablecoinMcapRow) error {
	return nil
}
func (r *fakeRawRepo) SaveExchangeBalance(_ context.Context, _ []ExchangeBalanceRow) error {
	return nil
}
func (r *fakeRawRepo) SaveBitfinexMargin(_ context.Context, _ []BitfinexMarginRow) error {
	return nil
}
func (r *fakeRawRepo) GetStablecoinMcapAsOf(_ context.Context, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetStablecoinMcapSeries(_ context.Context, _, _, _ time.Time) ([]float64, error) {
	return nil, nil
}
func (r *fakeRawRepo) GetExchangeBalanceChange7dSumAsOf(_ context.Context, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetExchangeBalanceChange30dSumAsOf(_ context.Context, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetBitfinexMarginAsOf(_ context.Context, _ string, _, _ time.Time) (float64, float64, bool, error) {
	return 0, 0, false, nil
}

func (r *fakeRawRepo) SaveETFListSnapshot(_ context.Context, _ []ETFListItemRow) error {
	return nil
}
func (r *fakeRawRepo) SaveETFAUMHistory(_ context.Context, _ []ETFAUMHistoryRow) error {
	return nil
}
func (r *fakeRawRepo) SaveOptionsMaxPainNearest(_ context.Context, _ []OptionsMaxPainRow) error {
	return nil
}
func (r *fakeRawRepo) GetETFListAUMTotalAsOf(_ context.Context, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetETFListConcentrationHHIAsOf(_ context.Context, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetETFAUMHistoryTotalAsOf(_ context.Context, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetOptionsDealerSkewProxyAsOf(_ context.Context, _, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

func TestComputeScore_PostETF_UsesPostWeights(t *testing.T) {
	in := ScoreInputs{
		Asset:         domain.AssetGlobal,
		ValueDate:     domain.MustParseDay("2026-04-22"),
		ZETF:          1.0,
		ZETFAvailable: true,
		ZLTH:          0.5,
		ZLTHAvailable: true,
	}
	cfg := Config{ModelVersion: "capital_flows_v0.2.0"}
	s := computeScore(in, cfg)
	if s.Score <= 0 {
		t.Fatalf("expected positive score on post-ETF bullish inputs, got %v", s.Score)
	}
	if s.Score > 1 || s.Score < -1 {
		t.Fatalf("score out of range: %v", s.Score)
	}
	if got := s.DataQuality["post_etf"]; got != true {
		t.Fatalf("expected post_etf=true, got %v", got)
	}

	if c := s.Components["component_etf"]; math.Abs(c) < 1e-9 {
		t.Fatalf("expected component_etf non-zero post-launch, got %v", c)
	}
}

func TestComputeScore_PreETF_UsesPreWeights(t *testing.T) {
	in := ScoreInputs{
		Asset:         domain.AssetGlobal,
		ValueDate:     domain.MustParseDay("2023-06-01"),
		ZETF:          5.0,
		ZETFAvailable: true,
		ZLTH:          0.5,
		ZLTHAvailable: true,
	}
	cfg := Config{ModelVersion: "capital_flows_v0.2.0"}
	s := computeScore(in, cfg)
	if got := s.DataQuality["post_etf"]; got != false {
		t.Fatalf("expected post_etf=false, got %v", got)
	}

	want := math.Tanh(0.5 / 2.0)
	if math.Abs(s.Score-want) > 1e-9 {
		t.Fatalf("pre-launch score expected to equal tanh(z_lth/scale)=%v (LTH-only), got %v", want, s.Score)
	}

}

func TestComputeScore_PreETF_NoPartialCoverageOnMissingETF(t *testing.T) {
	in := ScoreInputs{
		Asset:         domain.AssetGlobal,
		ValueDate:     domain.MustParseDay("2023-01-15"),
		ZLTH:          0.2,
		ZLTHAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})

	if got := s.DataQuality["etf_available"]; got != false {
		t.Fatalf("expected etf_available=false, got %v", got)
	}
	if got := s.DataQuality["post_etf"]; got != false {
		t.Fatalf("expected post_etf=false, got %v", got)
	}

	if got := s.DataQuality["partial_coverage"]; got != true {
		t.Fatalf("expected partial_coverage=true (miner stub absent), got %v", got)
	}
}

func TestComputeScore_MinerStub_ContributesZero(t *testing.T) {
	in := ScoreInputs{
		Asset:           domain.AssetGlobal,
		ValueDate:       domain.MustParseDay("2026-04-22"),
		ZETF:            1.0,
		ZETFAvailable:   true,
		ZLTH:            0.5,
		ZLTHAvailable:   true,
		ZMiner:          0.0,
		ZMinerAvailable: true,
	}
	cfg := Config{ModelVersion: "capital_flows_v0.2.0"}
	s := computeScore(in, cfg)
	if c := s.Components["component_miner"]; math.Abs(c) > 1e-9 {
		t.Fatalf("miner stub should contribute 0, got component_miner=%v", c)
	}
}

func TestComputeScore_ScoreClampedToOne(t *testing.T) {
	in := ScoreInputs{
		Asset:         domain.AssetGlobal,
		ValueDate:     domain.MustParseDay("2026-04-22"),
		ZETF:          100,
		ZETFAvailable: true,
		ZLTH:          100,
		ZLTHAvailable: true,

		ZStablecoinVelocity: 100, ZStablecoinVelocityAvailable: true,
		ZExchangeBalance: -100, ZExchangeBalanceAvailable: true,
		ZBitfinexMargin: 100, ZBitfinexMarginAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	if math.Abs(s.Score-1.0) > 1e-9 {
		t.Fatalf("expected clamped score=1.0 on saturation, got %v", s.Score)
	}
}

func TestComputeScore_DataQualityCarriesDisplayName(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetGlobal,
		ValueDate: domain.MustParseDay("2026-04-22"),
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	if got := s.DataQuality["display_name"]; got != "Holder Conviction" {
		t.Fatalf("expected display_name=Holder Conviction, got %v", got)
	}
	if _, ok := s.DataQuality["rename_note"]; !ok {
		t.Fatalf("expected rename_note in DataQuality")
	}
}

func TestService_GatherScoreInputs_AllPresent(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{
		IntermediateVersion: "int_v1.0.0",
		FinalVersion:        "final_v1.0.0",
	}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)

	day := domain.MustParseDay("2026-04-22")
	feats.latest[featKey(domain.FeatureKey{Name: features.ETFNetflowZScore90dName, Version: cfg.FinalVersion}, domain.AssetGlobal, day)] = 1.5
	feats.latest[featKey(domain.FeatureKey{Name: features.LTHSupplyChangeZScore180dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = 0.8

	in, err := s.gatherScoreInputs(context.Background(), domain.AssetGlobal, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if !in.ZETFAvailable || in.ZETF != 1.5 {
		t.Errorf("ZETF: got %v %v", in.ZETF, in.ZETFAvailable)
	}
	if !in.ZLTHAvailable || in.ZLTH != 0.8 {
		t.Errorf("ZLTH: got %v %v", in.ZLTH, in.ZLTHAvailable)
	}
	if in.ZMinerAvailable {
		t.Errorf("ZMiner should be permanently unavailable today, got available")
	}

	if len(in.FeatureCodesUsed) != 11 {
		t.Errorf("expected 11 feature codes, got %d", len(in.FeatureCodesUsed))
	}
}

func TestService_GatherScoreInputs_MissingFeaturesGracefullyDegrade(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{IntermediateVersion: "int_v1", FinalVersion: "final_v1"}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)

	day := domain.MustParseDay("2026-04-22")
	in, err := s.gatherScoreInputs(context.Background(), domain.AssetGlobal, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if in.ZETFAvailable || in.ZLTHAvailable || in.ZMinerAvailable {
		t.Fatal("expected all unavailable on empty repo")
	}
}

func TestService_GatherScoreInputs_UnexpectedRepoErrorBubblesUp(t *testing.T) {
	feats := &erroringFeatureRepo{err: errors.New("connection lost")}
	cfg := Config{FinalVersion: "final_v1"}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)
	_, err := s.gatherScoreInputs(context.Background(), domain.AssetGlobal, time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected error to propagate, got nil")
	}
}

func TestService_AssetsDefaultToGlobal(t *testing.T) {
	feats := newFakeFeatureRepo()
	scores := &fakeScoreRepo{}
	pub := &fakePublisher{}
	raws := &fakeRawRepo{}
	cfg := Config{ModelVersion: "x", FinalVersion: "f", IntermediateVersion: "i"}
	cfg.Defaults()

	s := NewService(feats, scores, raws, nil, nil, nil, nil, nil, pub,
		fakeClock{t: domain.MustParseDay("2026-04-22")}, cfg, nil)

	if got := len(s.assets); got != 1 || s.assets[0] != domain.AssetGlobal {
		t.Fatalf("expected assets=[GLOBAL], got %v", s.assets)
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
