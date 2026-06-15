package volatility

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"marketengine/internal/domain"
	"marketengine/internal/volatility/features"
	mmath "marketengine/pkg/math"
)

type Service struct {
	features  FeatureRepo
	scores    ScoreRepo
	raws      RawRepo
	dvol      DvolProvider
	options   OptionsChainProvider
	cgOptions CoinglassOptionsProvider
	chain     DeribitChainProvider
	publisher Publisher
	clock     Clock
	cfg       Config
	assets    []domain.Asset
}

func NewService(
	featRepo FeatureRepo,
	scoreRepo ScoreRepo,
	rawRepo RawRepo,
	dvol DvolProvider,
	options OptionsChainProvider,
	cgOptions CoinglassOptionsProvider,
	chain DeribitChainProvider,
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
		dvol: dvol, options: options, cgOptions: cgOptions, chain: chain,
		publisher: pub, clock: clk, cfg: cfg, assets: assets,
	}
}

func (s *Service) RunOnce(ctx context.Context, valueDate time.Time) error {
	valueDate = domain.UTCDay(valueDate)

	if err := s.ingestDVOL(ctx, valueDate.AddDate(0, 0, -7), valueDate); err != nil {
		return fmt.Errorf("ingest dvol: %w", err)
	}
	if err := s.ingestOptionsSnapshot(ctx, valueDate); err != nil {
		return fmt.Errorf("ingest options: %w", err)
	}
	if err := s.ingestCoinglassOptions(ctx, valueDate); err != nil {
		return fmt.Errorf("ingest coinglass options: %w", err)
	}
	if err := s.ingestDeribitOptionsChain(ctx, valueDate); err != nil {
		return fmt.Errorf("ingest deribit chain: %w", err)
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
	if err := s.ingestDVOL(ctx, from, to); err != nil {
		return fmt.Errorf("ingest dvol: %w", err)
	}

	if err := s.ingestOptionsSnapshot(ctx, to); err != nil {
		return fmt.Errorf("ingest options: %w", err)
	}
	if err := s.ingestCoinglassOptions(ctx, to); err != nil {
		return fmt.Errorf("ingest coinglass options: %w", err)
	}
	cutoff := s.clock.Now().Add(24 * time.Hour)
	return s.computeRange(ctx, from, to, cutoff)
}

func (s *Service) ingestDVOL(ctx context.Context, from, to time.Time) error {
	if s.dvol == nil {
		return nil
	}
	for _, asset := range s.assets {

		pts, err := s.dvol.FetchDVOL(ctx, asset, from, to.Add(24*time.Hour))
		if err != nil {

			slog.Warn("dvol fetch failed (non-fatal)", "asset", asset, "err", err)
			continue
		}
		if len(pts) == 0 {
			continue
		}
		rows := make([]DVOLRow, 0, len(pts))
		for _, p := range pts {
			rows = append(rows, DVOLRow{
				ValueDate:     p.Date,
				Asset:         asset,
				DVOLClose:     p.Close,
				SourceVersion: "deribit_dvol_v1",
				PayloadHash:   p.PayloadHash,
			})
		}
		if err := s.raws.SaveDVOL(ctx, rows); err != nil {
			return fmt.Errorf("save dvol %s: %w", asset, err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.volatility.dvol.ingested.v1",
			AggregateID: fmt.Sprintf("%s:dvol:%s", asset, pts[len(pts)-1].Date.Format("2006-01-02")),
			Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ingestOptionsSnapshot(ctx context.Context, valueDate time.Time) error {
	if s.options == nil {
		return nil
	}
	for _, asset := range s.assets {
		snap, err := s.options.FetchOptionsSnapshot(ctx, asset)
		if err != nil {

			slog.Warn("options snapshot fetch failed (non-fatal)", "asset", asset, "err", err)
			continue
		}
		if !snap.HasTermSlope && !snap.HasSkew {
			continue
		}
		intKey := func(name string) domain.FeatureKey {
			return domain.FeatureKey{Name: name, Version: s.cfg.IntermediateVersion}
		}
		if snap.HasTermSlope {
			s.saveFeature(ctx, intKey(features.IVTermSlopeDailyName), asset, valueDate, snap.TermSlope, "deribit_options")
		}
		if snap.HasSkew {
			s.saveFeature(ctx, intKey(features.IVSkewDailyName), asset, valueDate, snap.Skew, "deribit_options")
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.volatility.options.ingested.v1",
			AggregateID: fmt.Sprintf("%s:options:%s", asset, valueDate.Format("2006-01-02")),
			Payload: map[string]any{
				"asset":       asset.String(),
				"value_date":  valueDate.Format("2006-01-02"),
				"num_options": snap.NumOptions,
				"term_slope":  snap.HasTermSlope,
				"skew":        snap.HasSkew,
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ingestCoinglassOptions(ctx context.Context, valueDate time.Time) error {
	if s.cgOptions == nil {
		return nil
	}
	for _, asset := range s.assets {
		sym := string(asset)

		infoPts, err := s.cgOptions.FetchOptionsInfo(ctx, sym)
		if err != nil {
			slog.Warn("coinglass options info fetch failed (non-fatal)", "asset", asset, "err", err)
		} else if len(infoPts) > 0 {
			rows := make([]CoinglassOptionsInfoRow, 0, len(infoPts))
			for _, p := range infoPts {
				rows = append(rows, CoinglassOptionsInfoRow{
					ValueDate:        valueDate,
					Symbol:           sym,
					Exchange:         p.Exchange,
					OpenInterest:     p.OpenInterest,
					OIMarketShare:    p.OIMarketShare,
					OIChange24h:      p.OIChange24h,
					OpenInterestUSD:  p.OpenInterestUSD,
					VolumeUSD24h:     p.VolumeUSD24h,
					VolumeChangePct:  p.VolumeChangePct,
					CallOpenInterest: p.CallOpenInterest,
					PutOpenInterest:  p.PutOpenInterest,
					SourceVersion:    "coinglass_v4",
					PayloadHash:      p.PayloadHash,
				})
			}
			if err := s.raws.SaveCoinglassOptionsInfo(ctx, rows); err != nil {
				return fmt.Errorf("save options info %s: %w", asset, err)
			}
		}

		oiPts, err := s.cgOptions.FetchOptionsOIHistory(ctx, sym)
		if err != nil {
			slog.Warn("coinglass options OI history fetch failed (non-fatal)", "asset", asset, "err", err)
		} else if len(oiPts) > 0 {
			rows := make([]CoinglassOptionsOIHistoryRow, 0, len(oiPts))
			for _, p := range oiPts {
				rows = append(rows, CoinglassOptionsOIHistoryRow{
					ValueDate:     p.Date,
					Symbol:        sym,
					Exchange:      p.Exchange,
					OpenInterest:  p.OpenInterest,
					SourceVersion: "coinglass_v4",
					PayloadHash:   p.PayloadHash,
				})
			}
			if err := s.raws.SaveCoinglassOptionsOIHistory(ctx, rows); err != nil {
				return fmt.Errorf("save options OI history %s: %w", asset, err)
			}
		}

		mpPts, err := s.cgOptions.FetchOptionsMaxPain(ctx, sym, "Deribit")
		if err != nil {
			slog.Warn("coinglass max pain fetch failed (non-fatal)", "asset", asset, "err", err)
		} else if len(mpPts) > 0 {
			rows := make([]CoinglassOptionsMaxPainRow, 0, len(mpPts))
			for _, p := range mpPts {
				rows = append(rows, CoinglassOptionsMaxPainRow{
					ValueDate:         valueDate,
					ExpiryDate:        p.Date,
					Symbol:            sym,
					Exchange:          p.Exchange,
					MaxPainPrice:      p.MaxPainPrice,
					CallOIContracts:   p.CallOIContracts,
					PutOIContracts:    p.PutOIContracts,
					CallOINotionalUSD: p.CallOINotionalUSD,
					PutOINotionalUSD:  p.PutOINotionalUSD,
					CallMarketValue:   p.CallMarketValue,
					PutMarketValue:    p.PutMarketValue,
					SourceVersion:     "coinglass_v4",
					PayloadHash:       p.PayloadHash,
				})
			}
			if err := s.raws.SaveCoinglassOptionsMaxPain(ctx, rows); err != nil {
				return fmt.Errorf("save max pain %s: %w", asset, err)
			}
		}
	}
	return nil
}

func (s *Service) ingestDeribitOptionsChain(ctx context.Context, valueDate time.Time) error {
	if s.chain == nil {
		return nil
	}
	for _, asset := range s.assets {
		snaps, err := s.chain.FetchOptionsChain(ctx, asset)
		if err != nil {
			slog.Warn("deribit options chain fetch failed (non-fatal)", "asset", asset, "err", err)
			continue
		}
		if len(snaps) == 0 {
			continue
		}
		rows := make([]DeribitOptionsChainRow, 0, len(snaps))
		for _, p := range snaps {
			rows = append(rows, DeribitOptionsChainRow{
				ValueDate:          valueDate,
				Asset:              asset,
				InstrumentName:     p.InstrumentName,
				ExpiryDate:         p.ExpiryDate,
				StrikePrice:        p.StrikePrice,
				IsPut:              p.IsPut,
				OpenInterest:       p.OpenInterest,
				MarkIVPct:          p.MarkIVPct,
				UnderlyingPriceUSD: p.UnderlyingPriceUSD,
				SourceVersion:      "deribit_v1",
				PayloadHash:        p.PayloadHash,
			})
		}
		if err := s.raws.SaveDeribitOptionsChain(ctx, rows); err != nil {
			return fmt.Errorf("save options chain %s: %w", asset, err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.volatility.deribit_options_chain.ingested.v1",
			AggregateID: fmt.Sprintf("%s:options_chain:%s", asset, valueDate.Format("2006-01-02")),
			Payload:     map[string]any{"rows": len(rows), "asset": asset.String()},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) computeGEXNetDealer(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (float64, bool) {
	chain, err := s.raws.GetDeribitOptionsChainAsOf(ctx, asset, valueDate, cutoff)
	if err != nil || len(chain) == 0 {
		return 0, false
	}
	var (
		totalGEX float64
		anyValid bool
		spot     float64
		now      = valueDate
	)
	for _, c := range chain {
		if c.MarkIVPct <= 0 || c.UnderlyingPriceUSD <= 0 {
			continue
		}
		if spot == 0 {
			spot = c.UnderlyingPriceUSD
		}
		dte := int(c.ExpiryDate.Sub(now).Hours() / 24)
		if dte <= 0 {
			continue
		}
		gamma := mmath.BSGamma(c.UnderlyingPriceUSD, c.StrikePrice, c.MarkIVPct, dte, 0)
		if gamma <= 0 {
			continue
		}

		oiSigned := c.OpenInterest
		if c.IsPut {
			oiSigned = -c.OpenInterest
		}
		notional := oiSigned * gamma * c.UnderlyingPriceUSD * c.UnderlyingPriceUSD
		totalGEX += notional
		anyValid = true
	}
	if !anyValid {
		return 0, false
	}

	return totalGEX / 1_000_000, true
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
	intV := s.cfg.IntermediateVersion
	finalV := s.cfg.FinalVersion

	dvolKey := domain.FeatureKey{Name: features.DVOLDailyName, Version: intV}
	rvKey := domain.FeatureKey{Name: features.RealizedVol30dName, Version: intV}
	spreadKey := domain.FeatureKey{Name: features.IVRVSpreadDailyName, Version: intV}
	vovKey := domain.FeatureKey{Name: features.DVOLOfDVOL30dName, Version: intV}

	spreadZKey := domain.FeatureKey{Name: features.IVRVSpreadZScore90dName, Version: finalV}
	skewProxyZKey := domain.FeatureKey{Name: features.IVSkewProxyZScore90dName, Version: finalV}
	vovZKey := domain.FeatureKey{Name: features.DVOLOfDVOLZScore180dName, Version: finalV}
	termSlopeZKey := domain.FeatureKey{Name: features.IVTermSlopeZScore90dName, Version: finalV}
	skewRealZKey := domain.FeatureKey{Name: features.IVSkewRealZScore90dName, Version: finalV}

	if v, err := s.raws.GetDVOLCloseAsOf(ctx, asset, valueDate, cutoff); err == nil {
		s.saveFeature(ctx, dvolKey, asset, valueDate, v, "deribit_dvol")
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	if rv, ok, err := s.raws.RealizedVol30d(ctx, asset, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveFeature(ctx, rvKey, asset, valueDate, rv, "binance_klines")
	}

	if dvol, err := s.features.GetLatest(ctx, dvolKey, asset, valueDate, cutoff); err == nil {
		if rv, err := s.features.GetLatest(ctx, rvKey, asset, valueDate, cutoff); err == nil {
			if spread, ok := features.Diff(dvol, rv); ok {
				s.saveFeature(ctx, spreadKey, asset, valueDate, spread, "deribit_dvol")
			}
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	if sd, ok := s.tryComputeStdDev(ctx, dvolKey, asset, valueDate, cutoff,
		features.VovWindowDays, features.VovMinObs); ok {
		s.saveFeature(ctx, vovKey, asset, valueDate, sd, "deribit_dvol")
	}

	if z, ok := s.tryComputeZScore(ctx, spreadKey, asset, valueDate, cutoff,
		features.ZScore90dWindowDays, features.ZScore90dMinObs); ok {
		s.saveFeature(ctx, spreadZKey, asset, valueDate, z, "deribit_dvol")
	}

	if z, ok := s.tryComputeZScore(ctx, dvolKey, asset, valueDate, cutoff,
		features.ZScore90dWindowDays, features.ZScore90dMinObs); ok {
		s.saveFeature(ctx, skewProxyZKey, asset, valueDate, z, "deribit_dvol")
	}

	if z, ok := s.tryComputeZScore(ctx, vovKey, asset, valueDate, cutoff,
		features.ZScore180dWindowDays, features.ZScore180dMinObs); ok {
		s.saveFeature(ctx, vovZKey, asset, valueDate, z, "deribit_dvol")
	}

	termSlopeIntKey := domain.FeatureKey{Name: features.IVTermSlopeDailyName, Version: intV}
	if z, ok := s.tryComputeZScore(ctx, termSlopeIntKey, asset, valueDate, cutoff,
		features.ZScore90dWindowDays, features.ZScore90dMinObs); ok {
		s.saveFeature(ctx, termSlopeZKey, asset, valueDate, z, "deribit_options")
	}

	skewIntKey := domain.FeatureKey{Name: features.IVSkewDailyName, Version: intV}
	if z, ok := s.tryComputeZScore(ctx, skewIntKey, asset, valueDate, cutoff,
		features.ZScore90dWindowDays, features.ZScore90dMinObs); ok {
		s.saveFeature(ctx, skewRealZKey, asset, valueDate, z, "deribit_options")
	}

	sym := string(asset)
	cgOIDailyKey := domain.FeatureKey{Name: features.CGOptionsAggOIDailyName, Version: intV}
	cgOIVelKey := domain.FeatureKey{Name: features.CGOptionsOIVelocity30dName, Version: intV}
	cgOIVelZKey := domain.FeatureKey{Name: features.CGOptionsOIVelocityZScoreName, Version: finalV}
	cgPCRKey := domain.FeatureKey{Name: features.CGPutCallRatioDailyName, Version: intV}
	cgPCRZKey := domain.FeatureKey{Name: features.CGPutCallRatioZScoreName, Version: finalV}
	cgMPKey := domain.FeatureKey{Name: features.CGMaxPainDistancePctName, Version: intV}
	cgMPZKey := domain.FeatureKey{Name: features.CGMaxPainDistanceZScoreName, Version: finalV}

	if oi, ok, err := s.raws.GetCoinglassOptionsAggregatedOIAsOf(ctx, sym, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveFeature(ctx, cgOIDailyKey, asset, valueDate, oi, "coinglass_options_oi")
	}

	if nowOI, err := s.features.GetLatest(ctx, cgOIDailyKey, asset, valueDate, cutoff); err == nil {
		past := valueDate.AddDate(0, 0, -features.CGOptionsVelocityWindowDays)
		if pastOI, err2 := s.features.GetLatest(ctx, cgOIDailyKey, asset, past, cutoff); err2 == nil {
			if v, ok := features.CGOptionsOIVelocity30d(nowOI, pastOI); ok {
				s.saveFeature(ctx, cgOIVelKey, asset, valueDate, v, "coinglass_options_oi")
			}
		}
	}

	if z, ok := s.tryComputeZScore(ctx, cgOIVelKey, asset, valueDate, cutoff,
		features.CGOptionsZScoreWindowDays, features.CGOptionsZScoreMinObs); ok {
		s.saveFeature(ctx, cgOIVelZKey, asset, valueDate, z, "coinglass_options_oi")
	}

	if pcr, ok, err := s.raws.GetCoinglassOptionsPutCallRatioAsOf(ctx, sym, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveFeature(ctx, cgPCRKey, asset, valueDate, pcr, "coinglass_options_info")
	}
	if z, ok := s.tryComputeZScore(ctx, cgPCRKey, asset, valueDate, cutoff,
		features.CGOptionsZScoreWindowDays, features.CGOptionsZScoreMinObs); ok {
		s.saveFeature(ctx, cgPCRZKey, asset, valueDate, z, "coinglass_options_info")
	}

	binSym := features.AssetToBinanceSymbol(sym)
	if maxPain, ok, err := s.raws.GetCoinglassOptionsMaxPainNearestAsOf(ctx, sym, "Deribit", valueDate, cutoff); err != nil {
		return err
	} else if ok && binSym != "" {
		if spot, sok, err := s.raws.SpotCloseAsOf(ctx, binSym, valueDate, cutoff); err == nil && sok {
			if v, ok2 := features.CGMaxPainDistancePct(spot, maxPain); ok2 {
				s.saveFeature(ctx, cgMPKey, asset, valueDate, v, "coinglass_options_max_pain")
			}
		}
	}
	if z, ok := s.tryComputeZScore(ctx, cgMPKey, asset, valueDate, cutoff,
		features.CGOptionsZScoreWindowDays, features.CGOptionsZScoreMinObs); ok {
		s.saveFeature(ctx, cgMPZKey, asset, valueDate, z, "coinglass_options_max_pain")
	}

	gexKey := domain.FeatureKey{Name: features.GEXNetDealerDailyName, Version: intV}
	gexZKey := domain.FeatureKey{Name: features.GEXNetDealerZScore90dName, Version: finalV}

	if v, ok := s.computeGEXNetDealer(ctx, asset, valueDate, cutoff); ok {
		s.saveFeature(ctx, gexKey, asset, valueDate, v, "deribit_options_chain")
	}
	if z, ok := s.tryComputeZScore(ctx, gexKey, asset, valueDate, cutoff,
		features.CGOptionsZScoreWindowDays, features.CGOptionsZScoreMinObs); ok {
		s.saveFeature(ctx, gexZKey, asset, valueDate, z, "deribit_options_chain")
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
		Topic:       "scores.volatility.completed.v1",
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

func (s *Service) saveFeature(ctx context.Context, k domain.FeatureKey, asset domain.Asset, valueDate time.Time, v float64, srcName string) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	srcVer := sourceVersion(srcName)
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

func sourceVersion(srcName string) string {
	switch srcName {
	case "deribit_dvol":
		return "deribit_dvol_v1"
	case "deribit_options":
		return "deribit_options_v1"
	case "binance_klines":
		return "binance_spot_v1"
	}
	return ""
}

func (s *Service) tryComputeStdDev(ctx context.Context, sourceKey domain.FeatureKey, asset domain.Asset, valueDate, cutoff time.Time, window, minObs int) (float64, bool) {
	from := valueDate.AddDate(0, 0, -(window - 1))
	series, err := s.features.GetSeries(ctx, sourceKey, asset, from, valueDate, cutoff)
	if err != nil {
		return 0, false
	}
	return features.StdDev(series, minObs)
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
	finalV := s.cfg.FinalVersion

	spreadKey := domain.FeatureKey{Name: features.IVRVSpreadZScore90dName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, spreadKey, asset, valueDate, cutoff); err == nil {
		in.ZSpread, in.ZSpreadAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	skewRealKey := domain.FeatureKey{Name: features.IVSkewRealZScore90dName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, skewRealKey, asset, valueDate, cutoff); err == nil {
		in.ZSkewReal, in.ZSkewRealAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	skewProxyKey := domain.FeatureKey{Name: features.IVSkewProxyZScore90dName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, skewProxyKey, asset, valueDate, cutoff); err == nil {
		in.ZSkewProxy, in.ZSkewProxyAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	vovKey := domain.FeatureKey{Name: features.DVOLOfDVOLZScore180dName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, vovKey, asset, valueDate, cutoff); err == nil {
		in.ZVoV, in.ZVoVAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	termSlopeKey := domain.FeatureKey{Name: features.IVTermSlopeZScore90dName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, termSlopeKey, asset, valueDate, cutoff); err == nil {
		in.ZTermSlope, in.ZTermSlopeAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	in.FeatureCodesUsed = []string{
		spreadKey.String(), vovKey.String(),
	}
	if in.ZSkewRealAvailable {
		in.FeatureCodesUsed = append(in.FeatureCodesUsed, skewRealKey.String())
	} else if in.ZSkewProxyAvailable {
		in.FeatureCodesUsed = append(in.FeatureCodesUsed, skewProxyKey.String())
	}
	if in.ZTermSlopeAvailable {
		in.FeatureCodesUsed = append(in.FeatureCodesUsed, termSlopeKey.String())
	}

	cgOIVelZKey := domain.FeatureKey{Name: features.CGOptionsOIVelocityZScoreName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, cgOIVelZKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZCGOptionsOIVelocity, in.ZCGOptionsOIVelocityAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	cgPCRZKey := domain.FeatureKey{Name: features.CGPutCallRatioZScoreName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, cgPCRZKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZCGPutCallRatio, in.ZCGPutCallRatioAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	cgMPZKey := domain.FeatureKey{Name: features.CGMaxPainDistanceZScoreName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, cgMPZKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZCGMaxPainDistance, in.ZCGMaxPainDistanceAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	if in.ZCGOptionsOIVelocityAvailable {
		in.FeatureCodesUsed = append(in.FeatureCodesUsed, cgOIVelZKey.String())
	}
	if in.ZCGPutCallRatioAvailable {
		in.FeatureCodesUsed = append(in.FeatureCodesUsed, cgPCRZKey.String())
	}
	if in.ZCGMaxPainDistanceAvailable {
		in.FeatureCodesUsed = append(in.FeatureCodesUsed, cgMPZKey.String())
	}

	gexZKey := domain.FeatureKey{Name: features.GEXNetDealerZScore90dName, Version: finalV}
	if v, err := s.features.GetLatest(ctx, gexZKey, asset, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZGEXNetDealer, in.ZGEXNetDealerAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	if in.ZGEXNetDealerAvailable {
		in.FeatureCodesUsed = append(in.FeatureCodesUsed, gexZKey.String())
	}
	return in, nil
}
