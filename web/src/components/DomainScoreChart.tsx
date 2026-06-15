import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DomainScoreRowJSON } from "../types";

interface Props {
  rows: DomainScoreRowJSON[];
  mini?: boolean;
}

export function DomainScoreChart({ rows, mini = false }: Props) {
  const data = rows.map((r) => ({
    value_date: r.value_date,
    score: r.score,
  }));

  return (
    <div style={{ width: "100%", height: mini ? 70 : 280 }}>
      <ResponsiveContainer>
        <LineChart
          data={data}
          margin={mini ? { top: 4, right: 4, bottom: 0, left: 0 } : { top: 8, right: 16, bottom: 4, left: 0 }}
        >
          {!mini && <CartesianGrid stroke="#d7dee7" strokeDasharray="3 3" />}
          {!mini && (
            <XAxis dataKey="value_date" stroke="#5b6573" fontSize={11} minTickGap={40} />
          )}
          {!mini && (
            <YAxis domain={[-1, 1]} stroke="#5b6573" fontSize={11} tickFormatter={(v) => v.toFixed(1)} />
          )}
          {!mini && <ReferenceLine y={0} stroke="#5b6573" strokeDasharray="2 2" />}
          {!mini && (
            <Tooltip
              contentStyle={{ background: "#ffffff", border: "1px solid #d7dee7", fontSize: 12 }}
              labelStyle={{ color: "#1a2029" }}
            />
          )}
          <Line
            type="monotone"
            dataKey="score"
            stroke="#58a6ff"
            strokeWidth={mini ? 1.5 : 2}
            dot={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
