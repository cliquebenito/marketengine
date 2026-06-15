package liquidity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/liquidity/features"
)

type Service struct {
	features  FeatureRepo
	scores    ScoreRepo
	raws      RawRepo
	stable    StablecoinProvider
	tvl       ChainTVLProvider
	netflow   ExchangeNetflowProvider
	mcap      MarketCapProvider
	publisher Publisher
	clock     Clock
	cfg       Config
	assets    []domain.Asset
}

func NewService(
	featRepo FeatureRepo,
	scoreRepo ScoreRepo,
	rawRepo RawRepo,
	stable StablecoinProvider,
	tvl ChainTVLProvider,
	netflow ExchangeNetflowProvider,
	mcap MarketCapProvider,
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
		stable: stable, tvl: tvl, netflow: netflow, mcap: mcap,
		publisher: pub, clock: clk, cfg: cfg, assets: assets,
	}
}

func (s *Service) RunOnce(ctx context.Context, valueDate time.Time) error {
	valueDate = domain.UTCDay(valueDate)
	if err := s.ingestAll(ctx, valueDate.AddDate(0, 0, -210), valueDate); err != nil {
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
	if err := s.ingestStablecoinSupply(ctx); err != nil {
		return fmt.Errorf("ingest supply: %w", err)
	}
	if err := s.ingestPerStablecoinSupply(ctx); err != nil {
		return fmt.Errorf("ingest per-stablecoin supply: %w", err)
	}
	if err := s.ingestChainTVL(ctx); err != nil {
		return fmt.Errorf("ingest tvl: %w", err)
	}
	if err := s.ingestMarketCap(ctx, []string{"ethereum"}); err != nil {
		return fmt.Errorf("ingest market_cap: %w", err)
	}
	if err := s.ingestExchangeNetflow(ctx, from, to); err != nil {
		return fmt.Errorf("ingest netflow: %w", err)
	}
	return nil
}

func (s *Service) ingestStablecoinSupply(ctx context.Context) error {
	if s.stable == nil {
		return nil
	}
	pts, err := s.stable.FetchAllStablecoinsChart(ctx)
	if err != nil {
		return err
	}
	if len(pts) == 0 {
		return nil
	}
	rows := make([]StablecoinSupplyRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, StablecoinSupplyRow{
			ValueDate:     p.Date,
			Stablecoin:    "AGGREGATE",
			Metric:        "circulating_supply_usd",
			Value:         p.CirculatingUSD,
			SourceVersion: "defillama_v1",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveStablecoinSupply(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "raw.liquidity.stablecoin_supply.ingested.v1",
		AggregateID: fmt.Sprintf("defillama:%s", pts[len(pts)-1].Date.Format("2006-01-02")),
		Payload:     map[string]any{"rows_touched": len(rows)},
	})
}

func (s *Service) ingestPerStablecoinSupply(ctx context.Context) error {
	if s.stable == nil {
		return nil
	}
	pts, err := s.stable.FetchPerStablecoinSupply(ctx)
	if err != nil {
		return err
	}
	if len(pts) == 0 {
		return nil
	}
	rows := make([]StablecoinSupplyRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, StablecoinSupplyRow{
			ValueDate:     p.Date,
			Stablecoin:    p.Symbol,
			Metric:        "circulating_supply_usd",
			Value:         p.CirculatingUSD,
			SourceVersion: "defillama_v1",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveStablecoinSupply(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "raw.liquidity.per_stablecoin_supply.ingested.v1",
		AggregateID: fmt.Sprintf("defillama_per_stablecoin:%s", pts[len(pts)-1].Date.Format("2006-01-02")),
		Payload:     map[string]any{"rows_touched": len(rows)},
	})
}

func (s *Service) ingestChainTVL(ctx context.Context) error {
	if s.tvl == nil {
		return nil
	}
	pts, err := s.tvl.FetchChainTVL(ctx, "Ethereum")
	if err != nil {
		return err
	}
	if len(pts) == 0 {
		return nil
	}
	rows := make([]ChainTVLRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, ChainTVLRow{
			ValueDate:     p.Date,
			Chain:         "Ethereum",
			TVLUSD:        p.TVLUSD,
			SourceVersion: "defillama_v1",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveChainTVL(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "raw.liquidity.tvl.ingested.v1",
		AggregateID: fmt.Sprintf("defillama_tvl:%s", pts[len(pts)-1].Date.Format("2006-01-02")),
		Payload:     map[string]any{"rows_touched": len(rows), "chain": "Ethereum"},
	})
}

func (s *Service) ingestMarketCap(ctx context.Context, coinIDs []string) error {
	if s.mcap == nil || len(coinIDs) == 0 {
		return nil
	}
	for _, coinID := range coinIDs {
		pts, err := s.mcap.FetchMarketCapHistory(ctx, coinID)
		if err != nil {

			slog.Warn("market_cap fetch failed (non-fatal)", "coin_id", coinID, "err", err)
			continue
		}
		if len(pts) == 0 {
			continue
		}
		rows := make([]MarketCapRow, 0, len(pts))
		for _, p := range pts {
			rows = append(rows, MarketCapRow{
				ValueDate:     p.Date,
				CoinID:        p.CoinID,
				MarketCapUSD:  p.MarketCapUSD,
				PriceUSD:      p.PriceUSD,
				SourceVersion: "coingecko_v1",
				PayloadHash:   p.PayloadHash,
			})
		}
		if err := s.raws.SaveMarketCap(ctx, rows); err != nil {
			return err
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.liquidity.market_cap.ingested.v1",
			AggregateID: fmt.Sprintf("coingecko:%s:%s", coinID, pts[len(pts)-1].Date.Format("2006-01-02")),
			Payload:     map[string]any{"rows_touched": len(rows), "coin_id": coinID},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ingestExchangeNetflow(ctx context.Context, start, end time.Time) error {
	if s.netflow == nil {
		return nil
	}
	pts, err := s.netflow.FetchExchangeNetflow(ctx, s.assets, start, end)
	if err != nil {
		return err
	}
	if len(pts) == 0 {
		return nil
	}
	rows := make([]ExchangeNetflowRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, ExchangeNetflowRow{
			ValueDate:     p.Date,
			Asset:         p.Asset,
			InflowUSD:     p.InflowUSD,
			OutflowUSD:    p.OutflowUSD,
			NetflowUSD:    p.NetflowUSD,
			SourceVersion: "coinmetrics_v1",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveExchangeNetflow(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "raw.liquidity.exchange_netflow.ingested.v1",
		AggregateID: fmt.Sprintf("coinmetrics:%s", pts[len(pts)-1].Date.Format("2006-01-02")),
		Payload:     map[string]any{"rows_touched": len(rows)},
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

	supplyKey := domain.FeatureKey{Name: features.StablecoinSupplyTotalName, Version: s.cfg.SupplyFeatureVersion}
	if v, err := s.raws.GetStablecoinSupplyAsOf(ctx, "AGGREGATE", valueDate, cutoff); err == nil {
		s.saveGlobalFeature(ctx, supplyKey, valueDate, v, "defillama_stablecoin_supply", "defillama_v1")
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	majorKey := domain.FeatureKey{Name: features.StablecoinSupplyMajorName, Version: s.cfg.SupplyMajorFeatureVersion}
	if sum, found, err := s.raws.SumStablecoinSupplyAsOf(ctx, []string{"USDT", "USDC", "DAI"}, valueDate, cutoff); err != nil {
		return err
	} else if found > 0 {
		s.saveGlobalFeature(ctx, majorKey, valueDate, sum, "defillama_stablecoin_supply", "defillama_v1")
	}

	growthKey := domain.FeatureKey{Name: features.StablecoinGrowth30dName, Version: s.cfg.GrowthFeatureVersion}
	if growth, ok := s.tryComputeGrowth(ctx, supplyKey, valueDate, cutoff, features.GrowthWindowDays); ok {
		s.saveGlobalFeature(ctx, growthKey, valueDate, growth, "defillama_stablecoin_supply", "defillama_v1")
	}

	zscoreKey := domain.FeatureKey{Name: features.StablecoinGrowthZScore90dName, Version: s.cfg.ZScoreFeatureVersion}
	if z, ok := s.tryComputeZScore(ctx, growthKey, domain.AssetGlobal, valueDate, cutoff,
		features.StablecoinZScoreWindowDays, features.StablecoinZScoreMinObs); ok {
		s.saveGlobalFeature(ctx, zscoreKey, valueDate, z, "defillama_stablecoin_supply", "defillama_v1")
	}

	tvlKey := domain.FeatureKey{Name: features.DefiTVLUsdName, Version: s.cfg.TVLFeatureVersion}
	if v, err := s.raws.GetChainTVLAsOf(ctx, "Ethereum", valueDate, cutoff); err == nil {
		s.saveGlobalFeature(ctx, tvlKey, valueDate, v, "defillama_tvl", "defillama_v1")
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	tvlGrowthKey := domain.FeatureKey{Name: features.DefiTVLGrowth30dName, Version: s.cfg.TVLGrowthFeatureVersion}
	if growth, ok := s.tryComputeGrowth(ctx, tvlKey, valueDate, cutoff, features.TVLGrowthWindowDays); ok {
		s.saveGlobalFeature(ctx, tvlGrowthKey, valueDate, growth, "defillama_tvl", "defillama_v1")
	}

	tvlZKey := domain.FeatureKey{Name: features.DefiTVLGrowthZScore180dName, Version: s.cfg.TVLZScoreFeatureVersion}
	if z, ok := s.tryComputeZScore(ctx, tvlGrowthKey, domain.AssetGlobal, valueDate, cutoff,
		features.TVLZScoreWindowDays, features.TVLZScoreMinObs); ok {
		s.saveGlobalFeature(ctx, tvlZKey, valueDate, z, "defillama_tvl", "defillama_v1")
	}
	return nil
}

func (s *Service) computeAssetDay(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) error {
	netflow7Key := domain.FeatureKey{Name: features.ExchangeNetflow7dName, Version: s.cfg.Netflow7dFeatureVersion}
	netflowZKey := domain.FeatureKey{Name: features.ExchangeNetflowZScore180dName, Version: s.cfg.NetflowZScoreFeatureVersion}
	ssrKey := domain.FeatureKey{Name: features.SSRName, Version: s.cfg.SSRFeatureVersion}
	ssrPctKey := domain.FeatureKey{Name: features.SSRPercentileRank365dName, Version: s.cfg.SSRPercentileFeatureVersion}
	supplyKey := domain.FeatureKey{Name: features.StablecoinSupplyTotalName, Version: s.cfg.SupplyFeatureVersion}

	if sum, complete, err := s.raws.Sum7dNetflow(ctx, asset, valueDate, cutoff); err != nil {
		return err
	} else if complete {
		s.saveFeature(ctx, netflow7Key, asset, valueDate, sum, "coinmetrics_exchange_netflow", "coinmetrics_v1")
	}

	if z, ok := s.tryComputeZScore(ctx, netflow7Key, asset, valueDate, cutoff,
		features.NetflowZScoreWindowDays, features.NetflowZScoreMinObs); ok {
		s.saveFeature(ctx, netflowZKey, asset, valueDate, z, "coinmetrics_exchange_netflow", "coinmetrics_v1")
	}

	if mcap, err := s.raws.GetMarketCapAsOf(ctx, features.CoinIDForAsset(string(asset)), valueDate, cutoff); err == nil && mcap > 0 {
		if supply, err := s.features.GetLatest(ctx, supplyKey, domain.AssetGlobal, valueDate, cutoff); err == nil && supply > 0 {
			s.saveFeature(ctx, ssrKey, asset, valueDate, mcap/supply, "coingecko_market_cap", "coingecko_v1")
		}
	}

	if pct, ok := s.tryComputePercentileRank(ctx, ssrKey, asset, valueDate, cutoff,
		features.SSRPercentileWindowDays, features.SSRPercentileMinObs); ok {
		s.saveFeature(ctx, ssrPctKey, asset, valueDate, pct, "coingecko_market_cap", "coingecko_v1")
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
		Topic:       "scores.liquidity.completed.v1",
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

func (s *Service) saveGlobalFeature(ctx context.Context, k domain.FeatureKey, valueDate time.Time, v float64, srcName, srcVer string) {
	s.saveFeature(ctx, k, domain.AssetGlobal, valueDate, v, srcName, srcVer)
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

func (s *Service) tryComputeGrowth(ctx context.Context, sourceKey domain.FeatureKey, valueDate, cutoff time.Time, window int) (float64, bool) {
	now, err := s.features.GetLatest(ctx, sourceKey, domain.AssetGlobal, valueDate, cutoff)
	if err != nil {
		return 0, false
	}
	past := valueDate.AddDate(0, 0, -window)
	earlier, err := s.features.GetLatest(ctx, sourceKey, domain.AssetGlobal, past, cutoff)
	if err != nil {
		return 0, false
	}
	return features.PctChange(now, earlier)
}

func (s *Service) tryComputeZScore(ctx context.Context, sourceKey domain.FeatureKey, asset domain.Asset, valueDate, cutoff time.Time, window, minObs int) (float64, bool) {
	from := valueDate.AddDate(0, 0, -(window - 1))
	series, err := s.features.GetSeries(ctx, sourceKey, asset, from, valueDate, cutoff)
	if err != nil {
		return 0, false
	}
	return features.ZScoreLatest(series, minObs)
}

func (s *Service) tryComputePercentileRank(ctx context.Context, sourceKey domain.FeatureKey, asset domain.Asset, valueDate, cutoff time.Time, window, minObs int) (float64, bool) {
	current, err := s.features.GetLatest(ctx, sourceKey, asset, valueDate, cutoff)
	if err != nil {
		return 0, false
	}
	from := valueDate.AddDate(0, 0, -window)
	series, err := s.features.GetSeries(ctx, sourceKey, asset, from, valueDate, cutoff)
	if err != nil {
		return 0, false
	}
	if len(series) < minObs {
		return 0, false
	}
	return features.PercentileRank(current, series)
}

func (s *Service) gatherScoreInputs(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (ScoreInputs, error) {
	in := ScoreInputs{Asset: asset, ValueDate: valueDate}

	stableKey := domain.FeatureKey{Name: features.StablecoinGrowthZScore90dName, Version: s.cfg.ZScoreFeatureVersion}
	if v, err := s.features.GetLatest(ctx, stableKey, domain.AssetGlobal, valueDate, cutoff); err == nil {
		in.ZStable, in.ZStableAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	ssrKey := domain.FeatureKey{Name: features.SSRPercentileRank365dName, Version: s.cfg.SSRPercentileFeatureVersion}
	if v, err := s.features.GetLatest(ctx, ssrKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.SSRPercentile, in.SSRAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	netflowKey := domain.FeatureKey{Name: features.ExchangeNetflowZScore180dName, Version: s.cfg.NetflowZScoreFeatureVersion}
	if v, err := s.features.GetLatest(ctx, netflowKey, asset, valueDate, cutoff); err == nil {
		in.ZNetflow, in.ZNetflowAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	tvlKey := domain.FeatureKey{Name: features.DefiTVLGrowthZScore180dName, Version: s.cfg.TVLZScoreFeatureVersion}
	if v, err := s.features.GetLatest(ctx, tvlKey, domain.AssetGlobal, valueDate, cutoff); err == nil {
		in.ZTVL, in.ZTVLAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	in.FeatureCodesUsed = []string{
		stableKey.String(), ssrKey.String(), netflowKey.String(), tvlKey.String(),
	}
	return in, nil
}
