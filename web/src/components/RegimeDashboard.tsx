import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import {
  daysAgoISO,
  getDomainScores,
  getRegimeHistory,
  getRegimeLatest,
  todayISO,
} from "../api";
import type {
  Asset,
  DomainScoreRowJSON,
  DomainSlug,
  RegimeStateJSON,
} from "../types";
import { DOMAIN_CODE_TO_SLUG } from "../types";
import { AssetSelector } from "./AssetSelector";
import { DateRangePicker } from "./DateRangePicker";
import { DomainScoreChart } from "./DomainScoreChart";
import { RegimeGauge } from "./RegimeGauge";
import { RegimeHistoryChart } from "./RegimeHistoryChart";
import { TopDriversTable } from "./TopDriversTable";

interface Props {
  asset: Asset;
  onAssetChange: (a: Asset) => void;
}

export function RegimeDashboard({ asset, onAssetChange }: Props) {
  const [from, setFrom] = useState(daysAgoISO(365));
  const [to, setTo] = useState(todayISO());

  const latestQ = useQuery({
    queryKey: ["regime-latest", asset],
    queryFn: () => getRegimeLatest(asset),
  });

  const historyQ = useQuery({
    queryKey: ["regime-history", asset, from, to],
    queryFn: () => getRegimeHistory(asset, from, to),
  });

  const domainWindowFrom = useMemo(() => daysAgoISO(180), []);
  const domainWindowTo = useMemo(() => todayISO(), []);

  const domainSlugs: DomainSlug[] = [
    "liquidity",
    "leverage",
    "market-stress",
    "capital-flows",
    "volatility",
  ];

  const domainQs = useQueries({
    queries: domainSlugs.map((slug) => ({
      queryKey: ["domain-scores", slug, asset, domainWindowFrom, domainWindowTo],
      queryFn: () =>
        getDomainScores(slug, asset, domainWindowFrom, domainWindowTo),
    })),
  });

  const [openSlug, setOpenSlug] = useState<DomainSlug | null>(null);

  return (
    <div className="app">
      <header className="header">
        <div>
          <h1>Market Regime Engine — {asset}</h1>
          <div className="meta">
            Оценка режима рынка по пяти доменам риска
          </div>
        </div>
        <AssetSelector asset={asset} onChange={onAssetChange} />
      </header>

      <section className="panel">
        <h2>Режим рынка сейчас</h2>
        {latestQ.isLoading && <div className="loading">Загрузка…</div>}
        {latestQ.error && (
          <div className="error">Не удалось загрузить текущий режим: {String(latestQ.error)}</div>
        )}
        {latestQ.data && <RegimeGauge state={latestQ.data} />}
      </section>

      <section className="panel">
        <h2>История режима ({from} → {to})</h2>
        <DateRangePicker
          from={from}
          to={to}
          onApply={(f, t) => {
            setFrom(f);
            setTo(t);
          }}
        />
        {historyQ.isLoading && <div className="loading">Loading history…</div>}
        {historyQ.error && (
          <div className="error">Failed to load regime history: {String(historyQ.error)}</div>
        )}
        {historyQ.data && historyQ.data.length > 0 && (
          <RegimeHistoryChart history={historyQ.data} />
        )}
        {historyQ.data && historyQ.data.length === 0 && (
          <div className="loading">No history in the selected range.</div>
        )}
      </section>

      <section className="panel">
        <h2>Главные драйверы (что повлияло на оценку)</h2>
        {latestQ.data && <TopDriversTable drivers={latestQ.data.top_drivers} />}
      </section>

      <section className="panel">
        <h2>Домены (180 дней)</h2>
        <div className="domain-grid">
          {domainSlugs.map((slug, idx) => {
            const q = domainQs[idx];
            const rows = (q.data as DomainScoreRowJSON[] | undefined) ?? [];
            const latest = rows.length > 0 ? rows[rows.length - 1] : undefined;

            const displayFromRegime =
              latest && latestQ.data
                ? latestQ.data.domain_display_names[latest.domain]
                : undefined;
            const display =
              displayFromRegime ??
              latest?.domain_display ??
              latest?.domain ??
              slug;
            const score = latest?.score;
            const tone =
              score === undefined
                ? "flat"
                : score > 0.15
                  ? "up"
                  : score < -0.15
                    ? "down"
                    : "flat";
            return (
              <div
                key={slug}
                className="domain-card"
                onClick={() => setOpenSlug(slug)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") setOpenSlug(slug);
                }}
              >
                <div className="name">{display}</div>
                <div className={`latest ${tone}`}>
                  {score === undefined
                    ? "—"
                    : `${score >= 0 ? "+" : ""}${score.toFixed(2)}`}
                </div>
                {q.isLoading && (
                  <div style={{ fontSize: 11, color: "var(--muted)" }}>Loading…</div>
                )}
                {q.error && <div className="error">Error</div>}
                {rows.length > 0 && <DomainScoreChart rows={rows} mini />}
              </div>
            );
          })}
        </div>
      </section>

      {openSlug && (
        <DomainDetailModal
          slug={openSlug}
          rows={
            (domainQs[domainSlugs.indexOf(openSlug)]
              .data as DomainScoreRowJSON[] | undefined) ?? []
          }
          latest={latestQ.data}
          onClose={() => setOpenSlug(null)}
        />
      )}
    </div>
  );
}

