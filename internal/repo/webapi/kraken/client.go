package kraken

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

const DefaultBaseURL = "https://api.kraken.com"

type OHLCPoint struct {
	Date        time.Time
	Pair        string
	Open        float64
	High        float64
	Low         float64
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
		),
	}
}

func (c *Client) FetchOHLC(ctx context.Context, pair string, interval int, since int64) ([]OHLCPoint, error) {
	path := fmt.Sprintf("/0/public/OHLC?pair=%s&interval=%d", pair, interval)
	if since > 0 {
		path += fmt.Sprintf("&since=%d", since)
	}

	body, err := c.http.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("fetch OHLC %s: %w", pair, err)
	}

	var resp struct {
		Error  []string                   `json:"error"`
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(resp.Error) > 0 {
		return nil, fmt.Errorf("kraken API error: %v", resp.Error)
	}

	var rawCandles json.RawMessage
	for k, v := range resp.Result {
		if k == "last" {
			continue
		}
		rawCandles = v
		break
	}
	if rawCandles == nil {
		return nil, fmt.Errorf("no candle data in response for %s", pair)
	}

	var candles [][]json.RawMessage
	if err := json.Unmarshal(rawCandles, &candles); err != nil {
		return nil, fmt.Errorf("decode candles: %w", err)
	}

	out := make([]OHLCPoint, 0, len(candles))
	for i, cdl := range candles {
		if len(cdl) < 7 {
			continue
		}
		var ts int64
		if err := json.Unmarshal(cdl[0], &ts); err != nil {
			continue
		}
		out = append(out, OHLCPoint{
			Date:        time.Unix(ts, 0).UTC().Truncate(24 * time.Hour),
			Pair:        pair,
			Open:        parseJSONFloat(cdl[1]),
			High:        parseJSONFloat(cdl[2]),
			Low:         parseJSONFloat(cdl[3]),
			Close:       parseJSONFloat(cdl[4]),
			Volume:      parseJSONFloat(cdl[6]),
			PayloadHash: hashElement(rawCandles, i),
		})
	}
	return out, nil
}

func parseJSONFloat(raw json.RawMessage) float64 {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	var f float64
	_ = json.Unmarshal(raw, &f)
	return f
}

func hashElement(body json.RawMessage, i int) string {
	var arr []json.RawMessage
	if json.Unmarshal(body, &arr) == nil && i < len(arr) {
		s := sha256.Sum256(arr[i])
		return hex.EncodeToString(s[:])
	}
	s := sha256.Sum256([]byte(strconv.Itoa(i)))
	return hex.EncodeToString(s[:])
}
