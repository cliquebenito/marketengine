package binance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"marketengine/pkg/httpclient"
)

const (
	DefaultBaseURL = "https://fapi.binance.com"
	spotBaseURL    = "https://api.binance.com"

	pageThrottle = 350 * time.Millisecond
)

type OIPoint struct {
	Date           time.Time
	Symbol         string
	OIContractsUSD float64
	PayloadHash    string
}

type FundingPoint struct {
	Timestamp   time.Time
	Symbol      string
	Rate        float64
	PayloadHash string
}

type LiquidationPoint struct {
	Date         time.Time
	Symbol       string
	LongLiqsUSD  float64
	ShortLiqsUSD float64
	PayloadHash  string
}

type KlinePoint struct {
	OpenTime    time.Time
	Close       float64
	PayloadHash string
}

type Client struct {
	futures *httpclient.Client
	spot    *httpclient.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		futures: httpclient.New(
			httpclient.WithBaseURL(baseURL),
			httpclient.WithTimeout(timeout),
			httpclient.WithThrottle(pageThrottle),
		),
		spot: httpclient.New(
			httpclient.WithBaseURL(spotBaseURL),
			httpclient.WithTimeout(timeout),
			httpclient.WithThrottle(pageThrottle),
		),
	}
}

func (c *Client) FetchOpenInterest(ctx context.Context, symbol string, start, end time.Time) ([]OIPoint, error) {
	seen := make(map[int64]struct{})
	var out []OIPoint

	{
		path := fmt.Sprintf("/futures/data/openInterestHist?symbol=%s&period=1d&limit=500", symbol)
		body, err := c.futures.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("openInterestHist: %w", err)
		}
		var hist []struct {
			Symbol               string      `json:"symbol"`
			SumOpenInterestValue json.Number `json:"sumOpenInterestValue"`
			Timestamp            json.Number `json:"timestamp"`
		}
		if err := json.Unmarshal(body, &hist); err != nil {
			return nil, fmt.Errorf("decode openInterestHist: %w", err)
		}
		for i, h := range hist {
			ts, _ := h.Timestamp.Int64()
			dayMs := time.UnixMilli(ts).UTC().Truncate(24 * time.Hour).UnixMilli()
			if _, dup := seen[dayMs]; dup {
				continue
			}
			seen[dayMs] = struct{}{}
			val, _ := h.SumOpenInterestValue.Float64()
			out = append(out, OIPoint{
				Date:           time.UnixMilli(dayMs).UTC(),
				Symbol:         h.Symbol,
				OIContractsUSD: val,
				PayloadHash:    hashRawElement(body, i),
			})
		}
	}

	body, err := c.futures.Get(ctx, fmt.Sprintf("/fapi/v1/openInterest?symbol=%s", symbol))
	if err != nil {
		return nil, fmt.Errorf("openInterest snapshot: %w", err)
	}
	var snap struct {
		Symbol       string      `json:"symbol"`
		OpenInterest json.Number `json:"openInterest"`
		Time         json.Number `json:"time"`
	}
	if err := json.Unmarshal(body, &snap); err != nil {
		return nil, fmt.Errorf("decode openInterest: %w", err)
	}
	ts, _ := snap.Time.Int64()
	oi, _ := snap.OpenInterest.Float64()
	sum := sha256.Sum256(body)
	dayMs := time.UnixMilli(ts).UTC().Truncate(24 * time.Hour).UnixMilli()
	if _, dup := seen[dayMs]; !dup {
		out = append(out, OIPoint{
			Date:           time.UnixMilli(dayMs).UTC(),
			Symbol:         snap.Symbol,
			OIContractsUSD: oi,
			PayloadHash:    hex.EncodeToString(sum[:]),
		})
	}
	return out, nil
}

