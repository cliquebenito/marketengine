package volatility

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/volatility/features"
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

func seriesKey(k domain.FeatureKey, asset domain.Asset) string {
	return k.Name + "|" + k.Version + "|" + string(asset)
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
	return f.series[seriesKey(k, a)], nil
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
	dvol    map[domain.Asset]float64
	rv      map[domain.Asset]float64
	rvOK    map[domain.Asset]bool
	saveErr error
	savedDV []DVOLRow
	missing bool
}

func (r *fakeRawRepo) SaveDVOL(_ context.Context, rows []DVOLRow) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.savedDV = append(r.savedDV, rows...)
	return nil
}
func (r *fakeRawRepo) GetDVOLCloseAsOf(_ context.Context, a domain.Asset, _, _ time.Time) (float64, error) {
	if r.missing {
		return 0, domain.ErrNotFound
	}
	v, ok := r.dvol[a]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}
func (r *fakeRawRepo) RealizedVol30d(_ context.Context, a domain.Asset, _, _ time.Time) (float64, bool, error) {
	return r.rv[a], r.rvOK[a], nil
}

func (r *fakeRawRepo) SaveCoinglassOptionsInfo(_ context.Context, _ []CoinglassOptionsInfoRow) error {
	return nil
}
func (r *fakeRawRepo) SaveCoinglassOptionsOIHistory(_ context.Context, _ []CoinglassOptionsOIHistoryRow) error {
	return nil
}
func (r *fakeRawRepo) SaveCoinglassOptionsMaxPain(_ context.Context, _ []CoinglassOptionsMaxPainRow) error {
	return nil
}
func (r *fakeRawRepo) GetCoinglassOptionsAggregatedOIAsOf(_ context.Context, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetCoinglassOptionsPutCallRatioAsOf(_ context.Context, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) GetCoinglassOptionsMaxPainNearestAsOf(_ context.Context, _, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}
func (r *fakeRawRepo) SpotCloseAsOf(_ context.Context, _ string, _, _ time.Time) (float64, bool, error) {
	return 0, false, nil
}

func (r *fakeRawRepo) SaveDeribitOptionsChain(_ context.Context, _ []DeribitOptionsChainRow) error {
	return nil
}
func (r *fakeRawRepo) GetDeribitOptionsChainAsOf(_ context.Context, _ domain.Asset, _, _ time.Time) ([]DeribitOptionsChainSnapshot, error) {
	return nil, nil
}

type fakeDVOLProvider struct {
	pts map[domain.Asset][]DVOLPoint
	err error
}

func (p *fakeDVOLProvider) FetchDVOL(_ context.Context, asset domain.Asset, _, _ time.Time) ([]DVOLPoint, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.pts[asset], nil
}

type fakeOptionsProvider struct {
	snaps map[domain.Asset]OptionsSnapshot
	err   error
}

func (p *fakeOptionsProvider) FetchOptionsSnapshot(_ context.Context, asset domain.Asset) (OptionsSnapshot, error) {
	if p.err != nil {
		return OptionsSnapshot{}, p.err
	}
	return p.snaps[asset], nil
}

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

func TestComputeScore_HappyPath_LowStressIsRiskOn(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),
		ZSpread:   -1.2, ZSpreadAvailable: true,
		ZSkewProxy: -0.8, ZSkewProxyAvailable: true,
		ZVoV: -0.6, ZVoVAvailable: true,
	}
	cfg := Config{ModelVersion: "volatility_v0.1.0", ConfigVersion: "sha256:test", CodeSHA: "abc"}
	s := computeScore(in, cfg)
	if s.Score <= 0 {
		t.Fatalf("expected positive (risk-on) score with low-stress signals, got %v", s.Score)
	}
	if s.Score > 1 || s.Score < -1 {
		t.Fatalf("score out of range: %v", s.Score)
	}
	if s.Domain != domain.DomainVolatility {
		t.Fatalf("wrong domain: %v", s.Domain)
	}
	if s.DataQuality["partial_coverage"] != false {
		t.Fatalf("expected partial_coverage=false, got %v", s.DataQuality["partial_coverage"])
	}
	if s.DataQuality["using_4_component"] != false {
		t.Fatalf("expected 3-component path when term slope unavailable")
	}
}

