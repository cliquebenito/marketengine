

export type Asset = "BTC" | "ETH";

export type Direction = "risk_on" | "risk_off" | "neutral";

export interface TopDriverJSON {
  domain: string;
  domain_display: string;
  contribution: number;
  share: number;
  direction: Direction;
}

export interface RegimeStateJSON {
  asset: string;
  value_date: string;
  regime_indicator: number;
  regime_indicator_raw: number;
  risk_on_probability: number;
  risk_off_probability: number;
  transition_risk: number;
  domain_contributions: Record<string, number>;
  domain_display_names: Record<string, string>;
  top_drivers: TopDriverJSON[];
  effective_weights: Record<string, number>;
  feature_coverage: Record<string, boolean>;
  interaction_flags: string[];
  model_version: string;
  config_version: string;
  code_sha: string;
}

export interface DomainScoreRowJSON {
  asset: string;
  domain: string;
  domain_display: string;
  value_date: string;
  score: number;
  components: Record<string, number> | null;
  data_quality: Record<string, unknown> | null;
  model_version: string;
}

export interface HealthJSON {
  status: string;
  git_sha: string;
}

export const DOMAIN_SLUGS = [
  "liquidity",
  "leverage",
  "market-stress",
  "capital-flows",
  "volatility",
] as const;
export type DomainSlug = (typeof DOMAIN_SLUGS)[number];

export const DOMAIN_CODE_TO_SLUG: Record<string, DomainSlug> = {
  LIQUIDITY: "liquidity",
  LEVERAGE: "leverage",
  MARKET_STRESS: "market-stress",
  CAPITAL_FLOWS: "capital-flows",
  VOLATILITY_REGIME: "volatility",
};
