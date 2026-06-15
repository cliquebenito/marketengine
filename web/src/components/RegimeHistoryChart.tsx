import {
  Area,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { RegimeStateJSON } from "../types";

interface Props {
  history: RegimeStateJSON[];
}

export function RegimeHistoryChart({ history }: Props) {
  const data = history.map((s) => ({
    value_date: s.value_date,
    regime_indicator: s.regime_indicator,
    transition_risk: s.transition_risk,
  }));

  return (
    <div style={{ width: "100%", height: 340 }}>
      <ResponsiveContainer>
        <ComposedChart data={data} margin={{ top: 8, right: 20, bottom: 4, left: 0 }}>
          <defs>
            {}
            <linearGradient id="riFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#3fb950" stopOpacity={0.55} />
              <stop offset="50%" stopColor="#3fb950" stopOpacity={0.08} />
              <stop offset="50%" stopColor="#f85149" stopOpacity={0.08} />
              <stop offset="100%" stopColor="#f85149" stopOpacity={0.55} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="#d7dee7" strokeDasharray="3 3" />
          <XAxis dataKey="value_date" stroke="#5b6573" fontSize={11} minTickGap={40} />
          <YAxis
            yAxisId="ri"
            domain={[-1, 1]}
            stroke="#5b6573"
            fontSize={11}
            tickFormatter={(v) => v.toFixed(1)}
          />
          <YAxis
            yAxisId="tr"
            orientation="right"
            domain={[0, 1]}
            stroke="#a371f7"
            fontSize={11}
            tickFormatter={(v) => v.toFixed(1)}
          />
          <ReferenceLine yAxisId="ri" y={0} stroke="#5b6573" />
          <Tooltip
            contentStyle={{ background: "#ffffff", border: "1px solid #d7dee7", fontSize: 12 }}
            labelStyle={{ color: "#1a2029" }}
            formatter={(value: number, name: string) => [value.toFixed(3), name]}
          />
          <Legend wrapperStyle={{ fontSize: 12 }} />
          <Area
            yAxisId="ri"
            type="monotone"
            dataKey="regime_indicator"
            name="Индикатор режима"
            stroke="#5b6573"
            strokeWidth={1.5}
            fill="url(#riFill)"
            isAnimationActive={false}
          />
          <Line
            yAxisId="tr"
            type="monotone"
            dataKey="transition_risk"
            name="Неопределённость"
            stroke="#a371f7"
            strokeWidth={1.2}
            strokeOpacity={0.6}
            dot={false}
            isAnimationActive={false}
          />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}
