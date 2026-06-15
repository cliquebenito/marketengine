package coinglass

import (
	"context"

	"marketengine/internal/capitalflows"
)

type CapitalFlowsAdapter struct{ client *Client }

func NewCapitalFlowsAdapter(c *Client) *CapitalFlowsAdapter {
	return &CapitalFlowsAdapter{client: c}
}

var (
	_ capitalflows.ETFFlowProvider       = (*CapitalFlowsAdapter)(nil)
	_ capitalflows.LTHSupplyProvider     = (*CapitalFlowsAdapter)(nil)
	_ capitalflows.BTCMarketCapProvider  = (*CapitalFlowsAdapter)(nil)
	_ capitalflows.LiquidityFlowProvider = (*CapitalFlowsAdapter)(nil)
	_ capitalflows.InstitutionalProvider = (*CapitalFlowsAdapter)(nil)
)

const lthSupplyLimit = 10000

func (a *CapitalFlowsAdapter) FetchBTCETFFlows(ctx context.Context) ([]capitalflows.ETFFlowPoint, error) {
	pts, err := a.client.FetchBTCETFFlows(ctx)
	if err != nil {
		return nil, err
	}
	return convertETFFlows(pts), nil
}

func (a *CapitalFlowsAdapter) FetchETHETFFlows(ctx context.Context) ([]capitalflows.ETFFlowPoint, error) {
	pts, err := a.client.FetchETHETFFlows(ctx)
	if err != nil {
		return nil, err
	}
	return convertETFFlows(pts), nil
}

func convertETFFlows(pts []ETFFlowPoint) []capitalflows.ETFFlowPoint {
	out := make([]capitalflows.ETFFlowPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.ETFFlowPoint{
			Date:         p.Date,
			TotalFlowUSD: p.TotalFlowUSD,
			PriceUSD:     p.PriceUSD,
			PayloadHash:  p.PayloadHash,
		})
	}
	return out
}

