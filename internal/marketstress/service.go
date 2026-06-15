package marketstress

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/marketstress/features"
)

var KlineSymbols = []string{
	"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "ADAUSDT", "DOGEUSDT",
}

var KrakenPairs = []string{"USDTUSD", "USDCUSD"}

type Service struct {
	features  FeatureRepo
	scores    ScoreRepo
	raws      RawRepo
	binance   BinanceSpotProvider
	kraken    KrakenProvider
	coinbase  CoinbaseProvider
	coinglass CoinglassProvider
	cgMicro   CoinglassMicroProvider
	leverage  LeverageFeatureReader
	publisher Publisher
	clock     Clock
	cfg       Config
	assets    []domain.Asset
}

func NewService(
	featRepo FeatureRepo,
	scoreRepo ScoreRepo,
	rawRepo RawRepo,
	bin BinanceSpotProvider,
	kr KrakenProvider,
	cb CoinbaseProvider,
	cg CoinglassProvider,
	cgMicro CoinglassMicroProvider,
	lev LeverageFeatureReader,
	pub Publisher,
	clk Clock,
	cfg Config,
	assets []domain.Asset,
) *Service {
	cfg.Defaults()
	if len(assets) == 0 {
		assets = domain.AssetsTradeable()
	}
	return &Service{
		features: featRepo, scores: scoreRepo, raws: rawRepo,
		binance: bin, kraken: kr, coinbase: cb, coinglass: cg, cgMicro: cgMicro,
		leverage:  lev,
		publisher: pub, clock: clk, cfg: cfg, assets: assets,
	}
}

