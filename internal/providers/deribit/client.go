package deribit

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"marketengine/pkg/httpclient"
)

const (
	DefaultBaseURL = "https://www.deribit.com"

	minDaysToExpiry = 60
)

type Client struct {
	http *httpclient.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		http: httpclient.New(
			httpclient.WithBaseURL(baseURL),
			httpclient.WithTimeout(timeout),
		),
	}
}

type FutureInstrument struct {
	Name                string
	Currency            string
	ExpirationTimestamp int64
	IsActive            bool
}

type Ticker struct {
	InstrumentName string
	MarkPrice      float64
	IndexPrice     float64
	LastPrice      float64
	BestBid        float64
	BestAsk        float64
	OpenInterest   float64
	Timestamp      int64
}

type BasisSnapshot struct {
	Date               time.Time
	Currency           string
	FuturesPrice       float64
	SpotPrice          float64
	AnnualizedBasisPct float64
	DaysToExpiry       int
	InstrumentName     string
	PayloadHash        string
}

type DVOLPoint struct {
	Date        time.Time
	Currency    string
	Close       float64
	PayloadHash string
}

func (c *Client) FetchActiveFutures(ctx context.Context, currency string) ([]FutureInstrument, error) {
	path := fmt.Sprintf("/api/v2/public/get_instruments?currency=%s&kind=future&expired=false",
		strings.ToUpper(currency))
	body, err := c.http.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("fetch instruments: %w", err)
	}
	var resp struct {
		Result []struct {
			InstrumentName      string `json:"instrument_name"`
			BaseCurrency        string `json:"base_currency"`
			ExpirationTimestamp int64  `json:"expiration_timestamp"`
			IsActive            bool   `json:"is_active"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode instruments: %w", err)
	}
	out := make([]FutureInstrument, 0, len(resp.Result))
	for _, r := range resp.Result {
		out = append(out, FutureInstrument{
			Name:                r.InstrumentName,
			Currency:            strings.ToUpper(r.BaseCurrency),
			ExpirationTimestamp: r.ExpirationTimestamp,
			IsActive:            r.IsActive,
		})
	}
	return out, nil
}

func (c *Client) FetchTicker(ctx context.Context, instrumentName string) (*Ticker, error) {
	path := fmt.Sprintf("/api/v2/public/ticker?instrument_name=%s", instrumentName)
	body, err := c.http.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("fetch ticker: %w", err)
	}
	var resp struct {
		Result struct {
			InstrumentName string  `json:"instrument_name"`
			MarkPrice      float64 `json:"mark_price"`
			IndexPrice     float64 `json:"index_price"`
			LastPrice      float64 `json:"last_price"`
			BestBidPrice   float64 `json:"best_bid_price"`
			BestAskPrice   float64 `json:"best_ask_price"`
			OpenInterest   float64 `json:"open_interest"`
			Timestamp      int64   `json:"timestamp"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode ticker: %w", err)
	}
	r := resp.Result
	return &Ticker{
		InstrumentName: r.InstrumentName,
		MarkPrice:      r.MarkPrice,
		IndexPrice:     r.IndexPrice,
		LastPrice:      r.LastPrice,
		BestBid:        r.BestBidPrice,
		BestAsk:        r.BestAskPrice,
		OpenInterest:   r.OpenInterest,
		Timestamp:      r.Timestamp,
	}, nil
}