func (c *Client) FetchFundingRateHistory(ctx context.Context, symbol string, start, end time.Time) ([]FundingPoint, error) {
	var out []FundingPoint
	cursor, endMs := start.UnixMilli(), end.UnixMilli()
	for cursor < endMs {
		path := fmt.Sprintf("/fapi/v1/fundingRate?symbol=%s&startTime=%d&endTime=%d&limit=1000",
			symbol, cursor, endMs)
		body, err := c.futures.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("fundingRate: %w", err)
		}
		var page []struct {
			Symbol      string      `json:"symbol"`
			FundingRate json.Number `json:"fundingRate"`
			FundingTime json.Number `json:"fundingTime"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode fundingRate: %w", err)
		}
		if len(page) == 0 {
			break
		}
		for i, r := range page {
			ft, _ := r.FundingTime.Int64()
			rate, _ := r.FundingRate.Float64()
			out = append(out, FundingPoint{
				Timestamp:   time.UnixMilli(ft).UTC(),
				Symbol:      r.Symbol,
				Rate:        rate,
				PayloadHash: hashRawElement(body, i),
			})
		}
		lastTime, _ := page[len(page)-1].FundingTime.Int64()
		cursor = lastTime + 1
		if len(page) < 1000 {
			break
		}
	}
	return out, nil
}

func (c *Client) FetchLiquidations(ctx context.Context, symbol string, start, end time.Time) ([]LiquidationPoint, error) {
	type rawOrder struct {
		Symbol  string      `json:"symbol"`
		Price   json.Number `json:"price"`
		OrigQty json.Number `json:"origQty"`
		Side    string      `json:"side"`
		Time    json.Number `json:"time"`
	}
	var orders []rawOrder
	cursor, endMs := start.UnixMilli(), end.UnixMilli()
	for cursor < endMs {
		path := fmt.Sprintf("/fapi/v1/allForceOrders?symbol=%s&startTime=%d&endTime=%d&limit=1000",
			symbol, cursor, endMs)
		body, err := c.futures.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("allForceOrders: %w", err)
		}
		var page []rawOrder
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode allForceOrders: %w", err)
		}
		if len(page) == 0 {
			break
		}
		orders = append(orders, page...)
		lastTime, _ := page[len(page)-1].Time.Int64()
		cursor = lastTime + 1
		if len(page) < 1000 {
			break
		}
	}

	type dayKey struct {
		date   time.Time
		symbol string
	}
	type bucket struct{ longUSD, shortUSD float64 }
	buckets := make(map[dayKey]*bucket)
	for _, o := range orders {
		t, _ := o.Time.Int64()
		price, _ := o.Price.Float64()
		qty, _ := o.OrigQty.Float64()
		notional := price * qty
		k := dayKey{date: time.UnixMilli(t).UTC().Truncate(24 * time.Hour), symbol: o.Symbol}
		b := buckets[k]
		if b == nil {
			b = &bucket{}
			buckets[k] = b
		}
		if o.Side == "SELL" {
			b.longUSD += notional
		} else {
			b.shortUSD += notional
		}
	}
	out := make([]LiquidationPoint, 0, len(buckets))
	for k, b := range buckets {
		raw := fmt.Sprintf("%s|%s|%f|%f", k.date.Format("2006-01-02"), k.symbol, b.longUSD, b.shortUSD)
		out = append(out, LiquidationPoint{
			Date:         k.date,
			Symbol:       k.symbol,
			LongLiqsUSD:  b.longUSD,
			ShortLiqsUSD: b.shortUSD,
			PayloadHash:  httpclient.SHA256(raw),
		})
	}
	return out, nil
}

func (c *Client) FetchKlines(ctx context.Context, symbol string, interval string, start, end time.Time) ([]KlinePoint, error) {
	var out []KlinePoint
	cursor := start.UnixMilli()
	endMs := end.UnixMilli()

	for cursor < endMs {
		path := fmt.Sprintf("/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1500",
			symbol, interval, cursor, endMs)
		body, err := c.spot.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("klines %s: %w", symbol, err)
		}
		var page [][]json.RawMessage
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode klines: %w", err)
		}
		if len(page) == 0 {
			break
		}
		for i, k := range page {
			if len(k) < 5 {
				continue
			}
			var openTimeMs int64
			_ = json.Unmarshal(k[0], &openTimeMs)
			var closeStr string
			_ = json.Unmarshal(k[4], &closeStr)
			closeVal, _ := strconv.ParseFloat(closeStr, 64)

			out = append(out, KlinePoint{
				OpenTime:    time.UnixMilli(openTimeMs).UTC().Truncate(24 * time.Hour),
				Close:       closeVal,
				PayloadHash: hashRawElement(body, i),
			})
		}

		var lastOpenMs int64
		_ = json.Unmarshal(page[len(page)-1][0], &lastOpenMs)
		cursor = lastOpenMs + 1
		if len(page) < 1500 {
			break
		}
	}
	return out, nil
}

func hashRawElement(body []byte, i int) string {
	var arr []json.RawMessage
	if json.Unmarshal(body, &arr) == nil && i < len(arr) {
		s := sha256.Sum256(arr[i])
		return hex.EncodeToString(s[:])
	}
	s := sha256.Sum256([]byte(strconv.Itoa(i)))
	return hex.EncodeToString(s[:])
}
