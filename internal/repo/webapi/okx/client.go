package okx

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
	DefaultBaseURL = "https://www.okx.com"
	pageThrottle   = 200 * time.Millisecond
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
	InstID          string
	OpenInterestUSD float64
	PayloadHash     string
}

type FundingPoint struct {
	Timestamp   time.Time
	InstID      string
	Rate        float64
	PayloadHash string
}

type oiResponse struct {
	Code string     `json:"code"`
	Data [][]string `json:"data"`
}

func (c *Client) FetchOpenInterest(ctx context.Context, currency string) ([]OIPoint, error) {
	var out []OIPoint
	seen := make(map[int64]struct{})
	beginParam := ""

	for {
		path := fmt.Sprintf("/api/v5/rubik/stat/contracts/open-interest-volume?ccy=%s&period=1D", currency)
		if beginParam != "" {
			path += "&begin=" + beginParam
		}

		body, err := c.http.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("fetch open interest: %w", err)
		}
		var parsed oiResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("decode oi: %w", err)
		}
		if parsed.Code != "0" {
			return nil, fmt.Errorf("okx oi code %s: %s", parsed.Code, httpclient.Truncate(body, 200))
		}
		if len(parsed.Data) == 0 {
			break
		}

		pageCount := 0
		var oldestMs int64
		for _, row := range parsed.Data {
			if len(row) < 3 {
				continue
			}
			tsMs, err := strconv.ParseInt(row[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse ts %q: %w", row[0], err)
			}
			if _, dup := seen[tsMs]; dup {
				continue
			}
			seen[tsMs] = struct{}{}
			oi, err := strconv.ParseFloat(row[2], 64)
			if err != nil {
				return nil, fmt.Errorf("parse oi %q: %w", row[2], err)
			}
			out = append(out, OIPoint{
				Date:            time.UnixMilli(tsMs).UTC().Truncate(24 * time.Hour),
				InstID:          currency,
				OpenInterestUSD: oi,
				PayloadHash:     hashJSON(row),
			})
			pageCount++
			if oldestMs == 0 || tsMs < oldestMs {
				oldestMs = tsMs
			}
		}

		if pageCount == 0 {
			break
		}

		beginParam = strconv.FormatInt(oldestMs-1, 10)
	}
	return out, nil
}

type fundingRateRow struct {
	InstId      string `json:"instId"`
	FundingRate string `json:"fundingRate"`
	FundingTime string `json:"fundingTime"`
}

type fundingRateResponse struct {
	Code string           `json:"code"`
	Data []fundingRateRow `json:"data"`
}

func (c *Client) FetchFundingRateHistory(ctx context.Context,
	instId string, start, end time.Time,
) ([]FundingPoint, error) {
	startMs := start.UnixMilli()
	var (
		out    []FundingPoint
		cursor string
	)
	for {
		path := fmt.Sprintf("/api/v5/public/funding-rate-history?instId=%s&limit=100", instId)
		if cursor != "" {
			path += "&before=" + cursor
		}
		body, err := c.http.Get(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("fetch funding rate: %w", err)
		}
		var parsed fundingRateResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("decode funding: %w", err)
		}
		if parsed.Code != "0" {
			return nil, fmt.Errorf("okx funding code %s: %s", parsed.Code, httpclient.Truncate(body, 200))
		}
		if len(parsed.Data) == 0 {
			break
		}
		reachedStart := false
		for _, row := range parsed.Data {
			tsMs, err := strconv.ParseInt(row.FundingTime, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse fundingTime %q: %w", row.FundingTime, err)
			}
			if tsMs < startMs {
				reachedStart = true
				break
			}
			rate, err := strconv.ParseFloat(row.FundingRate, 64)
			if err != nil {
				return nil, fmt.Errorf("parse fundingRate %q: %w", row.FundingRate, err)
			}
			out = append(out, FundingPoint{
				Timestamp:   time.UnixMilli(tsMs).UTC(),
				InstID:      row.InstId,
				Rate:        rate,
				PayloadHash: hashJSON(row),
			})
		}
		if reachedStart {
			break
		}

		cursor = parsed.Data[len(parsed.Data)-1].FundingTime
	}
	return out, nil
}

func hashJSON(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
