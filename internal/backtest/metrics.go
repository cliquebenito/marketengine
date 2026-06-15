package backtest

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

type RegimeDay struct {
	ValueDate       time.Time
	RegimeIndicator float64
	RiskOnProb      float64
	RiskOffProb     float64
	TransitionRisk  float64
	Label           Label
}

type PricePoint struct {
	Date  time.Time
	Price float64
}

type ForwardReturnCell struct {
	Label   Label
	Horizon int
	N       int
	Mean    float64
	Median  float64
	HitRate float64
}

func ForwardReturnTable(days []RegimeDay, prices map[time.Time]float64, horizons []int) []ForwardReturnCell {
	type key struct {
		Label   Label
		Horizon int
	}
	buckets := make(map[key][]float64)

	for _, d := range days {
		p0, ok := prices[d.ValueDate]
		if !ok || p0 <= 0 {
			continue
		}
		for _, h := range horizons {
			p1, ok := prices[d.ValueDate.AddDate(0, 0, h)]
			if !ok || p1 <= 0 {
				continue
			}
			logRet := math.Log(p1 / p0)
			buckets[key{d.Label, h}] = append(buckets[key{d.Label, h}], logRet)
		}
	}

	var out []ForwardReturnCell
	for _, lbl := range []Label{LabelRiskOn, LabelRiskOff, LabelTransition} {
		for _, h := range horizons {
			vs := buckets[key{lbl, h}]
			cell := ForwardReturnCell{Label: lbl, Horizon: h, N: len(vs)}
			if len(vs) == 0 {
				out = append(out, cell)
				continue
			}

			var sum float64
			for _, v := range vs {
				sum += v
			}
			cell.Mean = sum / float64(len(vs))

			sorted := append([]float64(nil), vs...)
			sort.Float64s(sorted)
			if len(sorted)%2 == 1 {
				cell.Median = sorted[len(sorted)/2]
			} else {
				cell.Median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
			}

			if lbl != LabelTransition {
				hits := 0
				for _, v := range vs {
					if lbl == LabelRiskOn && v > 0 {
						hits++
					} else if lbl == LabelRiskOff && v < 0 {
						hits++
					}
				}
				cell.HitRate = float64(hits) / float64(len(vs))
			}
			out = append(out, cell)
		}
	}
	return out
}

type PersistenceStats struct {
	MeanDurationDays   map[Label]float64
	MedianDurationDays map[Label]float64
	Transitions        int
	TransitionsPerYear float64
	FlipFlopRate       float64
	TotalDays          int
}

func PersistenceMetrics(days []RegimeDay) PersistenceStats {
	stats := PersistenceStats{
		MeanDurationDays:   map[Label]float64{},
		MedianDurationDays: map[Label]float64{},
		TotalDays:          len(days),
	}
	if len(days) == 0 {
		return stats
	}

	type run struct {
		Label Label
		Start int
		End   int
	}
	var runs []run
	cur := run{Label: days[0].Label, Start: 0, End: 0}
	for i := 1; i < len(days); i++ {
		gap := days[i].ValueDate.Sub(days[i-1].ValueDate)
		if days[i].Label != cur.Label || gap > 36*time.Hour {
			cur.End = i - 1
			runs = append(runs, cur)
			cur = run{Label: days[i].Label, Start: i, End: i}
		}
	}
	cur.End = len(days) - 1
	runs = append(runs, cur)

	durByLabel := map[Label][]int{}
	for _, r := range runs {
		dur := r.End - r.Start + 1
		durByLabel[r.Label] = append(durByLabel[r.Label], dur)
	}
	for lbl, ds := range durByLabel {
		var sum int
		for _, d := range ds {
			sum += d
		}
		stats.MeanDurationDays[lbl] = float64(sum) / float64(len(ds))
		sorted := append([]int(nil), ds...)
		sort.Ints(sorted)
		if len(sorted)%2 == 1 {
			stats.MedianDurationDays[lbl] = float64(sorted[len(sorted)/2])
		} else {
			stats.MedianDurationDays[lbl] = float64(sorted[len(sorted)/2-1]+sorted[len(sorted)/2]) / 2
		}
	}

	stats.Transitions = len(runs) - 1
	if len(days) > 0 {
		span := days[len(days)-1].ValueDate.Sub(days[0].ValueDate).Hours() / 24
		if span > 0 {
			stats.TransitionsPerYear = float64(stats.Transitions) * 365.0 / span
		}
	}

	if stats.Transitions > 0 {
		flipFlops := 0
		for i := 1; i < len(runs); i++ {
			if runs[i].End-runs[i].Start+1 <= 7 && i+1 < len(runs) && runs[i+1].Label == runs[i-1].Label {
				flipFlops++
			}
		}
		stats.FlipFlopRate = float64(flipFlops) / float64(stats.Transitions)
	}

	return stats
}

type Event struct {
	Name     string
	PeakDate time.Time
}

