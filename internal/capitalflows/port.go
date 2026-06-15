package capitalflows

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
	SaveETFFlows(ctx context.Context, rows []ETFFlowRow) error
	SaveLTHSupply(ctx context.Context, rows []LTHSupplyRow) error
	SaveBTCMarketCap(ctx context.Context, rows []MarketCapRow) error

	CombinedETFFlowAsOf(ctx context.Context, valueDate, cutoff time.Time) (sum float64, ok bool, err error)

	GetLTHSupplyAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, error)

	SaveStablecoinMcap(ctx context.Context, rows []StablecoinMcapRow) error
	SaveExchangeBalance(ctx context.Context, rows []ExchangeBalanceRow) error
	SaveBitfinexMargin(ctx context.Context, rows []BitfinexMarginRow) error

	GetStablecoinMcapAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error)

	GetStablecoinMcapSeries(ctx context.Context, from, to, cutoff time.Time) ([]float64, error)

	GetExchangeBalanceChange7dSumAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error)

	GetExchangeBalanceChange30dSumAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error)

	GetBitfinexMarginAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (long, short float64, ok bool, err error)

	SaveETFListSnapshot(ctx context.Context, rows []ETFListItemRow) error
	SaveETFAUMHistory(ctx context.Context, rows []ETFAUMHistoryRow) error
	SaveOptionsMaxPainNearest(ctx context.Context, rows []OptionsMaxPainRow) error

	GetETFListAUMTotalAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error)

	GetETFListConcentrationHHIAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error)

	GetETFAUMHistoryTotalAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error)

	GetOptionsDealerSkewProxyAsOf(ctx context.Context, symbol, exchange string, valueDate, cutoff time.Time) (float64, bool, error)
}

type ETFFlowProvider interface {
	FetchBTCETFFlows(ctx context.Context) ([]ETFFlowPoint, error)
	FetchETHETFFlows(ctx context.Context) ([]ETFFlowPoint, error)
}

type LTHSupplyProvider interface {
	FetchLTHSupply(ctx context.Context) ([]LTHSupplyPoint, error)
}

type BTCMarketCapProvider interface {
	FetchBTCMarketCap(ctx context.Context) ([]MarketCapPoint, error)
}

type LiquidityFlowProvider interface {
	FetchStablecoinMcapHistory(ctx context.Context) ([]StablecoinMcapPoint, error)
	FetchExchangeBalanceList(ctx context.Context, symbol string) ([]ExchangeBalanceSnapshot, error)
	FetchBitfinexMargin(ctx context.Context, symbol string, limit int) ([]BitfinexMarginPoint, error)
}

type InstitutionalProvider interface {
	FetchETFList(ctx context.Context) ([]ETFListItemPoint, error)
	FetchETFAUMHistory(ctx context.Context, ticker string) ([]ETFAUMHistoryPoint, error)
	FetchOptionsMaxPain(ctx context.Context, symbol, exchange string) ([]OptionsMaxPainNearestPoint, error)
}

type Publisher interface {
	Publish(ctx context.Context, ev domain.Event) error
}

type Clock interface {
	Now() time.Time
}

type ETFFlowRow struct {
	ValueDate     time.Time
	FlowType      string
	TotalFlowUSD  float64
	PriceUSD      float64
	SourceVersion string
	PayloadHash   string
}

type ETFFlowPoint struct {
	Date         time.Time
	TotalFlowUSD float64
	PriceUSD     float64
	PayloadHash  string
}

type LTHSupplyRow struct {
	ValueDate     time.Time
	Asset         domain.Asset
	LTHSupply     float64
	SourceVersion string
	PayloadHash   string
}

type LTHSupplyPoint struct {
	Date        time.Time
	LTHSupply   float64
	PriceUSD    float64
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
	MarketCapUSD float64
	PriceUSD     float64
	PayloadHash  string
}

type StablecoinMcapPoint struct {
	Date        time.Time
	MarketCap   float64
	PriceUSD    float64
	PayloadHash string
}

type StablecoinMcapRow struct {
	ValueDate     time.Time
	MarketCap     float64
	PriceUSD      float64
	SourceVersion string
	PayloadHash   string
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

type ExchangeBalanceRow struct {
	ValueDate           time.Time
	Symbol              string
	Exchange            string
	TotalBalance        float64
	BalanceChange1d     float64
	BalanceChange7d     float64
	BalanceChange30d    float64
	BalanceChangePct1d  float64
	BalanceChangePct7d  float64
	BalanceChangePct30d float64
	SourceVersion       string
	PayloadHash         string
}

type BitfinexMarginPoint struct {
	Time        time.Time
	Symbol      string
	LongQty     float64
	ShortQty    float64
	PayloadHash string
}

type BitfinexMarginRow struct {
	ValueDate     time.Time
	Symbol        string
	LongQty       float64
	ShortQty      float64
	SourceVersion string
	PayloadHash   string
}

type ETFListItemPoint struct {
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

type ETFListItemRow struct {
	ValueDate          time.Time
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
	SourceVersion      string
	PayloadHash        string
}

type ETFAUMHistoryPoint struct {
	Date        time.Time
	Ticker      string
	AUMUSD      float64
	PayloadHash string
}

type ETFAUMHistoryRow struct {
	ValueDate     time.Time
	Ticker        string
	AUMUSD        float64
	SourceVersion string
	PayloadHash   string
}

type OptionsMaxPainNearestPoint struct {
	ExpiryDate         time.Time
	Symbol             string
	Exchange           string
	MaxPainPrice       float64
	CallOIContracts    float64
	PutOIContracts     float64
	CallOINotionalUSD  float64
	PutOINotionalUSD   float64
	CallMarketValueUSD float64
	PutMarketValueUSD  float64
	PayloadHash        string
}

type OptionsMaxPainRow struct {
	ValueDate          time.Time
	ExpiryDate         time.Time
	Symbol             string
	Exchange           string
	MaxPainPrice       float64
	CallOIContracts    float64
	PutOIContracts     float64
	CallOINotionalUSD  float64
	PutOINotionalUSD   float64
	CallMarketValueUSD float64
	PutMarketValueUSD  float64
	SourceVersion      string
	PayloadHash        string
}
