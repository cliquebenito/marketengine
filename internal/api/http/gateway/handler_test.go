package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"marketengine/internal/domain"
)

type fakeRegime struct {
	latest    *domain.RegimeState
	latestErr error
	history   []domain.RegimeState
	histErr   error
	byDate    *domain.RegimeState
	byDateErr error

	gotLatestAsset domain.Asset
	gotHistArgs    struct {
		asset    domain.Asset
		from, to time.Time
	}
	gotByDateArgs struct {
		asset domain.Asset
		date  time.Time
	}
}

func (f *fakeRegime) GetLatest(_ context.Context, asset domain.Asset) (*domain.RegimeState, error) {
	f.gotLatestAsset = asset
	return f.latest, f.latestErr
}

func (f *fakeRegime) GetHistory(_ context.Context, asset domain.Asset, from, to time.Time) ([]domain.RegimeState, error) {
	f.gotHistArgs.asset = asset
	f.gotHistArgs.from = from
	f.gotHistArgs.to = to
	return f.history, f.histErr
}

func (f *fakeRegime) GetByDate(_ context.Context, asset domain.Asset, date time.Time) (*domain.RegimeState, error) {
	f.gotByDateArgs.asset = asset
	f.gotByDateArgs.date = date
	return f.byDate, f.byDateErr
}

type fakeScores struct {
	rows []domain.DomainScore
	err  error

	gotAsset  domain.Asset
	gotDomain domain.DomainCode
}

func (f *fakeScores) GetTimeline(_ context.Context, asset domain.Asset, dom domain.DomainCode, _, _ time.Time) ([]domain.DomainScore, error) {
	f.gotAsset = asset
	f.gotDomain = dom
	return f.rows, f.err
}

type fakeHealth struct{ err error }

func (f *fakeHealth) Ping(context.Context) error { return f.err }

func newServer(h *Handler) *httptest.Server {
	return httptest.NewServer(h.Routes())
}

func sampleRegimeState() *domain.RegimeState {
	return &domain.RegimeState{
		Asset:              domain.AssetBTC,
		ValueDate:          time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		RegimeIndicator:    0.42,
		RegimeIndicatorRaw: 0.51,
		RiskOnProbability:  0.7,
		RiskOffProbability: 0.2,
		TransitionRisk:     0.1,
		ModelVersion:       "engine_v1.1.0",
		ConfigVersion:      "cfg_abc",
		CodeSHA:            "deadbeef",
		DomainContributions: map[domain.DomainCode]float64{
			domain.DomainLiquidity: 0.3,
			domain.DomainLeverage:  -0.1,
		},
		TopDrivers: []domain.TopDriver{
			{Domain: domain.DomainLiquidity, Contribution: 0.3, Share: 0.75},
			{Domain: domain.DomainLeverage, Contribution: -0.1, Share: 0.25},
		},
		EffectiveWeights: map[domain.DomainCode]float64{
			domain.DomainLiquidity: 0.5, domain.DomainLeverage: 0.5,
		},
		FeatureCoverageFlag: map[domain.DomainCode]bool{
			domain.DomainLiquidity: true,
		},
		InteractionFlags: []string{},
	}
}

