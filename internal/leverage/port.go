package leverage

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
	SaveExchangeOI(ctx context.Context, rows []ExchangeOIRow) error
	SaveExchangeFunding(ctx context.Context, rows []ExchangeFundingRow) error
	SaveCoinglassFuturesBasis(ctx context.Context, rows []CoinglassFuturesBasisRow) error
	SaveDeribitBasis(ctx context.Context, row DeribitBasisRow) error
	SaveExchangeLiquidations(ctx context.Context, rows []ExchangeLiquidationsRow) error

	AggregatedOIAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error)

	CoinglassAggregatedOIAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error)

	DailyAvgFundingAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error)

	DailyTotalLiquidationsAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error)

	CoinglassAggregatedLiquidationsAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error)

	GetCoinglassFuturesBasisAsOf(ctx context.Context, symbol, exchange string, valueDate, cutoff time.Time) (float64, bool, error)

	GetDeribitBasisAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error)

	GetMarketCapAsOf(ctx context.Context, coinID string, valueDate, cutoff time.Time) (float64, error)

	SaveCoinglassLSRatio(ctx context.Context, kind LSRatioKind, rows []CoinglassLSRatioRow) error
	SaveCoinglassTakerVolume(ctx context.Context, rows []CoinglassTakerVolumeRow) error
	SaveCoinglassBorrowRate(ctx context.Context, rows []CoinglassBorrowRateRow) error

	CoinglassLSRatioAvgAsOf(ctx context.Context, kind LSRatioKind, symbol string, valueDate, cutoff time.Time) (float64, bool, error)

	CoinglassTakerVolumeAsOf(ctx context.Context, coinSymbol string, valueDate, cutoff time.Time) (buy, sell float64, ok bool, err error)
}

type LSRatioKind string

const (
	LSGlobal      LSRatioKind = "global"
	LSTopAccount  LSRatioKind = "top_account"
	LSTopPosition LSRatioKind = "top_position"
)

type OIProvider interface {
	FetchOpenInterest(ctx context.Context, asset domain.Asset, from, to time.Time) ([]OIPoint, error)
}

type FundingProvider interface {
	FetchFundingRateHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]FundingPoint, error)
}

type BasisProvider interface {
	FetchBasis3mSnapshot(ctx context.Context, asset domain.Asset) (BasisSnapshotPoint, error)
}

type CoinglassOIProvider interface {
	FetchOIHistory(ctx context.Context, asset domain.Asset, exchange string, limit int) ([]CoinglassOIPoint, error)

	FetchAggregatedOIHistory(ctx context.Context, asset domain.Asset, limit int) ([]CoinglassOIPoint, error)

	FetchAggregatedOIHistoryRange(ctx context.Context, asset domain.Asset, start, end time.Time, limit int) ([]CoinglassOIPoint, error)
}

type CoinglassBasisProvider interface {
	FetchFuturesBasisHistory(ctx context.Context, asset domain.Asset, exchange string, limit int) ([]CoinglassBasisPoint, error)
}

type CoinglassCrowdProvider interface {
	FetchLSRatio(ctx context.Context, kind LSRatioKind, asset domain.Asset, exchange string, limit int) ([]CoinglassLSRatioPoint, error)
	FetchAggregatedTakerVolume(ctx context.Context, asset domain.Asset, exchanges []string, limit int) ([]CoinglassTakerVolumePoint, error)
	FetchBorrowRate(ctx context.Context, symbol, exchange string, limit int) ([]CoinglassBorrowRatePoint, error)
}

type CoinglassLiqProvider interface {
	FetchLiquidationHistory(ctx context.Context, asset domain.Asset, exchange string, limit int) ([]CoinglassLiqPoint, error)

	FetchAggregatedLiquidationHistory(ctx context.Context, asset domain.Asset, exchanges []string, limit int) ([]CoinglassLiqPoint, error)

	FetchAggregatedLiquidationHistoryRange(ctx context.Context, asset domain.Asset, exchanges []string, start, end time.Time, limit int) ([]CoinglassLiqPoint, error)
}

type Publisher interface {
	Publish(ctx context.Context, ev domain.Event) error
}

type Clock interface {
	Now() time.Time
}

type ExchangeOIRow struct {
	ValueDate     time.Time
	Asset         domain.Asset
	Exchange      string
	OIUSD         float64
	SourceVersion string
	PayloadHash   string
}

type ExchangeFundingRow struct {
	FundingTime   time.Time
	Asset         domain.Asset
	Exchange      string
	Rate          float64
	SourceVersion string
	PayloadHash   string
}

type CoinglassFuturesBasisRow struct {
	ValueDate          time.Time
	Symbol             string
	Exchange           string
	AnnualizedBasisPct float64
	CloseBasis         float64
	SourceVersion      string
	PayloadHash        string
}

type DeribitBasisRow struct {
	ValueDate       time.Time
	Asset           domain.Asset
	InstrumentName  string
	FuturesPrice    float64
	SpotPrice       float64
	AnnualizedBasis float64
	DaysToExpiry    int
	SourceVersion   string
	PayloadHash     string
}

type ExchangeLiquidationsRow struct {
	ValueDate     time.Time
	Asset         domain.Asset
	Exchange      string
	LongLiqsUSD   float64
	ShortLiqsUSD  float64
	SourceVersion string
	PayloadHash   string
}

type OIPoint struct {
	Date        time.Time
	Asset       domain.Asset
	OIUSD       float64
	PayloadHash string
}

type FundingPoint struct {
	Timestamp   time.Time
	Asset       domain.Asset
	Rate        float64
	PayloadHash string
}

type BasisSnapshotPoint struct {
	Date            time.Time
	Asset           domain.Asset
	InstrumentName  string
	FuturesPrice    float64
	SpotPrice       float64
	AnnualizedBasis float64
	DaysToExpiry    int
	PayloadHash     string
}

type CoinglassOIPoint struct {
	Date        time.Time
	Asset       domain.Asset
	Exchange    string
	OIUSD       float64
	PayloadHash string
}

type CoinglassBasisPoint struct {
	Date               time.Time
	Asset              domain.Asset
	Symbol             string
	Exchange           string
	AnnualizedBasisPct float64
	CloseBasis         float64
	PayloadHash        string
}

type CoinglassLiqPoint struct {
	Date         time.Time
	Asset        domain.Asset
	LongLiqsUSD  float64
	ShortLiqsUSD float64
	PayloadHash  string
}

type CoinglassLSRatioPoint struct {
	Date         time.Time
	Asset        domain.Asset
	Exchange     string
	LongPercent  float64
	ShortPercent float64
	Ratio        float64
	PayloadHash  string
}

type CoinglassLSRatioRow struct {
	ValueDate     time.Time
	Symbol        string
	Exchange      string
	LongPercent   float64
	ShortPercent  float64
	Ratio         float64
	SourceVersion string
	PayloadHash   string
}

type CoinglassTakerVolumePoint struct {
	Date          time.Time
	Asset         domain.Asset
	BuyVolumeUSD  float64
	SellVolumeUSD float64
	PayloadHash   string
}

type CoinglassTakerVolumeRow struct {
	ValueDate     time.Time
	Symbol        string
	BuyVolumeUSD  float64
	SellVolumeUSD float64
	SourceVersion string
	PayloadHash   string
}

type CoinglassBorrowRatePoint struct {
	Date         time.Time
	Symbol       string
	Exchange     string
	InterestRate float64
	PayloadHash  string
}

type CoinglassBorrowRateRow struct {
	ValueDate     time.Time
	Symbol        string
	Exchange      string
	InterestRate  float64
	SourceVersion string
	PayloadHash   string
}
