package pgvolatility

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/storage"
	"marketengine/internal/volatility"
	"marketengine/internal/volatility/features"
)

type RawRepo struct{ pool *storage.Pool }

func NewRawRepo(pool *storage.Pool) *RawRepo { return &RawRepo{pool: pool} }

var _ volatility.RawRepo = (*RawRepo)(nil)

func (r *RawRepo) SaveDVOL(ctx context.Context, rows []volatility.DVOLRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawDeribitDVOL(ctx, tx, storage.RawDeribitDVOL{
				ValueDate:     x.ValueDate,
				Asset:         string(x.Asset),
				DVOLClose:     x.DVOLClose,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw dvol %s/%s: %w", x.Asset, x.ValueDate.Format("2006-01-02"), err)
			}
		}
		return nil
	})
}

func (r *RawRepo) GetDVOLCloseAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, error) {
	var v float64
	var found bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, ok, err := storage.GetDVOLCloseAsOf(ctx, tx, string(asset), valueDate, cutoff)
		if err != nil {
			return err
		}
		v, found = x, ok
		return nil
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, err
	}
	if !found {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *RawRepo) RealizedVol30d(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	sym := features.AssetToBinanceSymbol(string(asset))
	if sym == "" {
		return 0, false, nil
	}
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, found, err := storage.RealizedVol30d(ctx, tx, sym, valueDate, cutoff)
		if err != nil {
			return err
		}
		v, ok = x, found
		return nil
	})
	if err != nil {
		return 0, false, err
	}
	return v, ok, nil
}

func (r *RawRepo) SaveCoinglassOptionsInfo(ctx context.Context, rows []volatility.CoinglassOptionsInfoRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassOptionsInfo(ctx, tx, storage.RawCoinglassOptionsInfo{
				ValueDate:        x.ValueDate,
				Symbol:           x.Symbol,
				Exchange:         x.Exchange,
				OpenInterest:     x.OpenInterest,
				OIMarketShare:    x.OIMarketShare,
				OIChange24h:      x.OIChange24h,
				OpenInterestUSD:  x.OpenInterestUSD,
				VolumeUSD24h:     x.VolumeUSD24h,
				VolumeChangePct:  x.VolumeChangePct,
				CallOpenInterest: x.CallOpenInterest,
				PutOpenInterest:  x.PutOpenInterest,
				SourceVersion:    x.SourceVersion,
				PayloadHash:      x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveCoinglassOptionsOIHistory(ctx context.Context, rows []volatility.CoinglassOptionsOIHistoryRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawCoinglassOptionsOIHistory(ctx, tx, storage.RawCoinglassOptionsOIHistory{
				ValueDate:     x.ValueDate,
				Symbol:        x.Symbol,
				Exchange:      x.Exchange,
				OpenInterest:  x.OpenInterest,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveCoinglassOptionsMaxPain(ctx context.Context, rows []volatility.CoinglassOptionsMaxPainRow) error {
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
				CallMarketValue:   x.CallMarketValue,
				PutMarketValue:    x.PutMarketValue,
				SourceVersion:     x.SourceVersion,
				PayloadHash:       x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) GetCoinglassOptionsAggregatedOIAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassOptionsAggregatedOIAsOf(ctx, tx, symbol, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetCoinglassOptionsPutCallRatioAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassOptionsPutCallRatioAsOf(ctx, tx, symbol, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) GetCoinglassOptionsMaxPainNearestAsOf(ctx context.Context, symbol, exchange string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetCoinglassOptionsMaxPainNearestAsOf(ctx, tx, symbol, exchange, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) SpotCloseAsOf(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	var v float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, c, err := storage.GetBinanceKlineCloseAsOf(ctx, tx, symbol, valueDate, cutoff)
		v, ok = x, c
		return err
	})
	return v, ok, err
}

func (r *RawRepo) SaveDeribitOptionsChain(ctx context.Context, rows []volatility.DeribitOptionsChainRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawDeribitOptionsChain(ctx, tx, storage.RawDeribitOptionsChain{
				ValueDate:          x.ValueDate,
				Asset:              string(x.Asset),
				InstrumentName:     x.InstrumentName,
				ExpiryDate:         x.ExpiryDate,
				StrikePrice:        x.StrikePrice,
				IsPut:              x.IsPut,
				OpenInterest:       x.OpenInterest,
				MarkIVPct:          x.MarkIVPct,
				UnderlyingPriceUSD: x.UnderlyingPriceUSD,
				SourceVersion:      x.SourceVersion,
				PayloadHash:        x.PayloadHash,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RawRepo) GetDeribitOptionsChainAsOf(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) ([]volatility.DeribitOptionsChainSnapshot, error) {
	var out []volatility.DeribitOptionsChainSnapshot
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		rows, err := storage.GetDeribitOptionsChainAsOf(ctx, tx, string(asset), valueDate, cutoff)
		if err != nil {
			return err
		}
		out = make([]volatility.DeribitOptionsChainSnapshot, 0, len(rows))
		for _, r := range rows {
			out = append(out, volatility.DeribitOptionsChainSnapshot{
				ExpiryDate:         r.ExpiryDate,
				StrikePrice:        r.StrikePrice,
				IsPut:              r.IsPut,
				OpenInterest:       r.OpenInterest,
				MarkIVPct:          r.MarkIVPct,
				UnderlyingPriceUSD: r.UnderlyingPriceUSD,
			})
		}
		return nil
	})
	return out, err
}
