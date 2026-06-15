package coinglass

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"marketengine/pkg/httpclient"
)

const baseURL = "https://open-api-v4.coinglass.com/api"

type OIHistoryPoint struct {
	Date        time.Time
	Symbol      string
	Exchange    string
	CloseOIUSD  float64
	PayloadHash string
}

type LiquidationPoint struct {
	Date         time.Time
	Symbol       string
	LongLiqsUSD  float64
	ShortLiqsUSD float64
	PayloadHash  string
}

type ETFFlowPoint struct {
	Date         time.Time
	TotalFlowUSD float64
	PriceUSD     float64
	PayloadHash  string
}

type FundingOHLCPoint struct {
	Time        time.Time
	Symbol      string
	Close       float64
	PayloadHash string
}

type LongShortRatioPoint struct {
	Date         time.Time
	Symbol       string
	Exchange     string
	LongPercent  float64
	ShortPercent float64
	Ratio        float64
	PayloadHash  string
}

type TakerVolumePoint struct {
	Date          time.Time
	Symbol        string
	BuyVolumeUSD  float64
	SellVolumeUSD float64
	PayloadHash   string
}

type BorrowRatePoint struct {
	Date         time.Time
	Symbol       string
	Exchange     string
	InterestRate float64
	PayloadHash  string
}

type StablecoinMcapPoint struct {
	Date        time.Time
	MarketCap   float64
	PriceUSD    float64
	PayloadHash string
}

type ExchangeBalanceSnapshot struct {
	Exchange            string
	Symbol              string
	TotalBalance        float64
	BalanceChange1d     float64
	BalanceChange7d     float64
	BalanceChange30d    float64
	BalanceChangePct1d  float64
	BalanceChangePct7d  float64
	BalanceChangePct30d float64
	PayloadHash         string
}

type BitfinexMarginPoint struct {
	Time        time.Time
	Symbol      string
	LongQty     float64
	ShortQty    float64
	PayloadHash string
}

type OptionsInfoSnapshot struct {
	Exchange         string
	Symbol           string
	OpenInterest     float64
	OIMarketShare    float64
	OIChange24h      float64
	OpenInterestUSD  float64
	VolumeUSD24h     float64
	VolumeChangePct  float64
	CallOpenInterest float64
	PutOpenInterest  float64
	PayloadHash      string
}

type OptionsExchangeOIPoint struct {
	Date         time.Time
	Exchange     string
	Symbol       string
	OpenInterest float64
	PayloadHash  string
}

type BitcoinETFListItem struct {
	Ticker             string
	FundName           string
	Region             string
	MarketStatus       string
	PrimaryExchange    string
	FundType           string
	SharesOutstanding  float64
	AUMUSD             float64
	ManagementFeePct   float64
	VolumeUSD          float64
	PriceChangePct     float64
	NetAssetValueUSD   float64
	PremiumDiscountPct float64
	HoldingQuantity    float64
	ChangePct24h       float64
	ChangeQty24h       float64
	ChangePct7d        float64
	ChangeQty7d        float64
	UpdateDate         string
	PayloadHash        string
}

type BitcoinETFAUMPoint struct {
	Date        time.Time
	Ticker      string
	AUMUSD      float64
	PayloadHash string
}

type AggregatedOrderbookPoint struct {
	Date        time.Time
	Symbol      string
	BidsUSD     float64
	BidsQty     float64
	AsksUSD     float64
	AsksQty     float64
	PayloadHash string
}

type FuturesSpotVolumeRatioPoint struct {
	Date             time.Time
	Symbol           string
	FuturesSpotRatio float64
	FuturesVolUSD    float64
	SpotVolUSD       float64
	PayloadHash      string
}

type OptionsMaxPainPoint struct {
	Date              time.Time
	Symbol            string
	Exchange          string
	MaxPainPrice      float64
	CallOIContracts   float64
	PutOIContracts    float64
	CallOINotionalUSD float64
	PutOINotionalUSD  float64
	CallMarketValue   float64
	PutMarketValue    float64
	PayloadHash       string
}

type Client struct {
	http *httpclient.Client
}

func New(apiKey string, timeout time.Duration) *Client {
	return &Client{
		http: httpclient.New(
			httpclient.WithBaseURL(baseURL),
			httpclient.WithTimeout(timeout),
			httpclient.WithHeader("CG-API-KEY", apiKey),
		),
	}
}