func (s *Service) RunOnce(ctx context.Context, valueDate time.Time) error {
	valueDate = domain.UTCDay(valueDate)
	from := valueDate.AddDate(0, 0, -210)
	if err := s.ingestAll(ctx, from, valueDate); err != nil {
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
	histStart := from.AddDate(0, 0, -210)
	if err := s.ingestAll(ctx, histStart, to); err != nil {
		return err
	}
	cutoff := s.clock.Now().Add(24 * time.Hour)
	return s.computeRange(ctx, from, to, cutoff)
}

func (s *Service) BackfillKlines(ctx context.Context, from, to time.Time) error {
	from, to = domain.UTCDay(from), domain.UTCDay(to)
	if from.After(to) {
		return fmt.Errorf("backfill klines from (%s) after to (%s)",
			from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
	return s.ingestKlines(ctx, from, to)
}

func (s *Service) ingestAll(ctx context.Context, from, to time.Time) error {
	if err := s.ingestKlines(ctx, from, to); err != nil {
		return fmt.Errorf("ingest klines: %w", err)
	}
	if err := s.ingestKrakenOHLC(ctx, from); err != nil {
		return fmt.Errorf("ingest kraken: %w", err)
	}
	if err := s.ingestCoinbaseCandles(ctx, from, to); err != nil {
		return fmt.Errorf("ingest coinbase: %w", err)
	}
	if err := s.ingestCoinbasePremium(ctx); err != nil {
		return fmt.Errorf("ingest coinglass coinbase premium: %w", err)
	}
	if err := s.ingestCoinglassMicro(ctx); err != nil {
		return fmt.Errorf("ingest coinglass micro: %w", err)
	}
	return nil
}

func (s *Service) ingestCoinglassMicro(ctx context.Context) error {
	if s.cgMicro == nil {
		return nil
	}
	const limit = 1000
	for _, asset := range s.assets {
		sym := string(asset)

		obPts, err := s.cgMicro.FetchOrderbookBidAsk(ctx, sym, limit, time.Time{}, time.Time{})
		if err != nil {
			slog.Warn("coinglass orderbook fetch failed (non-fatal)", "asset", asset, "err", err)
		} else if len(obPts) > 0 {
			rows := make([]CoinglassOrderbookRow, 0, len(obPts))
			for _, p := range obPts {
				rows = append(rows, CoinglassOrderbookRow{
					ValueDate:     p.Date,
					Symbol:        sym,
					RangePct:      "1",
					BidsUSD:       p.BidsUSD,
					BidsQty:       p.BidsQty,
					AsksUSD:       p.AsksUSD,
					AsksQty:       p.AsksQty,
					SourceVersion: "coinglass_v4",
					PayloadHash:   p.PayloadHash,
				})
			}
			if err := s.raws.SaveCoinglassOrderbookAggregated(ctx, rows); err != nil {
				return fmt.Errorf("save orderbook %s: %w", asset, err)
			}
			if err := s.publisher.Publish(ctx, domain.Event{
				Topic:       "raw.market_stress.orderbook.coinglass.ingested.v1",
				AggregateID: fmt.Sprintf("%s:orderbook", asset),
				Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
			}); err != nil {
				return err
			}
		}

		fsPts, err := s.cgMicro.FetchFuturesSpotVolRatio(ctx, sym, limit, time.Time{}, time.Time{})
		if err != nil {
			slog.Warn("coinglass futures/spot ratio fetch failed (non-fatal)", "asset", asset, "err", err)
		} else if len(fsPts) > 0 {
			rows := make([]CoinglassFuturesSpotVolRatioRow, 0, len(fsPts))
			for _, p := range fsPts {
				rows = append(rows, CoinglassFuturesSpotVolRatioRow{
					ValueDate:        p.Date,
					Symbol:           sym,
					FuturesSpotRatio: p.FuturesSpotRatio,
					FuturesVolUSD:    p.FuturesVolUSD,
					SpotVolUSD:       p.SpotVolUSD,
					SourceVersion:    "coinglass_v4",
					PayloadHash:      p.PayloadHash,
				})
			}
			if err := s.raws.SaveCoinglassFuturesSpotVolRatio(ctx, rows); err != nil {
				return fmt.Errorf("save futures/spot ratio %s: %w", asset, err)
			}
			if err := s.publisher.Publish(ctx, domain.Event{
				Topic:       "raw.market_stress.futures_spot_ratio.coinglass.ingested.v1",
				AggregateID: fmt.Sprintf("%s:futures_spot_ratio", asset),
				Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ingestKlines(ctx context.Context, from, to time.Time) error {
	if s.binance == nil {
		return nil
	}
	for _, sym := range KlineSymbols {
		pts, err := s.binance.FetchKlines(ctx, sym, "1d", from, to)
		if err != nil {
			return fmt.Errorf("binance klines %s: %w", sym, err)
		}
		if len(pts) == 0 {
			continue
		}
		rows := make([]BinanceKlineRow, 0, len(pts))
		for _, p := range pts {
			rows = append(rows, BinanceKlineRow{
				ValueDate:     p.OpenTime,
				Symbol:        sym,
				Close:         p.Close,
				Volume:        0,
				SourceVersion: "binance_spot_v1",
				PayloadHash:   p.PayloadHash,
			})
		}
		if err := s.raws.SaveBinanceKlines(ctx, rows); err != nil {
			return err
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.market_stress.klines.ingested.v1",
			AggregateID: fmt.Sprintf("binance:%s", sym),
			Payload:     map[string]any{"rows": len(pts), "symbol": sym},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ingestKrakenOHLC(ctx context.Context, from time.Time) error {
	if s.kraken == nil {
		return nil
	}
	for _, pair := range KrakenPairs {
		pts, err := s.kraken.FetchOHLC(ctx, pair, 1440, from.Unix())
		if err != nil {
			return fmt.Errorf("kraken OHLC %s: %w", pair, err)
		}
		if len(pts) == 0 {
			continue
		}
		rows := make([]KrakenOHLCRow, 0, len(pts))
		for _, p := range pts {
			rows = append(rows, KrakenOHLCRow{
				ValueDate:     p.Date,
				Pair:          pair,
				Open:          p.Open,
				High:          p.High,
				Low:           p.Low,
				Close:         p.Close,
				Volume:        p.Volume,
				SourceVersion: "kraken_v1",
				PayloadHash:   p.PayloadHash,
			})
		}
		if err := s.raws.SaveKrakenOHLC(ctx, rows); err != nil {
			return err
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.market_stress.kraken.ingested.v1",
			AggregateID: fmt.Sprintf("kraken:%s", pair),
			Payload:     map[string]any{"rows": len(pts), "pair": pair},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ingestCoinbaseCandles(ctx context.Context, from, to time.Time) error {
	if s.coinbase == nil {
		return nil
	}
	pts, err := s.coinbase.FetchCandles(ctx, "BTC-USD", from, to, 86400)
	if err != nil {
		return fmt.Errorf("coinbase candles: %w", err)
	}
	if len(pts) == 0 {
		return nil
	}
	rows := make([]CoinbaseCandleRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, CoinbaseCandleRow{
			ValueDate:     p.Date,
			ProductID:     p.ProductID,
			Close:         p.Close,
			Volume:        p.Volume,
			SourceVersion: "coinbase_v1",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveCoinbaseCandles(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "raw.market_stress.coinbase.ingested.v1",
		AggregateID: "coinbase:BTC-USD",
		Payload:     map[string]any{"rows": len(pts)},
	})
}

func (s *Service) ingestCoinbasePremium(ctx context.Context) error {
	if s.coinglass == nil {
		return nil
	}
	pts, err := s.coinglass.FetchCoinbasePremiumHistory(ctx, 1000)
	if err != nil {
		return fmt.Errorf("coinglass coinbase premium: %w", err)
	}
	if len(pts) == 0 {
		return nil
	}
	rows := make([]CoinglassCoinbasePremiumRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, CoinglassCoinbasePremiumRow{
			ValueDate:     p.Date,
			PremiumUSD:    p.PremiumUSD,
			PremiumRate:   p.PremiumRate,
			CoinbasePrice: p.CoinbasePrice,
			SourceVersion: "coinglass_v4",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveCoinglassCoinbasePremium(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "raw.market_stress.coinglass_coinbase_premium.ingested.v1",
		AggregateID: "coinglass:coinbase_premium",
		Payload:     map[string]any{"rows": len(pts)},
	})
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

	if err := s.computeGlobalFeatures(ctx, valueDate, cutoff); err != nil {
		return fmt.Errorf("global features: %w", err)
	}

	for _, asset := range s.assets {
		if err := s.computeAssetDay(ctx, asset, valueDate, cutoff); err != nil {
			return fmt.Errorf("asset %s: %w", asset, err)
		}
	}
	return nil
}

func (s *Service) computeGlobalFeatures(ctx context.Context, valueDate, cutoff time.Time) error {
	intKey := func(name string) domain.FeatureKey {
		return domain.FeatureKey{Name: name, Version: s.cfg.IntermediateVersion}
	}
	finKey := func(name string) domain.FeatureKey {
		return domain.FeatureKey{Name: name, Version: s.cfg.FinalVersion}
	}

	corrKey := intKey(features.BtcAltCorrelation30dName)
	if v, ok, err := s.computeBtcAltCorrelation(ctx, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveGlobalFeature(ctx, corrKey, valueDate, v, "binance_spot_v1")
	}

	pegKey := intKey(features.PegDeviationDailyName)
	if v, ok, err := s.computePegDeviation(ctx, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveGlobalFeature(ctx, pegKey, valueDate, v, "kraken_v1")
	}

	premKey := intKey(features.CoinbasePremiumAbsName)
	if v, ok, err := s.computeCoinbasePremiumAbs(ctx, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveGlobalFeature(ctx, premKey, valueDate, v, "coinglass_v4")
	}

	corrZKey := finKey(features.BtcAltCorrelationZScore180dName)
	if z, ok := s.tryComputeZScore(ctx, corrKey, domain.AssetGlobal, valueDate, cutoff,
		features.CorrelationZScoreWindowDays, features.CorrelationZScoreMinObs); ok {
		s.saveGlobalFeature(ctx, corrZKey, valueDate, z, "binance_spot_v1")
	}

	pegZKey := finKey(features.StablecoinPegStressScoreName)
	if z, ok := s.tryComputeZScore(ctx, pegKey, domain.AssetGlobal, valueDate, cutoff,
		features.PegZScoreWindowDays, features.PegZScoreMinObs); ok {
		s.saveGlobalFeature(ctx, pegZKey, valueDate, z, "kraken_v1")
	}

	premZKey := finKey(features.CoinbasePremiumAbsZScore90dName)
	if z, ok := s.tryComputeZScore(ctx, premKey, domain.AssetGlobal, valueDate, cutoff,
		features.CoinbasePremiumZScoreWindowDays, features.CoinbasePremiumZScoreMinObs); ok {
		s.saveGlobalFeature(ctx, premZKey, valueDate, z, "coinglass_v4")
	}
	return nil
}

func (s *Service) computeAssetDay(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) error {
	intKey := domain.FeatureKey{Name: features.BasisInversionDepthName, Version: s.cfg.IntermediateVersion}
	finKey := domain.FeatureKey{Name: features.BasisInversionDepthZScore180dName, Version: s.cfg.FinalVersion}

	if v, ok, err := s.computeBasisInversionDepth(ctx, asset, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveFeature(ctx, intKey, asset, valueDate, v, "leverage_basis", s.cfg.LeverageBasisVersion)
	}

	if z, ok := s.tryComputeZScore(ctx, intKey, asset, valueDate, cutoff,
		features.BasisInversionZScoreWindowDays, features.BasisInversionZScoreMinObs); ok {
		s.saveFeature(ctx, finKey, asset, valueDate, z, "leverage_basis", s.cfg.LeverageBasisVersion)
	}

	sym := string(asset)
	bookKey := domain.FeatureKey{Name: features.BookImbalanceDailyName, Version: s.cfg.IntermediateVersion}
	bookZKey := domain.FeatureKey{Name: features.BookImbalanceZScoreName, Version: s.cfg.FinalVersion}
	fsKey := domain.FeatureKey{Name: features.FuturesSpotRatioDailyName, Version: s.cfg.IntermediateVersion}
	fsZKey := domain.FeatureKey{Name: features.FuturesSpotRatioZScoreName, Version: s.cfg.FinalVersion}

	if v, ok, err := s.raws.GetOrderbookImbalanceAsOf(ctx, sym, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveFeature(ctx, bookKey, asset, valueDate, v, "coinglass_orderbook", "coinglass_v4")
	}
	if z, ok := s.tryComputeZScore(ctx, bookKey, asset, valueDate, cutoff,
		features.MicrostructureZScoreWindowDays, features.MicrostructureZScoreMinObs); ok {
		s.saveFeature(ctx, bookZKey, asset, valueDate, z, "coinglass_orderbook", "coinglass_v4")
	}

	if v, ok, err := s.raws.GetFuturesSpotRatioAsOf(ctx, sym, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveFeature(ctx, fsKey, asset, valueDate, v, "coinglass_futures_spot", "coinglass_v4")
	}
	if z, ok := s.tryComputeZScore(ctx, fsKey, asset, valueDate, cutoff,
		features.MicrostructureZScoreWindowDays, features.MicrostructureZScoreMinObs); ok {
		s.saveFeature(ctx, fsZKey, asset, valueDate, z, "coinglass_futures_spot", "coinglass_v4")
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
		Topic:       "scores.market_stress.completed.v1",
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

func (s *Service) computeBtcAltCorrelation(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error) {
	from := valueDate.AddDate(0, 0, -features.CorrelationWindowDays)
	btcPrices, err := s.raws.GetBinanceKlineCloseSeries(ctx, "BTCUSDT", from, valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	if len(btcPrices) < features.CorrelationMinPrices {
		return 0, false, nil
	}
	btcReturns := features.LogReturns(btcPrices)
	altLists := make([][]float64, 0, len(features.AltSymbols))
	for _, sym := range features.AltSymbols {
		altPrices, err := s.raws.GetBinanceKlineCloseSeries(ctx, sym, from, valueDate, cutoff)
		if err != nil {

			continue
		}
		altLists = append(altLists, features.LogReturns(altPrices))
	}
	avg, ok := features.AvgBtcAltCorrelation(btcReturns, altLists)
	return avg, ok, nil
}

func (s *Service) computePegDeviation(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error) {
	usdt, usdtOk, err := s.getKrakenClose(ctx, "USDTUSD", valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	usdc, usdcOk, err := s.getKrakenClose(ctx, "USDCUSD", valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	v, ok := features.PegDeviationScaled(usdt, usdtOk, usdc, usdcOk)
	return v, ok, nil
}

func (s *Service) computeCoinbasePremiumAbs(ctx context.Context, valueDate, cutoff time.Time) (float64, bool, error) {

	rate, err := s.raws.GetCoinglassCoinbasePremiumRateAsOf(ctx, valueDate, cutoff)
	if err == nil {
		return math.Abs(rate), true, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return 0, false, err
	}

	cb, cbOk, err := s.getCoinbaseClose(ctx, "BTC-USD", valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	bin, binOk, err := s.getBinanceClose(ctx, "BTCUSDT", valueDate, cutoff)
	if err != nil {
		return 0, false, err
	}
	if !cbOk || !binOk {
		return 0, false, nil
	}
	v, ok := features.CoinbasePremiumAbsFromPrices(cb, bin)
	return v, ok, nil
}

func (s *Service) computeBasisInversionDepth(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool, error) {
	if s.leverage == nil {
		return 0, false, nil
	}
	basis, err := s.leverage.GetBasis3mDailyAnyVersion(ctx, asset, valueDate, cutoff)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return features.BasisInversionDepth(basis), true, nil
}

func (s *Service) getBinanceClose(ctx context.Context, symbol string, valueDate, cutoff time.Time) (float64, bool, error) {
	v, err := s.raws.GetBinanceKlineCloseAsOf(ctx, symbol, valueDate, cutoff)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

func (s *Service) getKrakenClose(ctx context.Context, pair string, valueDate, cutoff time.Time) (float64, bool, error) {
	v, err := s.raws.GetKrakenCloseAsOf(ctx, pair, valueDate, cutoff)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

func (s *Service) getCoinbaseClose(ctx context.Context, productID string, valueDate, cutoff time.Time) (float64, bool, error) {
	v, err := s.raws.GetCoinbaseCloseAsOf(ctx, productID, valueDate, cutoff)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, true, nil
}

func (s *Service) saveGlobalFeature(ctx context.Context, k domain.FeatureKey, valueDate time.Time, v float64, srcName string) {
	s.saveFeature(ctx, k, domain.AssetGlobal, valueDate, v, srcName, sourceVersionFor(srcName))
}

func sourceVersionFor(srcName string) string {
	switch srcName {
	case "binance_spot_v1":
		return "binance_spot_v1"
	case "kraken_v1":
		return "kraken_v1"
	case "coinbase_v1":
		return "coinbase_v1"
	case "coinglass_v4":
		return "coinglass_v4"
	}
	return srcName
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
	corrKey := domain.FeatureKey{Name: features.BtcAltCorrelationZScore180dName, Version: s.cfg.FinalVersion}
	pegKey := domain.FeatureKey{Name: features.StablecoinPegStressScoreName, Version: s.cfg.FinalVersion}
	cbKey := domain.FeatureKey{Name: features.CoinbasePremiumAbsZScore90dName, Version: s.cfg.FinalVersion}
	basisKey := domain.FeatureKey{Name: features.BasisInversionDepthZScore180dName, Version: s.cfg.FinalVersion}

	if v, err := s.features.GetLatest(ctx, corrKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZCorrelation, in.ZCorrelationAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	if v, err := s.features.GetLatest(ctx, pegKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZPeg, in.ZPegAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	if v, err := s.features.GetLatest(ctx, cbKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZCoinbase, in.ZCoinbaseAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	if v, err := s.features.GetLatest(ctx, basisKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZBasis, in.ZBasisAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	bookZKey := domain.FeatureKey{Name: features.BookImbalanceZScoreName, Version: s.cfg.FinalVersion}
	fsZKey := domain.FeatureKey{Name: features.FuturesSpotRatioZScoreName, Version: s.cfg.FinalVersion}
	if v, err := s.features.GetLatest(ctx, bookZKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZBookImbalance, in.ZBookImbalanceAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	if v, err := s.features.GetLatest(ctx, fsZKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZFuturesSpotRatio, in.ZFuturesSpotRatioAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	in.FeatureCodesUsed = []string{
		corrKey.String(), pegKey.String(), cbKey.String(), basisKey.String(),
		bookZKey.String(), fsZKey.String(),
	}
	return in, nil
}
