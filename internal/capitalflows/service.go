package capitalflows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"marketengine/internal/capitalflows/features"
	"marketengine/internal/domain"
)

type Service struct {
	features  FeatureRepo
	scores    ScoreRepo
	raws      RawRepo
	etf       ETFFlowProvider
	lth       LTHSupplyProvider
	mcap      BTCMarketCapProvider
	liquidity LiquidityFlowProvider
	inst      InstitutionalProvider
	publisher Publisher
	clock     Clock
	cfg       Config
	assets    []domain.Asset
}

func NewService(
	featRepo FeatureRepo,
	scoreRepo ScoreRepo,
	rawRepo RawRepo,
	etf ETFFlowProvider,
	lth LTHSupplyProvider,
	mcap BTCMarketCapProvider,
	liquidity LiquidityFlowProvider,
	inst InstitutionalProvider,
	pub Publisher,
	clk Clock,
	cfg Config,
	assets []domain.Asset,
) *Service {
	cfg.Defaults()
	if len(assets) == 0 {

		assets = []domain.Asset{domain.AssetGlobal}
	}
	return &Service{
		features: featRepo, scores: scoreRepo, raws: rawRepo,
		etf: etf, lth: lth, mcap: mcap, liquidity: liquidity, inst: inst,
		publisher: pub, clock: clk, cfg: cfg, assets: assets,
	}
}

