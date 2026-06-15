package liquidity

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
	SaveStablecoinSupply(ctx context.Context, rows []StablecoinSupplyRow) error
	SaveChainTVL(ctx context.Context, rows []ChainTVLRow) error
	SaveExchangeNetflow(ctx context.Context, rows []ExchangeNetflowRow) error
	SaveMarketCap(ctx context.Context, rows []MarketCapRow) error

	GetStablecoinSupplyAsOf(ctx context.Context, stablecoin string, valueDate, cutoff time.Time) (float64, error)

	SumStablecoinSupplyAsOf(ctx context.Context, symbols []string, valueDate, cutoff time.Time) (float64, int, error)
	GetChainTVLAsOf(ctx context.Context, chain string, valueDate, cutoff time.Time) (float64, error)

	Sum7dNetflow(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error)
	GetMarketCapAsOf(ctx context.Context, coinID string, valueDate, cutoff time.Time) (float64, error)
}

type StablecoinProvider interface {
	FetchAllStablecoinsChart(ctx context.Context) ([]StablecoinSupplyPoint, error)
	FetchPerStablecoinSupply(ctx context.Context) ([]StablecoinSupplyPoint, error)
}

type ChainTVLProvider interface {
	FetchChainTVL(ctx context.Context, chain string) ([]ChainTVLPoint, error)
}

type ExchangeNetflowProvider interface {
	FetchExchangeNetflow(ctx context.Context, assets []domain.Asset, start, end time.Time) ([]ExchangeNetflowPoint, error)
}

type MarketCapProvider interface {
	FetchMarketCapHistory(ctx context.Context, coinID string) ([]MarketCapPoint, error)
}

type Publisher interface {
	Publish(ctx context.Context, ev domain.Event) error
}

type Clock interface {
	Now() time.Time
}

type StablecoinSupplyRow struct {
	ValueDate     time.Time
	Stablecoin    string
	Metric        string
	Value         float64
	SourceVersion string
	PayloadHash   string
}

type StablecoinSupplyPoint struct {
	Date           time.Time
	Symbol         string
	CirculatingUSD float64
	PayloadHash    string
}

type ChainTVLRow struct {
	ValueDate     time.Time
	Chain         string
	TVLUSD        float64
	SourceVersion string
	PayloadHash   string
}

type ChainTVLPoint struct {
	Date        time.Time
	TVLUSD      float64
	PayloadHash string
}

type ExchangeNetflowRow struct {
	ValueDate     time.Time
	Asset         domain.Asset
	InflowUSD     *float64
	OutflowUSD    *float64
	NetflowUSD    *float64
	SourceVersion string
	PayloadHash   string
}

type ExchangeNetflowPoint struct {
	Date        time.Time
	Asset       domain.Asset
	InflowUSD   *float64
	OutflowUSD  *float64
	NetflowUSD  *float64
	PayloadHash string
}

type MarketCapRow struct {
	ValueDate     time.Time
	CoinID        string
	MarketCapUSD  float64
	PriceUSD      float64
	SourceVersion string
	PayloadHash   string
}

type MarketCapPoint struct {
	Date         time.Time
	CoinID       string
	MarketCapUSD float64
	PriceUSD     float64
	PayloadHash  string
}
