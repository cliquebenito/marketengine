package coinmetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"marketengine/pkg/httpclient"
)

const (
	DefaultBaseURL = "https://community-api.coinmetrics.io/v4"

	defaultPageSize = 10000
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

type ExchangeNetflowPoint struct {
	Date           time.Time
	Asset          string
	InflowUSD      *float64
	OutflowUSD     *float64
	NetflowUSD     *float64
	RawPayloadHash string
}

type assetMetricsResponse struct {
	Data []map[string]any `json:"data"`

	NextPageToken string `json:"next_page_token,omitempty"`
}

func (c *Client) FetchExchangeNetflow(ctx context.Context,
	assets []string, start, end time.Time,
) ([]ExchangeNetflowPoint, error) {
	params := url.Values{}

	lower := make([]string, len(assets))
	for i, a := range assets {
		lower[i] = strings.ToLower(a)
	}
	params.Set("assets", strings.Join(lower, ","))
	params.Set("metrics", "FlowInExUSD,FlowOutExUSD")
	params.Set("frequency", "1d")
	params.Set("start_time", start.UTC().Format("2006-01-02"))
	params.Set("end_time", end.UTC().Format("2006-01-02"))
	params.Set("page_size", fmt.Sprintf("%d", defaultPageSize))

	var out []ExchangeNetflowPoint
	pageToken := ""

	for {
		if pageToken != "" {
			params.Set("next_page_token", pageToken)
		}
		body, err := c.http.Get(ctx, "/timeseries/asset-metrics?"+params.Encode())
		if err != nil {
			return nil, fmt.Errorf("coinmetrics: %w", err)
		}

		var parsed assetMetricsResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		for _, row := range parsed.Data {
			point, err := parseRow(row)
			if err != nil {
				return nil, fmt.Errorf("parse row %v: %w", row, err)
			}
			out = append(out, point)
		}
		if parsed.NextPageToken == "" {
			break
		}
		pageToken = parsed.NextPageToken
	}

	return out, nil
}

func parseRow(row map[string]any) (ExchangeNetflowPoint, error) {
	asset, _ := row["asset"].(string)
	if asset == "" {
		return ExchangeNetflowPoint{}, fmt.Errorf("missing asset")
	}
	ts, _ := row["time"].(string)
	if ts == "" {
		return ExchangeNetflowPoint{}, fmt.Errorf("missing time")
	}
	date, err := time.Parse(time.RFC3339, ts)
	if err != nil {

		date, err = time.Parse("2006-01-02T15:04:05.000Z", ts)
		if err != nil {
			return ExchangeNetflowPoint{}, fmt.Errorf("parse time %q: %w", ts, err)
		}
	}

	rowJSON, _ := json.Marshal(row)
	inflow := parseOptionalFloat(row["FlowInExUSD"])
	outflow := parseOptionalFloat(row["FlowOutExUSD"])

	var netflow *float64
	if inflow != nil && outflow != nil {
		nf := *inflow - *outflow
		netflow = &nf
	}
	return ExchangeNetflowPoint{
		Date:           date.UTC().Truncate(24 * time.Hour),
		Asset:          strings.ToUpper(asset),
		InflowUSD:      inflow,
		OutflowUSD:     outflow,
		NetflowUSD:     netflow,
		RawPayloadHash: httpclient.SHA256(string(rowJSON)),
	}, nil
}

func parseOptionalFloat(v any) *float64 {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case float64:
		return &x
	case string:
		var f float64
		if _, err := fmt.Sscanf(x, "%g", &f); err != nil {
			return nil
		}
		return &f
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}