type v4Response struct {
	Code string          `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  string          `json:"msg"`
}

func (c *Client) fetch(ctx context.Context, path string) (json.RawMessage, error) {
	body, err := c.http.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("coinglass: %w", err)
	}
	var env v4Response
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("coinglass: decode envelope: %w", err)
	}
	if env.Code != "0" {
		return nil, fmt.Errorf("coinglass API error code=%s msg=%s", env.Code, env.Msg)
	}
	return env.Data, nil
}

type oiHistoryRow struct {
	Time  int64       `json:"time"`
	Close json.Number `json:"close"`
}

func (c *Client) FetchOIHistory(ctx context.Context, symbol, exchange string, limit int) ([]OIHistoryPoint, error) {
	path := fmt.Sprintf("/futures/open-interest/history?symbol=%s&exchange=%s&interval=1d&limit=%d",
		symbol, exchange, limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("OI history: %w", err)
	}
	var rows []oiHistoryRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode OI history: %w", err)
	}

	out := make([]OIHistoryPoint, 0, len(rows))
	for _, r := range rows {
		closeVal, _ := r.Close.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("%s:%s:%d:%s", symbol, exchange, r.Time, r.Close.String()))
		out = append(out, OIHistoryPoint{
			Date:        time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:      symbol,
			Exchange:    exchange,
			CloseOIUSD:  closeVal,
			PayloadHash: hash,
		})
	}
	return out, nil
}

type liqHistoryRow struct {
	Time             int64       `json:"time"`
	LongLiquidation  json.Number `json:"long_liquidation_usd"`
	ShortLiquidation json.Number `json:"short_liquidation_usd"`
}

func (c *Client) FetchLiquidationHistory(ctx context.Context, symbol, exchange string, limit int) ([]LiquidationPoint, error) {
	path := fmt.Sprintf("/futures/liquidation/history?symbol=%s&exchange=%s&interval=1d&limit=%d",
		symbol, exchange, limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("liquidation history: %w", err)
	}
	var rows []liqHistoryRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode liquidation: %w", err)
	}

	out := make([]LiquidationPoint, 0, len(rows))
	for _, r := range rows {
		longVal, _ := r.LongLiquidation.Float64()
		shortVal, _ := r.ShortLiquidation.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("%s:%s:%d:%s:%s",
			symbol, exchange, r.Time, r.LongLiquidation.String(), r.ShortLiquidation.String()))
		out = append(out, LiquidationPoint{
			Date:         time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:       symbol,
			LongLiqsUSD:  longVal,
			ShortLiqsUSD: shortVal,
			PayloadHash:  hash,
		})
	}
	return out, nil
}

type aggregatedLiqRow struct {
	Time            int64       `json:"time"`
	AggregatedLong  json.Number `json:"aggregated_long_liquidation_usd"`
	AggregatedShort json.Number `json:"aggregated_short_liquidation_usd"`
}

func (c *Client) FetchAggregatedLiquidationHistory(
	ctx context.Context,
	coinSymbol string,
	exchanges []string,
	limit int,
	start, end time.Time,
) ([]LiquidationPoint, error) {
	path := fmt.Sprintf("/futures/liquidation/aggregated-history?symbol=%s&exchange_list=%s&interval=1d&limit=%d",
		coinSymbol, strings.Join(exchanges, ","), limit)
	if !start.IsZero() {
		path += fmt.Sprintf("&start_time=%d", start.UnixMilli())
	}
	if !end.IsZero() {
		path += fmt.Sprintf("&end_time=%d", end.UnixMilli())
	}

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("aggregated liquidation history: %w", err)
	}
	var rows []aggregatedLiqRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode aggregated liquidation: %w", err)
	}

	out := make([]LiquidationPoint, 0, len(rows))
	for _, r := range rows {
		longVal, _ := r.AggregatedLong.Float64()
		shortVal, _ := r.AggregatedShort.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("aggliq:%s:%s:%d:%s:%s",
			coinSymbol, strings.Join(exchanges, ","), r.Time,
			r.AggregatedLong.String(), r.AggregatedShort.String()))
		out = append(out, LiquidationPoint{
			Date:         time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:       coinSymbol,
			LongLiqsUSD:  longVal,
			ShortLiqsUSD: shortVal,
			PayloadHash:  hash,
		})
	}
	return out, nil
}

func (c *Client) FetchAggregatedOIHistory(
	ctx context.Context,
	coinSymbol string,
	limit int,
	unit string,
	start, end time.Time,
) ([]OIHistoryPoint, error) {
	if unit == "" {
		unit = "usd"
	}
	path := fmt.Sprintf("/futures/open-interest/aggregated-history?symbol=%s&interval=1d&limit=%d&unit=%s",
		coinSymbol, limit, unit)
	if !start.IsZero() {
		path += fmt.Sprintf("&start_time=%d", start.UnixMilli())
	}
	if !end.IsZero() {
		path += fmt.Sprintf("&end_time=%d", end.UnixMilli())
	}

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("aggregated OI history: %w", err)
	}
	var rows []oiHistoryRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode aggregated OI: %w", err)
	}

	out := make([]OIHistoryPoint, 0, len(rows))
	for _, r := range rows {
		closeVal, _ := r.Close.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("aggoi:%s:%d:%s", coinSymbol, r.Time, r.Close.String()))
		out = append(out, OIHistoryPoint{
			Date:        time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:      coinSymbol,
			Exchange:    "aggregated",
			CloseOIUSD:  closeVal,
			PayloadHash: hash,
		})
	}
	return out, nil
}

type etfFlowRow struct {
	Timestamp int64   `json:"timestamp"`
	FlowUSD   float64 `json:"flow_usd"`
	PriceUSD  float64 `json:"price_usd"`
}

func (c *Client) FetchBTCETFFlows(ctx context.Context) ([]ETFFlowPoint, error) {
	return c.fetchETFFlows(ctx, "/etf/bitcoin/flow-history")
}

func (c *Client) FetchETHETFFlows(ctx context.Context) ([]ETFFlowPoint, error) {
	return c.fetchETFFlows(ctx, "/etf/ethereum/flow-history")
}

func (c *Client) fetchETFFlows(ctx context.Context, path string) ([]ETFFlowPoint, error) {
	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("ETF flows: %w", err)
	}
	var rows []etfFlowRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode ETF flow: %w", err)
	}

	out := make([]ETFFlowPoint, 0, len(rows))
	for _, r := range rows {
		hash := httpclient.SHA256(fmt.Sprintf("%d:%.2f:%.2f", r.Timestamp, r.FlowUSD, r.PriceUSD))
		out = append(out, ETFFlowPoint{
			Date:         time.UnixMilli(r.Timestamp).UTC().Truncate(24 * time.Hour),
			TotalFlowUSD: r.FlowUSD,
			PriceUSD:     r.PriceUSD,
			PayloadHash:  hash,
		})
	}
	return out, nil
}

type fundingOHLCRow struct {
	Time  int64       `json:"time"`
	Close json.Number `json:"close"`
}

func (c *Client) FetchFundingRateHistory(ctx context.Context, symbol, exchange string, limit int) ([]FundingOHLCPoint, error) {
	path := fmt.Sprintf("/futures/funding-rate/history?symbol=%s&exchange=%s&interval=8h&limit=%d",
		symbol, exchange, limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("funding rate history: %w", err)
	}
	var rows []fundingOHLCRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode funding rate: %w", err)
	}

	out := make([]FundingOHLCPoint, 0, len(rows))
	for _, r := range rows {
		closeVal, _ := r.Close.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("%s:%s:%d:%s", symbol, exchange, r.Time, r.Close.String()))
		out = append(out, FundingOHLCPoint{
			Time:        time.UnixMilli(r.Time).UTC(),
			Symbol:      symbol,
			Close:       closeVal,
			PayloadHash: hash,
		})
	}
	return out, nil
}

type LTHSupplyPoint struct {
	Timestamp   time.Time
	LTHSupply   float64
	PriceUSD    float64
	PayloadHash string
}

type lthSupplyRow struct {
	Timestamp            int64   `json:"timestamp"`
	LongTermHolderSupply float64 `json:"long_term_holder_supply"`
	Price                float64 `json:"price"`
}

func (c *Client) FetchLTHSupply(ctx context.Context, limit int) ([]LTHSupplyPoint, error) {
	path := fmt.Sprintf("/index/bitcoin-long-term-holder-supply?limit=%d", limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("LTH supply: %w", err)
	}
	var rows []lthSupplyRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode LTH supply: %w", err)
	}

	out := make([]LTHSupplyPoint, 0, len(rows))
	for _, r := range rows {
		hash := httpclient.SHA256(fmt.Sprintf("lth:%d:%.8f:%.2f", r.Timestamp, r.LongTermHolderSupply, r.Price))
		out = append(out, LTHSupplyPoint{
			Timestamp:   time.UnixMilli(r.Timestamp).UTC().Truncate(24 * time.Hour),
			LTHSupply:   r.LongTermHolderSupply,
			PriceUSD:    r.Price,
			PayloadHash: hash,
		})
	}
	return out, nil
}

type MarketDataPoint struct {
	Timestamp    time.Time
	Symbol       string
	PriceUSD     float64
	MarketCapUSD float64
	CircSupply   float64
	PayloadHash  string
}

type marketDataRow struct {
	Timestamp         int64       `json:"timestamp"`
	Price             json.Number `json:"price"`
	CirculatingSupply json.Number `json:"circulating_supply"`
	MarketCap         json.Number `json:"market_cap"`
}

func (c *Client) FetchMarketDataHistory(ctx context.Context, symbol string) ([]MarketDataPoint, error) {
	path := fmt.Sprintf("/coin/market-data-history?symbol=%s", symbol)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("market_data_history %s: %w", symbol, err)
	}
	var rows []marketDataRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode market_data_history %s: %w", symbol, err)
	}

	out := make([]MarketDataPoint, 0, len(rows))
	for _, r := range rows {
		price, _ := r.Price.Float64()
		mcap, _ := r.MarketCap.Float64()
		supply, _ := r.CirculatingSupply.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("mcap:%s:%d:%s:%s:%s",
			symbol, r.Timestamp, r.Price.String(), r.MarketCap.String(), r.CirculatingSupply.String()))
		out = append(out, MarketDataPoint{
			Timestamp:    time.UnixMilli(r.Timestamp).UTC().Truncate(24 * time.Hour),
			Symbol:       symbol,
			PriceUSD:     price,
			MarketCapUSD: mcap,
			CircSupply:   supply,
			PayloadHash:  hash,
		})
	}
	return out, nil
}

type BTCMarketCapPoint struct {
	Timestamp    time.Time
	MarketCapUSD float64
	PriceUSD     float64
	PayloadHash  string
}

func (c *Client) FetchBTCMarketCap(ctx context.Context, limit int) ([]BTCMarketCapPoint, error) {
	rows, err := c.FetchMarketDataHistory(ctx, "BTC")
	if err != nil {
		return nil, err
	}
	out := make([]BTCMarketCapPoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, BTCMarketCapPoint{
			Timestamp:    r.Timestamp,
			MarketCapUSD: r.MarketCapUSD,
			PriceUSD:     r.PriceUSD,
			PayloadHash:  r.PayloadHash,
		})
	}
	return out, nil
}

type CoinbasePremiumPoint struct {
	Date          time.Time
	PremiumUSD    float64
	PremiumRate   float64
	CoinbasePrice float64
	PayloadHash   string
}

type coinbasePremiumRow struct {
	Time          int64       `json:"time"`
	Premium       json.Number `json:"premium"`
	PremiumRate   json.Number `json:"premium_rate"`
	CoinbasePrice json.Number `json:"coinbase_price"`
}

func (c *Client) FetchCoinbasePremiumHistory(ctx context.Context, limit int) ([]CoinbasePremiumPoint, error) {
	path := fmt.Sprintf("/coinbase-premium-index?interval=1d&limit=%d", limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("coinbase premium: %w", err)
	}
	var rows []coinbasePremiumRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode coinbase premium: %w", err)
	}

	out := make([]CoinbasePremiumPoint, 0, len(rows))
	for _, r := range rows {
		premiumVal, _ := r.Premium.Float64()
		premiumRateVal, _ := r.PremiumRate.Float64()
		cbPriceVal, _ := r.CoinbasePrice.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("cbprem:%d:%s:%s:%s",
			r.Time, r.Premium.String(), r.PremiumRate.String(), r.CoinbasePrice.String()))

		out = append(out, CoinbasePremiumPoint{
			Date:          time.Unix(r.Time, 0).UTC().Truncate(24 * time.Hour),
			PremiumUSD:    premiumVal,
			PremiumRate:   premiumRateVal,
			CoinbasePrice: cbPriceVal,
			PayloadHash:   hash,
		})
	}
	return out, nil
}

type lsRatioRow struct {
	Time         int64       `json:"time"`
	LongPercent  json.Number `json:"long_percent,omitempty"`
	ShortPercent json.Number `json:"short_percent,omitempty"`
	LSRatio      json.Number `json:"long_short_ratio,omitempty"`

	GlobalLong  json.Number `json:"global_account_long_percent,omitempty"`
	GlobalShort json.Number `json:"global_account_short_percent,omitempty"`
	GlobalRatio json.Number `json:"global_account_long_short_ratio,omitempty"`

	TopAcctLong  json.Number `json:"top_account_long_percent,omitempty"`
	TopAcctShort json.Number `json:"top_account_short_percent,omitempty"`
	TopAcctRatio json.Number `json:"top_account_long_short_ratio,omitempty"`

	TopPosLong  json.Number `json:"top_position_long_percent,omitempty"`
	TopPosShort json.Number `json:"top_position_short_percent,omitempty"`
	TopPosRatio json.Number `json:"top_position_long_short_ratio,omitempty"`
}

func (c *Client) fetchLSRatio(
	ctx context.Context,
	endpointPath, symbol, exchange, kind string,
	limit int,
) ([]LongShortRatioPoint, error) {
	path := fmt.Sprintf("%s?exchange=%s&symbol=%s&interval=1d&limit=%d",
		endpointPath, exchange, symbol, limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("%s %s/%s: %w", kind, exchange, symbol, err)
	}
	var rows []lsRatioRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode %s: %w", kind, err)
	}

	out := make([]LongShortRatioPoint, 0, len(rows))
	for _, r := range rows {
		var longP, shortP, ratio json.Number
		switch kind {
		case "global":
			longP, shortP, ratio = r.GlobalLong, r.GlobalShort, r.GlobalRatio
		case "top_account":
			longP, shortP, ratio = r.TopAcctLong, r.TopAcctShort, r.TopAcctRatio
		case "top_position":
			longP, shortP, ratio = r.TopPosLong, r.TopPosShort, r.TopPosRatio
		default:
			longP, shortP, ratio = r.LongPercent, r.ShortPercent, r.LSRatio
		}
		lv, _ := longP.Float64()
		sv, _ := shortP.Float64()
		rv, _ := ratio.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("%s:%s:%s:%d:%s:%s:%s",
			kind, exchange, symbol, r.Time, longP.String(), shortP.String(), ratio.String()))
		out = append(out, LongShortRatioPoint{
			Date:         time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:       symbol,
			Exchange:     exchange,
			LongPercent:  lv,
			ShortPercent: sv,
			Ratio:        rv,
			PayloadHash:  hash,
		})
	}
	return out, nil
}

func (c *Client) FetchGlobalLongShortAccountRatio(ctx context.Context, symbol, exchange string, limit int) ([]LongShortRatioPoint, error) {
	return c.fetchLSRatio(ctx, "/futures/global-long-short-account-ratio/history", symbol, exchange, "global", limit)
}

func (c *Client) FetchTopLongShortAccountRatio(ctx context.Context, symbol, exchange string, limit int) ([]LongShortRatioPoint, error) {
	return c.fetchLSRatio(ctx, "/futures/top-long-short-account-ratio/history", symbol, exchange, "top_account", limit)
}

func (c *Client) FetchTopLongShortPositionRatio(ctx context.Context, symbol, exchange string, limit int) ([]LongShortRatioPoint, error) {
	return c.fetchLSRatio(ctx, "/futures/top-long-short-position-ratio/history", symbol, exchange, "top_position", limit)
}

type aggTakerRow struct {
	Time    int64       `json:"time"`
	BuyUSD  json.Number `json:"aggregated_buy_volume_usd"`
	SellUSD json.Number `json:"aggregated_sell_volume_usd"`
}

func (c *Client) FetchAggregatedTakerBuySellVolume(
	ctx context.Context,
	coinSymbol string,
	exchanges []string,
	limit int,
	start, end time.Time,
) ([]TakerVolumePoint, error) {
	path := fmt.Sprintf("/futures/aggregated-taker-buy-sell-volume/history?symbol=%s&exchange_list=%s&interval=1d&unit=usd&limit=%d",
		coinSymbol, strings.Join(exchanges, ","), limit)
	if !start.IsZero() {
		path += fmt.Sprintf("&start_time=%d", start.UnixMilli())
	}
	if !end.IsZero() {
		path += fmt.Sprintf("&end_time=%d", end.UnixMilli())
	}

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("aggregated taker volume: %w", err)
	}
	var rows []aggTakerRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode aggregated taker volume: %w", err)
	}

	out := make([]TakerVolumePoint, 0, len(rows))
	for _, r := range rows {
		bv, _ := r.BuyUSD.Float64()
		sv, _ := r.SellUSD.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("aggtaker:%s:%s:%d:%s:%s",
			coinSymbol, strings.Join(exchanges, ","), r.Time, r.BuyUSD.String(), r.SellUSD.String()))
		out = append(out, TakerVolumePoint{
			Date:          time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:        coinSymbol,
			BuyVolumeUSD:  bv,
			SellVolumeUSD: sv,
			PayloadHash:   hash,
		})
	}
	return out, nil
}

type borrowRateRow struct {
	Time         int64       `json:"time"`
	InterestRate json.Number `json:"interest_rate"`
}

func (c *Client) FetchBorrowInterestRate(ctx context.Context, symbol, exchange string, limit int) ([]BorrowRatePoint, error) {
	path := fmt.Sprintf("/borrow-interest-rate/history?exchange=%s&symbol=%s&interval=1d&limit=%d",
		exchange, symbol, limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("borrow rate %s/%s: %w", exchange, symbol, err)
	}
	var rows []borrowRateRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode borrow rate: %w", err)
	}

	out := make([]BorrowRatePoint, 0, len(rows))
	for _, r := range rows {
		rate, _ := r.InterestRate.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("borrow:%s:%s:%d:%s",
			exchange, symbol, r.Time, r.InterestRate.String()))

		var ts time.Time
		if r.Time > 1_000_000_000_000 {
			ts = time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour)
		} else {
			ts = time.Unix(r.Time, 0).UTC().Truncate(24 * time.Hour)
		}
		out = append(out, BorrowRatePoint{
			Date:         ts,
			Symbol:       symbol,
			Exchange:     exchange,
			InterestRate: rate,
			PayloadHash:  hash,
		})
	}
	return out, nil
}

type bitcoinETFListRow struct {
	Ticker            string    `json:"ticker"`
	FundName          string    `json:"fund_name"`
	Region            string    `json:"region"`
	MarketStatus      string    `json:"market_status"`
	PrimaryExchange   string    `json:"primary_exchange"`
	FundType          string    `json:"fund_type"`
	SharesOutstanding flexFloat `json:"shares_outstanding"`
	AUMUSD            flexFloat `json:"aum_usd"`
	ManagementFeePct  flexFloat `json:"management_fee_percent"`
	VolumeUSD         flexFloat `json:"volume_usd"`
	PriceChangePct    flexFloat `json:"price_change_percent"`
	AssetDetails      struct {
		NetAssetValueUSD   flexFloat `json:"net_asset_value_usd"`
		PremiumDiscountPct flexFloat `json:"premium_discount_percent"`
		HoldingQuantity    flexFloat `json:"holding_quantity"`
		ChangePct24h       flexFloat `json:"change_percent_24h"`
		ChangeQty24h       flexFloat `json:"change_quantity_24h"`
		ChangePct7d        flexFloat `json:"change_percent_7d"`
		ChangeQty7d        flexFloat `json:"change_quantity_7d"`
		UpdateDate         string    `json:"update_date"`
	} `json:"asset_details"`
}

func parseFloatTolerant(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

type flexFloat struct {
	Value float64
	Raw   string
}

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {

		s := string(b[1 : len(b)-1])
		f.Raw = s
		f.Value = parseFloatTolerant(s)
		return nil
	}
	f.Raw = string(b)
	v, err := strconv.ParseFloat(f.Raw, 64)
	if err != nil {
		return nil
	}
	f.Value = v
	return nil
}

func (c *Client) FetchBitcoinETFList(ctx context.Context) ([]BitcoinETFListItem, error) {
	data, err := c.fetch(ctx, "/etf/bitcoin/list")
	if err != nil {
		return nil, fmt.Errorf("etf list: %w", err)
	}
	var rows []bitcoinETFListRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode etf list: %w", err)
	}
	out := make([]BitcoinETFListItem, 0, len(rows))
	for _, r := range rows {
		shares := r.SharesOutstanding.Value
		aum := r.AUMUSD.Value
		fee := r.ManagementFeePct.Value
		vol := r.VolumeUSD.Value
		pchg := r.PriceChangePct.Value
		nav := r.AssetDetails.NetAssetValueUSD.Value
		pd := r.AssetDetails.PremiumDiscountPct.Value
		hq := r.AssetDetails.HoldingQuantity.Value
		cp24 := r.AssetDetails.ChangePct24h.Value
		cq24 := r.AssetDetails.ChangeQty24h.Value
		cp7 := r.AssetDetails.ChangePct7d.Value
		cq7 := r.AssetDetails.ChangeQty7d.Value
		hash := httpclient.SHA256(fmt.Sprintf("etflist:%s:%s:%s:%s",
			r.Ticker, r.AUMUSD.Raw,
			r.AssetDetails.HoldingQuantity.Raw,
			r.AssetDetails.UpdateDate))
		out = append(out, BitcoinETFListItem{
			Ticker:             r.Ticker,
			FundName:           r.FundName,
			Region:             r.Region,
			MarketStatus:       r.MarketStatus,
			PrimaryExchange:    r.PrimaryExchange,
			FundType:           r.FundType,
			SharesOutstanding:  shares,
			AUMUSD:             aum,
			ManagementFeePct:   fee,
			VolumeUSD:          vol,
			PriceChangePct:     pchg,
			NetAssetValueUSD:   nav,
			PremiumDiscountPct: pd,
			HoldingQuantity:    hq,
			ChangePct24h:       cp24,
			ChangeQty24h:       cq24,
			ChangePct7d:        cp7,
			ChangeQty7d:        cq7,
			UpdateDate:         r.AssetDetails.UpdateDate,
			PayloadHash:        hash,
		})
	}
	return out, nil
}

type bitcoinETFAUMRow struct {
	Time   int64       `json:"time"`
	AUMUSD json.Number `json:"aum_usd"`
}

func (c *Client) FetchBitcoinETFAUMHistory(ctx context.Context, ticker string) ([]BitcoinETFAUMPoint, error) {
	path := "/etf/bitcoin/aum"
	if ticker != "" {
		path = fmt.Sprintf("/etf/bitcoin/aum?ticker=%s", ticker)
	}
	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("etf aum %s: %w", ticker, err)
	}
	var rows []bitcoinETFAUMRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode etf aum: %w", err)
	}
	out := make([]BitcoinETFAUMPoint, 0, len(rows))
	for _, r := range rows {
		v, _ := r.AUMUSD.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("etfaum:%s:%d:%s", ticker, r.Time, r.AUMUSD.String()))
		out = append(out, BitcoinETFAUMPoint{
			Date:        time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Ticker:      ticker,
			AUMUSD:      v,
			PayloadHash: hash,
		})
	}
	return out, nil
}

type aggregatedOrderbookRow struct {
	Time         int64       `json:"time"`
	BidsUSD      json.Number `json:"aggregated_bids_usd"`
	BidsQuantity json.Number `json:"aggregated_bids_quantity"`
	AsksUSD      json.Number `json:"aggregated_asks_usd"`
	AsksQuantity json.Number `json:"aggregated_asks_quantity"`
}

func (c *Client) FetchAggregatedOrderbookBidAsk(
	ctx context.Context,
	coinSymbol, exchangeList, rangePct string,
	limit int,
	start, end time.Time,
) ([]AggregatedOrderbookPoint, error) {
	if exchangeList == "" {
		exchangeList = "Binance,OKX,Bybit"
	}
	if rangePct == "" {
		rangePct = "1"
	}
	path := fmt.Sprintf("/futures/orderbook/aggregated-ask-bids-history?symbol=%s&exchange_list=%s&interval=1d&range=%s&limit=%d",
		coinSymbol, exchangeList, rangePct, limit)
	if !start.IsZero() {
		path += fmt.Sprintf("&start_time=%d", start.UnixMilli())
	}
	if !end.IsZero() {
		path += fmt.Sprintf("&end_time=%d", end.UnixMilli())
	}

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("aggregated orderbook %s: %w", coinSymbol, err)
	}
	var rows []aggregatedOrderbookRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode aggregated orderbook: %w", err)
	}

	out := make([]AggregatedOrderbookPoint, 0, len(rows))
	for _, r := range rows {
		bUSD, _ := r.BidsUSD.Float64()
		bQty, _ := r.BidsQuantity.Float64()
		aUSD, _ := r.AsksUSD.Float64()
		aQty, _ := r.AsksQuantity.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("ob:%s:%s:%d:%s:%s",
			coinSymbol, exchangeList, r.Time, r.BidsUSD.String(), r.AsksUSD.String()))
		out = append(out, AggregatedOrderbookPoint{
			Date:        time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:      coinSymbol,
			BidsUSD:     bUSD,
			BidsQty:     bQty,
			AsksUSD:     aUSD,
			AsksQty:     aQty,
			PayloadHash: hash,
		})
	}
	return out, nil
}

type futuresSpotVolRatioRow struct {
	Time             int64       `json:"time"`
	FuturesSpotRatio json.Number `json:"futures_spot_vol_ratio"`
	FuturesVolUSD    json.Number `json:"futures_vol_usd"`
	SpotVolUSD       json.Number `json:"spot_vol_usd"`
}

func (c *Client) FetchFuturesSpotVolumeRatio(
	ctx context.Context,
	coinSymbol, exchangeList string,
	limit int,
	start, end time.Time,
) ([]FuturesSpotVolumeRatioPoint, error) {
	if exchangeList == "" {
		exchangeList = "Binance,OKX,Bybit"
	}
	path := fmt.Sprintf("/futures_spot_volume_ratio?symbol=%s&exchange_list=%s&interval=1d&limit=%d",
		coinSymbol, exchangeList, limit)
	if !start.IsZero() {
		path += fmt.Sprintf("&start_time=%d", start.UnixMilli())
	}
	if !end.IsZero() {
		path += fmt.Sprintf("&end_time=%d", end.UnixMilli())
	}

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("futures_spot_volume_ratio %s: %w", coinSymbol, err)
	}
	var rows []futuresSpotVolRatioRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode futures_spot_volume_ratio: %w", err)
	}

	out := make([]FuturesSpotVolumeRatioPoint, 0, len(rows))
	for _, r := range rows {
		ratio, _ := r.FuturesSpotRatio.Float64()
		fv, _ := r.FuturesVolUSD.Float64()
		sv, _ := r.SpotVolUSD.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("fsr:%s:%s:%d:%s:%s:%s",
			coinSymbol, exchangeList, r.Time,
			r.FuturesSpotRatio.String(), r.FuturesVolUSD.String(), r.SpotVolUSD.String()))
		out = append(out, FuturesSpotVolumeRatioPoint{
			Date:             time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:           coinSymbol,
			FuturesSpotRatio: ratio,
			FuturesVolUSD:    fv,
			SpotVolUSD:       sv,
			PayloadHash:      hash,
		})
	}
	return out, nil
}

type optionsInfoRow struct {
	ExchangeName           string      `json:"exchange_name"`
	OpenInterest           json.Number `json:"open_interest"`
	OIMarketShare          json.Number `json:"oi_market_share"`
	OpenInterestChange24h  json.Number `json:"open_interest_change_24h"`
	OpenInterestUSD        json.Number `json:"open_interest_usd"`
	VolumeUSD24h           json.Number `json:"volume_usd_24h"`
	VolumeChangePercent24h json.Number `json:"volume_change_percent_24h"`
	CallOpenInterest       json.Number `json:"call_open_interest"`
	PutOpenInterest        json.Number `json:"put_open_interest"`
}

func (c *Client) FetchOptionsInfo(ctx context.Context, symbol string) ([]OptionsInfoSnapshot, error) {
	path := fmt.Sprintf("/option/info?symbol=%s", symbol)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("options info %s: %w", symbol, err)
	}
	var rows []optionsInfoRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode options info: %w", err)
	}
	out := make([]OptionsInfoSnapshot, 0, len(rows))
	for _, r := range rows {
		oi, _ := r.OpenInterest.Float64()
		ms, _ := r.OIMarketShare.Float64()
		ch, _ := r.OpenInterestChange24h.Float64()
		ousd, _ := r.OpenInterestUSD.Float64()
		v24, _ := r.VolumeUSD24h.Float64()
		vchg, _ := r.VolumeChangePercent24h.Float64()
		callOI, _ := r.CallOpenInterest.Float64()
		putOI, _ := r.PutOpenInterest.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("optinfo:%s:%s:%s:%s:%s:%s",
			r.ExchangeName, symbol, r.OpenInterest.String(), r.OpenInterestUSD.String(),
			r.CallOpenInterest.String(), r.PutOpenInterest.String()))
		out = append(out, OptionsInfoSnapshot{
			Exchange:         r.ExchangeName,
			Symbol:           symbol,
			OpenInterest:     oi,
			OIMarketShare:    ms,
			OIChange24h:      ch,
			OpenInterestUSD:  ousd,
			VolumeUSD24h:     v24,
			VolumeChangePct:  vchg,
			CallOpenInterest: callOI,
			PutOpenInterest:  putOI,
			PayloadHash:      hash,
		})
	}
	return out, nil
}

type optionsHistoryEnvelope struct {
	TimeList  []int64                  `json:"time_list"`
	PriceList []json.Number            `json:"price_list"`
	DataMap   map[string][]json.Number `json:"data_map"`
}

func (c *Client) FetchOptionsExchangeOIHistory(ctx context.Context, symbol, unit, rng string) ([]OptionsExchangeOIPoint, error) {
	if unit == "" {
		unit = "USD"
	}
	if rng == "" {
		rng = "all"
	}
	path := fmt.Sprintf("/option/exchange-oi-history?symbol=%s&unit=%s&range=%s", symbol, unit, rng)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("options OI history %s: %w", symbol, err)
	}
	var env optionsHistoryEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode options OI history: %w", err)
	}
	if len(env.TimeList) == 0 {
		return nil, nil
	}
	var out []OptionsExchangeOIPoint
	for exch, series := range env.DataMap {
		for i, ts := range env.TimeList {
			if i >= len(series) {
				break
			}
			oi, _ := series[i].Float64()
			hash := httpclient.SHA256(fmt.Sprintf("optoi:%s:%s:%d:%s", exch, symbol, ts, series[i].String()))
			out = append(out, OptionsExchangeOIPoint{
				Date:         time.UnixMilli(ts).UTC().Truncate(24 * time.Hour),
				Exchange:     exch,
				Symbol:       symbol,
				OpenInterest: oi,
				PayloadHash:  hash,
			})
		}
	}
	return out, nil
}

type optionsMaxPainRow struct {
	Date                        string      `json:"date"`
	MaxPainPrice                json.Number `json:"max_pain_price"`
	CallOpenInterest            json.Number `json:"call_open_interest"`
	PutOpenInterest             json.Number `json:"put_open_interest"`
	CallOpenInterestNotional    json.Number `json:"call_open_interest_notional"`
	PutOpenInterestNotional     json.Number `json:"put_open_interest_notional"`
	CallOpenInterestMarketValue json.Number `json:"call_open_interest_market_value"`
	PutOpenInterestMarketValue  json.Number `json:"put_open_interest_market_value"`
}

func (c *Client) FetchOptionsMaxPain(ctx context.Context, symbol, exchange string) ([]OptionsMaxPainPoint, error) {
	path := fmt.Sprintf("/option/max-pain?symbol=%s&exchange=%s", symbol, exchange)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("options max pain %s/%s: %w", symbol, exchange, err)
	}
	var rows []optionsMaxPainRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode max pain: %w", err)
	}
	out := make([]OptionsMaxPainPoint, 0, len(rows))
	for _, r := range rows {
		mp, _ := r.MaxPainPrice.Float64()
		callOI, _ := r.CallOpenInterest.Float64()
		putOI, _ := r.PutOpenInterest.Float64()
		callN, _ := r.CallOpenInterestNotional.Float64()
		putN, _ := r.PutOpenInterestNotional.Float64()
		callMV, _ := r.CallOpenInterestMarketValue.Float64()
		putMV, _ := r.PutOpenInterestMarketValue.Float64()

		d, perr := time.Parse("060102", r.Date)
		if perr != nil {
			continue
		}
		hash := httpclient.SHA256(fmt.Sprintf("maxpain:%s:%s:%s:%s",
			symbol, exchange, r.Date, r.MaxPainPrice.String()))
		out = append(out, OptionsMaxPainPoint{
			Date:              d.UTC(),
			Symbol:            symbol,
			Exchange:          exchange,
			MaxPainPrice:      mp,
			CallOIContracts:   callOI,
			PutOIContracts:    putOI,
			CallOINotionalUSD: callN,
			PutOINotionalUSD:  putN,
			CallMarketValue:   callMV,
			PutMarketValue:    putMV,
			PayloadHash:       hash,
		})
	}
	return out, nil
}

type stablecoinMcapEnvelope struct {
	DataList  []map[string]json.Number `json:"data_list"`
	PriceList []json.Number            `json:"price_list"`
	TimeList  []int64                  `json:"time_list"`
}

func (c *Client) FetchStablecoinMarketCapHistory(ctx context.Context) ([]StablecoinMcapPoint, error) {
	data, err := c.fetch(ctx, "/index/stableCoin-marketCap-history")
	if err != nil {
		return nil, fmt.Errorf("stablecoin mcap: %w", err)
	}
	var env stablecoinMcapEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode stablecoin mcap: %w", err)
	}
	n := len(env.TimeList)
	if n == 0 {
		return nil, nil
	}
	out := make([]StablecoinMcapPoint, 0, n)
	for i := 0; i < n; i++ {
		var mcap, price float64
		if i < len(env.DataList) {
			for _, v := range env.DataList[i] {
				if f, err := v.Float64(); err == nil {
					mcap += f
				}
			}
		}
		if i < len(env.PriceList) {
			price, _ = env.PriceList[i].Float64()
		}

		ts := time.UnixMilli(env.TimeList[i]).UTC().Truncate(24 * time.Hour)
		hash := httpclient.SHA256(fmt.Sprintf("stablemcap:%d:%.4f:%.4f", env.TimeList[i], mcap, price))
		out = append(out, StablecoinMcapPoint{
			Date:        ts,
			MarketCap:   mcap,
			PriceUSD:    price,
			PayloadHash: hash,
		})
	}
	return out, nil
}

type exchangeBalanceRow struct {
	ExchangeName        string      `json:"exchange_name"`
	TotalBalance        json.Number `json:"total_balance"`
	BalanceChange1d     json.Number `json:"balance_change_1d"`
	BalanceChange7d     json.Number `json:"balance_change_7d"`
	BalanceChange30d    json.Number `json:"balance_change_30d"`
	BalanceChangePct1d  json.Number `json:"balance_change_percent_1d"`
	BalanceChangePct7d  json.Number `json:"balance_change_percent_7d"`
	BalanceChangePct30d json.Number `json:"balance_change_percent_30d"`
}

func (c *Client) FetchExchangeBalanceList(ctx context.Context, symbol string) ([]ExchangeBalanceSnapshot, error) {
	path := fmt.Sprintf("/exchange/balance/list?symbol=%s", symbol)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("exchange balance list %s: %w", symbol, err)
	}
	var rows []exchangeBalanceRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode exchange balance list: %w", err)
	}
	out := make([]ExchangeBalanceSnapshot, 0, len(rows))
	for _, r := range rows {
		tb, _ := r.TotalBalance.Float64()
		c1, _ := r.BalanceChange1d.Float64()
		c7, _ := r.BalanceChange7d.Float64()
		c30, _ := r.BalanceChange30d.Float64()
		p1, _ := r.BalanceChangePct1d.Float64()
		p7, _ := r.BalanceChangePct7d.Float64()
		p30, _ := r.BalanceChangePct30d.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("exbal:%s:%s:%s:%s:%s",
			r.ExchangeName, symbol, r.TotalBalance.String(),
			r.BalanceChange7d.String(), r.BalanceChange30d.String()))
		out = append(out, ExchangeBalanceSnapshot{
			Exchange:            r.ExchangeName,
			Symbol:              symbol,
			TotalBalance:        tb,
			BalanceChange1d:     c1,
			BalanceChange7d:     c7,
			BalanceChange30d:    c30,
			BalanceChangePct1d:  p1,
			BalanceChangePct7d:  p7,
			BalanceChangePct30d: p30,
			PayloadHash:         hash,
		})
	}
	return out, nil
}

type bitfinexMarginRow struct {
	Time          int64       `json:"time"`
	LongQuantity  json.Number `json:"long_quantity"`
	ShortQuantity json.Number `json:"short_quantity"`
}

func (c *Client) FetchBitfinexMarginLongShort(ctx context.Context, symbol string, limit int) ([]BitfinexMarginPoint, error) {
	path := fmt.Sprintf("/bitfinex-margin-long-short?symbol=%s&interval=1d&limit=%d", symbol, limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("bitfinex margin %s: %w", symbol, err)
	}
	var rows []bitfinexMarginRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode bitfinex margin: %w", err)
	}
	out := make([]BitfinexMarginPoint, 0, len(rows))
	for _, r := range rows {
		lv, _ := r.LongQuantity.Float64()
		sv, _ := r.ShortQuantity.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("bfxmargin:%s:%d:%s:%s",
			symbol, r.Time, r.LongQuantity.String(), r.ShortQuantity.String()))
		var ts time.Time
		if r.Time > 1_000_000_000_000 {
			ts = time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour)
		} else {
			ts = time.Unix(r.Time, 0).UTC().Truncate(24 * time.Hour)
		}
		out = append(out, BitfinexMarginPoint{
			Time:        ts,
			Symbol:      symbol,
			LongQty:     lv,
			ShortQty:    sv,
			PayloadHash: hash,
		})
	}
	return out, nil
}

type FuturesBasisPoint struct {
	Date               time.Time
	Symbol             string
	Exchange           string
	AnnualizedBasisPct float64
	CloseBasis         float64
	PayloadHash        string
}

type futuresBasisRow struct {
	Time        int64       `json:"time"`
	CloseBasis  json.Number `json:"close_basis"`
	CloseChange json.Number `json:"close_change"`
}

func (c *Client) FetchFuturesBasisHistory(ctx context.Context, symbol, exchange string, limit int) ([]FuturesBasisPoint, error) {
	path := fmt.Sprintf("/futures/basis/history?exchange=%s&symbol=%s&interval=1d&limit=%d",
		exchange, symbol, limit)

	data, err := c.fetch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("futures basis: %w", err)
	}
	var rows []futuresBasisRow
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decode futures basis: %w", err)
	}

	out := make([]FuturesBasisPoint, 0, len(rows))
	for _, r := range rows {
		closeBasisVal, _ := r.CloseBasis.Float64()
		closeChangeVal, _ := r.CloseChange.Float64()
		hash := httpclient.SHA256(fmt.Sprintf("basis:%s:%s:%d:%s:%s",
			symbol, exchange, r.Time, r.CloseBasis.String(), r.CloseChange.String()))

		out = append(out, FuturesBasisPoint{
			Date:               time.UnixMilli(r.Time).UTC().Truncate(24 * time.Hour),
			Symbol:             symbol,
			Exchange:           exchange,
			AnnualizedBasisPct: closeChangeVal,
			CloseBasis:         closeBasisVal,
			PayloadHash:        hash,
		})
	}
	return out, nil
}
