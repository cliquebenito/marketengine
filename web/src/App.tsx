import { useState } from "react";
import { BacktestDashboard } from "./components/BacktestDashboard";
import { RegimeDashboard } from "./components/RegimeDashboard";
import type { Asset } from "./types";

type Tab = "regime" | "backtest";

function parseHash(): { tab: Tab; asset: Asset } {
  const [t, a] = window.location.hash.replace(/^#/, "").split("/");
  return {
    tab: t === "backtest" ? "backtest" : "regime",
    asset: a === "ETH" ? "ETH" : "BTC",
  };
}

export default function App() {
  const init = parseHash();
  const [asset, setAssetState] = useState<Asset>(init.asset);
  const [tab, setTab] = useState<Tab>(init.tab);
  const syncHash = (t: Tab, a: Asset) => {
    window.location.hash = `${t}/${a}`;
  };
  const selectTab = (t: Tab) => {
    setTab(t);
    syncHash(t, asset);
  };
  const setAsset = (a: Asset) => {
    setAssetState(a);
    syncHash(tab, a);
  };

  return (
    <div style={{ minHeight: "100vh", background: "#f5f7fa", color: "#1a2029" }}>
      <nav
        style={{
          display: "flex",
          gap: 4,
          padding: "8px 24px",
          borderBottom: "1px solid #d7dee7",
          background: "#ffffff",
          alignItems: "center",
        }}
      >
        <span style={{ fontWeight: 700, marginRight: 16, fontSize: 14 }}>
          Market Regime Engine
        </span>
        <TabButton active={tab === "regime"} onClick={() => selectTab("regime")}>
          Режим рынка
        </TabButton>
        <TabButton
          active={tab === "backtest"}
          onClick={() => selectTab("backtest")}
        >
          Бэктест
        </TabButton>
      </nav>
      {tab === "regime" ? (
        <RegimeDashboard asset={asset} onAssetChange={setAsset} />
      ) : (
        <BacktestDashboard asset={asset} onAssetChange={setAsset} />
      )}
    </div>
  );
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      style={{
        background: active ? "#e7ecf3" : "transparent",
        color: active ? "#1a2029" : "#6b7280",
        border: "1px solid",
        borderColor: active ? "#3b82f6" : "transparent",
        borderRadius: 6,
        padding: "6px 14px",
        fontSize: 13,
        fontWeight: 500,
        cursor: "pointer",
      }}
    >
      {children}
    </button>
  );
}

