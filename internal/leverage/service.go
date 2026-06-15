package leverage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/leverage/features"
)

type Service struct {
	features    FeatureRepo
	scores      ScoreRepo
	raws        RawRepo
	oiProviders []namedOIProvider
	fundingProv []namedFundingProvider
	basisProv   BasisProvider
	cgOI        CoinglassOIProvider
	cgBasis     CoinglassBasisProvider
	cgLiq       CoinglassLiqProvider
	cgCrowd     CoinglassCrowdProvider
	publisher   Publisher
	clock       Clock
	cfg         Config
	assets      []domain.Asset
}

type namedOIProvider struct {
	Name     string
	Provider OIProvider
}

type namedFundingProvider struct {
	Name     string
	Provider FundingProvider
}

func NewService(
	featRepo FeatureRepo,
	scoreRepo ScoreRepo,
	rawRepo RawRepo,
	oiProviders map[string]OIProvider,
	fundingProviders map[string]FundingProvider,
	basisProv BasisProvider,
	cgOI CoinglassOIProvider,
	cgBasis CoinglassBasisProvider,
	cgLiq CoinglassLiqProvider,
	cgCrowd CoinglassCrowdProvider,
	pub Publisher,
	clk Clock,
	cfg Config,
	assets []domain.Asset,
) *Service {
	cfg.Defaults()
	if len(assets) == 0 {
		assets = domain.AssetsTradeable()
	}
	oiList := make([]namedOIProvider, 0, len(oiProviders))
	for name, p := range oiProviders {
		if p == nil {
			continue
		}
		oiList = append(oiList, namedOIProvider{Name: name, Provider: p})
	}
	fundList := make([]namedFundingProvider, 0, len(fundingProviders))
	for name, p := range fundingProviders {
		if p == nil {
			continue
		}
		fundList = append(fundList, namedFundingProvider{Name: name, Provider: p})
	}
	return &Service{
		features:    featRepo,
		scores:      scoreRepo,
		raws:        rawRepo,
		oiProviders: oiList,
		fundingProv: fundList,
		basisProv:   basisProv,
		cgOI:        cgOI,
		cgBasis:     cgBasis,
		cgLiq:       cgLiq,
		cgCrowd:     cgCrowd,
		publisher:   pub,
		clock:       clk,
		cfg:         cfg,
		assets:      assets,
	}
}

func (s *Service) RunOnce(ctx context.Context, valueDate time.Time) error {
	valueDate = domain.UTCDay(valueDate)

	if err := s.ingestAll(ctx, valueDate.AddDate(0, 0, -7), valueDate); err != nil {
		return err
	}
	cutoff := s.clock.Now()
	return s.computeRange(ctx, valueDate, valueDate, cutoff)
}

