import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Area,
  ComposedChart,
  Line,
  ReferenceDot,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  type BacktestEventJSON,
  type BacktestPointJSON,
  type BacktestRunJSON,
  getBacktestEvents,
  getBacktestTimeline,
  listBacktestRuns,
} from "../api";
import type { Asset } from "../types";
import { AssetSelector } from "./AssetSelector";

interface Props {
  asset: Asset;
  onAssetChange: (a: Asset) => void;
}

export function BacktestDashboard({ asset, onAssetChange }: Props) {
  const runsQ = useQuery({ queryKey: ["bt-runs"], queryFn: () => listBacktestRuns(100) });
  const [runID, setRunID] = useState<string | null>(null);

  useEffect(() => {
    if (runID || !runsQ.data || runsQ.data.length === 0) return;
    const replay = runsQ.data.find((r) => r.mode === "replay") ?? runsQ.data[0];
    setRunID(replay.run_id);
  }, [runsQ.data, runID]);

  const tlQ = useQuery({
    queryKey: ["bt-tl", runID, asset],
    queryFn: () => getBacktestTimeline(runID!, asset),
    enabled: !!runID,
  });
  const evQ = useQuery({
    queryKey: ["bt-ev", runID, asset],
    queryFn: () => getBacktestEvents(runID!, asset),
    enabled: !!runID,
  });

  return (
    <div style={{ padding: "16px 24px", color: "#1a2029" }}>
      <header style={{ marginBottom: 16 }}>
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700 }}>
          Бэктест — {asset}
        </h1>
        <div style={{ opacity: 0.7, fontSize: 13, marginTop: 4 }}>
          Как режим рынка соотносился с ценой актива на истории
        </div>
      </header>

      <div style={{ display: "flex", gap: 12, alignItems: "center", marginBottom: 16, flexWrap: "wrap" }}>
        <AssetSelector asset={asset} onChange={onAssetChange} />
        <RunSelector
          runs={runsQ.data ?? []}
          loading={runsQ.isLoading}
          value={runID}
          onChange={setRunID}
        />
      </div>

      {tlQ.isLoading && <div style={{ opacity: 0.6 }}>Загрузка таймлайна…</div>}
      {tlQ.error && <div style={{ color: "#ff7b7b" }}>{(tlQ.error as Error).message}</div>}
      {tlQ.data && (
        <BacktestChart points={tlQ.data.points} events={evQ.data ?? []} />
      )}

      {evQ.data && <EventsTable events={evQ.data} />}
    </div>
  );
}

function RunSelector({
  runs,
  loading,
  value,
  onChange,
}: {
  runs: BacktestRunJSON[];
  loading: boolean;
  value: string | null;
  onChange: (id: string) => void;
}) {
  if (loading) return <span style={{ opacity: 0.6 }}>Загружаю прогоны…</span>;
  if (runs.length === 0) return <span style={{ opacity: 0.6 }}>Нет завершённых прогонов</span>;
  return (
    <select
      value={value ?? ""}
      onChange={(e) => onChange(e.target.value)}
      style={{
        background: "#e7ecf3",
        color: "#1a2029",
        border: "1px solid #d7dee7",
        borderRadius: 6,
        padding: "6px 10px",
        fontSize: 13,
        minWidth: 360,
      }}
    >
      {runs.map((r) => {
        const label = `${r.mode} • ${r.period_start} → ${r.period_end} • ${r.model_version} • ${r.run_id.slice(0, 8)}`;
        return (
          <option key={r.run_id} value={r.run_id}>
            {label}
          </option>
        );
      })}
    </select>
  );
}

