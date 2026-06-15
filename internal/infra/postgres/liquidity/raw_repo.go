package pgliquidity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"marketengine/internal/domain"
	"marketengine/internal/liquidity"
	"marketengine/internal/storage"
)

type RawRepo struct{ pool *storage.Pool }

func NewRawRepo(pool *storage.Pool) *RawRepo { return &RawRepo{pool: pool} }

var _ liquidity.RawRepo = (*RawRepo)(nil)

func (r *RawRepo) SaveStablecoinSupply(ctx context.Context, rows []liquidity.StablecoinSupplyRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawStablecoinSupply(ctx, tx, storage.RawStablecoinSupply{
				ValueDate:     x.ValueDate,
				Stablecoin:    x.Stablecoin,
				Metric:        x.Metric,
				Value:         x.Value,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw stablecoin %s/%s: %w", x.Stablecoin, x.ValueDate.Format("2006-01-02"), err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveChainTVL(ctx context.Context, rows []liquidity.ChainTVLRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawChainTVL(ctx, tx, storage.RawChainTVL{
				ValueDate:     x.ValueDate,
				Chain:         x.Chain,
				TVLUSD:        x.TVLUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw tvl: %w", err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveExchangeNetflow(ctx context.Context, rows []liquidity.ExchangeNetflowRow) error {
	if len(rows) == 0 {
		return nil
	}
	return r.pool.InTx(ctx, func(tx pgx.Tx) error {
		for _, x := range rows {
			if err := storage.InsertRawExchangeNetflow(ctx, tx, storage.RawExchangeNetflow{
				ValueDate:     x.ValueDate,
				Asset:         string(x.Asset),
				InflowUSD:     x.InflowUSD,
				OutflowUSD:    x.OutflowUSD,
				NetflowUSD:    x.NetflowUSD,
				SourceVersion: x.SourceVersion,
				PayloadHash:   x.PayloadHash,
			}); err != nil {
				return fmt.Errorf("insert raw netflow: %w", err)
			}
		}
		return nil
	})
}

func (r *RawRepo) SaveMarketCap(ctx context.Context, rows []liquidity.MarketCapRow) error {
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

func (r *RawRepo) GetStablecoinSupplyAsOf(ctx context.Context, stablecoin string, valueDate, cutoff time.Time) (float64, error) {
	var v float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, err := storage.GetRawStablecoinSupplyAsOf(ctx, tx, stablecoin, valueDate, cutoff)
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

func (r *RawRepo) SumStablecoinSupplyAsOf(ctx context.Context, symbols []string, valueDate, cutoff time.Time) (float64, int, error) {
	var sum float64
	var found int
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		s, n, err := storage.SumRawStablecoinSupplyAsOf(ctx, tx, symbols, valueDate, cutoff)
		sum, found = s, n
		return err
	})
	return sum, found, err
}

func (r *RawRepo) GetChainTVLAsOf(ctx context.Context, chain string, valueDate, cutoff time.Time) (float64, error) {
	var v float64
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		x, err := storage.GetRawChainTVLAsOf(ctx, tx, chain, valueDate, cutoff)
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

func (r *RawRepo) Sum7dNetflow(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	var sum float64
	var ok bool
	err := r.pool.InTx(ctx, func(tx pgx.Tx) error {
		s, c, err := storage.Sum7dNetflow(ctx, tx, string(asset), valueDate, cutoff)
		sum, ok = s, c
		return err
	})
	return sum, ok, err
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