func (a *CapitalFlowsAdapter) FetchLTHSupply(ctx context.Context) ([]capitalflows.LTHSupplyPoint, error) {
	pts, err := a.client.FetchLTHSupply(ctx, lthSupplyLimit)
	if err != nil {
		return nil, err
	}
	out := make([]capitalflows.LTHSupplyPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.LTHSupplyPoint{
			Date:        p.Timestamp,
			LTHSupply:   p.LTHSupply,
			PriceUSD:    p.PriceUSD,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

func (a *CapitalFlowsAdapter) FetchBTCMarketCap(ctx context.Context) ([]capitalflows.MarketCapPoint, error) {
	pts, err := a.client.FetchBTCMarketCap(ctx, lthSupplyLimit)
	if err != nil {
		return nil, err
	}
	out := make([]capitalflows.MarketCapPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.MarketCapPoint{
			Date:         p.Timestamp,
			MarketCapUSD: p.MarketCapUSD,
			PriceUSD:     p.PriceUSD,
			PayloadHash:  p.PayloadHash,
		})
	}
	return out, nil
}

func (a *CapitalFlowsAdapter) FetchStablecoinMcapHistory(ctx context.Context) ([]capitalflows.StablecoinMcapPoint, error) {
	pts, err := a.client.FetchStablecoinMarketCapHistory(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]capitalflows.StablecoinMcapPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.StablecoinMcapPoint{
			Date:        p.Date,
			MarketCap:   p.MarketCap,
			PriceUSD:    p.PriceUSD,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

func (a *CapitalFlowsAdapter) FetchExchangeBalanceList(ctx context.Context, symbol string) ([]capitalflows.ExchangeBalanceSnapshot, error) {
	pts, err := a.client.FetchExchangeBalanceList(ctx, symbol)
	if err != nil {
		return nil, err
	}
	out := make([]capitalflows.ExchangeBalanceSnapshot, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.ExchangeBalanceSnapshot{
			Exchange:            p.Exchange,
			Symbol:              p.Symbol,
			TotalBalance:        p.TotalBalance,
			BalanceChange1d:     p.BalanceChange1d,
			BalanceChange7d:     p.BalanceChange7d,
			BalanceChange30d:    p.BalanceChange30d,
			BalanceChangePct1d:  p.BalanceChangePct1d,
			BalanceChangePct7d:  p.BalanceChangePct7d,
			BalanceChangePct30d: p.BalanceChangePct30d,
			PayloadHash:         p.PayloadHash,
		})
	}
	return out, nil
}

func (a *CapitalFlowsAdapter) FetchBitfinexMargin(ctx context.Context, symbol string, limit int) ([]capitalflows.BitfinexMarginPoint, error) {
	pts, err := a.client.FetchBitfinexMarginLongShort(ctx, symbol, limit)
	if err != nil {
		return nil, err
	}
	out := make([]capitalflows.BitfinexMarginPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.BitfinexMarginPoint{
			Time:        p.Time,
			Symbol:      p.Symbol,
			LongQty:     p.LongQty,
			ShortQty:    p.ShortQty,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

func (a *CapitalFlowsAdapter) FetchETFList(ctx context.Context) ([]capitalflows.ETFListItemPoint, error) {
	pts, err := a.client.FetchBitcoinETFList(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]capitalflows.ETFListItemPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.ETFListItemPoint{
			Ticker:             p.Ticker,
			FundName:           p.FundName,
			Region:             p.Region,
			MarketStatus:       p.MarketStatus,
			PrimaryExchange:    p.PrimaryExchange,
			FundType:           p.FundType,
			SharesOutstanding:  p.SharesOutstanding,
			AUMUSD:             p.AUMUSD,
			ManagementFeePct:   p.ManagementFeePct,
			VolumeUSD:          p.VolumeUSD,
			PriceChangePct:     p.PriceChangePct,
			NetAssetValueUSD:   p.NetAssetValueUSD,
			PremiumDiscountPct: p.PremiumDiscountPct,
			HoldingQuantity:    p.HoldingQuantity,
			ChangePct24h:       p.ChangePct24h,
			ChangeQty24h:       p.ChangeQty24h,
			ChangePct7d:        p.ChangePct7d,
			ChangeQty7d:        p.ChangeQty7d,
			UpdateDate:         p.UpdateDate,
			PayloadHash:        p.PayloadHash,
		})
	}
	return out, nil
}

func (a *CapitalFlowsAdapter) FetchETFAUMHistory(ctx context.Context, ticker string) ([]capitalflows.ETFAUMHistoryPoint, error) {
	pts, err := a.client.FetchBitcoinETFAUMHistory(ctx, ticker)
	if err != nil {
		return nil, err
	}
	out := make([]capitalflows.ETFAUMHistoryPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.ETFAUMHistoryPoint{
			Date:        p.Date,
			Ticker:      p.Ticker,
			AUMUSD:      p.AUMUSD,
			PayloadHash: p.PayloadHash,
		})
	}
	return out, nil
}

func (a *CapitalFlowsAdapter) FetchOptionsMaxPain(ctx context.Context, symbol, exchange string) ([]capitalflows.OptionsMaxPainNearestPoint, error) {
	pts, err := a.client.FetchOptionsMaxPain(ctx, symbol, exchange)
	if err != nil {
		return nil, err
	}
	out := make([]capitalflows.OptionsMaxPainNearestPoint, 0, len(pts))
	for _, p := range pts {
		out = append(out, capitalflows.OptionsMaxPainNearestPoint{
			ExpiryDate:         p.Date,
			Symbol:             p.Symbol,
			Exchange:           p.Exchange,
			MaxPainPrice:       p.MaxPainPrice,
			CallOIContracts:    p.CallOIContracts,
			PutOIContracts:     p.PutOIContracts,
			CallOINotionalUSD:  p.CallOINotionalUSD,
			PutOINotionalUSD:   p.PutOINotionalUSD,
			CallMarketValueUSD: p.CallMarketValue,
			PutMarketValueUSD:  p.PutMarketValue,
			PayloadHash:        p.PayloadHash,
		})
	}
	return out, nil
}