function BacktestChart({
  points,
  events,
}: {
  points: BacktestPointJSON[];
  events: BacktestEventJSON[];
}) {
  // Единое поле ts (timestamp) для числовой оси X со шкалой времени.
  const data = useMemo(
    () =>
      points.map((p) => ({
        ...p,
        ts: new Date(p.value_date).getTime(),
      })),
    [points],
  );

  // Маркеры событий — только те, по которым были данные.
  const eventDots = useMemo(
    () =>
      events
        .filter((e) => e.data_present)
        .map((e) => ({
          ts: new Date(e.peak_date).getTime(),
          label: e.name,
          detected: e.first_risk_off_offset !== -999,
        })),
    [events],
  );

  const fmtDate = (t: number) => new Date(t).toISOString().slice(0, 10);

  return (
    <div style={{ width: "100%", height: 460, background: "#ffffff", border: "1px solid #d7dee7", borderRadius: 8, padding: 12 }}>
      <div style={{ fontSize: 12, color: "#5b6573", margin: "0 0 8px 4px" }}>
        Цена актива (линия) на фоне тилта режима: <span style={{ color: "#3fb950" }}>зелёный — Risk-On</span>,{" "}
        <span style={{ color: "#f85149" }}>красный — Risk-Off</span>. ● — исторический кризис.
      </div>
      <ResponsiveContainer>
        <ComposedChart data={data} margin={{ top: 16, right: 56, left: 8, bottom: 8 }}>
          <defs>
            <linearGradient id="btRegime" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#3fb950" stopOpacity={0.5} />
              <stop offset="50%" stopColor="#3fb950" stopOpacity={0.06} />
              <stop offset="50%" stopColor="#f85149" stopOpacity={0.06} />
              <stop offset="100%" stopColor="#f85149" stopOpacity={0.5} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="ts"
            type="number"
            domain={["dataMin", "dataMax"]}
            scale="time"
            tickFormatter={fmtDate}
            stroke="#aab2bf"
            fontSize={11}
          />
          {/* Левая ось — тилт режима (−1…+1), заливкой; служебная, без подписи. */}
          <YAxis yAxisId="regime" domain={[-1, 1]} hide />
          {/* Правая ось — цена, главный объект. */}
          <YAxis
            yAxisId="price"
            orientation="right"
            stroke="#fbbf24"
            fontSize={11}
            tickFormatter={(v: number) =>
              v >= 1000 ? `${(v / 1000).toFixed(1)}K` : v.toFixed(0)
            }
            label={{ value: "Цена, $", angle: 90, position: "insideRight", fill: "#fbbf24", fontSize: 11 }}
          />
          <ReferenceLine yAxisId="regime" y={0} stroke="#cdd5df" />
          <Tooltip
            contentStyle={{ background: "#e7ecf3", border: "1px solid #d7dee7", color: "#1a2029", fontSize: 12 }}
            labelFormatter={(t: number) => fmtDate(t)}
            formatter={(value: number, name: string) => {
              if (name === "Цена") return [`$${value.toFixed(2)}`, name];
              return [value.toFixed(3), name];
            }}
          />
          <Area
            yAxisId="regime"
            type="monotone"
            dataKey="regime_indicator"
            name="Тилт режима"
            stroke="#5b6573"
            strokeWidth={1}
            fill="url(#btRegime)"
            isAnimationActive={false}
          />
          <Line
            yAxisId="price"
            type="monotone"
            dataKey="price"
            name="Цена"
            stroke="#fbbf24"
            dot={false}
            strokeWidth={1.8}
            isAnimationActive={false}
          />
          {eventDots.map((e) => (
            <ReferenceDot
              key={`${e.label}-${e.ts}`}
              yAxisId="regime"
              x={e.ts}
              y={-0.92}
              r={5}
              fill={e.detected ? "#3fb950" : "#5b6573"}
              stroke="#ffffff"
              strokeWidth={2}
              ifOverflow="extendDomain"
              label={{ value: e.label, position: "bottom", fill: "#6b7280", fontSize: 10 }}
            />
          ))}
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}

function EventsTable({ events }: { events: BacktestEventJSON[] }) {
  // Считаем только события, по которым были данные на тот момент — остальные
  // вне зоны покрытия и в знаменатель честно не идут.
  const covered = events.filter((e) => e.data_present);
  const detected = covered.filter((e) => e.first_risk_off_offset !== -999);

  return (
    <div style={{ marginTop: 24 }}>
      <h2 style={{ fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
        Реакция на исторические кризисы
      </h2>
      <div style={{ fontSize: 13, color: "#5b6573", marginBottom: 10 }}>
        Модель распознала{" "}
        <b style={{ color: "#3fb950" }}>{detected.length} из {covered.length}</b>{" "}
        кризисов, по которым были данные. «—» — событие вне периода доступных данных.
      </div>
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
        <thead>
          <tr style={{ borderBottom: "1px solid #d7dee7", textAlign: "left" }}>
            <th style={th}>Событие</th>
            <th style={th}>Дата пика</th>
            <th style={th}>Покрытие</th>
            <th style={th}>Risk-Off, дней до</th>
            <th style={th}>Неопределённость, дней до</th>
          </tr>
        </thead>
        <tbody>
          {events.map((e) => {
            const off = e.first_risk_off_offset !== -999;
            const trn = e.first_trans_offset !== -999;
            return (
              <tr key={e.name} style={{ borderBottom: "1px solid #e7ecf3" }}>
                <td style={td}>
                  {e.data_present && off ? "✓ " : ""}
                  {e.name}
                </td>
                <td style={td}>{e.peak_date}</td>
                <td style={{ ...td, color: e.data_present ? "#3fb950" : "#5b6573" }}>
                  {e.data_present ? "есть" : "вне покрытия"}
                </td>
                <td style={{ ...td, color: off ? "#3fb950" : "#5b6573" }}>
                  {off ? `за ${e.first_risk_off_offset}` : "—"}
                </td>
                <td style={{ ...td, color: trn ? "#3fb950" : "#5b6573" }}>
                  {trn ? `за ${e.first_trans_offset}` : "—"}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

const th = { padding: "8px 12px", color: "#6b7280", fontWeight: 600 } as const;
const td = { padding: "8px 12px" } as const;