func TestComputeScore_HighStress_ScoreClampedToMinusOne(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),
		ZSpread:   100, ZSpreadAvailable: true,
		ZSkewProxy: 100, ZSkewProxyAvailable: true,
		ZVoV: 100, ZVoVAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "vol_v1"})
	if math.Abs(s.Score+1.0) > 1e-9 {
		t.Fatalf("expected score==-1 when all signals saturate stressful, got %v", s.Score)
	}
}

func TestComputeScore_AllMissing_ScoreZeroPartialCoverageTrue(t *testing.T) {
	in := ScoreInputs{Asset: domain.AssetBTC, ValueDate: domain.MustParseDay("2026-04-22")}
	s := computeScore(in, Config{ModelVersion: "vol_v1"})
	if math.Abs(s.Score) > 1e-9 {
		t.Fatalf("expected zero score with no inputs, got %v", s.Score)
	}
	if s.DataQuality["partial_coverage"] != true {
		t.Fatalf("expected partial_coverage=true")
	}
}

func TestComputeScore_RealSkewWinsOverProxy(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),
		ZSkewReal: 2.0, ZSkewRealAvailable: true,
		ZSkewProxy: -2.0, ZSkewProxyAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "vol_v1"})
	tail := s.Components["component_tail_risk"]
	if tail >= 0 {
		t.Fatalf("expected real skew to dominate (negative tail comp), got %v", tail)
	}
	if s.DataQuality["skew_real_available"] != true || s.DataQuality["skew_proxy_available"] != true {
		t.Fatalf("expected both flags true")
	}
}

func TestComputeScore_FourComponentPath_WhenTermSlopePresent(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),
		ZSpread:   0.5, ZSpreadAvailable: true,
		ZSkewProxy: 0.5, ZSkewProxyAvailable: true,
		ZVoV: 0.5, ZVoVAvailable: true,
		ZTermSlope: 1.0, ZTermSlopeAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "vol_v1"})
	if s.DataQuality["using_4_component"] != true {
		t.Fatalf("expected 4-component path")
	}
	if s.Components["component_term_structure"] == 0 {
		t.Fatalf("expected non-zero term_structure component when term slope available")
	}
}

func TestComputeScore_NaNSpread_StableComponentZero(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),
		ZSpread:   math.NaN(), ZSpreadAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "vol_v1"})
	if c := s.Components["component_fear_premium"]; c != 0 {
		t.Fatalf("expected NaN z to skip fear component, got %v", c)
	}
}