func CalibrationEvents() []Event {
	return []Event{
		{"China ban / leverage flush", mustDate("2021-05-19")},
		{"UST collapse", mustDate("2022-05-11")},
		{"3AC collapse", mustDate("2022-06-18")},
		{"FTX collapse", mustDate("2022-11-09")},
		{"USDC depeg / SVB", mustDate("2023-03-11")},
		{"Yen carry unwind", mustDate("2024-08-05")},
	}
}

type EventLead struct {
	Event              Event
	FirstRiskOffOffset int
	FirstTransOffset   int
	RegimeAtPeak       float64
	TransitionAtPeak   float64
	DataPresent        bool
}

func EventLeadTimes(byDate map[time.Time]RegimeDay, events []Event, transitionThreshold float64) []EventLead {
	out := make([]EventLead, 0, len(events))
	for _, ev := range events {
		lead := EventLead{Event: ev, FirstRiskOffOffset: -999, FirstTransOffset: -999}
		if d, ok := byDate[ev.PeakDate]; ok {
			lead.DataPresent = true
			lead.RegimeAtPeak = d.RegimeIndicator
			lead.TransitionAtPeak = d.TransitionRisk
		}

		for offset := -14; offset <= 3; offset++ {
			d, ok := byDate[ev.PeakDate.AddDate(0, 0, offset)]
			if !ok {
				continue
			}
			lead.DataPresent = true
			if lead.FirstRiskOffOffset == -999 && d.RegimeIndicator < 0 {
				lead.FirstRiskOffOffset = -offset
			}
			if lead.FirstTransOffset == -999 && d.TransitionRisk > transitionThreshold {
				lead.FirstTransOffset = -offset
			}
		}
		out = append(out, lead)
	}
	return out
}

type StrategyStats struct {
	AnnualizedReturn float64
	AnnualizedVol    float64
	Sharpe           float64
	MaxDrawdown      float64
	Calmar           float64
	FinalEquity      float64
	BuyAndHoldReturn float64
	BuyAndHoldMaxDD  float64
	BuyAndHoldCalmar float64
}

func ToyStrategy(days []RegimeDay, prices map[time.Time]float64) StrategyStats {
	var series []PricePoint
	for _, d := range days {
		if p, ok := prices[d.ValueDate]; ok && p > 0 {
			series = append(series, PricePoint{Date: d.ValueDate, Price: p})
		}
	}
	if len(series) < 2 {
		return StrategyStats{}
	}

	const rebalanceCost = 0.0005
	var weightFor = func(l Label) float64 {
		switch l {
		case LabelRiskOn:
			return 1.0
		case LabelRiskOff:
			return 0.0
		default:
			return 0.5
		}
	}

	labelByDate := make(map[time.Time]Label, len(days))
	for _, d := range days {
		labelByDate[d.ValueDate] = d.Label
	}

	equity := 1.0
	bh := 1.0
	var equitySeries, bhSeries []float64
	prevLabel := labelByDate[series[0].Date]
	weight := weightFor(prevLabel)
	var logRets []float64

	for i := 1; i < len(series); i++ {
		p0, p1 := series[i-1].Price, series[i].Price
		ret := p1/p0 - 1.0
		bh *= 1 + ret

		curLabel := labelByDate[series[i].Date]
		if curLabel != prevLabel {

			newWeight := weightFor(curLabel)
			turnover := math.Abs(newWeight - weight)
			equity *= 1 - turnover*rebalanceCost
			weight = newWeight
			prevLabel = curLabel
		}
		equity *= 1 + weight*ret
		equitySeries = append(equitySeries, equity)
		bhSeries = append(bhSeries, bh)

		if equity > 0 && i > 1 && equitySeries[len(equitySeries)-2] > 0 {
			logRets = append(logRets, math.Log(equity/equitySeries[len(equitySeries)-2]))
		}
	}

	stats := StrategyStats{FinalEquity: equity}
	spanDays := series[len(series)-1].Date.Sub(series[0].Date).Hours() / 24
	if spanDays > 0 {
		stats.AnnualizedReturn = math.Pow(equity, 365.0/spanDays) - 1
		stats.BuyAndHoldReturn = math.Pow(bh, 365.0/spanDays) - 1
	}

	if len(logRets) > 1 {
		var sum, sumSq float64
		for _, r := range logRets {
			sum += r
		}
		mean := sum / float64(len(logRets))
		for _, r := range logRets {
			sumSq += (r - mean) * (r - mean)
		}
		variance := sumSq / float64(len(logRets)-1)
		stats.AnnualizedVol = math.Sqrt(variance * 365)
		if stats.AnnualizedVol > 0 {
			stats.Sharpe = stats.AnnualizedReturn / stats.AnnualizedVol
		}
	}
	stats.MaxDrawdown = maxDrawdown(equitySeries)
	stats.BuyAndHoldMaxDD = maxDrawdown(bhSeries)
	if stats.MaxDrawdown > 0 {
		stats.Calmar = stats.AnnualizedReturn / stats.MaxDrawdown
	}
	if stats.BuyAndHoldMaxDD > 0 {
		stats.BuyAndHoldCalmar = stats.BuyAndHoldReturn / stats.BuyAndHoldMaxDD
	}
	return stats
}