func decode(t *testing.T, resp *http.Response, into any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

func TestHealth_OK(t *testing.T) {
	h := &Handler{Health: &fakeHealth{}, GitSHA: "test-sha"}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	decode(t, resp, &body)
	if body["status"] != "ok" || body["git_sha"] != "test-sha" {
		t.Fatalf("body = %#v", body)
	}
}

func TestHealth_DBDown(t *testing.T) {
	h := &Handler{Health: &fakeHealth{err: errors.New("conn refused")}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestRegimeLatest_Happy(t *testing.T) {
	regime := &fakeRegime{latest: sampleRegimeState()}
	h := &Handler{Regime: regime, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regime/latest?asset=BTC")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	decode(t, resp, &body)
	if body["asset"] != "BTC" || body["value_date"] != "2026-04-20" {
		t.Fatalf("body = %#v", body)
	}
	if body["regime_indicator"].(float64) != 0.42 {
		t.Fatalf("regime_indicator = %v", body["regime_indicator"])
	}

	display := body["domain_display_names"].(map[string]any)
	if display["LIQUIDITY"] != "Liquidity" {
		t.Fatalf("display[LIQUIDITY] = %v", display["LIQUIDITY"])
	}

	drivers := body["top_drivers"].([]any)
	first := drivers[0].(map[string]any)
	if first["direction"] != "risk_on" {
		t.Fatalf("driver direction = %v", first["direction"])
	}
	if regime.gotLatestAsset != domain.AssetBTC {
		t.Fatalf("got asset = %v", regime.gotLatestAsset)
	}
}

func TestRegimeLatest_GlobalRejected(t *testing.T) {
	h := &Handler{Regime: &fakeRegime{}, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regime/latest?asset=GLOBAL")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRegimeLatest_BadAsset(t *testing.T) {
	h := &Handler{Regime: &fakeRegime{}, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regime/latest?asset=ZZZ")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRegimeLatest_NotFound(t *testing.T) {
	h := &Handler{Regime: &fakeRegime{latestErr: domain.ErrNotFound}, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regime/latest?asset=BTC")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRegimeHistory_Happy(t *testing.T) {
	regime := &fakeRegime{history: []domain.RegimeState{*sampleRegimeState()}}
	h := &Handler{Regime: regime, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regime/history?asset=BTC&from=2026-04-01&to=2026-04-20")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body []map[string]any
	decode(t, resp, &body)
	if len(body) != 1 || body[0]["asset"] != "BTC" {
		t.Fatalf("body = %#v", body)
	}
	if !regime.gotHistArgs.from.Equal(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from = %v", regime.gotHistArgs.from)
	}
}

func TestRegimeContributions_Happy(t *testing.T) {
	regime := &fakeRegime{byDate: sampleRegimeState()}
	h := &Handler{Regime: regime, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regime/2026-04-20/contributions?asset=BTC")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	decode(t, resp, &body)
	if body["value_date"] != "2026-04-20" {
		t.Fatalf("value_date = %v", body["value_date"])
	}
	if !regime.gotByDateArgs.date.Equal(time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date = %v", regime.gotByDateArgs.date)
	}
}

func TestDomains_CapitalFlowsRewritesToGlobal(t *testing.T) {
	scores := &fakeScores{rows: []domain.DomainScore{{
		Asset:        domain.AssetGlobal,
		Domain:       domain.DomainCapitalFlows,
		ValueDate:    time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		Score:        0.5,
		ModelVersion: "v1",
	}}}
	h := &Handler{Scores: scores, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/domains/capital-flows/scores?asset=BTC&from=2026-04-01&to=2026-04-20")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if scores.gotAsset != domain.AssetGlobal {
		t.Fatalf("query asset = %v, want GLOBAL", scores.gotAsset)
	}
	if scores.gotDomain != domain.DomainCapitalFlows {
		t.Fatalf("domain = %v", scores.gotDomain)
	}
	var body []map[string]any
	decode(t, resp, &body)
	if len(body) != 1 || body[0]["domain"] != "CAPITAL_FLOWS" {
		t.Fatalf("body = %#v", body)
	}
	if body[0]["domain_display"] != "Holder Conviction" {
		t.Fatalf("domain_display = %v", body[0]["domain_display"])
	}
}

func TestDomains_UnknownSlug(t *testing.T) {
	h := &Handler{Scores: &fakeScores{}, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/domains/wat/scores?asset=BTC&from=2026-04-01&to=2026-04-20")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCORS_Preflight(t *testing.T) {
	h := &Handler{Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/regime/latest", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "GET, HEAD, OPTIONS" {
		t.Fatalf("Allow-Methods = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Fatalf("Allow-Headers = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("Max-Age = %q", got)
	}
}

func TestUnknownRoute_404(t *testing.T) {
	h := &Handler{Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRegimeContributions_BadDate(t *testing.T) {
	h := &Handler{Regime: &fakeRegime{}, Health: &fakeHealth{}}
	srv := newServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/regime/not-a-date/contributions?asset=BTC")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
