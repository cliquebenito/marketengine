package pgmarketstress

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/marketstress"
	"marketengine/internal/storage"
)

type RawRepo struct{ pool *storage.Pool }

func NewRawRepo(pool *storage.Pool) *RawRepo { return &RawRepo{pool: pool} }

var _ marketstress.RawRepo = (*RawRepo)(nil)

func (r *RawRepo) SaveBinanceKlines(ctx context.Context, rows []marketstress.BinanceKlineRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawBinanceKline(ctx, tx, storage.RawBinanceKline{
				ValueDate:     x.ValueDate,
				Symbol:        x.Symbol,
				Close:         x.Close,
				Volume:        x.Volume,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert binance kline %s/%s: %w",
					x.Symbol, x.ValueDate.Format("2006-01-02"), err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveKrakenOHLC(ctx context.Context, rows []marketstress.KrakenOHLCRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawKrakenOHLC(ctx, tx, storage.RawKrakenOHLC{
				ValueDate:     x.ValueDate,
				Pair:          x.Pair,
				Open:          x.Open,
				High:          x.High,
				Low:           x.Low,
				Close:         x.Close,
				Volume:        x.Volume,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert kraken ohlc: %w", err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveCoinbaseCandles(ctx context.Context, rows []marketstress.CoinbaseCandleRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinbaseCandle(ctx, tx, storage.RawCoinbaseCandle{
				ValueDate:     x.ValueDate,
				ProductID:     x.ProductID,
				Close:         x.Close,
				Volume:        x.Volume,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert coinbase candle: %w", err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveCoinglassCoinbasePremium(ctx context.Context, rows []marketstress.CoinglassCoinbasePremiumRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassCoinbasePremium(ctx, tx, storage.RawCoinglassCoinbasePremium{
				ValueDate:     x.ValueDate,
				PremiumUSD:    x.PremiumUSD,
				PremiumRate:   x.PremiumRate,
				CoinbasePrice: x.CoinbasePrice,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert coinglass coinbase premium: %w", err)
			}
		}
		return nil
	})
}

func (r *RawRepo) GetBinanceKlineCloseSeries(ctx context.Context, symbol string, from, to, cutoff time.Time) ([]float64, error) {
	var out []float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		s, err := storage.GetBinanceKlineCloseSeries(ctx, tx, symbol, from, to, cutoff)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	return out, err
}

func (r *RawRepo) GetBinanceKlineCloseAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, error) {
	var (
		v  float64
		ok bool
	)
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, found, err := storage.GetBinanceKlineCloseAsOf(ctx, tx, symbol, valueDate, cutoff)
		if err != nil {
			return err
		}
		v, ok = x, found
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, err
	}
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *RawRepo) GetKrakenCloseAsOf(ctx context.Context, pair string, valueDate, cutoff time.Time) (float64, error) {
	var (
		v  float64
		ok bool
	)
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, found, err := storage.GetKrakenCloseAsOf(ctx, tx, pair, valueDate, cutoff)
		if err != nil {
			return err
		}
		v, ok = x, found
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, err
	}
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *RawRepo) GetCoinbaseCloseAsOf(ctx context.Context, productID string, valueDate, cutoff time.Time) (float64, error) {
	var (
		v  float64
		ok bool
	)
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, found, err := storage.GetCoinbaseCloseAsOf(ctx, tx, productID, valueDate, cutoff)
		if err != nil {
			return err
		}
		v, ok = x, found
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, err
	}
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *RawRepo) GetCoinglassCoinbasePremiumRateAsOf(ctx context.Context, valueDate, cutoff time.Time) (float64, error) {
	var (
		v  float64
		ok bool
	)
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, found, err := storage.GetCoinglassCoinbasePremiumRateAsOf(ctx, tx, valueDate, cutoff)
		if err != nil {
			return err
		}
		v, ok = x, found
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, err
	}
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *RawRepo) SaveCoinglassOrderbookAggregated(ctx context.Context, rows []marketstress.CoinglassOrderbookRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassOrderbookAggregated(ctx, tx, storage.RawCoinglassOrderbookAggregated{
				ValueDate:     x.ValueDate,
				Symbol:        x.Symbol,
				RangePct:      x.RangePct,
				BidsUSD:       x.BidsUSD,
				BidsQty:       x.BidsQty,
				AsksUSD:       x.AsksUSD,
				AsksQty:       x.AsksQty,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveCoinglassFuturesSpotVolRatio(ctx context.Context, rows []marketstress.CoinglassFuturesSpotVolRatioRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassFuturesSpotVolRatio(ctx, tx, storage.RawCoinglassFuturesSpotVolRatio{
				ValueDate:        x.ValueDate,
				Symbol:           x.Symbol,
				FuturesSpotRatio: x.FuturesSpotRatio,
				FuturesVolUSD:    x.FuturesVolUSD,
				SpotVolUSD:       x.SpotVolUSD,
				SourceVersion:    x.SourceVersion,
				PayloadHash:      x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) GetOrderbookImbalanceAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassOrderbookImbalanceAsOf(ctx, tx, symbol, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetFuturesSpotRatioAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassFuturesSpotRatioAsOf(ctx, tx, symbol, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}
