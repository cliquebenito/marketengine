package coingecko

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"marketengine/pkg/httpclient"
)

const (
	defaultBaseURL = "https://api.coingecko.com/api/v3"
	throttle       = 3 * time.Second
)

type MarketCapPoint struct {
	Date         time.Time
	CoinID       string
	MarketCapUSD float64
	PriceUSD     float64
	PayloadHash  string
}

type Client struct {
	http *httpclient.Client
}

func New(timeout time.Duration) *Client {
	return &Client{
		http: httpclient.New(
			httpclient.WithBaseURL(defaultBaseURL),
			httpclient.WithTimeout(timeout),
			httpclient.WithThrottle(throttle),
		),
	}
}

func (c *Client) FetchMarketCapHistory(ctx context.Context, coinID string) ([]MarketCapPoint, error) {
	path := fmt.Sprintf("/coins/%s/market_chart?vs_currency=usd&days=max&interval=daily", coinID)
	body, err := c.http.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("coingecko: %w", err)
	}

	var raw struct {
		Prices     [][]json.Number `json:"prices"`
		MarketCaps [][]json.Number `json:"market_caps"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("coingecko: unmarshal: %w", err)
	}

	priceByTS := make(map[int64]float64, len(raw.Prices))
	for _, pair := range raw.Prices {
		if len(pair) != 2 {
			continue
		}
		tsMs, err := pair[0].Int64()
		if err != nil {
			continue
		}
		price, err := pair[1].Float64()
		if err != nil {
			continue
		}
		priceByTS[tsMs] = price
	}

	points := make([]MarketCapPoint, 0, len(raw.MarketCaps))
	for _, pair := range raw.MarketCaps {
		if len(pair) != 2 {
			continue
		}
		tsMs, err := pair[0].Int64()
		if err != nil {
			continue
		}
		mcap, err := pair[1].Float64()
		if err != nil {
			continue
		}

		t := time.UnixMilli(tsMs).UTC().Truncate(24 * time.Hour)
		rowJSON, _ := json.Marshal(map[string]any{
			"ts_ms":      tsMs,
			"coin_id":    coinID,
			"market_cap": pair[1],
			"price":      priceByTS[tsMs],
		})

		points = append(points, MarketCapPoint{
			Date:         t,
			CoinID:       coinID,
			MarketCapUSD: mcap,
			PriceUSD:     priceByTS[tsMs],
			PayloadHash:  httpclient.SHA256(string(rowJSON)),
		})
	}

	return points, nil
}
