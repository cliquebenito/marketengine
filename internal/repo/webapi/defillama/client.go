package defillama

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"marketengine/pkg/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		http: httpclient.New(
			httpclient.WithBaseURL(baseURL),
			httpclient.WithTimeout(timeout),
		),
	}
}

type StablecoinDailyPoint struct {
	Date                time.Time
	TotalCirculatingUSD float64
	RawPayloadHash      string
}

type stablecoinChartEntry struct {
	Date                int64                  `json:"-"`
	DateRaw             json.Number            `json:"date"`
	TotalCirculatingUSD map[string]json.Number `json:"totalCirculatingUSD"`
}

type ChainTVLPoint struct {
	Date           time.Time
	TVLUSD         float64
	RawPayloadHash string
}

type chainTVLEntry struct {
	Date json.Number `json:"date"`
	TVL  json.Number `json:"tvl"`
}

func (c *Client) FetchChainTVL(ctx context.Context, chain string) ([]ChainTVLPoint, error) {
	body, err := c.http.Get(ctx, "/v2/historicalChainTvl/"+chain)
	if err != nil {
		return nil, fmt.Errorf("defillama chain TVL: %w", err)
	}
	var entries []chainTVLEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decode chain tvl: %w", err)
	}
	out := make([]ChainTVLPoint, 0, len(entries))
	for _, e := range entries {
		ts, err := e.Date.Int64()
		if err != nil {
			return nil, fmt.Errorf("bad date %q: %w", e.Date.String(), err)
		}
		tvl, err := e.TVL.Float64()
		if err != nil {
			return nil, fmt.Errorf("bad tvl %q: %w", e.TVL.String(), err)
		}
		rowJSON, _ := json.Marshal(e)
		out = append(out, ChainTVLPoint{
			Date:           time.Unix(ts, 0).UTC().Truncate(24 * time.Hour),
			TVLUSD:         tvl,
			RawPayloadHash: httpclient.SHA256(string(rowJSON)),
		})
	}
	return out, nil
}

var TrackedStablecoins = []string{"USDT", "USDC", "DAI"}

type PerStablecoinPoint struct {
	Date           time.Time
	Symbol         string
	CirculatingUSD float64
	RawPayloadHash string
}

type stablecoinsListResponse struct {
	PeggedAssets []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	} `json:"peggedAssets"`
}

type stablecoinHistoryEntry struct {
	Date        json.Number            `json:"date"`
	Circulating map[string]json.Number `json:"circulating"`
}

type stablecoinHistoryResponse struct {
	Tokens []stablecoinHistoryEntry `json:"tokens"`
}

func (c *Client) FetchPerStablecoinSupply(ctx context.Context) ([]PerStablecoinPoint, error) {
	listBody, err := c.http.Get(ctx, "/stablecoins?includePrices=false")
	if err != nil {
		return nil, fmt.Errorf("defillama list: %w", err)
	}
	var listResp stablecoinsListResponse
	if err := json.Unmarshal(listBody, &listResp); err != nil {
		return nil, fmt.Errorf("decode stablecoins list: %w", err)
	}

	wanted := make(map[string]bool, len(TrackedStablecoins))
	for _, s := range TrackedStablecoins {
		wanted[s] = true
	}
	symbolToID := make(map[string]string)
	for _, asset := range listResp.PeggedAssets {
		if wanted[asset.Symbol] && symbolToID[asset.Symbol] == "" {
			symbolToID[asset.Symbol] = asset.ID
		}
	}

	var out []PerStablecoinPoint
	for symbol, id := range symbolToID {
		points, err := c.fetchStablecoinHistory(ctx, id, symbol)
		if err != nil {
			return nil, fmt.Errorf("fetch %s (id=%s): %w", symbol, id, err)
		}
		out = append(out, points...)
	}
	return out, nil
}

func (c *Client) fetchStablecoinHistory(ctx context.Context, id, symbol string) ([]PerStablecoinPoint, error) {
	body, err := c.http.Get(ctx, "/stablecoin/"+id)
	if err != nil {
		return nil, fmt.Errorf("defillama stablecoin/%s: %w", id, err)
	}
	var histResp stablecoinHistoryResponse
	if err := json.Unmarshal(body, &histResp); err != nil {
		return nil, fmt.Errorf("decode stablecoin history: %w", err)
	}

	out := make([]PerStablecoinPoint, 0, len(histResp.Tokens))
	for _, e := range histResp.Tokens {
		ts, err := e.Date.Int64()
		if err != nil {
			return nil, fmt.Errorf("bad date %q: %w", e.Date.String(), err)
		}

		var total float64
		for _, v := range e.Circulating {
			f, err := v.Float64()
			if err != nil {
				return nil, fmt.Errorf("bad circulating value: %w", err)
			}
			total += f
		}
		rowJSON, _ := json.Marshal(e)
		out = append(out, PerStablecoinPoint{
			Date:           time.Unix(ts, 0).UTC().Truncate(24 * time.Hour),
			Symbol:         symbol,
			CirculatingUSD: total,
			RawPayloadHash: httpclient.SHA256(string(rowJSON)),
		})
	}
	return out, nil
}

func (c *Client) FetchAllStablecoinsChart(ctx context.Context) ([]StablecoinDailyPoint, error) {
	body, err := c.http.Get(ctx, "/stablecoincharts/all")
	if err != nil {
		return nil, fmt.Errorf("defillama all stablecoins: %w", err)
	}
	var entries []stablecoinChartEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decode chart: %w", err)
	}

	out := make([]StablecoinDailyPoint, 0, len(entries))
	for _, e := range entries {
		ts, err := e.DateRaw.Int64()
		if err != nil {
			return nil, fmt.Errorf("bad date %q: %w", e.DateRaw.String(), err)
		}

		var total float64
		for _, v := range e.TotalCirculatingUSD {
			f, err := v.Float64()
			if err != nil {
				return nil, fmt.Errorf("bad value: %w", err)
			}
			total += f
		}
		rowJSON, _ := json.Marshal(e)
		out = append(out, StablecoinDailyPoint{
			Date:                time.Unix(ts, 0).UTC().Truncate(24 * time.Hour),
			TotalCirculatingUSD: total,
			RawPayloadHash:      httpclient.SHA256(string(rowJSON)),
		})
	}
	return out, nil
}
