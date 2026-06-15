package regime

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"marketengine/internal/domain"
)

type Service struct {
	repo        RegimeRepo
	scoreReader DomainScoreReader
	publisher   Publisher
	clock       Clock
	cfg         Config
	assets      []domain.Asset
}

func NewService(
	repo RegimeRepo,
	scoreReader DomainScoreReader,
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
		repo:        repo,
		scoreReader: scoreReader,
		publisher:   pub,
		clock:       clk,
		cfg:         cfg,
		assets:      assets,
	}
}

func (s *Service) RunOnce(ctx context.Context, valueDate time.Time) error {
	valueDate = domain.UTCDay(valueDate)
	cutoff := s.clock.Now()
	for _, a := range s.assets {
		if err := s.computeOne(ctx, a, valueDate, cutoff); err != nil {
			return fmt.Errorf("regime %s %s: %w", a, valueDate.Format("2006-01-02"), err)
		}
	}
	return nil
}

func (s *Service) RunBackfill(ctx context.Context, from, to time.Time) error {
	from, to = domain.UTCDay(from), domain.UTCDay(to)
	if from.After(to) {
		return fmt.Errorf("backfill from (%s) after to (%s)",
			from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
	cutoff := s.clock.Now().Add(24 * time.Hour)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		for _, a := range s.assets {
			if err := s.computeOne(ctx, a, d, cutoff); err != nil {
				slog.Warn("regime backfill skip",
					"asset", a, "date", d.Format("2006-01-02"), "err", err)
				continue
			}
		}
		if d.Day() == 1 {
			slog.Info("regime backfill progress", "date", d.Format("2006-01-02"))
		}
	}
	return nil
}

func (s *Service) Compute(ctx context.Context, asset domain.Asset, valueDate time.Time) error {
	cutoff := s.clock.Now()
	return s.computeOne(ctx, asset, domain.UTCDay(valueDate), cutoff)
}

func (s *Service) GetLatest(ctx context.Context, asset domain.Asset) (domain.RegimeState, error) {
	return s.repo.GetLatest(ctx, asset)
}

func (s *Service) GetByDate(ctx context.Context, asset domain.Asset, valueDate time.Time) (domain.RegimeState, error) {
	return s.repo.GetByDate(ctx, asset, domain.UTCDay(valueDate))
}

func (s *Service) GetHistory(ctx context.Context, asset domain.Asset, from, to time.Time) ([]domain.RegimeState, error) {
	return s.repo.GetHistory(ctx, asset, domain.UTCDay(from), domain.UTCDay(to))
}

func (s *Service) computeOne(ctx context.Context, asset domain.Asset, valueDate, cutoff time.Time) error {

	scores, err := s.scoreReader.GetLatestAll(ctx, asset, valueDate, cutoff)
	if err != nil {
		return fmt.Errorf("read domain scores: %w", err)
	}

	if coverageFraction(scores) < s.cfg.MinCoverage {
		return fmt.Errorf("%w: %.0f%% (need %.0f%%)",
			domain.ErrInsufficientCoverage,
			coverageFraction(scores)*100, s.cfg.MinCoverage*100)
	}

	present := presentSet(scores)
	effectiveWeights := redistributeWeights(s.cfg.Weights, present)

	histories := make(map[domain.DomainCode][]float64, len(scores))
	for d := range scores {
		h, herr := s.scoreReader.GetHistory(ctx, asset, d, s.cfg.NormLookbackDays)
		if herr != nil {
			return fmt.Errorf("domain history %s: %w", d, herr)
		}
		histories[d] = h
	}
	normalized := normalizeScores(scores, histories, s.cfg.NormMinSamples)

	rawIndicator := weightedSumRaw(scores, effectiveWeights)
	contributions, normIndicator, _ := weightedSumNormalized(normalized, effectiveWeights)

	trInputs, terr := s.gatherTransitionRiskInputs(ctx, asset, valueDate, cutoff, scores, normalized, present)
	if terr != nil {
		return fmt.Errorf("transition risk inputs: %w", terr)
	}
	transRisk := computeTransitionRisk(trInputs, s.cfg)

	indicator := normIndicator
	if s.cfg.SmoothingSpanDays > 1 {
		from := valueDate.AddDate(0, 0, -2*s.cfg.SmoothingSpanDays)
		to := valueDate.AddDate(0, 0, -1)
		trail, terr := s.repo.GetSmoothingTrailing(ctx, asset, from, to, cutoff)
		if terr != nil {
			return fmt.Errorf("smoothing trailing: %w", terr)
		}
		prevRI := make([]float64, 0, len(trail))
		prevTR := make([]float64, 0, len(trail))
		for _, p := range trail {
			prevRI = append(prevRI, p.RegimeIndicator)
			prevTR = append(prevTR, p.TransitionRisk)
		}
		indicator = emaSmooth(normIndicator, prevRI, s.cfg.SmoothingSpanDays)
		transRisk = clip(emaSmooth(transRisk, prevTR, s.cfg.SmoothingSpanDays), 0, 1)
	}

	riskOn, riskOff := finalProbabilities(indicator, transRisk, s.cfg.SigmoidK)

	topDrivers := buildTopDrivers(contributions)

	coverageFlag := make(map[domain.DomainCode]bool, len(allDomains))
	for _, d := range allDomains {
		coverageFlag[d] = present[d]
	}

	state := domain.RegimeState{
		Asset:               asset,
		ValueDate:           valueDate,
		RegimeIndicator:     indicator,
		RegimeIndicatorRaw:  rawIndicator,
		RiskOnProbability:   riskOn,
		RiskOffProbability:  riskOff,
		TransitionRisk:      transRisk,
		ModelVersion:        s.cfg.ModelVersion,
		ConfigVersion:       s.cfg.ConfigVersion,
		CodeSHA:             s.cfg.CodeSHA,
		DomainContributions: contributions,
		TopDrivers:          topDrivers,
		EffectiveWeights:    effectiveWeights,
		FeatureCoverageFlag: coverageFlag,
		InteractionFlags:    []string{},
	}
	if err := s.repo.Save(ctx, state); err != nil {
		return fmt.Errorf("save regime state: %w", err)
	}

	return s.publisher.Publish(ctx, domain.Event{
		Topic:       "regime.state.completed.v1",
		AggregateID: fmt.Sprintf("%s:%s", asset, valueDate.Format("2006-01-02")),
		Payload: map[string]any{
			"asset":            string(asset),
			"value_date":       valueDate.Format("2006-01-02"),
			"regime_indicator": indicator,
			"risk_on":          riskOn,
			"risk_off":         riskOff,
			"transition_risk":  transRisk,
			"model_version":    s.cfg.ModelVersion,
			"config_version":   s.cfg.ConfigVersion,
		},
	})
}

func (s *Service) gatherTransitionRiskInputs(
	ctx context.Context,
	asset domain.Asset,
	valueDate, cutoff time.Time,
	scores map[domain.DomainCode]float64,
	normalized map[domain.DomainCode]float64,
	present map[domain.DomainCode]bool,
) (TransitionRiskInputs, error) {
	in := TransitionRiskInputs{NormalizedScores: normalized}

	refDate := valueDate.AddDate(0, 0, -s.cfg.RocWindowDays)
	for _, d := range allDomains {
		if !present[d] {
			continue
		}
		past, found, err := s.scoreReader.GetByDate(ctx, asset, d, refDate, cutoff)
		if err != nil {
			return in, fmt.Errorf("domain score on %s: %w", refDate.Format("2006-01-02"), err)
		}
		if !found {
			continue
		}
		today := scores[d]
		in.RocPerDomain = append(in.RocPerDomain, math.Abs(today-past)/float64(s.cfg.RocWindowDays))
	}

	histFrom := valueDate.AddDate(0, 0, -90-s.cfg.RocWindowDays)
	histTo := valueDate.AddDate(0, 0, -1)
	rawSeries, err := s.repo.GetIndicatorRawSeries(ctx, asset, histFrom, histTo)
	if err != nil {
		return in, fmt.Errorf("indicator raw series: %w", err)
	}
	in.HistoricalRocs, in.HistoricalDivs = computeHistoricalRocDiv(rawSeries, valueDate, s.cfg.RocWindowDays, 90)

	threeDaysAgo := valueDate.AddDate(0, 0, -3)
	allSameDir := true
	checkedAny := false
	for _, d := range allDomains {
		if !present[d] {
			continue
		}
		past, found, err := s.scoreReader.GetByDate(ctx, asset, d, threeDaysAgo, cutoff)
		if err != nil {
			return in, fmt.Errorf("momentum lookup %s: %w", d, err)
		}
		if !found {
			allSameDir = false
			break
		}
		todayNorm := normalized[d]
		if signf(todayNorm) != signf(past) {
			allSameDir = false
			break
		}
		checkedAny = true
	}
	in.MomentumChecked = checkedAny
	in.MomentumSameDirection = checkedAny && allSameDir

	return in, nil
}