func (c *Client) Basis3mSnapshot(ctx context.Context, currency string) (BasisSnapshot, error) {
	futures, err := c.FetchActiveFutures(ctx, currency)
	if err != nil {
		return BasisSnapshot{}, fmt.Errorf("basis snapshot: %w", err)
	}

	now := time.Now().UTC()
	var best *FutureInstrument
	var bestDTE int

	for i := range futures {
		f := &futures[i]

		if f.ExpirationTimestamp == 0 {
			continue
		}
		expiry := time.UnixMilli(f.ExpirationTimestamp)
		dte := int(math.Ceil(expiry.Sub(now).Hours() / 24))
		if dte < minDaysToExpiry {
			continue
		}
		if best == nil || dte < bestDTE {
			best = f
			bestDTE = dte
		}
	}
	if best == nil {
		return BasisSnapshot{}, fmt.Errorf("no quarterly future with ≥%d DTE for %s", minDaysToExpiry, currency)
	}

	ticker, err := c.FetchTicker(ctx, best.Name)
	if err != nil {
		return BasisSnapshot{}, fmt.Errorf("basis snapshot ticker: %w", err)
	}

	spot := ticker.IndexPrice
	if spot == 0 {
		return BasisSnapshot{}, fmt.Errorf("index price is zero for %s", best.Name)
	}

	rawBasis := (ticker.MarkPrice - spot) / spot
	annualised := rawBasis * (365.0 / float64(bestDTE)) * 100.0
	hash := httpclient.SHA256(fmt.Sprintf("%s|%f|%f|%d", best.Name, ticker.MarkPrice, spot, ticker.Timestamp))

	return BasisSnapshot{
		Date:               now.Truncate(24 * time.Hour),
		Currency:           strings.ToUpper(currency),
		FuturesPrice:       ticker.MarkPrice,
		SpotPrice:          spot,
		AnnualizedBasisPct: annualised,
		DaysToExpiry:       bestDTE,
		InstrumentName:     best.Name,
		PayloadHash:        hash,
	}, nil
}

func (c *Client) FetchDVOL(ctx context.Context, currency string, start, end time.Time) ([]DVOLPoint, error) {
	path := fmt.Sprintf(
		"/api/v2/public/get_volatility_index_data?currency=%s&start_timestamp=%d&end_timestamp=%d&resolution=86400",
		strings.ToUpper(currency), start.UnixMilli(), end.UnixMilli(),
	)
	body, err := c.http.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("fetch dvol: %w", err)
	}
	var resp struct {
		Result struct {
			Data [][]float64 `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode dvol: %w", err)
	}
	cur := strings.ToUpper(currency)
	out := make([]DVOLPoint, 0, len(resp.Result.Data))
	for _, row := range resp.Result.Data {
		if len(row) < 5 {
			continue
		}
		ts := time.UnixMilli(int64(row[0])).UTC()
		rowJSON, _ := json.Marshal(row)
		out = append(out, DVOLPoint{
			Date:        ts.Truncate(24 * time.Hour),
			Currency:    cur,
			Close:       row[4],
			PayloadHash: httpclient.SHA256(string(rowJSON)),
		})
	}
	return out, nil
}

type OptionSummary struct {
	InstrumentName  string
	MarkIV          float64
	UnderlyingPrice float64
	StrikePrice     float64
	ExpiryTimestamp int64
	IsPut           bool
	OpenInterest    float64
}

func (c *Client) FetchOptionsSummary(ctx context.Context, currency string) ([]OptionSummary, error) {
	path := fmt.Sprintf("/api/v2/public/get_book_summary_by_currency?currency=%s&kind=option",
		strings.ToUpper(currency))
	body, err := c.http.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("fetch options summary: %w", err)
	}
	var resp struct {
		Result []struct {
			InstrumentName  string  `json:"instrument_name"`
			MarkIV          float64 `json:"mark_iv"`
			UnderlyingPrice float64 `json:"underlying_price"`
			BidPrice        float64 `json:"bid_price"`
			AskPrice        float64 `json:"ask_price"`
			OpenInterest    float64 `json:"open_interest"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode options summary: %w", err)
	}
	out := make([]OptionSummary, 0, len(resp.Result))
	for _, r := range resp.Result {
		if r.MarkIV <= 0 {
			continue
		}
		strike, expiry, isPut, ok := parseOptionName(r.InstrumentName)
		if !ok {
			continue
		}
		out = append(out, OptionSummary{
			InstrumentName:  r.InstrumentName,
			MarkIV:          r.MarkIV,
			UnderlyingPrice: r.UnderlyingPrice,
			StrikePrice:     strike,
			ExpiryTimestamp: expiry,
			IsPut:           isPut,
			OpenInterest:    r.OpenInterest,
		})
	}
	return out, nil
}

