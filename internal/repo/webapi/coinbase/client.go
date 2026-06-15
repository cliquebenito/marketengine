package coinbase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"marketengine/pkg/httpclient"
)

const (
	DefaultBaseURL = "https://api.exchange.coinbase.com"
	pageThrottle   = 200 * time.Millisecond
)

type CandlePoint struct {
	Date        time.Time
	ProductID   string
	Close       float64
	Volume      float64
	PayloadHash string
}

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

func (c *Client) FetchCandles(ctx context.Context, productID string, start, end time.Time, granularity int) ([]CandlePoint, error) {
	var all []CandlePoint

	windowDur := time.Duration(granularity) * time.Second * 300
	cursor := start
	for cursor.Before(end) {
		windowEnd := cursor.Add(windowDur)
		if windowEnd.After(end) {
			windowEnd = end
		}

		path := fmt.Sprintf("/products/%s/candles?start=%s&end=%s&granularity=%d",
			productID,
			cursor.UTC().Format(time.RFC3339),
			windowEnd.UTC().Format(time.RFC3339),
			granularity)

		body, err := c.http.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("fetch candles %s: %w", productID, err)
		}

		var raw [][]json.Number
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("decode candles: %w", err)
		}

		for _, r := range raw {
			if len(r) < 6 {
				continue
			}
			ts, _ := r[0].Int64()
			closeVal, _ := r[4].Float64()
			vol, _ := r[5].Float64()

			date := time.Unix(ts, 0).UTC().Truncate(24 * time.Hour)
			hash := httpclient.SHA256(fmt.Sprintf("%s|%d|%s|%s", productID, ts, r[4].String(), r[5].String()))

			all = append(all, CandlePoint{
				Date:        date,
				ProductID:   productID,
				Close:       closeVal,
				Volume:      vol,
				PayloadHash: hash,
			})
		}

		cursor = windowEnd
	}

	return all, nil
}
