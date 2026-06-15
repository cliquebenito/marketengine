package bybit

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
	DefaultBaseURL = "https://api.bybit.com"

	pageThrottle = 200 * time.Millisecond
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
			httpclient.WithThrottle(pageThrottle),
		),
	}
}

type OIPoint struct {
	Date            time.Time
	Symbol          string
	OpenInterestUSD float64
	PayloadHash     string
}

type FundingPoint struct {
	Timestamp   time.Time
	Symbol      string
	Rate        float64
	PayloadHash string
}

type v5Response struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

type listWrapper struct {
	List           json.RawMessage `json:"list"`
	NextPageCursor string          `json:"nextPageCursor"`
}

func (c *Client) fetchList(ctx context.Context, path string) (json.RawMessage, string, error) {
	body, err := c.http.Get(ctx, path)
	if err != nil {
		return nil, "", fmt.Errorf("bybit: %w", err)
	}
	var env v5Response
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, "", fmt.Errorf("bybit: decode envelope: %w", err)
	}
	if env.RetCode != 0 {
		return nil, "", fmt.Errorf("bybit retCode %d: %s", env.RetCode, env.RetMsg)
	}
	var wrap listWrapper
	if err := json.Unmarshal(env.Result, &wrap); err != nil {
		return nil, "", fmt.Errorf("bybit: decode result wrapper: %w", err)
	}
	return wrap.List, wrap.NextPageCursor, nil
}

func (c *Client) FetchOpenInterest(ctx context.Context,
	symbol, interval string, start, end time.Time,
) ([]OIPoint, error) {
	if interval == "" {
		interval = "1d"
	}
	basePath := fmt.Sprintf(
		"/v5/market/open-interest?category=linear&symbol=%s&intervalTime=%s&startTime=%d&endTime=%d&limit=200",
		symbol, interval, start.UnixMilli(), end.UnixMilli(),
	)

	var out []OIPoint
	cursor := ""
	for {
		path := basePath
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		raw, nextCursor, err := c.fetchList(ctx, path)
		if err != nil {
			return nil, err
		}
		var items []struct {
			OpenInterest string `json:"openInterest"`
			Timestamp    string `json:"timestamp"`
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("decode OI list: %w", err)
		}
		if len(items) == 0 {
			break
		}
		for _, it := range items {
			ms, err := strconv.ParseInt(it.Timestamp, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse timestamp %q: %w", it.Timestamp, err)
			}
			oi, err := strconv.ParseFloat(it.OpenInterest, 64)
			if err != nil {
				return nil, fmt.Errorf("parse openInterest %q: %w", it.OpenInterest, err)
			}
			out = append(out, OIPoint{
				Date:            time.UnixMilli(ms).UTC(),
				Symbol:          symbol,
				OpenInterestUSD: oi,
				PayloadHash:     hashJSON(it),
			})
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return out, nil
}

func (c *Client) FetchFundingRateHistory(ctx context.Context,
	symbol string, start, end time.Time,
) ([]FundingPoint, error) {
	const windowDays = 60

	var out []FundingPoint
	windowStart := start
	for windowStart.Before(end) {
		windowEnd := windowStart.AddDate(0, 0, windowDays)
		if windowEnd.After(end) {
			windowEnd = end
		}
		path := fmt.Sprintf(
			"/v5/market/funding/history?category=linear&symbol=%s&startTime=%d&endTime=%d&limit=200",
			symbol, windowStart.UnixMilli(), windowEnd.UnixMilli(),
		)
		raw, _, err := c.fetchList(ctx, path)
		if err != nil {
			return nil, err
		}
		var items []struct {
			Symbol               string `json:"symbol"`
			FundingRate          string `json:"fundingRate"`
			FundingRateTimestamp string `json:"fundingRateTimestamp"`
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("decode funding list: %w", err)
		}
		for _, it := range items {
			ms, err := strconv.ParseInt(it.FundingRateTimestamp, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse timestamp %q: %w", it.FundingRateTimestamp, err)
			}
			rate, err := strconv.ParseFloat(it.FundingRate, 64)
			if err != nil {
				return nil, fmt.Errorf("parse fundingRate %q: %w", it.FundingRate, err)
			}
			out = append(out, FundingPoint{
				Timestamp:   time.UnixMilli(ms).UTC(),
				Symbol:      it.Symbol,
				Rate:        rate,
				PayloadHash: hashJSON(it),
			})
		}
		windowStart = windowEnd
	}
	return out, nil
}

func hashJSON(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
