package liquidity

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/liquidity/features"
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
	mcap        map[string]float64
	stableSum   float64
	stableFound int
	netflow7d   map[domain.Asset]float64
	netflowOK   map[domain.Asset]bool
}

func (r *fakeRawRepo) SaveStablecoinSupply(_ context.Context, _ []StablecoinSupplyRow) error {
	return nil
}
func (r *fakeRawRepo) SaveChainTVL(_ context.Context, _ []ChainTVLRow) error { return nil }
func (r *fakeRawRepo) SaveExchangeNetflow(_ context.Context, _ []ExchangeNetflowRow) error {
	return nil
}
func (r *fakeRawRepo) SaveMarketCap(_ context.Context, _ []MarketCapRow) error { return nil }
func (r *fakeRawRepo) GetStablecoinSupplyAsOf(_ context.Context, _ string, _, _ time.Time) (float64, error) {
	return 0, domain.ErrNotFound
}
func (r *fakeRawRepo) SumStablecoinSupplyAsOf(_ context.Context, _ []string, _, _ time.Time) (float64, int, error) {
	return r.stableSum, r.stableFound, nil
}
func (r *fakeRawRepo) GetChainTVLAsOf(_ context.Context, _ string, _, _ time.Time) (float64, error) {
	return 0, domain.ErrNotFound
}
func (r *fakeRawRepo) Sum7dNetflow(_ context.Context, a domain.Asset, _, _ time.Time) (float64, bool, error) {
	return r.netflow7d[a], r.netflowOK[a], nil
}
func (r *fakeRawRepo) GetMarketCapAsOf(_ context.Context, coinID string, _, _ time.Time) (float64, error) {
	if v, ok := r.mcap[coinID]; ok {
		return v, nil
	}
	return 0, domain.ErrNotFound
}

type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

func TestComputeScore_HappyPath_RiskOnSignals(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),
		ZStable:   1.2, ZStableAvailable: true,
		SSRPercentile: 0.30, SSRAvailable: true,
		ZNetflow: -0.8, ZNetflowAvailable: true,
		ZTVL: 0.6, ZTVLAvailable: true,
	}
	cfg := Config{ModelVersion: "liquidity_v0.4.0", ConfigVersion: "sha256:test", CodeSHA: "abc"}
	s := computeScore(in, cfg)
	if s.Score <= 0 {
		t.Fatalf("expected positive (risk-on) score, got %v", s.Score)
	}
	if s.Score > 1 || s.Score < -1 {
		t.Fatalf("score out of range: %v", s.Score)
	}
	if s.Domain != domain.DomainLiquidity {
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
		ZStable:   100, ZStableAvailable: true,
		ZNetflow: -100, ZNetflowAvailable: true,
		ZTVL: 100, ZTVLAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	if math.Abs(s.Score-1.0) > 1e-9 {
		t.Fatalf("expected score==1 when all signals saturate positive, got %v", s.Score)
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

func TestComputeScore_NaNStable_FlagsUnavailable(t *testing.T) {
	in := ScoreInputs{
		Asset:     domain.AssetBTC,
		ValueDate: domain.MustParseDay("2026-04-22"),
		ZStable:   math.NaN(), ZStableAvailable: true,
	}
	s := computeScore(in, Config{ModelVersion: "x_v1.0.0"})
	c := s.Components["component_stablecoin_capital"]
	if c != 0 {
		t.Fatalf("expected NaN z to skip stable component, got %v", c)
	}
}

func TestService_GatherScoreInputs_AllPresent(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{
		ZScoreFeatureVersion:        "stablecoin_growth_zscore_90d_v1.0.0",
		SSRPercentileFeatureVersion: "ssr_percentile_rank_365d_v1.1.0",
		NetflowZScoreFeatureVersion: "exchange_netflow_zscore_180d_v1.0.0",
		TVLZScoreFeatureVersion:     "defi_tvl_growth_zscore_180d_v1.0.0",
	}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)

	day := domain.MustParseDay("2026-04-22")
	feats.latest[featKey(domain.FeatureKey{Name: features.StablecoinGrowthZScore90dName, Version: cfg.ZScoreFeatureVersion}, domain.AssetGlobal, day)] = 1.5
	feats.latest[featKey(domain.FeatureKey{Name: features.SSRPercentileRank365dName, Version: cfg.SSRPercentileFeatureVersion}, domain.AssetBTC, day)] = 0.42
	feats.latest[featKey(domain.FeatureKey{Name: features.ExchangeNetflowZScore180dName, Version: cfg.NetflowZScoreFeatureVersion}, domain.AssetBTC, day)] = -0.6
	feats.latest[featKey(domain.FeatureKey{Name: features.DefiTVLGrowthZScore180dName, Version: cfg.TVLZScoreFeatureVersion}, domain.AssetGlobal, day)] = 0.9

	in, err := s.gatherScoreInputs(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if !in.ZStableAvailable || in.ZStable != 1.5 {
		t.Errorf("ZStable: got %v %v", in.ZStable, in.ZStableAvailable)
	}
	if !in.SSRAvailable || in.SSRPercentile != 0.42 {
		t.Errorf("SSR: got %v %v", in.SSRPercentile, in.SSRAvailable)
	}
	if !in.ZNetflowAvailable || in.ZNetflow != -0.6 {
		t.Errorf("Netflow: got %v %v", in.ZNetflow, in.ZNetflowAvailable)
	}
	if !in.ZTVLAvailable || in.ZTVL != 0.9 {
		t.Errorf("TVL: got %v %v", in.ZTVL, in.ZTVLAvailable)
	}
}

func TestService_GatherScoreInputs_MissingFeaturesGracefullyDegrade(t *testing.T) {
	feats := newFakeFeatureRepo()
	cfg := Config{
		ZScoreFeatureVersion:        "z_v1",
		SSRPercentileFeatureVersion: "p_v1",
		NetflowZScoreFeatureVersion: "n_v1",
		TVLZScoreFeatureVersion:     "t_v1",
	}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)

	day := domain.MustParseDay("2026-04-22")
	in, err := s.gatherScoreInputs(context.Background(), domain.AssetBTC, day, time.Now())
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	if in.ZStableAvailable || in.SSRAvailable || in.ZNetflowAvailable || in.ZTVLAvailable {
		t.Fatal("expected all unavailable")
	}
}

func TestService_GatherScoreInputs_UnexpectedRepoErrorBubblesUp(t *testing.T) {
	feats := &erroringFeatureRepo{err: errors.New("connection lost")}
	cfg := Config{ZScoreFeatureVersion: "z"}
	cfg.Defaults()
	s := NewService(feats, &fakeScoreRepo{}, &fakeRawRepo{},
		nil, nil, nil, nil, &fakePublisher{}, fakeClock{t: time.Now()}, cfg, nil)
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
