# Market Regime Dashboard

React + Vite SPA over the Gateway API (`cmd/gateway`).

## Dev
Prereq: Go gateway running at `:8080` (or set `VITE_API_BASE_URL`).

```
npm install
npm run dev
```

Open http://localhost:5173

## Build

```
npm run build   # type-check + bundle to ./dist
npm run type-check
```

## Notes
- The backend must be running — the SPA is pure client-side and has no proxy.
- Asset selector supports BTC / ETH. Date range defaults to last 365 days for the
  history chart and last 180 days for per-domain sparklines.
- Domain display labels are resolved via the backend's `domain_display_names`
  map; nothing is hardcoded in React.
- `deploy/` docker-compose integration is intentionally not touched here.
