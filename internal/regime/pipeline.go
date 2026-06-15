package regime

import (
	"math"
	"sort"
	"time"

	"marketengine/internal/domain"
)

func RedistributeWeights(nominal map[domain.DomainCode]float64, present map[domain.DomainCode]bool) map[domain.DomainCode]float64 {
	return redistributeWeights(nominal, present)
}

func NormalizeScores(scores map[domain.DomainCode]float64, histories map[domain.DomainCode][]float64, minSamples int) map[domain.DomainCode]float64 {
	return normalizeScores(scores, histories, minSamples)
}

func WeightedSumRaw(scores, weights map[domain.DomainCode]float64) float64 {
	return weightedSumRaw(scores, weights)
}

func WeightedSumNormalized(norm, weights map[domain.DomainCode]float64) (map[domain.DomainCode]float64, float64, float64) {
	return weightedSumNormalized(norm, weights)
}

func ComputeTransitionRisk(in TransitionRiskInputs, cfg Config) float64 {
	return computeTransitionRisk(in, cfg)
}

func ComputeHistoricalRocDiv(series []domain.IndicatorPoint, valueDate time.Time, rocWindowDays, lookbackDays int) (rocs, divs []float64) {
	return computeHistoricalRocDiv(series, valueDate, rocWindowDays, lookbackDays)
}

func EmaSmooth(raw float64, prev []float64, span int) float64 {
	return emaSmooth(raw, prev, span)
}

func FinalProbabilities(indicator, transitionRisk, sigmoidK float64) (riskOn, riskOff float64) {
	return finalProbabilities(indicator, transitionRisk, sigmoidK)
}

func BuildTopDrivers(contributions map[domain.DomainCode]float64) []domain.TopDriver {
	return buildTopDrivers(contributions)
}

func PresentSet(scores map[domain.DomainCode]float64) map[domain.DomainCode]bool {
	return presentSet(scores)
}

func CoverageFraction(scores map[domain.DomainCode]float64) float64 {
	return coverageFraction(scores)
}

func SignF(x float64) float64 {
	return signf(x)
}

func Clip(x, lo, hi float64) float64 {
	return clip(x, lo, hi)
}

func redistributeWeights(nominal map[domain.DomainCode]float64, present map[domain.DomainCode]bool) map[domain.DomainCode]float64 {
	var sumPresent float64
	for d, w := range nominal {
		if present[d] {
			sumPresent += w
		}
	}
	if sumPresent == 0 {
		return nominal
	}
	eff := make(map[domain.DomainCode]float64, len(nominal))
	for d, w := range nominal {
		if present[d] {
			eff[d] = w / sumPresent
		}
	}
	return eff
}

func normalizeScores(
	scores map[domain.DomainCode]float64,
	histories map[domain.DomainCode][]float64,
	minSamples int,
) map[domain.DomainCode]float64 {
	out := make(map[domain.DomainCode]float64, len(scores))
	for d, s := range scores {
		hist := histories[d]
		if len(hist) < minSamples {
			out[d] = s
			continue
		}
		m := mean(hist)
		sd := stddev(hist)
		if sd == 0 {
			out[d] = s
			continue
		}
		out[d] = (s - m) / sd
	}
	return out
}

func weightedSumRaw(scores map[domain.DomainCode]float64, weights map[domain.DomainCode]float64) float64 {
	var raw float64
	for d, w := range weights {
		raw += w * scores[d]
	}
	return raw
}

func weightedSumNormalized(norm map[domain.DomainCode]float64, weights map[domain.DomainCode]float64) (map[domain.DomainCode]float64, float64, float64) {
	contributions := make(map[domain.DomainCode]float64, len(weights))
	var sum float64
	for d, w := range weights {
		c := w * norm[d]
		contributions[d] = c
		sum += c
	}
	return contributions, clip(sum, -2, 2), sum
}

type TransitionRiskInputs struct {
	NormalizedScores      map[domain.DomainCode]float64
	RocPerDomain          []float64
	HistoricalRocs        []float64
	HistoricalDivs        []float64
	MomentumSameDirection bool
	MomentumChecked       bool
}

