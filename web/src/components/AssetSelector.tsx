import type { Asset } from "../types";

interface Props {
  asset: Asset;
  onChange: (a: Asset) => void;
}

export function AssetSelector({ asset, onChange }: Props) {
  return (
    <div className="asset-selector" role="tablist">
      {(["BTC", "ETH"] as const).map((a) => (
        <button
          key={a}
          type="button"
          className={asset === a ? "active" : ""}
          onClick={() => onChange(a)}
        >
          {a}
        </button>
      ))}
    </div>
  );
}
