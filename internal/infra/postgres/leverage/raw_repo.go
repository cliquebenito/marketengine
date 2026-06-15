package pgleverage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/leverage"
	"marketengine/internal/storage"
)

type RawRepo struct{ pool *storage.Pool }

func NewRawRepo(pool *storage.Pool) *RawRepo { return &RawRepo{pool: pool} }

var _ leverage.RawRepo = (*RawRepo)(nil)

func (r *RawRepo) SaveExchangeOI(ctx context.Context, rows []leverage.ExchangeOIRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawExchangeOI(ctx, tx, storage.RawExchangeOI{
				ValueDate:     x.ValueDate,
				Asset:         string(x.Asset),
				Exchange:      x.Exchange,
				OIUSD:         x.OIUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw oi %s/%s: %w", x.Asset, x.Exchange, err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveExchangeFunding(ctx context.Context, rows []leverage.ExchangeFundingRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawExchangeFunding(ctx, tx, storage.RawExchangeFunding{
				FundingTime:   x.FundingTime,
				Asset:         string(x.Asset),
				Exchange:      x.Exchange,
				Rate:          x.Rate,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw funding %s/%s: %w", x.Asset, x.Exchange, err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveCoinglassFuturesBasis(ctx context.Context, rows []leverage.CoinglassFuturesBasisRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassFuturesBasis(ctx, tx, storage.RawCoinglassFuturesBasis{
				ValueDate:          x.ValueDate,
				Symbol:             x.Symbol,
				Exchange:           x.Exchange,
				AnnualizedBasisPct: x.AnnualizedBasisPct,
				CloseBasis:         x.CloseBasis,
				SourceVersion:      x.SourceVersion,
				PayloadHash:        x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw coinglass basis %s/%s: %w", x.Symbol, x.Exchange, err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveDeribitBasis(ctx context.Context, row leverage.DeribitBasisRow) error {
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		return storage.InsertRawDeribitBasis(ctx, tx, storage.RawDeribitBasis{
			ValueDate:       row.ValueDate,
			Asset:           string(row.Asset),
			InstrumentName:  row.InstrumentName,
			FuturesPrice:    row.FuturesPrice,
			SpotPrice:       row.SpotPrice,
			AnnualizedBasis: row.AnnualizedBasis,
			DaysToExpiry:    row.DaysToExpiry,
			SourceVersion:   row.SourceVersion,
			PayloadHash:     row.PayloadHash,
		})
	})
}

func (r *RawRepo) SaveExchangeLiquidations(ctx context.Context, rows []leverage.ExchangeLiquidationsRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawExchangeLiquidations(ctx, tx, storage.RawExchangeLiquidations{
				ValueDate:     x.ValueDate,
				Asset:         string(x.Asset),
				Exchange:      x.Exchange,
				LongLiqsUSD:   x.LongLiqsUSD,
				ShortLiqsUSD:  x.ShortLiqsUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw liqs %s/%s: %w", x.Asset, x.Exchange, err)
			}
		}
		return nil
	})
}

func (r *RawRepo) AggregatedOIAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	var sum float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		s, c, err := storage.AggregatedOIAsOf(ctx, tx, string(asset), valueDate, cutoff)
		sum, ok = s, c
		return err
	})
	return sum, ok, err
}

func (r *RawRepo) CoinglassAggregatedOIAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		val, c, err := storage.CoinglassAggregatedOIAsOf(ctx, tx, string(asset), valueDate, cutoff)
		v, ok = val, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) DailyAvgFundingAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		val, c, err := storage.DailyAvgFundingAsOf(ctx, tx, string(asset), valueDate, cutoff)
		v, ok = val, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) DailyTotalLiquidationsAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		val, c, err := storage.DailyTotalLiquidationsAsOf(ctx, tx, string(asset), valueDate, cutoff)
		v, ok = val, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) CoinglassAggregatedLiquidationsAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		val, c, err := storage.CoinglassAggregatedLiquidationsAsOf(ctx, tx, string(asset), valueDate, cutoff)
		v, ok = val, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetCoinglassFuturesBasisAsOf(ctx context.Context, symbol, exchange string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		val, c, err := storage.GetCoinglassFuturesBasisAsOf(ctx, tx, symbol, exchange, valueDate, cutoff)
		v, ok = val, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetDeribitBasisAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		val, c, err := storage.GetDeribitBasisAsOf(ctx, tx, string(asset), valueDate, cutoff)
		v, ok = val, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) SaveCoinglassLSRatio(ctx context.Context, kind leverage.LSRatioKind, rows []leverage.CoinglassLSRatioRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassLongShortRatio(ctx, tx, storage.LSRatioKind(kind), storage.RawCoinglassLongShortRatio{
				ValueDate:     x.ValueDate,
				Symbol:        x.Symbol,
				Exchange:      x.Exchange,
				LongPercent:   x.LongPercent,
				ShortPercent:  x.ShortPercent,
				Ratio:         x.Ratio,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw L/S %s %s/%s: %w", kind, x.Symbol, x.Exchange, err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveCoinglassTakerVolume(ctx context.Context, rows []leverage.CoinglassTakerVolumeRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassTakerVolume(ctx, tx, storage.RawCoinglassTakerVolume{
				ValueDate:     x.ValueDate,
				Symbol:        x.Symbol,
				BuyVolumeUSD:  x.BuyVolumeUSD,
				SellVolumeUSD: x.SellVolumeUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw taker volume %s: %w", x.Symbol, err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveCoinglassBorrowRate(ctx context.Context, rows []leverage.CoinglassBorrowRateRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassBorrowRate(ctx, tx, storage.RawCoinglassBorrowRate{
				ValueDate:     x.ValueDate,
				Symbol:        x.Symbol,
				Exchange:      x.Exchange,
				InterestRate:  x.InterestRate,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw borrow %s/%s: %w", x.Symbol, x.Exchange, err)
			}
		}
		return nil
	})
}

func (r *RawRepo) CoinglassLSRatioAvgAsOf(ctx context.Context, kind leverage.LSRatioKind, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassLSRatioAvgAsOf(ctx, tx, storage.LSRatioKind(kind), symbol, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) CoinglassTakerVolumeAsOf(ctx context.Context, coinSymbol string, valueDate, cutoff time.Time) (float64, float64, bool, error) {
	var buy, sell float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		b, s, c, err := storage.GetCoinglassTakerVolumeAsOf(ctx, tx, coinSymbol, valueDate, cutoff)
		buy, sell, ok = b, s, c
		return err
	})
	return buy, sell, ok, err
}

func (r *RawRepo) GetMarketCapAsOf(ctx context.Context, coinID string, valueDate, cutoff time.Time) (float64, error) {
	var v float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, err := storage.GetMarketCapAsOf(ctx, tx, coinID, valueDate, cutoff)
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
