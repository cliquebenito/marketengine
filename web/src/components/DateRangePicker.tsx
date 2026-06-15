import { useState } from "react";

interface Props {
  from: string;
  to: string;
  onApply: (from: string, to: string) => void;
}

export function DateRangePicker({ from, to, onApply }: Props) {
  const [f, setF] = useState(from);
  const [t, setT] = useState(to);
  return (
    <div className="date-range">
      <label>
        <span>From</span>
        <input type="date" value={f} onChange={(e) => setF(e.target.value)} />
      </label>
      <label>
        <span>To</span>
        <input type="date" value={t} onChange={(e) => setT(e.target.value)} />
      </label>
      <button
        type="button"
        onClick={() => onApply(f, t)}
        disabled={!f || !t || f > t}
      >
        Apply
      </button>
    </div>
  );
}