func (s *Service) RunBackfill(ctx context.Context, from, to time.Time) error {
	from, to = domain.UTCDay(from), domain.UTCDay(to)
	if from.After(to) {
		return fmt.Errorf("backfill from (%s) after to (%s)",
			from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
	if err := s.ingestAll(ctx, from, to); err != nil {
		return err
	}
	cutoff := s.clock.Now().Add(24 * time.Hour)
	return s.computeRange(ctx, from, to, cutoff)
}

func (s *Service) ingestAll(ctx context.Context, from, to time.Time) error {
	if err := s.ingestExchangeOI(ctx, from, to); err != nil {
		return fmt.Errorf("ingest oi: %w", err)
	}
	if err := s.ingestExchangeFunding(ctx, from, to); err != nil {
		return fmt.Errorf("ingest funding: %w", err)
	}
	if err := s.ingestDeribitBasis(ctx); err != nil {
		return fmt.Errorf("ingest deribit basis: %w", err)
	}
	if err := s.ingestCoinglassOI(ctx, from, to); err != nil {
		return fmt.Errorf("ingest coinglass oi: %w", err)
	}
	if err := s.ingestCoinglassBasis(ctx); err != nil {
		return fmt.Errorf("ingest coinglass basis: %w", err)
	}
	if err := s.ingestCoinglassLiquidations(ctx); err != nil {
		return fmt.Errorf("ingest coinglass liquidations: %w", err)
	}
	if err := s.ingestCoinglassCrowd(ctx); err != nil {
		return fmt.Errorf("ingest coinglass crowd: %w", err)
	}
	return nil
}

var crowdExchanges = []string{"Binance", "OKX", "Bybit"}

func (s *Service) ingestCoinglassCrowd(ctx context.Context) error {
	if s.cgCrowd == nil {
		return nil
	}
	const limit = 1000
	for _, asset := range s.assets {

		for _, kind := range []LSRatioKind{LSGlobal, LSTopAccount, LSTopPosition} {
			for _, exch := range crowdExchanges {
				pts, err := s.cgCrowd.FetchLSRatio(ctx, kind, asset, exch, limit)
				if err != nil {
					slog.Warn("coinglass L/S fetch failed (non-fatal)",
						"asset", asset, "kind", kind, "exchange", exch, "err", err)
					continue
				}
				if len(pts) == 0 {
					continue
				}
				rows := make([]CoinglassLSRatioRow, 0, len(pts))
				for _, p := range pts {
					rows = append(rows, CoinglassLSRatioRow{
						ValueDate:     p.Date,
						Symbol:        string(asset),
						Exchange:      p.Exchange,
						LongPercent:   p.LongPercent,
						ShortPercent:  p.ShortPercent,
						Ratio:         p.Ratio,
						SourceVersion: "coinglass_v4",
						PayloadHash:   p.PayloadHash,
					})
				}
				if err := s.raws.SaveCoinglassLSRatio(ctx, kind, rows); err != nil {
					return fmt.Errorf("save L/S %s %s/%s: %w", kind, asset, exch, err)
				}
				if err := s.publisher.Publish(ctx, domain.Event{
					Topic:       fmt.Sprintf("raw.leverage.long_short.%s.coinglass_%s.ingested.v1", kind, exch),
					AggregateID: fmt.Sprintf("%s:%s:%s", asset, kind, exch),
					Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
				}); err != nil {
					return err
				}
			}
		}

		takerPts, err := s.cgCrowd.FetchAggregatedTakerVolume(ctx, asset, nil, limit)
		if err != nil {
			slog.Warn("coinglass taker volume fetch failed (non-fatal)", "asset", asset, "err", err)
		} else if len(takerPts) > 0 {
			rows := make([]CoinglassTakerVolumeRow, 0, len(takerPts))
			for _, p := range takerPts {
				rows = append(rows, CoinglassTakerVolumeRow{
					ValueDate:     p.Date,
					Symbol:        string(asset),
					BuyVolumeUSD:  p.BuyVolumeUSD,
					SellVolumeUSD: p.SellVolumeUSD,
					SourceVersion: "coinglass_v4",
					PayloadHash:   p.PayloadHash,
				})
			}
			if err := s.raws.SaveCoinglassTakerVolume(ctx, rows); err != nil {
				return fmt.Errorf("save taker volume %s: %w", asset, err)
			}
			if err := s.publisher.Publish(ctx, domain.Event{
				Topic:       "raw.leverage.taker_volume.coinglass_aggregated.ingested.v1",
				AggregateID: fmt.Sprintf("%s:taker_volume", asset),
				Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
			}); err != nil {
				return err
			}
		}

		for _, exch := range crowdExchanges {
			borrowPts, err := s.cgCrowd.FetchBorrowRate(ctx, "USDT", exch, limit)
			if err != nil {
				slog.Warn("coinglass borrow rate fetch failed (non-fatal)",
					"exchange", exch, "err", err)
				continue
			}
			if len(borrowPts) == 0 {
				continue
			}
			rows := make([]CoinglassBorrowRateRow, 0, len(borrowPts))
			for _, p := range borrowPts {
				rows = append(rows, CoinglassBorrowRateRow{
					ValueDate:     p.Date,
					Symbol:        p.Symbol,
					Exchange:      p.Exchange,
					InterestRate:  p.InterestRate,
					SourceVersion: "coinglass_v4",
					PayloadHash:   p.PayloadHash,
				})
			}
			if err := s.raws.SaveCoinglassBorrowRate(ctx, rows); err != nil {
				return fmt.Errorf("save borrow rate USDT/%s: %w", exch, err)
			}
		}
	}
	return nil
}

func (s *Service) ingestExchangeOI(ctx context.Context, from, to time.Time) error {
	for _, np := range s.oiProviders {
		for _, asset := range s.assets {
			pts, err := np.Provider.FetchOpenInterest(ctx, asset, from, to)
			if err != nil {
				return fmt.Errorf("%s OI %s: %w", np.Name, asset, err)
			}
			if len(pts) == 0 {
				continue
			}
			rows := make([]ExchangeOIRow, 0, len(pts))
			for _, p := range pts {
				rows = append(rows, ExchangeOIRow{
					ValueDate:     p.Date,
					Asset:         asset,
					Exchange:      np.Name,
					OIUSD:         p.OIUSD,
					SourceVersion: "exchange_v1",
					PayloadHash:   p.PayloadHash,
				})
			}
			if err := s.raws.SaveExchangeOI(ctx, rows); err != nil {
				return fmt.Errorf("save %s OI %s: %w", np.Name, asset, err)
			}
			if err := s.publisher.Publish(ctx, domain.Event{
				Topic:       fmt.Sprintf("raw.leverage.oi.%s.ingested.v1", np.Name),
				AggregateID: fmt.Sprintf("%s:%s", asset, np.Name),
				Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ingestExchangeFunding(ctx context.Context, from, to time.Time) error {
	for _, np := range s.fundingProv {
		for _, asset := range s.assets {
			pts, err := np.Provider.FetchFundingRateHistory(ctx, asset, from, to)
			if err != nil {
				return fmt.Errorf("%s funding %s: %w", np.Name, asset, err)
			}
			if len(pts) == 0 {
				continue
			}
			rows := make([]ExchangeFundingRow, 0, len(pts))
			for _, p := range pts {
				rows = append(rows, ExchangeFundingRow{
					FundingTime:   p.Timestamp,
					Asset:         asset,
					Exchange:      np.Name,
					Rate:          p.Rate,
					SourceVersion: "exchange_v1",
					PayloadHash:   p.PayloadHash,
				})
			}
			if err := s.raws.SaveExchangeFunding(ctx, rows); err != nil {
				return fmt.Errorf("save %s funding %s: %w", np.Name, asset, err)
			}
			if err := s.publisher.Publish(ctx, domain.Event{
				Topic:       fmt.Sprintf("raw.leverage.funding.%s.ingested.v1", np.Name),
				AggregateID: fmt.Sprintf("%s:%s", asset, np.Name),
				Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ingestDeribitBasis(ctx context.Context) error {
	if s.basisProv == nil {
		return nil
	}
	for _, asset := range s.assets {
		snap, err := s.basisProv.FetchBasis3mSnapshot(ctx, asset)
		if err != nil {

			slog.Warn("deribit basis snapshot failed (non-fatal)", "asset", asset, "err", err)
			continue
		}
		row := DeribitBasisRow{
			ValueDate:       snap.Date,
			Asset:           asset,
			InstrumentName:  snap.InstrumentName,
			FuturesPrice:    snap.FuturesPrice,
			SpotPrice:       snap.SpotPrice,
			AnnualizedBasis: snap.AnnualizedBasis,
			DaysToExpiry:    snap.DaysToExpiry,
			SourceVersion:   "deribit_v1",
			PayloadHash:     snap.PayloadHash,
		}
		if err := s.raws.SaveDeribitBasis(ctx, row); err != nil {
			return fmt.Errorf("save deribit basis %s: %w", asset, err)
		}
	}
	return nil
}

func (s *Service) ingestCoinglassOI(ctx context.Context, from, to time.Time) error {
	if s.cgOI == nil {
		return nil
	}
	exchanges := []string{"Binance", "Bybit", "OKX"}
	fromDay, toDay := domain.UTCDay(from), domain.UTCDay(to)
	for _, asset := range s.assets {
		for _, exch := range exchanges {
			pts, err := s.cgOI.FetchOIHistory(ctx, asset, exch, 2000)
			if err != nil {

				slog.Warn("coinglass OI fetch failed (non-fatal)", "asset", asset, "exchange", exch, "err", err)
				continue
			}
			rows := make([]ExchangeOIRow, 0, len(pts))
			exchKey := fmt.Sprintf("coinglass_%s", exch)
			for _, p := range pts {
				if p.Date.Before(fromDay) || p.Date.After(toDay) {
					continue
				}
				rows = append(rows, ExchangeOIRow{
					ValueDate:     p.Date,
					Asset:         asset,
					Exchange:      exchKey,
					OIUSD:         p.OIUSD,
					SourceVersion: "coinglass_v4",
					PayloadHash:   p.PayloadHash,
				})
			}
			if len(rows) == 0 {
				continue
			}
			if err := s.raws.SaveExchangeOI(ctx, rows); err != nil {
				return fmt.Errorf("save coinglass OI %s/%s: %w", asset, exch, err)
			}
			if err := s.publisher.Publish(ctx, domain.Event{
				Topic:       fmt.Sprintf("raw.leverage.oi.coinglass_%s.ingested.v1", exch),
				AggregateID: fmt.Sprintf("%s:%s", asset, exchKey),
				Payload:     map[string]any{"rows": len(rows), "asset": asset.String(), "exchange": exch},
			}); err != nil {
				return err
			}
		}

		aggPts, err := s.backfillAggregatedOI(ctx, asset)
		if err != nil {
			slog.Warn("coinglass aggregated OI fetch failed (non-fatal)", "asset", asset, "err", err)
			continue
		}
		aggRows := make([]ExchangeOIRow, 0, len(aggPts))
		for _, p := range aggPts {
			aggRows = append(aggRows, ExchangeOIRow{
				ValueDate:     p.Date,
				Asset:         asset,
				Exchange:      "coinglass_aggregated",
				OIUSD:         p.OIUSD,
				SourceVersion: "coinglass_v4",
				PayloadHash:   p.PayloadHash,
			})
		}
		if len(aggRows) == 0 {
			continue
		}
		if err := s.raws.SaveExchangeOI(ctx, aggRows); err != nil {
			return fmt.Errorf("save coinglass aggregated OI %s: %w", asset, err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.leverage.oi.coinglass_aggregated.ingested.v1",
			AggregateID: fmt.Sprintf("%s:coinglass_aggregated", asset),
			Payload:     map[string]any{"rows": len(aggRows), "asset": asset.String()},
		}); err != nil {
			return err
		}
	}
	return nil
}

var coinglassAggMinDate = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

const coinglassAggMaxPages = 10

func (s *Service) backfillAggregatedOI(ctx context.Context, asset domain.Asset) ([]CoinglassOIPoint, error) {
	all := make(map[int64]CoinglassOIPoint)
	var (
		end        time.Time
		prevOldest time.Time
	)
	for page := 0; page < coinglassAggMaxPages; page++ {
		pts, err := s.cgOI.FetchAggregatedOIHistoryRange(ctx, asset, time.Time{}, end, 1000)
		if err != nil {
			return nil, err
		}
		if len(pts) == 0 {
			break
		}
		oldest := pts[0].Date
		for _, p := range pts {
			if p.Date.Before(oldest) {
				oldest = p.Date
			}
			all[p.Date.Unix()] = p
		}
		if !oldest.After(coinglassAggMinDate) {
			break
		}
		if !prevOldest.IsZero() && !oldest.Before(prevOldest) {

			break
		}
		prevOldest = oldest
		end = oldest.Add(-24 * time.Hour)
	}
	out := make([]CoinglassOIPoint, 0, len(all))
	for _, p := range all {
		out = append(out, p)
	}
	return out, nil
}

func (s *Service) ingestCoinglassBasis(ctx context.Context) error {
	if s.cgBasis == nil {
		return nil
	}
	for _, asset := range s.assets {
		pts, err := s.cgBasis.FetchFuturesBasisHistory(ctx, asset, "Binance", 1000)
		if err != nil {

			slog.Warn("coinglass futures basis fetch failed (non-fatal)", "asset", asset, "err", err)
			continue
		}
		if len(pts) == 0 {
			continue
		}
		rows := make([]CoinglassFuturesBasisRow, 0, len(pts))
		for _, p := range pts {
			rows = append(rows, CoinglassFuturesBasisRow{
				ValueDate:          p.Date,
				Symbol:             p.Symbol,
				Exchange:           p.Exchange,
				AnnualizedBasisPct: p.AnnualizedBasisPct,
				CloseBasis:         p.CloseBasis,
				SourceVersion:      "coinglass_v4",
				PayloadHash:        p.PayloadHash,
			})
		}
		if err := s.raws.SaveCoinglassFuturesBasis(ctx, rows); err != nil {
			return fmt.Errorf("save coinglass basis %s: %w", asset, err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.leverage.coinglass_futures_basis.ingested.v1",
			AggregateID: fmt.Sprintf("%s:coinglass_basis", asset),
			Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ingestCoinglassLiquidations(ctx context.Context) error {
	if s.cgLiq == nil {
		return nil
	}
	const aggExchange = "coinglass_aggregated"
	for _, asset := range s.assets {
		pts, err := s.backfillAggregatedLiquidations(ctx, asset)
		if err != nil {
			return fmt.Errorf("coinglass aggregated liquidations %s: %w", asset, err)
		}
		if len(pts) == 0 {
			continue
		}
		rows := make([]ExchangeLiquidationsRow, 0, len(pts))
		for _, p := range pts {
			rows = append(rows, ExchangeLiquidationsRow{
				ValueDate:     p.Date,
				Asset:         asset,
				Exchange:      aggExchange,
				LongLiqsUSD:   p.LongLiqsUSD,
				ShortLiqsUSD:  p.ShortLiqsUSD,
				SourceVersion: "coinglass_v4",
				PayloadHash:   p.PayloadHash,
			})
		}
		if err := s.raws.SaveExchangeLiquidations(ctx, rows); err != nil {
			return fmt.Errorf("save coinglass liquidations %s: %w", asset, err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.leverage.liquidations.coinglass_aggregated.ingested.v1",
			AggregateID: fmt.Sprintf("%s:%s", asset, aggExchange),
			Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) backfillAggregatedLiquidations(ctx context.Context, asset domain.Asset) ([]CoinglassLiqPoint, error) {
	all := make(map[int64]CoinglassLiqPoint)
	var (
		end        time.Time
		prevOldest time.Time
	)
	for page := 0; page < coinglassAggMaxPages; page++ {
		pts, err := s.cgLiq.FetchAggregatedLiquidationHistoryRange(ctx, asset, nil, time.Time{}, end, 1000)
		if err != nil {
			return nil, err
		}
		if len(pts) == 0 {
			break
		}
		oldest := pts[0].Date
		for _, p := range pts {
			if p.Date.Before(oldest) {
				oldest = p.Date
			}
			all[p.Date.Unix()] = p
		}
		if !oldest.After(coinglassAggMinDate) {
			break
		}
		if !prevOldest.IsZero() && !oldest.Before(prevOldest) {
			break
		}
		prevOldest = oldest
		end = oldest.Add(-24 * time.Hour)
	}
	out := make([]CoinglassLiqPoint, 0, len(all))
	for _, p := range all {
		out = append(out, p)
	}
	return out, nil
}

func (s *Service) computeRange(ctx context.Context, from, to, cutoff time.Time) error {
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if err := s.computeDay(ctx, d, cutoff); err != nil {
			return fmt.Errorf("day %s: %w", d.Format("2006-01-02"), err)
		}
	}
	return nil
}

func (s *Service) computeDay(ctx context.Context, valueDate, cutoff time.Time) error {
	for _, asset := range s.assets {
		if err := s.computeAssetDay(ctx, asset, valueDate, cutoff); err != nil {
			return fmt.Errorf("asset %s: %w", asset, err)
		}
	}
	return nil
}

func (s *Service) computeAssetDay(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) error {
	v := s.cfg.IntermediateVersion
	fv := s.cfg.FinalVersion

	oiRawKey := domain.FeatureKey{Name: features.OIUsdRawName, Version: v}
	var oiRaw float64
	var oiRawAvail bool
	if val, ok, err := s.raws.CoinglassAggregatedOIAsOf(ctx, asset, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		oiRaw, oiRawAvail = val, true
		s.saveFeature(ctx, oiRawKey, asset, valueDate, val, "exchange_oi", "coinglass_v4")
	} else if val, ok, err := s.raws.AggregatedOIAsOf(ctx, asset, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		oiRaw, oiRawAvail = val, true
		s.saveFeature(ctx, oiRawKey, asset, valueDate, val, "exchange_oi", "exchange_v1")
	}

	oiMcapKey := domain.FeatureKey{Name: features.OIMcapRatioName, Version: v}
	if oiRawAvail {
		if mcap, err := s.raws.GetMarketCapAsOf(ctx, "bitcoin", valueDate, cutoff); err == nil && mcap > 0 {
			if r, ok := features.Ratio(oiRaw, mcap); ok {
				s.saveFeature(ctx, oiMcapKey, asset, valueDate, r, "coingecko_market_cap", "coingecko_v1")
			}
		} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
	}

	fundDailyKey := domain.FeatureKey{Name: features.FundingRateDailyName, Version: v}
	var fundingRate float64
	var fundingAvail bool
	if val, ok, err := s.raws.DailyAvgFundingAsOf(ctx, asset, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		fundingRate, fundingAvail = val, true
		s.saveFeature(ctx, fundDailyKey, asset, valueDate, val, "exchange_funding", "exchange_v1")
	}

	basisApproxKey := domain.FeatureKey{Name: features.BasisFromFundingApproxName, Version: v}
	if fundingAvail {
		s.saveFeature(ctx, basisApproxKey, asset, valueDate,
			features.AnnualizedBasisFromFunding(fundingRate),
			"exchange_funding", "exchange_v1")
	}

	basisDailyKey := domain.FeatureKey{Name: features.Basis3mDailyName, Version: v}
	if sym := features.PerpSymbolForAsset(string(asset)); sym != "" {
		if val, ok, err := s.raws.GetCoinglassFuturesBasisAsOf(ctx, sym, "Binance", valueDate, cutoff); err != nil {
			return err
		} else if ok {
			s.saveFeature(ctx, basisDailyKey, asset, valueDate, val, "coinglass_basis", "coinglass_v4")
		} else if val, ok, err := s.raws.GetDeribitBasisAsOf(ctx, asset, valueDate, cutoff); err != nil {
			return err
		} else if ok {
			s.saveFeature(ctx, basisDailyKey, asset, valueDate, val, "deribit_basis", "deribit_v1")
		} else if fundingAvail {
			s.saveFeature(ctx, basisDailyKey, asset, valueDate,
				features.AnnualizedBasisFromFunding(fundingRate),
				"exchange_funding", "exchange_v1")
		}
	}

	liqDailyKey := domain.FeatureKey{Name: features.LiquidationsDailyLogName, Version: v}
	if val, ok, err := s.raws.CoinglassAggregatedLiquidationsAsOf(ctx, asset, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		l := features.Log1p(val)
		if !math.IsNaN(l) {
			s.saveFeature(ctx, liqDailyKey, asset, valueDate, l, "exchange_liqs", "coinglass_v4")
		}
	} else if val, ok, err := s.raws.DailyTotalLiquidationsAsOf(ctx, asset, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		l := features.Log1p(val)
		if !math.IsNaN(l) {
			s.saveFeature(ctx, liqDailyKey, asset, valueDate, l, "exchange_liqs", "exchange_v1")
		}
	}

	oiChangeKey := domain.FeatureKey{Name: features.OIChange30dName, Version: v}
	if oiRawAvail {
		past := valueDate.AddDate(0, 0, -features.OIChangeLagDays)
		if earlier, err := s.features.GetLatest(ctx, oiRawKey, asset, past, cutoff); err == nil {
			if pct, ok := features.PctChange(oiRaw, earlier); ok {
				s.saveFeature(ctx, oiChangeKey, asset, valueDate, pct, "exchange_oi", "exchange_v1")
			}
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
	}

	oiPercKey := domain.FeatureKey{Name: features.OIMcapPercentile365dName, Version: fv}
	if pct, ok, err := s.computeOIPercentile(ctx, asset, valueDate, cutoff, oiMcapKey, oiRawKey, oiRaw, oiRawAvail); err != nil {
		return err
	} else if ok {
		s.saveFeature(ctx, oiPercKey, asset, valueDate, pct, "exchange_oi", "exchange_v1")
	}

	fundZKey := domain.FeatureKey{Name: features.FundingRateZScore90dName, Version: fv}
	if z, ok := s.tryComputeZScore(ctx, fundDailyKey, asset, valueDate, cutoff,
		features.FundingZScoreWindowDays, features.FundingZScoreMinObs); ok {
		s.saveFeature(ctx, fundZKey, asset, valueDate, z, "exchange_funding", "exchange_v1")
	}

	basisZKey := domain.FeatureKey{Name: features.Basis3mZScore90dName, Version: fv}
	if z, ok := s.tryComputeZScore(ctx, basisDailyKey, asset, valueDate, cutoff,
		features.BasisZScoreWindowDays, features.BasisZScoreMinObs); ok {
		s.saveFeature(ctx, basisZKey, asset, valueDate, z, "coinglass_basis", "coinglass_v4")
	}

	liqZKey := domain.FeatureKey{Name: features.LiquidationsStress60dName, Version: fv}
	if z, ok := s.tryComputeZScore(ctx, liqDailyKey, asset, valueDate, cutoff,
		features.LiqStressWindowDays, features.LiqStressMinObs); ok {
		s.saveFeature(ctx, liqZKey, asset, valueDate, z, "exchange_liqs", "exchange_v1")
	}

	oiChangeZKey := domain.FeatureKey{Name: features.OIChangeZScore30d180dName, Version: fv}
	if z, ok := s.tryComputeZScore(ctx, oiChangeKey, asset, valueDate, cutoff,
		features.OIChangeZScoreWindowDays, features.OIChangeZScoreMinObs); ok {
		s.saveFeature(ctx, oiChangeZKey, asset, valueDate, z, "exchange_oi", "exchange_v1")
	}

	cpInputs, cpHave := s.gatherCrowdInputs(ctx, asset, valueDate, cutoff)
	if cpHave {
		if val, ok := features.TopTradersPositionSkewDaily(cpInputs); ok {
			s.saveFeature(ctx,
				domain.FeatureKey{Name: features.TopTradersPositionSkewDailyName, Version: v},
				asset, valueDate, val, "coinglass_long_short", "coinglass_v4")
		}
		if val, ok := features.CrowdVsSmartDivergenceDaily(cpInputs); ok {
			s.saveFeature(ctx,
				domain.FeatureKey{Name: features.CrowdVsSmartDivergenceDailyName, Version: v},
				asset, valueDate, val, "coinglass_long_short", "coinglass_v4")
		}
		if val, ok := features.TakerAggressionDaily(cpInputs); ok {
			s.saveFeature(ctx,
				domain.FeatureKey{Name: features.TakerAggressionDailyName, Version: v},
				asset, valueDate, val, "coinglass_taker_volume", "coinglass_v4")
		}
	}

	for _, p := range []struct {
		dailyName, zName, srcName, srcVer string
	}{
		{features.TopTradersPositionSkewDailyName, features.TopTradersPositionSkewZName, "coinglass_long_short", "coinglass_v4"},
		{features.CrowdVsSmartDivergenceDailyName, features.CrowdVsSmartDivergenceZName, "coinglass_long_short", "coinglass_v4"},
		{features.TakerAggressionDailyName, features.TakerAggressionZName, "coinglass_taker_volume", "coinglass_v4"},
	} {
		dailyKey := domain.FeatureKey{Name: p.dailyName, Version: v}
		zKey := domain.FeatureKey{Name: p.zName, Version: fv}
		if z, ok := s.tryComputeZScore(ctx, dailyKey, asset, valueDate, cutoff,
			features.CrowdPositioningZScoreWindowDays, features.CrowdPositioningZScoreMinObs); ok {
			s.saveFeature(ctx, zKey, asset, valueDate, z, p.srcName, p.srcVer)
		}
	}

	in, err := s.gatherScoreInputs(ctx, asset, valueDate, cutoff)
	if err != nil {
		return fmt.Errorf("gather score inputs: %w", err)
	}
	score := computeScore(in, s.cfg)
	if err := s.scores.Save(ctx, score); err != nil {
		return fmt.Errorf("save score: %w", err)
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "scores.leverage.completed.v1",
		AggregateID: fmt.Sprintf("%s:%s", asset, valueDate.Format("2006-01-02")),
		Payload: map[string]any{
			"asset":          asset.String(),
			"value_date":     valueDate.Format("2006-01-02"),
			"score":          score.Score,
			"model_version":  s.cfg.ModelVersion,
			"config_version": s.cfg.ConfigVersion,
		},
	})
}

func (s *Service) saveFeature(ctx context.Context, k domain.FeatureKey, asset domain.Asset, valueDate time.Time, v float64, srcName, srcVer string) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	f := domain.Feature{
		Key:               k,
		Asset:             asset,
		ValueDate:         valueDate,
		Timeframe:         "1d",
		Value:             v,
		SourceRawVersions: map[string]string{srcName: srcVer},
		CodeSHA:           s.cfg.CodeSHA,
	}
	if err := s.features.Save(ctx, f); err != nil {
		slog.Error("feature save failed", "key", k.String(), "asset", asset, "err", err)
	}
}

func (s *Service) computeOIPercentile(
	ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time,
	oiMcapKey, oiRawKey domain.FeatureKey, oiRaw float64, oiRawAvail bool,
) (float64, bool, error) {
	from := valueDate.AddDate(0, 0, -features.OIPercentileWindowDays)
	ratioSeries, err := s.features.GetSeries(ctx, oiMcapKey, asset, from, valueDate, cutoff)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return 0, false, err
	}
	if len(ratioSeries) >= features.OIPercentileMinObs {
		current := ratioSeries[len(ratioSeries)-1]
		if pct, ok := features.PercentileRank(current, ratioSeries); ok {
			return pct, true, nil
		}
	}

	if !oiRawAvail {
		return 0, false, nil
	}
	rawSeries, err := s.features.GetSeries(ctx, oiRawKey, asset, from, valueDate, cutoff)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return 0, false, err
	}
	rawSeries = append(rawSeries, oiRaw)
	if len(rawSeries) < features.OIPercentileMinObs {
		return 0, false, nil
	}
	pct, ok := features.PercentileRank(oiRaw, rawSeries)
	return pct, ok, nil
}

func (s *Service) tryComputeZScore(ctx context.Context, sourceKey domain.FeatureKey, asset domain.Asset, valueDate, cutoff time.Time, window, minObs int) (float64, bool) {
	from := valueDate.AddDate(0, 0, -(window - 1))
	series, err := s.features.GetSeries(ctx, sourceKey, asset, from, valueDate, cutoff)
	if err != nil {
		return 0, false
	}
	return features.ZScoreLatest(series, minObs)
}

func (s *Service) gatherScoreInputs(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (ScoreInputs, error) {
	in := ScoreInputs{Asset: asset, ValueDate: valueDate}
	fv := s.cfg.FinalVersion

	oiKey := domain.FeatureKey{Name: features.OIMcapPercentile365dName, Version: fv}
	if v, err := s.features.GetLatest(ctx, oiKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.OIPercentile, in.OIPercentileAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	fundKey := domain.FeatureKey{Name: features.FundingRateZScore90dName, Version: fv}
	if v, err := s.features.GetLatest(ctx, fundKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.FundingZ, in.FundingZAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	basisKey := domain.FeatureKey{Name: features.Basis3mZScore90dName, Version: fv}
	if v, err := s.features.GetLatest(ctx, basisKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.BasisZ, in.BasisZAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	liqKey := domain.FeatureKey{Name: features.LiquidationsStress60dName, Version: fv}
	if v, err := s.features.GetLatest(ctx, liqKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.LiqZ, in.LiqZAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	oiChangeKey := domain.FeatureKey{Name: features.OIChangeZScore30d180dName, Version: fv}
	if v, err := s.features.GetLatest(ctx, oiChangeKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.OIChangeZ, in.OIChangeZAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	posKey := domain.FeatureKey{Name: features.TopTradersPositionSkewZName, Version: fv}
	if v, err := s.features.GetLatest(ctx, posKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.PositionSkewZ, in.PositionSkewZAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	divKey := domain.FeatureKey{Name: features.CrowdVsSmartDivergenceZName, Version: fv}
	if v, err := s.features.GetLatest(ctx, divKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.CrowdDivergenceZ, in.CrowdDivergenceZAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	takKey := domain.FeatureKey{Name: features.TakerAggressionZName, Version: fv}
	if v, err := s.features.GetLatest(ctx, takKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.TakerAggressionZ, in.TakerAggressionZAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	in.FeatureCodesUsed = []string{
		oiKey.String(), fundKey.String(), basisKey.String(), liqKey.String(), oiChangeKey.String(),
		posKey.String(), divKey.String(), takKey.String(),
	}
	return in, nil
}

func (s *Service) gatherCrowdInputs(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (features.CrowdPositioningInputs, bool) {
	in := features.CrowdPositioningInputs{}
	any := false
	sym := string(asset)
	if v, ok, err := s.raws.CoinglassLSRatioAvgAsOf(ctx, LSGlobal, sym, valueDate, cutoff); err == nil && ok {
		in.GlobalAccountRatio, any = v, true
	}
	if v, ok, err := s.raws.CoinglassLSRatioAvgAsOf(ctx, LSTopAccount, sym, valueDate, cutoff); err == nil && ok {
		in.TopAccountRatio, any = v, true
	}
	if v, ok, err := s.raws.CoinglassLSRatioAvgAsOf(ctx, LSTopPosition, sym, valueDate, cutoff); err == nil && ok {
		in.TopPositionRatio, any = v, true
	}
	if buy, sell, ok, err := s.raws.CoinglassTakerVolumeAsOf(ctx, sym, valueDate, cutoff); err == nil && ok {
		in.TakerBuyUSD, in.TakerSellUSD = buy, sell
		any = true
	}
	return in, any
}