func maxDrawdown(equity []float64) float64 {
	if len(equity) == 0 {
		return 0
	}
	peak := equity[0]
	maxDD := 0.0
	for _, v := range equity {
		if v > peak {
			peak = v
		}
		dd := (peak - v) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

type ForwardReturnCellWithCI struct {
	ForwardReturnCell
	MeanCILow  float64
	MeanCIHigh float64
	NResamples int
}

func BootstrapForwardReturnCI(days []RegimeDay, prices map[time.Time]float64, horizons []int, nResamples int) []ForwardReturnCellWithCI {
	if nResamples <= 0 {
		nResamples = 1000
	}
	type key struct {
		Label   Label
		Horizon int
	}
	buckets := make(map[key][]float64)
	for _, d := range days {
		p0, ok := prices[d.ValueDate]
		if !ok || p0 <= 0 {
			continue
		}
		for _, h := range horizons {
			p1, ok := prices[d.ValueDate.AddDate(0, 0, h)]
			if !ok || p1 <= 0 {
				continue
			}
			buckets[key{d.Label, h}] = append(buckets[key{d.Label, h}], math.Log(p1/p0))
		}
	}

	rng := rand.New(rand.NewSource(1))
	cells := ForwardReturnTable(days, prices, horizons)
	out := make([]ForwardReturnCellWithCI, 0, len(cells))
	for _, c := range cells {
		vs := buckets[key{c.Label, c.Horizon}]
		cell := ForwardReturnCellWithCI{ForwardReturnCell: c, NResamples: nResamples}
		if len(vs) >= 2 {
			means := make([]float64, nResamples)
			for i := 0; i < nResamples; i++ {
				var sum float64
				for j := 0; j < len(vs); j++ {
					sum += vs[rng.Intn(len(vs))]
				}
				means[i] = sum / float64(len(vs))
			}
			sort.Float64s(means)
			loIdx := int(0.025 * float64(nResamples))
			hiIdx := int(0.975 * float64(nResamples))
			if hiIdx >= nResamples {
				hiIdx = nResamples - 1
			}
			cell.MeanCILow = means[loIdx]
			cell.MeanCIHigh = means[hiIdx]
		}
		out = append(out, cell)
	}
	return out
}

func AnnualizedSharpe(equitySeries []float64) float64 {
	if len(equitySeries) < 3 {
		return 0
	}
	logRets := make([]float64, 0, len(equitySeries)-1)
	for i := 1; i < len(equitySeries); i++ {
		if equitySeries[i-1] > 0 && equitySeries[i] > 0 {
			logRets = append(logRets, math.Log(equitySeries[i]/equitySeries[i-1]))
		}
	}
	if len(logRets) < 2 {
		return 0
	}
	var sum float64
	for _, r := range logRets {
		sum += r
	}
	mean := sum / float64(len(logRets))
	var sumSq float64
	for _, r := range logRets {
		sumSq += (r - mean) * (r - mean)
	}
	variance := sumSq / float64(len(logRets)-1)
	std := math.Sqrt(variance)
	if std == 0 {
		return 0
	}
	return mean * math.Sqrt(365) / std
}

func AnnualizedCalmar(equitySeries []float64, spanDays float64) float64 {
	if len(equitySeries) < 2 || spanDays <= 0 {
		return 0
	}
	final := equitySeries[len(equitySeries)-1]
	if final <= 0 {
		return 0
	}
	annRet := math.Pow(final/equitySeries[0], 365.0/spanDays) - 1
	maxDD := maxDrawdown(equitySeries)
	if maxDD <= 0 {
		return 0
	}
	return annRet / maxDD
}

type SensitivityRow struct {
	Parameter string
	Value     float64
	Metrics   map[string]float64
}

func SensitivitySweep(rows []SensitivityRow) string {
	if len(rows) == 0 {
		return "(empty sensitivity sweep)\n"
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Parameter != rows[j].Parameter {
			return rows[i].Parameter < rows[j].Parameter
		}
		return rows[i].Value < rows[j].Value
	})
	metricSet := map[string]bool{}
	for _, r := range rows {
		for k := range r.Metrics {
			metricSet[k] = true
		}
	}
	metrics := make([]string, 0, len(metricSet))
	for k := range metricSet {
		metrics = append(metrics, k)
	}
	sort.Strings(metrics)

	var b strings.Builder
	fmt.Fprintf(&b, "%-32s %10s", "parameter", "value")
	for _, m := range metrics {
		fmt.Fprintf(&b, " %14s", m)
	}
	b.WriteString("\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-32s %10.4f", r.Parameter, r.Value)
		for _, m := range metrics {
			if v, ok := r.Metrics[m]; ok {
				fmt.Fprintf(&b, " %14.6f", v)
			} else {
				fmt.Fprintf(&b, " %14s", "-")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}