func (s *Service) RunOnce(ctx context.Context, valueDate time.Time) error {
	valueDate = domain.UTCDay(valueDate)
	if err := s.ingestAll(ctx); err != nil {
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
	if err := s.ingestAll(ctx); err != nil {
		return err
	}
	cutoff := s.clock.Now().Add(24 * time.Hour)
	return s.computeRange(ctx, from, to, cutoff)
}

func (s *Service) ingestAll(ctx context.Context) error {
	if err := s.ingestETFFlows(ctx); err != nil {
		return fmt.Errorf("ingest etf flows: %w", err)
	}
	if err := s.ingestLTHSupply(ctx); err != nil {
		return fmt.Errorf("ingest lth supply: %w", err)
	}
	if err := s.ingestBTCMarketCap(ctx); err != nil {
		return fmt.Errorf("ingest btc market cap: %w", err)
	}
	if err := s.ingestLiquidityFlow(ctx); err != nil {
		return fmt.Errorf("ingest liquidity flow: %w", err)
	}
	if err := s.ingestInstitutional(ctx); err != nil {
		return fmt.Errorf("ingest institutional: %w", err)
	}
	return nil
}

func (s *Service) ingestInstitutional(ctx context.Context) error {
	if s.inst == nil {
		return nil
	}
	today := domain.UTCDay(s.clock.Now())

	listItems, err := s.inst.FetchETFList(ctx)
	if err != nil {
		slog.Warn("etf list fetch failed (non-fatal)", "err", err)
	} else if len(listItems) > 0 {
		rows := make([]ETFListItemRow, 0, len(listItems))
		topTickers := make([]string, 0, 5)
		for i, p := range listItems {
			rows = append(rows, ETFListItemRow{
				ValueDate:          today,
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
				SourceVersion:      "coinglass_v4",
				PayloadHash:        p.PayloadHash,
			})

			if i < 5 {
				topTickers = append(topTickers, p.Ticker)
			}
		}
		if err := s.raws.SaveETFListSnapshot(ctx, rows); err != nil {
			return fmt.Errorf("save etf list: %w", err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.capital_flows.etf_list.ingested.v1",
			AggregateID: "GLOBAL:etf_list",
			Payload:     map[string]any{"rows": len(rows)},
		}); err != nil {
			return err
		}

		for _, ticker := range topTickers {
			pts, err := s.inst.FetchETFAUMHistory(ctx, ticker)
			if err != nil {
				slog.Warn("etf aum history fetch failed (non-fatal)", "ticker", ticker, "err", err)
				continue
			}
			if len(pts) == 0 {
				continue
			}
			aumRows := make([]ETFAUMHistoryRow, 0, len(pts))
			for _, p := range pts {
				if p.AUMUSD <= 0 {
					continue
				}
				aumRows = append(aumRows, ETFAUMHistoryRow{
					ValueDate:     p.Date,
					Ticker:        ticker,
					AUMUSD:        p.AUMUSD,
					SourceVersion: "coinglass_v4",
					PayloadHash:   p.PayloadHash,
				})
			}
			if err := s.raws.SaveETFAUMHistory(ctx, aumRows); err != nil {
				return fmt.Errorf("save etf aum history %s: %w", ticker, err)
			}
		}
	}

	for _, sym := range []string{"BTC", "ETH"} {
		mpPts, err := s.inst.FetchOptionsMaxPain(ctx, sym, "Deribit")
		if err != nil {
			slog.Warn("max pain fetch failed (non-fatal)", "symbol", sym, "err", err)
			continue
		}
		if len(mpPts) == 0 {
			continue
		}
		mpRows := make([]OptionsMaxPainRow, 0, len(mpPts))
		for _, p := range mpPts {
			mpRows = append(mpRows, OptionsMaxPainRow{
				ValueDate:          today,
				ExpiryDate:         p.ExpiryDate,
				Symbol:             p.Symbol,
				Exchange:           p.Exchange,
				MaxPainPrice:       p.MaxPainPrice,
				CallOIContracts:    p.CallOIContracts,
				PutOIContracts:     p.PutOIContracts,
				CallOINotionalUSD:  p.CallOINotionalUSD,
				PutOINotionalUSD:   p.PutOINotionalUSD,
				CallMarketValueUSD: p.CallMarketValueUSD,
				PutMarketValueUSD:  p.PutMarketValueUSD,
				SourceVersion:      "coinglass_v4",
				PayloadHash:        p.PayloadHash,
			})
		}
		if err := s.raws.SaveOptionsMaxPainNearest(ctx, mpRows); err != nil {
			return fmt.Errorf("save max pain %s: %w", sym, err)
		}
	}
	return nil
}

func (s *Service) ingestLiquidityFlow(ctx context.Context) error {
	if s.liquidity == nil {
		return nil
	}

	mcaps, err := s.liquidity.FetchStablecoinMcapHistory(ctx)
	if err != nil {
		slog.Warn("stablecoin mcap fetch failed (non-fatal)", "err", err)
	} else if len(mcaps) > 0 {
		rows := make([]StablecoinMcapRow, 0, len(mcaps))
		for _, p := range mcaps {
			rows = append(rows, StablecoinMcapRow{
				ValueDate:     p.Date,
				MarketCap:     p.MarketCap,
				PriceUSD:      p.PriceUSD,
				SourceVersion: "coinglass_v4",
				PayloadHash:   p.PayloadHash,
			})
		}
		if err := s.raws.SaveStablecoinMcap(ctx, rows); err != nil {
			return fmt.Errorf("save stablecoin mcap: %w", err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.capital_flows.stablecoin_mcap.ingested.v1",
			AggregateID: "GLOBAL:stablecoin_mcap",
			Payload:     map[string]any{"rows": len(rows)},
		}); err != nil {
			return err
		}
	}

	today := domain.UTCDay(s.clock.Now())

	for _, sym := range []string{"BTC", "ETH", "USDT(ETH)", "USDC"} {
		snaps, err := s.liquidity.FetchExchangeBalanceList(ctx, sym)
		if err != nil {
			slog.Warn("exchange balance fetch failed (non-fatal)", "symbol", sym, "err", err)
			continue
		}
		if len(snaps) == 0 {
			continue
		}
		rows := make([]ExchangeBalanceRow, 0, len(snaps))
		for _, p := range snaps {
			rows = append(rows, ExchangeBalanceRow{
				ValueDate:           today,
				Symbol:              sym,
				Exchange:            p.Exchange,
				TotalBalance:        p.TotalBalance,
				BalanceChange1d:     p.BalanceChange1d,
				BalanceChange7d:     p.BalanceChange7d,
				BalanceChange30d:    p.BalanceChange30d,
				BalanceChangePct1d:  p.BalanceChangePct1d,
				BalanceChangePct7d:  p.BalanceChangePct7d,
				BalanceChangePct30d: p.BalanceChangePct30d,
				SourceVersion:       "coinglass_v4",
				PayloadHash:         p.PayloadHash,
			})
		}
		if err := s.raws.SaveExchangeBalance(ctx, rows); err != nil {
			return fmt.Errorf("save exchange balance %s: %w", sym, err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.capital_flows.exchange_balance.ingested.v1",
			AggregateID: fmt.Sprintf("%s:exchange_balance", sym),
			Payload:     map[string]any{"rows": len(rows), "symbol": sym},
		}); err != nil {
			return err
		}
	}

	for _, sym := range []string{"BTC", "ETH"} {
		pts, err := s.liquidity.FetchBitfinexMargin(ctx, sym, 1000)
		if err != nil {
			slog.Warn("bitfinex margin fetch failed (non-fatal)", "symbol", sym, "err", err)
			continue
		}
		if len(pts) == 0 {
			continue
		}
		rows := make([]BitfinexMarginRow, 0, len(pts))
		for _, p := range pts {
			rows = append(rows, BitfinexMarginRow{
				ValueDate:     p.Time,
				Symbol:        sym,
				LongQty:       p.LongQty,
				ShortQty:      p.ShortQty,
				SourceVersion: "coinglass_v4",
				PayloadHash:   p.PayloadHash,
			})
		}
		if err := s.raws.SaveBitfinexMargin(ctx, rows); err != nil {
			return fmt.Errorf("save bitfinex margin %s: %w", sym, err)
		}
		if err := s.publisher.Publish(ctx, domain.Event{
			Topic:       "raw.capital_flows.bitfinex_margin.ingested.v1",
			AggregateID: fmt.Sprintf("%s:bitfinex_margin", sym),
			Payload:     map[string]any{"rows": len(rows), "symbol": sym},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ingestETFFlows(ctx context.Context) error {
	if s.etf == nil {
		return nil
	}
	btcPts, err := s.etf.FetchBTCETFFlows(ctx)
	if err != nil {
		return fmt.Errorf("fetch BTC ETF flows: %w", err)
	}
	if err := s.writeETFFlows(ctx, "BTC", btcPts); err != nil {
		return err
	}
	ethPts, err := s.etf.FetchETHETFFlows(ctx)
	if err != nil {
		return fmt.Errorf("fetch ETH ETF flows: %w", err)
	}
	return s.writeETFFlows(ctx, "ETH", ethPts)
}

func (s *Service) writeETFFlows(ctx context.Context, flowType string, pts []ETFFlowPoint) error {
	if len(pts) == 0 {
		return nil
	}
	rows := make([]ETFFlowRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, ETFFlowRow{
			ValueDate:     p.Date,
			FlowType:      flowType,
			TotalFlowUSD:  p.TotalFlowUSD,
			PriceUSD:      p.PriceUSD,
			SourceVersion: "coinglass_v4",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveETFFlows(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       fmt.Sprintf("raw.capital_flows.etf_%s.ingested.v1", flowType),
		AggregateID: fmt.Sprintf("coinglass_etf:%s:%s", flowType, pts[len(pts)-1].Date.Format("2006-01-02")),
		Payload:     map[string]any{"rows_touched": len(rows), "flow_type": flowType},
	})
}

func (s *Service) ingestLTHSupply(ctx context.Context) error {
	if s.lth == nil {
		return nil
	}
	pts, err := s.lth.FetchLTHSupply(ctx)
	if err != nil {
		return fmt.Errorf("fetch LTH supply: %w", err)
	}
	if len(pts) == 0 {
		return nil
	}
	rows := make([]LTHSupplyRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, LTHSupplyRow{
			ValueDate:     p.Date,
			Asset:         domain.AssetBTC,
			LTHSupply:     p.LTHSupply,
			SourceVersion: "coinglass_v4",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveLTHSupply(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "raw.capital_flows.lth_supply.ingested.v1",
		AggregateID: fmt.Sprintf("coinglass_lth:BTC:%s", pts[len(pts)-1].Date.Format("2006-01-02")),
		Payload:     map[string]any{"rows_touched": len(rows)},
	})
}

func (s *Service) ingestBTCMarketCap(ctx context.Context) error {
	if s.mcap == nil {
		return nil
	}
	pts, err := s.mcap.FetchBTCMarketCap(ctx)
	if err != nil {

		slog.Warn("btc market_cap fetch failed (non-fatal)", "err", err)
		return nil
	}
	if len(pts) == 0 {
		return nil
	}
	rows := make([]MarketCapRow, 0, len(pts))
	for _, p := range pts {
		rows = append(rows, MarketCapRow{
			ValueDate:     p.Date,
			CoinID:        "bitcoin",
			MarketCapUSD:  p.MarketCapUSD,
			PriceUSD:      p.PriceUSD,
			SourceVersion: "coinglass_v4",
			PayloadHash:   p.PayloadHash,
		})
	}
	if err := s.raws.SaveBTCMarketCap(ctx, rows); err != nil {
		return err
	}
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "raw.capital_flows.btc_market_cap.ingested.v1",
		AggregateID: fmt.Sprintf("coinglass_mcap:bitcoin:%s", pts[len(pts)-1].Date.Format("2006-01-02")),
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

	if err := s.computeFeatures(ctx, valueDate, cutoff); err != nil {
		return fmt.Errorf("features: %w", err)
	}

	for _, asset := range s.assets {
		if err := s.scoreAsset(ctx, asset, valueDate, cutoff); err != nil {
			return fmt.Errorf("asset %s: %w", asset, err)
		}
	}
	return nil
}

func (s *Service) computeFeatures(ctx context.Context, valueDate, cutoff time.Time) error {
	intermediateVer := s.cfg.IntermediateVersion
	finalVer := s.cfg.FinalVersion

	etfDailyKey := domain.FeatureKey{Name: features.ETFNetflowDailyName, Version: intermediateVer}
	etfZKey := domain.FeatureKey{Name: features.ETFNetflowZScore90dName, Version: finalVer}
	lthDailyKey := domain.FeatureKey{Name: features.LTHSupplyDailyName, Version: intermediateVer}
	lthChangeKey := domain.FeatureKey{Name: features.LTHSupplyChange30dName, Version: intermediateVer}
	lthZKey := domain.FeatureKey{Name: features.LTHSupplyChangeZScore180dName, Version: finalVer}

	if sum, ok, err := s.raws.CombinedETFFlowAsOf(ctx, valueDate, cutoff); err != nil {
		return err
	} else if ok {
		s.saveFeature(ctx, etfDailyKey, domain.AssetGlobal, valueDate, sum,
			"coinglass_etf_flows", "coinglass_v4")
	}

	if z, ok := s.tryComputeZScore(ctx, etfDailyKey, domain.AssetGlobal, valueDate, cutoff,
		features.ETFZScoreWindowDays, features.ETFZScoreMinObs); ok {
		s.saveFeature(ctx, etfZKey, domain.AssetGlobal, valueDate, z,
			"coinglass_etf_flows", "coinglass_v4")
	}

	if v, err := s.raws.GetLTHSupplyAsOf(ctx, domain.AssetBTC, valueDate, cutoff); err == nil {
		s.saveFeature(ctx, lthDailyKey, domain.AssetBTC, valueDate, v,
			"lth_supply", "coinglass_v4")
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	if change, ok := s.tryComputeLTHChange(ctx, lthDailyKey, valueDate, cutoff); ok {
		s.saveFeature(ctx, lthChangeKey, domain.AssetBTC, valueDate, change,
			"lth_supply", "coinglass_v4")
	}

	if z, ok := s.tryComputeZScore(ctx, lthChangeKey, domain.AssetBTC, valueDate, cutoff,
		features.LTHZScoreWindowDays, features.LTHZScoreMinObs); ok {
		s.saveFeature(ctx, lthZKey, domain.AssetBTC, valueDate, z,
			"lth_supply", "coinglass_v4")
	}

	stableVelKey := domain.FeatureKey{Name: features.StablecoinMcapVelocity30dName, Version: intermediateVer}
	stableZKey := domain.FeatureKey{Name: features.StablecoinVelocityZScore90dName, Version: finalVer}
	exBalKey := domain.FeatureKey{Name: features.ExchangeBalanceChange7dName, Version: intermediateVer}
	exBalZKey := domain.FeatureKey{Name: features.ExchangeBalanceChangeZScore90dName, Version: finalVer}
	bfxSkewKey := domain.FeatureKey{Name: features.BitfinexMarginSkewDailyName, Version: intermediateVer}
	bfxSkewZKey := domain.FeatureKey{Name: features.BitfinexMarginSkewZScore90dName, Version: finalVer}

	if nowMcap, ok, err := s.raws.GetStablecoinMcapAsOf(ctx, valueDate, cutoff); err == nil && ok {
		past := valueDate.AddDate(0, 0, -features.StablecoinVelocityWindowDays)
		if pastMcap, ok2, err2 := s.raws.GetStablecoinMcapAsOf(ctx, past, cutoff); err2 == nil && ok2 {
			if v, ok3 := features.StablecoinMcapVelocity30d(nowMcap, pastMcap); ok3 {
				s.saveFeature(ctx, stableVelKey, domain.AssetGlobal, valueDate, v,
					"coinglass_stablecoin_mcap", "coinglass_v4")
			}
		}
	}
	if z, ok := s.tryComputeZScore(ctx, stableVelKey, domain.AssetGlobal, valueDate, cutoff,
		features.LiquidityFlowZWindowDays, features.LiquidityFlowZMinObs); ok {
		s.saveFeature(ctx, stableZKey, domain.AssetGlobal, valueDate, z,
			"coinglass_stablecoin_mcap", "coinglass_v4")
	}

	if v, ok, err := s.raws.GetExchangeBalanceChange7dSumAsOf(ctx, "BTC", valueDate, cutoff); err == nil && ok {
		s.saveFeature(ctx, exBalKey, domain.AssetGlobal, valueDate, v,
			"coinglass_exchange_balance", "coinglass_v4")
	}
	if z, ok := s.tryComputeZScore(ctx, exBalKey, domain.AssetGlobal, valueDate, cutoff,
		features.LiquidityFlowZWindowDays, features.LiquidityFlowZMinObs); ok {
		s.saveFeature(ctx, exBalZKey, domain.AssetGlobal, valueDate, z,
			"coinglass_exchange_balance", "coinglass_v4")
	}

	dpKey7d := domain.FeatureKey{Name: features.StablecoinDryPowderChange7dName, Version: intermediateVer}
	dpZKey7d := domain.FeatureKey{Name: features.StablecoinDryPowderChange7dZName, Version: finalVer}
	dpKey30d := domain.FeatureKey{Name: features.StablecoinDryPowderChange30dName, Version: intermediateVer}
	dpZKey30d := domain.FeatureKey{Name: features.StablecoinDryPowderChange30dZName, Version: finalVer}

	for _, p := range []struct {
		dailyKey, zKey domain.FeatureKey
		fn             func(context.Context, string, time.Time, time.Time) (float64, bool, error)
	}{
		{dpKey7d, dpZKey7d, s.raws.GetExchangeBalanceChange7dSumAsOf},
		{dpKey30d, dpZKey30d, s.raws.GetExchangeBalanceChange30dSumAsOf},
	} {
		var sum float64
		var any bool
		for _, sym := range []string{"USDT(ETH)", "USDC"} {
			if v, ok, err := p.fn(ctx, sym, valueDate, cutoff); err == nil && ok {
				sum += v
				any = true
			}
		}
		if any {
			s.saveFeature(ctx, p.dailyKey, domain.AssetGlobal, valueDate, sum,
				"coinglass_exchange_balance", "coinglass_v4")
		}
		if z, ok := s.tryComputeZScore(ctx, p.dailyKey, domain.AssetGlobal, valueDate, cutoff,
			features.LiquidityFlowZWindowDays, features.LiquidityFlowZMinObs); ok {
			s.saveFeature(ctx, p.zKey, domain.AssetGlobal, valueDate, z,
				"coinglass_exchange_balance", "coinglass_v4")
		}
	}

	if long, short, ok, err := s.raws.GetBitfinexMarginAsOf(ctx, "BTC", valueDate, cutoff); err == nil && ok {
		if v, ok2 := features.BitfinexMarginSkew(long, short); ok2 {
			s.saveFeature(ctx, bfxSkewKey, domain.AssetGlobal, valueDate, v,
				"coinglass_bitfinex_margin", "coinglass_v4")
		}
	}
	if z, ok := s.tryComputeZScore(ctx, bfxSkewKey, domain.AssetGlobal, valueDate, cutoff,
		features.LiquidityFlowZWindowDays, features.LiquidityFlowZMinObs); ok {
		s.saveFeature(ctx, bfxSkewZKey, domain.AssetGlobal, valueDate, z,
			"coinglass_bitfinex_margin", "coinglass_v4")
	}

	etfTotalKey := domain.FeatureKey{Name: features.ETFAUMTotalDailyName, Version: intermediateVer}
	etfVelKey := domain.FeatureKey{Name: features.ETFAUMVelocity30dName, Version: intermediateVer}
	etfVelZKey := domain.FeatureKey{Name: features.ETFAUMVelocityZScore90dName, Version: finalVer}
	hhiKey := domain.FeatureKey{Name: features.ETFConcentrationHHIDailyName, Version: intermediateVer}
	hhiZKey := domain.FeatureKey{Name: features.ETFConcentrationHHIZScore90dName, Version: finalVer}
	dealerKey := domain.FeatureKey{Name: features.OptionsDealerSkewProxyDailyName, Version: intermediateVer}
	dealerZKey := domain.FeatureKey{Name: features.OptionsDealerSkewProxyZScoreName, Version: finalVer}

	if v, ok, err := s.raws.GetETFListAUMTotalAsOf(ctx, valueDate, cutoff); err == nil && ok {
		s.saveFeature(ctx, etfTotalKey, domain.AssetGlobal, valueDate, v,
			"coinglass_etf_list", "coinglass_v4")
	} else if v, ok, err := s.raws.GetETFAUMHistoryTotalAsOf(ctx, valueDate, cutoff); err == nil && ok {
		s.saveFeature(ctx, etfTotalKey, domain.AssetGlobal, valueDate, v,
			"coinglass_etf_aum_history", "coinglass_v4")
	}

	if nowAUM, err := s.features.GetLatest(ctx, etfTotalKey, domain.AssetGlobal, valueDate, cutoff); err == nil {
		past := valueDate.AddDate(0, 0, -30)
		if pastAUM, err2 := s.features.GetLatest(ctx, etfTotalKey, domain.AssetGlobal, past, cutoff); err2 == nil {
			if v, ok := features.ETFAUMVelocity30d(nowAUM, pastAUM); ok {
				s.saveFeature(ctx, etfVelKey, domain.AssetGlobal, valueDate, v,
					"coinglass_etf_aum_history", "coinglass_v4")
			}
		}
	}
	if z, ok := s.tryComputeZScore(ctx, etfVelKey, domain.AssetGlobal, valueDate, cutoff,
		features.LiquidityFlowZWindowDays, features.LiquidityFlowZMinObs); ok {
		s.saveFeature(ctx, etfVelZKey, domain.AssetGlobal, valueDate, z,
			"coinglass_etf_aum_history", "coinglass_v4")
	}

	if v, ok, err := s.raws.GetETFListConcentrationHHIAsOf(ctx, valueDate, cutoff); err == nil && ok {
		s.saveFeature(ctx, hhiKey, domain.AssetGlobal, valueDate, v,
			"coinglass_etf_list", "coinglass_v4")
	}
	if z, ok := s.tryComputeZScore(ctx, hhiKey, domain.AssetGlobal, valueDate, cutoff,
		features.LiquidityFlowZWindowDays, features.LiquidityFlowZMinObs); ok {
		s.saveFeature(ctx, hhiZKey, domain.AssetGlobal, valueDate, z,
			"coinglass_etf_list", "coinglass_v4")
	}

	if v, ok, err := s.raws.GetOptionsDealerSkewProxyAsOf(ctx, "BTC", "Deribit", valueDate, cutoff); err == nil && ok {
		s.saveFeature(ctx, dealerKey, domain.AssetGlobal, valueDate, v,
			"coinglass_options_max_pain", "coinglass_v4")
	}
	if z, ok := s.tryComputeZScore(ctx, dealerKey, domain.AssetGlobal, valueDate, cutoff,
		features.LiquidityFlowZWindowDays, features.LiquidityFlowZMinObs); ok {
		s.saveFeature(ctx, dealerZKey, domain.AssetGlobal, valueDate, z,
			"coinglass_options_max_pain", "coinglass_v4")
	}

	return nil
}

func (s *Service) scoreAsset(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) error {
	in, err := s.gatherScoreInputs(ctx, asset, valueDate, cutoff)
	if err != nil {
		return fmt.Errorf("gather score inputs: %w", err)
	}
	score := computeScore(in, s.cfg)
	if err := s.scores.Save(ctx, score); err != nil {
		return fmt.Errorf("save score: %w", err)
	}
	postETF := !valueDate.Before(ETFLaunchDate)
	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "scores.capital_flows.completed.v1",
		AggregateID: fmt.Sprintf("%s:%s", asset, valueDate.Format("2006-01-02")),
		Payload: map[string]any{
			"asset":          asset.String(),
			"value_date":     valueDate.Format("2006-01-02"),
			"score":          score.Score,
			"post_etf":       postETF,
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

func (s *Service) tryComputeLTHChange(ctx context.Context, sourceKey domain.FeatureKey, valueDate, cutoff time.Time) (float64, bool) {
	now, err := s.features.GetLatest(ctx, sourceKey, domain.AssetBTC, valueDate, cutoff)
	if err != nil {
		return 0, false
	}
	past := valueDate.AddDate(0, 0, -features.LTHChangeWindowDays)
	earlier, err := s.features.GetLatest(ctx, sourceKey, domain.AssetBTC, past, cutoff)
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

func (s *Service) gatherScoreInputs(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) (ScoreInputs, error) {
	in := ScoreInputs{Asset: asset, ValueDate: valueDate}
	finalVer := s.cfg.FinalVersion

	etfKey := domain.FeatureKey{Name: features.ETFNetflowZScore90dName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, etfKey, domain.AssetGlobal, valueDate, cutoff); err == nil {
		in.ZETF, in.ZETFAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	lthKey := domain.FeatureKey{Name: features.LTHSupplyChangeZScore180dName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, lthKey, domain.AssetBTC, valueDate, cutoff); err == nil {
		in.ZLTH, in.ZLTHAvailable = v, true
	} else if !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	minerKey := domain.FeatureKey{Name: features.MinerNetflowZScore180dName, Version: finalVer}

	stableZKey := domain.FeatureKey{Name: features.StablecoinVelocityZScore90dName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, stableZKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZStablecoinVelocity, in.ZStablecoinVelocityAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	exBalZKey := domain.FeatureKey{Name: features.ExchangeBalanceChangeZScore90dName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, exBalZKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZExchangeBalance, in.ZExchangeBalanceAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	bfxSkewZKey := domain.FeatureKey{Name: features.BitfinexMarginSkewZScore90dName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, bfxSkewZKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZBitfinexMargin, in.ZBitfinexMarginAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	dpZKey7d := domain.FeatureKey{Name: features.StablecoinDryPowderChange7dZName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, dpZKey7d, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZStablecoinDryPowder7d, in.ZStablecoinDryPowder7dAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	dpZKey30d := domain.FeatureKey{Name: features.StablecoinDryPowderChange30dZName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, dpZKey30d, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZStablecoinDryPowder30d, in.ZStablecoinDryPowder30dAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	etfVelZKey := domain.FeatureKey{Name: features.ETFAUMVelocityZScore90dName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, etfVelZKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZETFAUMVelocity, in.ZETFAUMVelocityAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	hhiZKey := domain.FeatureKey{Name: features.ETFConcentrationHHIZScore90dName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, hhiZKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZETFConcentrationHHI, in.ZETFConcentrationHHIAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}
	dealerZKey := domain.FeatureKey{Name: features.OptionsDealerSkewProxyZScoreName, Version: finalVer}
	if v, err := s.features.GetLatest(ctx, dealerZKey, domain.AssetGlobal, valueDate, cutoff); err == nil && !math.IsNaN(v) {
		in.ZOptionsDealerSkew, in.ZOptionsDealerSkewAvailable = v, true
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return in, err
	}

	in.FeatureCodesUsed = []string{
		etfKey.String(), lthKey.String(), minerKey.String(),
		stableZKey.String(), exBalZKey.String(), bfxSkewZKey.String(),
		dpZKey7d.String(), dpZKey30d.String(),
		etfVelZKey.String(), hhiZKey.String(), dealerZKey.String(),
	}
	return in, nil
}