interface ModalProps {
  slug: DomainSlug;
  rows: DomainScoreRowJSON[];
  latest: RegimeStateJSON | undefined;
  onClose: () => void;
}

function DomainDetailModal({ slug, rows, latest, onClose }: ModalProps) {
  const latestRow = rows.length > 0 ? rows[rows.length - 1] : undefined;
  const domainCode = latestRow?.domain;
  const displayFromRegime =
    domainCode && latest ? latest.domain_display_names[domainCode] : undefined;
  const display = displayFromRegime ?? latestRow?.domain_display ?? slug;
  const components = latestRow?.components ?? null;

  // Effective weight, if the engine emitted one for this domain_code. The
  // backend sends a map keyed by the stable code.
  const weight =
    domainCode && latest ? latest.effective_weights[domainCode] : undefined;

  // Find this domain's contribution on the latest date, if present.
  const contribution =
    domainCode && latest ? latest.domain_contributions[domainCode] : undefined;

  return (
    <div
      className="modal-overlay"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <header>
          <h3>{display}</h3>
          <button type="button" onClick={onClose}>
            Close
          </button>
        </header>
        <div style={{ display: "flex", gap: 20, fontSize: 13, color: "var(--muted)", marginBottom: 8 }}>
          <span>
            Latest score:{" "}
            <strong style={{ color: "var(--text)" }}>
              {latestRow ? latestRow.score.toFixed(3) : "—"}
            </strong>
          </span>
          {weight !== undefined && (
            <span>
              Effective weight:{" "}
              <strong style={{ color: "var(--text)" }}>{weight.toFixed(2)}</strong>
            </span>
          )}
          {contribution !== undefined && (
            <span>
              Contribution:{" "}
              <strong style={{ color: "var(--text)" }}>
                {contribution >= 0 ? "+" : ""}
                {contribution.toFixed(4)}
              </strong>
            </span>
          )}
        </div>
        <DomainScoreChart rows={rows} />
        {components && Object.keys(components).length > 0 && (
          <div className="components-list">
            <div style={{ fontSize: 11, color: "var(--muted)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6 }}>
              Components (latest)
            </div>
            {Object.entries(components).map(([k, v]) => (
              <div className="row" key={k}>
                <span>{k}</span>
                <span style={{ fontVariantNumeric: "tabular-nums" }}>{v.toFixed(4)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// Re-export so App can import the type location without a deeper path. Keeps
// the main.tsx ↔ App.tsx boundary minimal.
export { DOMAIN_CODE_TO_SLUG };