func TestService_GatherScoreInputs_AllPresent(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{
		IntermediateVersion: "volatility_int_v1.0.0",
		FinalVersion:        "volatility_final_v1.0.0",
	}
	cfg.Defaults()
	svc := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)

	day := domain.MustParseDay("2026-04-22")
	feats.latest[featKey(domain.FeatureKey{Name: features.IVRVSpreadZScore90dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = -0.5
	feats.latest[featKey(domain.FeatureKey{Name: features.IVSkewProxyZScore90dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = 0.3
	feats.latest[featKey(domain.FeatureKey{Name: features.DVOLOfDVOLZScore180dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = -1.2
	feats.latest[featKey(domain.FeatureKey{Name: features.IVTermSlopeZScore90dName, Version: cfg.FinalVersion}, domain.AssetBTC, day)] = 0.8

	in, err := svc.gatherScoreInputs(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if !in.ZSpreadAvailable || in.ZSpread != -0.5 {
		t.Errorf("ZSpread: got %v %v", in.ZSpread, in.ZSpreadAvailable)
	}
	if !in.ZSkewProxyAvailable || in.ZSkewProxy != 0.3 {
		t.Errorf("ZSkewProxy: got %v %v", in.ZSkewProxy, in.ZSkewProxyAvailable)
	}
	if !in.ZVoVAvailable || in.ZVoV != -1.2 {
		t.Errorf("ZVoV: got %v %v", in.ZVoV, in.ZVoVAvailable)
	}
	if !in.ZTermSlopeAvailable || in.ZTermSlope != 0.8 {
		t.Errorf("ZTermSlope: got %v %v", in.ZTermSlope, in.ZTermSlopeAvailable)
	}
}

func TestService_GatherScoreInputs_MissingFeaturesGracefullyDegrade(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{IntermediateVersion: "i_v1", FinalVersion: "f_v1"}
	cfg.Defaults()
	svc := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)
	day := domain.MustParseDay("2026-04-22")
	in, err := svc.gatherScoreInputs(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if in.ZSpreadAvailable || in.ZSkewRealAvailable || in.ZSkewProxyAvailable ||
		in.ZVoVAvailable || in.ZTermSlopeAvailable {
		t.Fatal("expected all unavailable when feature repo is empty")
	}
}

func TestService_GatherScoreInputs_UnexpectedRepoErrorBubblesUp(t *testing.T) {
	feats := &erroringFeatureRepo{err: errors.New("connection lost")}
	cfg := Config{IntermediateVersion: "i", FinalVersion: "f"}
	cfg.Defaults()
	svc := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)
	_, err := svc.gatherScoreInputs(context.Background(), domain.AssetBTC, time.Now(), time.Now())
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

func TestService_RunOnce_PersistsScore_WithMissingProvidersIsOk(t *testing.T) {
	feats := newFakeFeatureRepo()
	scoreRepo := &fakeScoreRepo{}
	pub := &fakePublisher{}
	cfg := Config{
		ModelVersion:        "volatility_v0.1.0",
		ConfigVersion:       "sha256:test",
		IntermediateVersion: "i_v1", FinalVersion: "f_v1",
	}
	cfg.Defaults()
	svc := NewService(feats, scoreRepo, &fakeRawRepo{missing: true},
		nil, nil, nil, nil,
		pub, fakeClock{t: domain.MustParseDay("2026-04-22").Add(12 * time.Hour)},
		cfg, []domain.Asset{domain.AssetBTC})

	if err := svc.RunOnce(context.Background(), domain.MustParseDay("2026-04-22")); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(scoreRepo.saved) != 1 {
		t.Fatalf("want 1 score, got %d", len(scoreRepo.saved))
	}
	got := scoreRepo.saved[0]
	if got.Domain != domain.DomainVolatility {
		t.Errorf("wrong domain: %v", got.Domain)
	}
	if got.DataQuality["partial_coverage"] != true {
		t.Errorf("expected partial_coverage=true with no inputs")
	}

	var sawScoreEvent bool
	for _, ev := range pub.events {
		if ev.Topic == "scores.volatility.completed.v1" {
			sawScoreEvent = true
		}
	}
	if !sawScoreEvent {
		t.Errorf("expected scores.volatility.completed.v1 outbox event")
	}
}

func TestService_RunOnce_IngestsDVOLAndOptionsSnapshot(t *testing.T) {
	feats := newFakeFeatureRepo()
	rawRepo := &fakeRawRepo{missing: true}
	day := domain.MustParseDay("2026-04-22")
	dvolProv := &fakeDVOLProvider{
		pts: map[domain.Asset][]DVOLPoint{
			domain.AssetBTC: {
				{Date: day.AddDate(0, 0, -1), Asset: domain.AssetBTC, Close: 60.5, PayloadHash: "h1"},
				{Date: day, Asset: domain.AssetBTC, Close: 61.2, PayloadHash: "h2"},
			},
		},
	}
	optProv := &fakeOptionsProvider{
		snaps: map[domain.Asset]OptionsSnapshot{
			domain.AssetBTC: {
				Asset:        domain.AssetBTC,
				TermSlope:    0.05,
				HasTermSlope: true,
				Skew:         0.012,
				HasSkew:      true,
				NumOptions:   42,
			},
		},
	}
	cfg := Config{
		ModelVersion:        "volatility_v0.1.0",
		ConfigVersion:       "sha256:test",
		IntermediateVersion: "i_v1", FinalVersion: "f_v1",
	}
	cfg.Defaults()
	svc := NewService(feats, &fakeScoreRepo{}, rawRepo,
		dvolProv, optProv, nil, nil, &fakePublisher{},
		fakeClock{t: day.Add(12 * time.Hour)}, cfg,
		[]domain.Asset{domain.AssetBTC})

	if err := svc.RunOnce(context.Background(), day); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(rawRepo.savedDV) != 2 {
		t.Errorf("expected 2 DVOL rows saved, got %d", len(rawRepo.savedDV))
	}

	var sawTerm, sawSkew bool
	for _, f := range feats.saved {
		if f.Key.Name == features.IVTermSlopeDailyName {
			sawTerm = true
		}
		if f.Key.Name == features.IVSkewDailyName {
			sawSkew = true
		}
	}
	if !sawTerm || !sawSkew {
		t.Errorf("expected term-slope and skew intermediates persisted; got term=%v skew=%v", sawTerm, sawSkew)
	}
}
