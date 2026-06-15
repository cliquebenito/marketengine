package pgcapitalflows

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/capitalflows"
	"marketengine/internal/domain"
	"marketengine/internal/storage"
)

type RawRepo struct{ pool *storage.Pool }

func NewRawRepo(pool *storage.Pool) *RawRepo { return &RawRepo{pool: pool} }

var _ capitalflows.RawRepo = (*RawRepo)(nil)

func (r *RawRepo) SaveETFFlows(ctx context.Context, rows []capitalflows.ETFFlowRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassETFFlow(ctx, tx, storage.RawCoinglassETFFlow{
				ValueDate:     x.ValueDate,
				FlowType:      x.FlowType,
				TotalFlowUSD:  x.TotalFlowUSD,
				PriceUSD:      x.PriceUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw etf flow %s/%s: %w", x.FlowType, x.ValueDate.Format("2006-01-02"), err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveLTHSupply(ctx context.Context, rows []capitalflows.LTHSupplyRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawGlassnodeLTHSupply(ctx, tx, storage.RawGlassnodeLTHSupply{
				ValueDate:     x.ValueDate,
				Asset:         string(x.Asset),
				LTHSupply:     x.LTHSupply,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw lth supply: %w", err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveBTCMarketCap(ctx context.Context, rows []capitalflows.MarketCapRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawMarketCap(ctx, tx, storage.RawMarketCap{
				ValueDate:     x.ValueDate,
				CoinID:        x.CoinID,
				MarketCapUSD:  x.MarketCapUSD,
				PriceUSD:      x.PriceUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw mcap: %w", err)
			}
		}
		return nil
	})
}

func (r *RawRepo) CombinedETFFlowAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error) {
	var sum float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		s, present, err := storage.CombinedETFFlowAsOf(ctx, tx, valueDate, cutoff)
		sum, ok = s, present
		return err
	})
	return sum, ok, err
}

func (r *RawRepo) GetLTHSupplyAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, error) {
	var v float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, err := storage.GetLTHSupplyAsOf(ctx, tx, string(asset), valueDate, cutoff)
		if err != nil {
			return err
		}
		v = x
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, err
	}
	return v, nil
}

func (r *RawRepo) SaveStablecoinMcap(ctx context.Context, rows []capitalflows.StablecoinMcapRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassStablecoinMcap(ctx, tx, storage.RawCoinglassStablecoinMcap{
				ValueDate:     x.ValueDate,
				MarketCap:     x.MarketCap,
				PriceUSD:      x.PriceUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveExchangeBalance(ctx context.Context, rows []capitalflows.ExchangeBalanceRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassExchangeBalance(ctx, tx, storage.RawCoinglassExchangeBalance{
				ValueDate:           x.ValueDate,
				Symbol:              x.Symbol,
				Exchange:            x.Exchange,
				TotalBalance:        x.TotalBalance,
				BalanceChange1d:     x.BalanceChange1d,
				BalanceChange7d:     x.BalanceChange7d,
				BalanceChange30d:    x.BalanceChange30d,
				BalanceChangePct1d:  x.BalanceChangePct1d,
				BalanceChangePct7d:  x.BalanceChangePct7d,
				BalanceChangePct30d: x.BalanceChangePct30d,
				SourceVersion:       x.SourceVersion,
				PayloadHash:         x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveBitfinexMargin(ctx context.Context, rows []capitalflows.BitfinexMarginRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassBitfinexMargin(ctx, tx, storage.RawCoinglassBitfinexMargin{
				ValueDate:     x.ValueDate,
				Symbol:        x.Symbol,
				LongQty:       x.LongQty,
				ShortQty:      x.ShortQty,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) GetStablecoinMcapAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassStablecoinMcapAsOf(ctx, tx, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetStablecoinMcapSeries(ctx context.Context, from, to, cutoff time.Time) ([]float64, error) {
	var out []float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		s, err := storage.GetCoinglassStablecoinMcapSeries(ctx, tx, from, to, cutoff)
		out = s
		return err
	})
	return out, err
}

func (r *RawRepo) GetExchangeBalanceChange7dSumAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassExchangeBalanceChange7dSumAsOf(ctx, tx, symbol, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetExchangeBalanceChange30dSumAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassExchangeBalanceChange30dSumAsOf(ctx, tx, symbol, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetBitfinexMarginAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, float64, bool, error) {
	var long, short float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		l, s, c, err := storage.GetCoinglassBitfinexMarginAsOf(ctx, tx, symbol, valueDate, cutoff)
		long, short, ok = l, s, c
		return err
	})
	return long, short, ok, err
}

func (r *RawRepo) SaveETFListSnapshot(ctx context.Context, rows []capitalflows.ETFListItemRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassETFListItem(ctx, tx, storage.RawCoinglassETFListItem{
				ValueDate:          x.ValueDate,
				Ticker:             x.Ticker,
				FundName:           x.FundName,
				Region:             x.Region,
				MarketStatus:       x.MarketStatus,
				PrimaryExchange:    x.PrimaryExchange,
				FundType:           x.FundType,
				SharesOutstanding:  x.SharesOutstanding,
				AUMUSD:             x.AUMUSD,
				ManagementFeePct:   x.ManagementFeePct,
				VolumeUSD:          x.VolumeUSD,
				PriceChangePct:     x.PriceChangePct,
				NetAssetValueUSD:   x.NetAssetValueUSD,
				PremiumDiscountPct: x.PremiumDiscountPct,
				HoldingQuantity:    x.HoldingQuantity,
				ChangePct24h:       x.ChangePct24h,
				ChangeQty24h:       x.ChangeQty24h,
				ChangePct7d:        x.ChangePct7d,
				ChangeQty7d:        x.ChangeQty7d,
				SourceVersion:      x.SourceVersion,
				PayloadHash:        x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveETFAUMHistory(ctx context.Context, rows []capitalflows.ETFAUMHistoryRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassETFAUMHistory(ctx, tx, storage.RawCoinglassETFAUMHistory{
				ValueDate:     x.ValueDate,
				Ticker:        x.Ticker,
				AUMUSD:        x.AUMUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveOptionsMaxPainNearest(ctx context.Context, rows []capitalflows.OptionsMaxPainRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassOptionsMaxPain(ctx, tx, storage.RawCoinglassOptionsMaxPain{
				ValueDate:         x.ValueDate,
				ExpiryDate:        x.ExpiryDate,
				Symbol:            x.Symbol,
				Exchange:          x.Exchange,
				MaxPainPrice:      x.MaxPainPrice,
				CallOIContracts:   x.CallOIContracts,
				PutOIContracts:    x.PutOIContracts,
				CallOINotionalUSD: x.CallOINotionalUSD,
				PutOINotionalUSD:  x.PutOINotionalUSD,
				CallMarketValue:   x.CallMarketValueUSD,
				PutMarketValue:    x.PutMarketValueUSD,
				SourceVersion:     x.SourceVersion,
				PayloadHash:       x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) GetETFListAUMTotalAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassETFListAUMTotalAsOf(ctx, tx, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetETFListConcentrationHHIAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassETFListConcentrationHHIAsOf(ctx, tx, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetETFAUMHistoryTotalAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassETFAUMHistoryTotalAsOf(ctx, tx, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetOptionsDealerSkewProxyAsOf(ctx context.Context, symbol, exchange string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassOptionsDealerSkewProxyAsOf(ctx, tx, symbol, exchange, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}