func parseOptionName(name string) (strike float64, expiryMs int64, isPut bool, ok bool) {
	parts := strings.Split(name, "-")
	if len(parts) < 4 {
		return 0, 0, false, false
	}

	t, err := time.Parse("2Jan06", strings.ToUpper(parts[1]))
	if err != nil {
		return 0, 0, false, false
	}
	expiryMs = t.UTC().UnixMilli()

	s := 0.0
	if _, err := fmt.Sscanf(parts[2], "%f", &s); err != nil || s <= 0 {
		return 0, 0, false, false
	}

	switch strings.ToUpper(parts[3]) {
	case "P":
		isPut = true
	case "C":
		isPut = false
	default:
		return 0, 0, false, false
	}
	return s, expiryMs, isPut, true
}

func ComputeIVTermSlope(opts []OptionSummary) (float64, bool) {
	if len(opts) == 0 {
		return 0, false
	}
	underlying := opts[0].UnderlyingPrice
	if underlying <= 0 {
		return 0, false
	}

	now := time.Now().UTC()
	best := map[int64]*struct {
		dte      float64
		bestDist float64
		bestIV   float64
	}{}

	for _, o := range opts {
		expiry := time.UnixMilli(o.ExpiryTimestamp).UTC()
		dte := expiry.Sub(now).Hours() / 24.0
		if dte < 7 {
			continue
		}
		dist := math.Abs(o.StrikePrice - underlying)
		b, exists := best[o.ExpiryTimestamp]
		if !exists {
			best[o.ExpiryTimestamp] = &struct {
				dte      float64
				bestDist float64
				bestIV   float64
			}{dte: dte, bestDist: dist, bestIV: o.MarkIV}
		} else if dist < b.bestDist {
			b.bestDist = dist
			b.bestIV = o.MarkIV
		}
	}

	var iv30, iv90 float64
	dist30, dist90 := math.MaxFloat64, math.MaxFloat64
	for _, b := range best {
		if d30 := math.Abs(b.dte - 30); d30 < dist30 {
			dist30 = d30
			iv30 = b.bestIV
		}
		if d90 := math.Abs(b.dte - 90); d90 < dist90 {
			dist90 = d90
			iv90 = b.bestIV
		}
	}
	if iv30 <= 0 || dist30 > 20 || dist90 > 45 {
		return 0, false
	}
	return (iv90 - iv30) / iv30, true
}

func ComputeIVSkew(opts []OptionSummary) (float64, bool) {
	if len(opts) == 0 {
		return 0, false
	}
	underlying := opts[0].UnderlyingPrice
	if underlying <= 0 {
		return 0, false
	}

	now := time.Now().UTC()
	targetPutStrike := 0.90 * underlying
	targetCallStrike := 1.10 * underlying

	var bestPutIV, bestCallIV float64
	bestPutDist := math.MaxFloat64
	bestCallDist := math.MaxFloat64
	bestPutDTE := math.MaxFloat64
	bestCallDTE := math.MaxFloat64

	for _, o := range opts {
		expiry := time.UnixMilli(o.ExpiryTimestamp).UTC()
		dte := expiry.Sub(now).Hours() / 24.0
		dteDist := math.Abs(dte - 30)
		if dteDist > 20 {
			continue
		}
		if o.IsPut {
			strikeDist := math.Abs(o.StrikePrice - targetPutStrike)
			if dteDist < bestPutDTE || (dteDist == bestPutDTE && strikeDist < bestPutDist) {
				bestPutDTE = dteDist
				bestPutDist = strikeDist
				bestPutIV = o.MarkIV
			}
		} else {
			strikeDist := math.Abs(o.StrikePrice - targetCallStrike)
			if dteDist < bestCallDTE || (dteDist == bestCallDTE && strikeDist < bestCallDist) {
				bestCallDTE = dteDist
				bestCallDist = strikeDist
				bestCallIV = o.MarkIV
			}
		}
	}
	if bestPutIV <= 0 || bestCallIV <= 0 {
		return 0, false
	}
	return bestPutIV - bestCallIV, true
}