func computeTransitionRisk(in TransitionRiskInputs, cfg Config) float64 {
	var meanRoc float64
	if len(in.RocPerDomain) > 0 {
		meanRoc = mean(in.RocPerDomain)
	}

	var normVals []float64
	for _, d := range allDomains {
		if v, ok := in.NormalizedScores[d]; ok {
			normVals = append(normVals, v)
		}
	}
	var divergence float64
	if len(normVals) >= 2 {
		divergence = stddev(normVals)
	}

	var rocZ, divZ float64
	if len(in.HistoricalRocs) >= 10 {
		m := mean(in.HistoricalRocs)
		sd := stddev(in.HistoricalRocs)
		if sd > 0 {
			rocZ = (meanRoc - m) / sd
		}
	} else {

		rocZ = meanRoc * 10
	}
	if len(in.HistoricalDivs) >= 10 {
		m := mean(in.HistoricalDivs)
		sd := stddev(in.HistoricalDivs)
		if sd > 0 {
			divZ = (divergence - m) / sd
		}
	} else {
		divZ = divergence
	}

	consistencyDiscount := 1.0
	if in.MomentumChecked && in.MomentumSameDirection {
		consistencyDiscount = 0.7
	}

	raw := cfg.RocWeight*math.Tanh(math.Abs(rocZ)/2) + cfg.DivergenceWeight*math.Tanh(math.Abs(divZ)/2)
	baseline := cfg.TransitionBaseline
	if baseline < 0 || baseline >= 1 {
		baseline = 0
	}
	var shifted float64
	if baseline > 0 {
		shifted = (raw - baseline) / (1 - baseline)
	} else {
		shifted = raw
	}
	return clip(consistencyDiscount*clip(shifted, 0, 1), 0, 1)
}

func computeHistoricalRocDiv(
	series []domain.IndicatorPoint,
	valueDate time.Time,
	rocWindowDays int,
	lookbackDays int,
) (rocs []float64, divs []float64) {
	indByDate := make(map[string]float64, len(series))
	for _, p := range series {
		indByDate[p.ValueDate.Format("2006-01-02")] = p.RegimeIndicator
	}
	for i := 0; i < lookbackDays; i++ {
		d := valueDate.AddDate(0, 0, -(i + 1))
		dKey := d.Format("2006-01-02")
		refKey := d.AddDate(0, 0, -rocWindowDays).Format("2006-01-02")
		curr, okC := indByDate[dKey]
		prev, okP := indByDate[refKey]
		if okC && okP {
			rocs = append(rocs, math.Abs(curr-prev)/float64(rocWindowDays))
		}
		var window []float64
		for j := -3; j <= 3; j++ {
			wKey := d.AddDate(0, 0, j).Format("2006-01-02")
			if v, ok := indByDate[wKey]; ok {
				window = append(window, v)
			}
		}
		if len(window) >= 2 {
			divs = append(divs, stddev(window))
		}
	}
	return rocs, divs
}

func finalProbabilities(indicator, transitionRisk, sigmoidK float64) (riskOn, riskOff float64) {
	baselineOn := sigmoid(sigmoidK * indicator)
	return (1 - transitionRisk) * baselineOn, (1 - transitionRisk) * (1 - baselineOn)
}

func buildTopDrivers(contributions map[domain.DomainCode]float64) []domain.TopDriver {
	type entry struct {
		domain  domain.DomainCode
		contrib float64
		absVal  float64
	}
	entries := make([]entry, 0, len(contributions))
	var absSum float64
	for d, c := range contributions {
		a := math.Abs(c)
		entries = append(entries, entry{d, c, a})
		absSum += a
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].absVal > entries[j].absVal
	})
	drivers := make([]domain.TopDriver, 0, len(entries))
	for _, e := range entries {
		share := 0.0
		if absSum > 0 {
			share = e.absVal / absSum
		}
		drivers = append(drivers, domain.TopDriver{
			Domain:       e.domain,
			Contribution: e.contrib,
			Share:        share,
		})
	}
	return drivers
}

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func clip(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stddev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	var s float64
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func signf(x float64) float64 {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}
