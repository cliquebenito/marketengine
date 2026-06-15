package regime

import "marketengine/internal/domain"

var allDomains = domain.AllDomains()

func presentSet(scores map[domain.DomainCode]float64) map[domain.DomainCode]bool {
	out := make(map[domain.DomainCode]bool, len(allDomains))
	for _, d := range allDomains {
		_, ok := scores[d]
		out[d] = ok
	}
	return out
}

func coverageFraction(scores map[domain.DomainCode]float64) float64 {
	return float64(len(scores)) / float64(len(allDomains))
}
