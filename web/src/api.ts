

import type {
  Asset,
  DomainScoreRowJSON,
  DomainSlug,
  HealthJSON,
  RegimeStateJSON,
} from "./types";

const BASE_URL: string =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ??
  "http://localhost:8080";

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`);
  if (!res.ok) {
    let detail = "";
    try {
      const body = await res.json();
      if (body && typeof body === "object" && "error" in body) {
        detail = String((body as { error: unknown }).error);
      }
    } catch {
      // swallow — backend might not send JSON on 500
    }
    throw new Error(
      `GET ${path} → ${res.status} ${res.statusText}${detail ? ` — ${detail}` : ""}`,
    );
  }
  return (await res.json()) as T;
}

export function getHealth(): Promise<HealthJSON> {
  return getJSON<HealthJSON>("/health");
}

export function getRegimeLatest(asset: Asset): Promise<RegimeStateJSON> {
  return getJSON<RegimeStateJSON>(`/regime/latest?asset=${asset}`);
}

export function getRegimeHistory(
  asset: Asset,
  from: string,
  to: string,
): Promise<RegimeStateJSON[]> {
  return getJSON<RegimeStateJSON[]>(
    `/regime/history?asset=${asset}&from=${from}&to=${to}`,
  );
}

export function getRegimeOnDate(
  asset: Asset,
  date: string,
): Promise<RegimeStateJSON> {
  return getJSON<RegimeStateJSON>(
    `/regime/${date}/contributions?asset=${asset}`,
  );
}

export function getDomainScores(
  domainSlug: DomainSlug,
  asset: Asset,
  from: string,
  to: string,
): Promise<DomainScoreRowJSON[]> {
  return getJSON<DomainScoreRowJSON[]>(
    `/domains/${domainSlug}/scores?asset=${asset}&from=${from}&to=${to}`,
  );
}

// Helper: YYYY-MM-DD for N days back from today (UTC).
export function daysAgoISO(days: number): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - days);
  return d.toISOString().slice(0, 10);
}

export function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

// --- Backtest ---------------------------------------------------------------

export interface BacktestRunJSON {
  run_id: string;
  mode: string;
  period_start: string;
  period_end: string;
  model_version: string;
  config_version: string;
  status: string;
  started_at: string;
  completed_at?: string;
  parent_run_id?: string;
}

export interface BacktestPointJSON {
  value_date: string;
  regime_indicator: number;
  risk_on_prob: number;
  risk_off_prob: number;
  transition_risk: number;
  price?: number;
}

export interface BacktestTimelineJSON {
  run_id: string;
  asset: Asset;
  points: BacktestPointJSON[];
}

export interface BacktestEventJSON {
  name: string;
  peak_date: string;
  first_risk_off_offset: number;
  first_trans_offset: number;
  data_present: boolean;
}

export function listBacktestRuns(limit = 50): Promise<BacktestRunJSON[]> {
  return getJSON<BacktestRunJSON[]>(`/backtest/runs?limit=${limit}`);
}

export function getBacktestTimeline(
  runID: string,
  asset: Asset,
): Promise<BacktestTimelineJSON> {
  return getJSON<BacktestTimelineJSON>(
    `/backtest/${runID}/timeline?asset=${asset}`,
  );
}

export function getBacktestEvents(
  runID: string,
  asset: Asset,
): Promise<BacktestEventJSON[]> {
  return getJSON<BacktestEventJSON[]>(
    `/backtest/${runID}/events?asset=${asset}`,
  );
}
