package coinglass

import (
	"context"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/leverage"
)

var defaultAggregatedExchanges = []string{
	"Binance", "OKX", "Bybit", "Bitget", "HTX",
	"Gate", "Bitmex", "Bitfinex", "dYdX", "KuCoin",
}

func leverageSymbolFor(asset domain.Asset) string {
	switch asset {
	case domain.AssetBTC:
		return "BTCUSDT"
	case domain.AssetETH:
		return "ETHUSDT"
	}
	return ""
}

func coinFor(asset domain.Asset) string {
	switch asset {
	case domain.AssetBTC:
		return "BTC"
	case domain.AssetETH:
		return "ETH"
	}
	return ""
}

type LeverageOIAdapter struct{ client *Client }

func NewLeverageOIAdapter(c *Client) *LeverageOIAdapter { return &LeverageOIAdapter{client: c} }

var _ leverage.CoinglassOIProvider = (*LeverageOIAdapter)(nil)

func (a *LeverageOIAdapter) FetchOIHistory(ctx context.Context, asset domain.Asset, exchange string, limit int) ([]leverage.CoinglassOIPoint, error) {
	sym := leverageSymbolFor(asset)
	if sym == "" {
		return nil, nil
	}
	pts, err := a.client.FetchOIHistory(ctx, sym, exchange, limit)
	if err != nil {
		return nil, err
	}
	out := make([]leverage.CoinglassOIPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.CoinglassOIPoint{
			Date:        p.Date,
			Asset:       asset,
			Exchange:    p.Exchange,
			OIUSD:       p.CloseOIUSD,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

func (a *LeverageOIAdapter) FetchAggregatedOIHistory(ctx context.Context, asset domain.Asset, limit int) ([]leverage.CoinglassOIPoint, error) {
	return a.FetchAggregatedOIHistoryRange(ctx, asset, time.Time{}, time.Time{}, limit)
}

func (a *LeverageOIAdapter) FetchAggregatedOIHistoryRange(ctx context.Context, asset domain.Asset, start, end time.Time, limit int) ([]leverage.CoinglassOIPoint, error) {
	coin := coinFor(asset)
	if coin == "" {
		return nil, nil
	}
	pts, err := a.client.FetchAggregatedOIHistory(ctx, coin, limit, "usd", start, end)
	if err != nil {
		return nil, err
	}
	out := make([]leverage.CoinglassOIPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.CoinglassOIPoint{
			Date:        p.Date,
			Asset:       asset,
			Exchange:    "coinglass_aggregated",
			OIUSD:       p.CloseOIUSD,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

type LeverageBasisAdapter struct{ client *Client }

func NewLeverageBasisAdapter(c *Client) *LeverageBasisAdapter {
	return &LeverageBasisAdapter{client: c}
}

var _ leverage.CoinglassBasisProvider = (*LeverageBasisAdapter)(nil)

func (a *LeverageBasisAdapter) FetchFuturesBasisHistory(ctx context.Context, asset domain.Asset, exchange string, limit int) ([]leverage.CoinglassBasisPoint, error) {
	sym := leverageSymbolFor(asset)
	if sym == "" {
		return nil, nil
	}
	pts, err := a.client.FetchFuturesBasisHistory(ctx, sym, exchange, limit)
	if err != nil {
		return nil, err
	}
	out := make([]leverage.CoinglassBasisPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.CoinglassBasisPoint{
			Date:               p.Date,
			Asset:              asset,
			Symbol:             p.Symbol,
			Exchange:           p.Exchange,
			AnnualizedBasisPct: p.AnnualizedBasisPct,
			CloseBasis:         p.CloseBasis,
			PayloadHash:        p.PayloadHash,
		})
	}
	return out, nil
}

type LeverageCrowdAdapter struct{ client *Client }

func NewLeverageCrowdAdapter(c *Client) *LeverageCrowdAdapter {
	return &LeverageCrowdAdapter{client: c}
}

var _ leverage.CoinglassCrowdProvider = (*LeverageCrowdAdapter)(nil)

func (a *LeverageCrowdAdapter) FetchLSRatio(
	ctx context.Context,
	kind leverage.LSRatioKind,
	asset domain.Asset,
	exchange string,
	limit int,
) ([]leverage.CoinglassLSRatioPoint, error) {
	sym := leverageSymbolFor(asset)
	if sym == "" {
		return nil, nil
	}
	var (
		raw []LongShortRatioPoint
		err error
	)
	switch kind {
	case leverage.LSGlobal:
		raw, err = a.client.FetchGlobalLongShortAccountRatio(ctx, sym, exchange, limit)
	case leverage.LSTopAccount:
		raw, err = a.client.FetchTopLongShortAccountRatio(ctx, sym, exchange, limit)
	case leverage.LSTopPosition:
		raw, err = a.client.FetchTopLongShortPositionRatio(ctx, sym, exchange, limit)
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]leverage.CoinglassLSRatioPoint, 0, len(raw))
	for _, p := range raw {
		out = append(out, leverage.CoinglassLSRatioPoint{
			Date:         p.Date,
			Asset:        asset,
			Exchange:     p.Exchange,
			LongPercent:  p.LongPercent,
			ShortPercent: p.ShortPercent,
			Ratio:        p.Ratio,
			PayloadHash:  p.PayloadHash,
		})
	}
	return out, nil
}

func (a *LeverageCrowdAdapter) FetchAggregatedTakerVolume(
	ctx context.Context,
	asset domain.Asset,
	exchanges []string,
	limit int,
) ([]leverage.CoinglassTakerVolumePoint, error) {
	coin := coinFor(asset)
	if coin == "" {
		return nil, nil
	}
	if len(exchanges) == 0 {
		exchanges = defaultAggregatedExchanges
	}
	pts, err := a.client.FetchAggregatedTakerBuySellVolume(ctx, coin, exchanges, limit, time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	out := make([]leverage.CoinglassTakerVolumePoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.CoinglassTakerVolumePoint{
			Date:          p.Date,
			Asset:         asset,
			BuyVolumeUSD:  p.BuyVolumeUSD,
			SellVolumeUSD: p.SellVolumeUSD,
			PayloadHash:   p.PayloadHash,
		})
	}
	return out, nil
}

func (a *LeverageCrowdAdapter) FetchBorrowRate(
	ctx context.Context, symbol, exchange string, limit int,
) ([]leverage.CoinglassBorrowRatePoint, error) {
	pts, err := a.client.FetchBorrowInterestRate(ctx, symbol, exchange, limit)
	if err != nil {
		return nil, err
	}
	out := make([]leverage.CoinglassBorrowRatePoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.CoinglassBorrowRatePoint{
			Date:         p.Date,
			Symbol:       p.Symbol,
			Exchange:     p.Exchange,
			InterestRate: p.InterestRate,
			PayloadHash:  p.PayloadHash,
		})
	}
	return out, nil
}

type LeverageLiqAdapter struct{ client *Client }

func NewLeverageLiqAdapter(c *Client) *LeverageLiqAdapter {
	return &LeverageLiqAdapter{client: c}
}

var _ leverage.CoinglassLiqProvider = (*LeverageLiqAdapter)(nil)

func (a *LeverageLiqAdapter) FetchLiquidationHistory(ctx context.Context, asset domain.Asset, exchange string, limit int) ([]leverage.CoinglassLiqPoint, error) {
	sym := leverageSymbolFor(asset)
	if sym == "" {
		return nil, nil
	}
	pts, err := a.client.FetchLiquidationHistory(ctx, sym, exchange, limit)
	if err != nil {
		return nil, err
	}
	out := make([]leverage.CoinglassLiqPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.CoinglassLiqPoint{
			Date:         p.Date,
			Asset:        asset,
			LongLiqsUSD:  p.LongLiqsUSD,
			ShortLiqsUSD: p.ShortLiqsUSD,
			PayloadHash:  p.PayloadHash,
		})
	}
	return out, nil
}

func (a *LeverageLiqAdapter) FetchAggregatedLiquidationHistory(
	ctx context.Context,
	asset domain.Asset,
	exchanges []string,
	limit int,
) ([]leverage.CoinglassLiqPoint, error) {
	return a.FetchAggregatedLiquidationHistoryRange(ctx, asset, exchanges, time.Time{}, time.Time{}, limit)
}

func (a *LeverageLiqAdapter) FetchAggregatedLiquidationHistoryRange(
	ctx context.Context,
	asset domain.Asset,
	exchanges []string,
	start, end time.Time,
	limit int,
) ([]leverage.CoinglassLiqPoint, error) {
	coin := coinFor(asset)
	if coin == "" {
		return nil, nil
	}
	if len(exchanges) == 0 {
		exchanges = defaultAggregatedExchanges
	}
	pts, err := a.client.FetchAggregatedLiquidationHistory(ctx, coin, exchanges, limit, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]leverage.CoinglassLiqPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, leverage.CoinglassLiqPoint{
			Date:         p.Date,
			Asset:        asset,
			LongLiqsUSD:  p.LongLiqsUSD,
			ShortLiqsUSD: p.ShortLiqsUSD,
			PayloadHash:  p.PayloadHash,
		})
	}
	return out, nil
}
