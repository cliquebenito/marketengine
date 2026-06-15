package domain

var displayNames = map[string]string{
	"LIQUIDITY":         "Liquidity",
	"LEVERAGE":          "Leverage",
	"MARKET_STRESS":     "Market Stress",
	"CAPITAL_FLOWS":     "Holder Conviction",
	"VOLATILITY_REGIME": "Volatility Regime",
}

func DisplayName(domainCode string) string {
	if n, ok := displayNames[domainCode]; ok {
		return n
	}
	return domainCode
}
