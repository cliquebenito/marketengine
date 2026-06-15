import type { RegimeStateJSON } from "../types";

interface Props {
  state: RegimeStateJSON;
}

type Verdict = {
  label: string;
  sub: string;
  color: string;
  value: number;
};

function verdictOf(on: number, off: number, tr: number): Verdict {
  const max = Math.max(on, off, tr);
  if (max === off)
    return { label: "RISK-OFF", sub: "Уход от риска — рынок в обороне", color: "var(--red)", value: off };
  if (max === tr)
    return { label: "НЕОПРЕДЕЛЁННОСТЬ", sub: "Нейтрально — ни рост, ни уход от риска не преобладают", color: "var(--amber)", value: tr };
  return { label: "RISK-ON", sub: "Аппетит к риску — склонность к росту", color: "var(--green)", value: on };
}

const pct = (x: number) => `${Math.round(x * 100)}%`;

// Verdict badge (what regime is it now) + a stacked probability bar (how
// confident, split across the three states). The raw smoothed indicator is
// demoted to a footnote — it is an internal quantity, not a headline.
export function RegimeGauge({ state }: Props) {
  const on = state.risk_on_probability;
  const off = state.risk_off_probability;
  const tr = state.transition_risk;
  const v = verdictOf(on, off, tr);
  const [yy, mm, dd] = state.value_date.split("-");
  const dateRU = `${dd}.${mm}.${yy}`;

  return (
    <div className="regime-verdict">
      <div className="verdict-main" style={{ borderColor: v.color }}>
        <div className="verdict-label" style={{ color: v.color }}>{v.label}</div>
        <div className="verdict-big" style={{ color: v.color }}>{pct(v.value)}</div>
        <div className="verdict-sub">{v.sub}</div>
      </div>

      <div className="verdict-detail">
        <div className="prob-bar" role="img" aria-label="Распределение режима">
          <div className="seg on" style={{ width: pct(on) }} title={`Risk-On ${pct(on)}`} />
          <div className="seg tr" style={{ width: pct(tr) }} title={`Неопределённость ${pct(tr)}`} />
          <div className="seg off" style={{ width: pct(off) }} title={`Risk-Off ${pct(off)}`} />
        </div>
        <div className="prob-legend">
          <span><i className="dot on" /> Risk-On <b>{pct(on)}</b></span>
          <span><i className="dot tr" /> Неопределённость <b>{pct(tr)}</b></span>
          <span><i className="dot off" /> Risk-Off <b>{pct(off)}</b></span>
        </div>
        <div className="verdict-foot">Обновлено {dateRU}</div>
      </div>
    </div>
  );
}
