package marketstress

import (
	"context"
	"time"

	"marketengine/internal/domain"
)

type FeatureRepo interface {
	Save(ctx context.Context, f domain.Feature) error
	GetLatest(ctx context.Context, key domain.FeatureKey, asset domain.Asset, valueDate, cutoff time.Time) (float64, error)
	GetSeries(ctx context.Context, key domain.FeatureKey, asset domain.Asset, from, to, cutoff time.Time) ([]float64, error)
}

type ScoreRepo interface {
	Save(ctx context.Context, s domain.DomainScore) error
}

type RawRepo interface {
	SaveBinanceKlines(ctx context.Context, rows []BinanceKlineRow) error
	SaveKrakenOHLC(ctx context.Context, rows []KrakenOHLCRow) error
	SaveCoinbaseCandles(ctx context.Context, rows []CoinbaseCandleRow) error
	SaveCoinglassCoinbasePremium(ctx context.Context, rows []CoinglassCoinbasePremiumRow) error

	GetBinanceKlineCloseSeries(ctx context.Context, symbol string, from, to, cutoff time.Time) ([]float64, error)

	GetBinanceKlineCloseAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, error)

	GetKrakenCloseAsOf(ctx context.Context, pair string, valueDate, cutoff time.Time) (float64, error)

	GetCoinbaseCloseAsOf(ctx context.Context, productID string, valueDate, cutoff time.Time) (float64, error)

	GetCoinglassCoinbasePremiumRateAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, error)

	SaveCoinglassOrderbookAggregated(ctx context.Context, rows []CoinglassOrderbookRow) error
	SaveCoinglassFuturesSpotVolRatio(ctx context.Context, rows []CoinglassFuturesSpotVolRatioRow) error

	GetOrderbookImbalanceAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error)
	GetFuturesSpotRatioAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error)
}

type BinanceSpotProvider interface {
	FetchKlines(ctx context.Context, symbol, interval string, start, end time.Time) ([]BinanceKlinePoint, error)
}

type KrakenProvider interface {
	FetchOHLC(ctx context.Context, pair string, intervalMinutes int, since int64) ([]KrakenOHLCPoint, error)
}

type CoinbaseProvider interface {
	FetchCandles(ctx context.Context, productID string, start, end time.Time, granularitySec int) ([]CoinbaseCandlePoint, error)
}

type CoinglassProvider interface {
	FetchCoinbasePremiumHistory(ctx context.Context, limit int) ([]CoinbasePremiumPoint, error)
}

type CoinglassMicroProvider interface {
	FetchOrderbookBidAsk(ctx context.Context, coinSymbol string, limit int, start, end time.Time) ([]CoinglassOrderbookPoint, error)
	FetchFuturesSpotVolRatio(ctx context.Context, coinSymbol string, limit int, start, end time.Time) ([]CoinglassFuturesSpotVolRatioPoint, error)
}

type LeverageFeatureReader interface {
	GetBasis3mDailyAnyVersion(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, error)
}

type Publisher interface {
	Publish(ctx context.Context, ev domain.Event) error
}

type Clock interface {
	Now() time.Time
}

type BinanceKlineRow struct {
	ValueDate     time.Time
	Symbol        string
	Close         float64
	Volume        float64
	SourceVersion string
	PayloadHash   string
}

type BinanceKlinePoint struct {
	OpenTime    time.Time
	Close       float64
	PayloadHash string
}

type KrakenOHLCRow struct {
	ValueDate     time.Time
	Pair          string
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Volume        float64
	SourceVersion string
	PayloadHash   string
}

type KrakenOHLCPoint struct {
	Date        time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      float64
	PayloadHash string
}

type CoinbaseCandleRow struct {
	ValueDate     time.Time
	ProductID     string
	Close         float64
	Volume        float64
	SourceVersion string
	PayloadHash   string
}

type CoinbaseCandlePoint struct {
	Date        time.Time
	ProductID   string
	Close       float64
	Volume      float64
	PayloadHash string
}

type CoinglassCoinbasePremiumRow struct {
	ValueDate     time.Time
	PremiumUSD    float64
	PremiumRate   float64
	CoinbasePrice float64
	SourceVersion string
	PayloadHash   string
}

type CoinbasePremiumPoint struct {
	Date          time.Time
	PremiumUSD    float64
	PremiumRate   float64
	CoinbasePrice float64
	PayloadHash   string
}

type CoinglassOrderbookPoint struct {
	Date        time.Time
	Symbol      string
	BidsUSD     float64
	BidsQty     float64
	AsksUSD     float64
	AsksQty     float64
	PayloadHash string
}

type CoinglassOrderbookRow struct {
	ValueDate     time.Time
	Symbol        string
	RangePct      string
	BidsUSD       float64
	BidsQty       float64
	AsksUSD       float64
	AsksQty       float64
	SourceVersion string
	PayloadHash   string
}

type CoinglassFuturesSpotVolRatioPoint struct {
	Date             time.Time
	Symbol           string
	FuturesSpotRatio float64
	FuturesVolUSD    float64
	SpotVolUSD       float64
	PayloadHash      string
}

type CoinglassFuturesSpotVolRatioRow struct {
	ValueDate        time.Time
	Symbol           string
	FuturesSpotRatio float64
	FuturesVolUSD    float64
	SpotVolUSD       float64
	SourceVersion    string
	PayloadHash      string
}
