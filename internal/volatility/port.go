package volatility

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
	SaveDVOL(ctx context.Context, rows []DVOLRow) error
	GetDVOLCloseAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, error)

	RealizedVol30d(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error)

	SaveCoinglassOptionsInfo(ctx context.Context, rows []CoinglassOptionsInfoRow) error
	SaveCoinglassOptionsOIHistory(ctx context.Context, rows []CoinglassOptionsOIHistoryRow) error
	SaveCoinglassOptionsMaxPain(ctx context.Context, rows []CoinglassOptionsMaxPainRow) error

	GetCoinglassOptionsAggregatedOIAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error)
	GetCoinglassOptionsPutCallRatioAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error)
	GetCoinglassOptionsMaxPainNearestAsOf(ctx context.Context, symbol, exchange string, valueDate, cutoff time.Time) (float64, bool, error)

	SpotCloseAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error)

	SaveDeribitOptionsChain(ctx context.Context, rows []DeribitOptionsChainRow) error

	GetDeribitOptionsChainAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) ([]DeribitOptionsChainSnapshot, error)
}

type DeribitChainProvider interface {
	FetchOptionsChain(ctx context.Context, asset domain.Asset) ([]DeribitOptionsChainSnapshot, error)
}

type CoinglassOptionsProvider interface {
	FetchOptionsInfo(ctx context.Context, symbol string) ([]CoinglassOptionsInfoPoint, error)
	FetchOptionsOIHistory(ctx context.Context, symbol string) ([]CoinglassOptionsOIHistoryPoint, error)
	FetchOptionsMaxPain(ctx context.Context, symbol, exchange string) ([]CoinglassOptionsMaxPainPoint, error)
}

type DvolProvider interface {
	FetchDVOL(ctx context.Context, asset domain.Asset, start, end time.Time) ([]DVOLPoint, error)
}

type OptionsChainProvider interface {
	FetchOptionsSnapshot(ctx context.Context, asset domain.Asset) (OptionsSnapshot, error)
}

type Publisher interface {
	Publish(ctx context.Context, ev domain.Event) error
}

type Clock interface {
	Now() time.Time
}

type DVOLRow struct {
	ValueDate     time.Time
	Asset         domain.Asset
	DVOLClose     float64
	SourceVersion string
	PayloadHash   string
}

type DVOLPoint struct {
	Date        time.Time
	Asset       domain.Asset
	Close       float64
	PayloadHash string
}

type OptionsSnapshot struct {
	Asset        domain.Asset
	TermSlope    float64
	HasTermSlope bool
	Skew         float64
	HasSkew      bool
	NumOptions   int
}

type CoinglassOptionsInfoPoint struct {
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

type CoinglassOptionsInfoRow struct {
	ValueDate        time.Time
	Symbol           string
	Exchange         string
	OpenInterest     float64
	OIMarketShare    float64
	OIChange24h      float64
	OpenInterestUSD  float64
	VolumeUSD24h     float64
	VolumeChangePct  float64
	CallOpenInterest float64
	PutOpenInterest  float64
	SourceVersion    string
	PayloadHash      string
}

type CoinglassOptionsOIHistoryPoint struct {
	Date         time.Time
	Exchange     string
	Symbol       string
	OpenInterest float64
	PayloadHash  string
}

type CoinglassOptionsOIHistoryRow struct {
	ValueDate     time.Time
	Symbol        string
	Exchange      string
	OpenInterest  float64
	SourceVersion string
	PayloadHash   string
}

type CoinglassOptionsMaxPainPoint struct {
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

type CoinglassOptionsMaxPainRow struct {
	ValueDate         time.Time
	ExpiryDate        time.Time
	Symbol            string
	Exchange          string
	MaxPainPrice      float64
	CallOIContracts   float64
	PutOIContracts    float64
	CallOINotionalUSD float64
	PutOINotionalUSD  float64
	CallMarketValue   float64
	PutMarketValue    float64
	SourceVersion     string
	PayloadHash       string
}

type DeribitOptionsChainSnapshot struct {
	InstrumentName     string
	ExpiryDate         time.Time
	StrikePrice        float64
	IsPut              bool
	OpenInterest       float64
	MarkIVPct          float64
	UnderlyingPriceUSD float64
	PayloadHash        string
}

type DeribitOptionsChainRow struct {
	ValueDate          time.Time
	Asset              domain.Asset
	InstrumentName     string
	ExpiryDate         time.Time
	StrikePrice        float64
	IsPut              bool
	OpenInterest       float64
	MarkIVPct          float64
	UnderlyingPriceUSD float64
	SourceVersion      string
	PayloadHash        string
}
