# Market Regime Engine

Decision-oriented market regime detection for crypto (BTC and ETH), mid-term
horizon. The system aggregates signals across five independent risk domains into
a single regime verdict — Risk-On, Risk-Off or Uncertainty — together with
transition probabilities and the main contributing factors.

## Architecture

The system is a set of Go microservices on top of PostgreSQL:

- **Five domain services** compute a `DomainScore` each:
  - `liquidity-service`
  - `leverage-service`
  - `capital-flows-service`
  - `market-stress-service`
  - `volatility-service`
- **`regime-engine`** aggregates the five domain scores into Risk-On /
  Risk-Off / Uncertainty probabilities.
- **`gateway`** exposes a REST API for the web UI and external consumers.
- **`orchestrator`** schedules data ingestion and recomputation.
- **`backtest-runner` / `backtest-metrics`** run replays and sensitivity
  sweeps over historical data.
- **`web/`** is a React + TypeScript dashboard.

Data flows from external providers (exchanges and market-data vendors) into a
bitemporal PostgreSQL schema, then to per-domain scores, the aggregation core,
and finally the API and UI. Inter-service messaging uses a transactional outbox
in PostgreSQL — no external broker required. Every stored row carries the model
and config versions and the ingestion timestamp, which keeps results
reproducible and point-in-time correct (backtests filter on `ingested_at` to
avoid look-ahead).

## Tech stack

- Go 1.25+
- PostgreSQL 15+ (bitemporal schema)
- Docker + Docker Compose
- React + TypeScript (web UI)

## Configuration

Service configuration lives in `configs/*.yaml`. Secrets are provided via
environment variables and are not committed to the repo:

- `COINGLASS_API_KEY` — required by the orchestrator and the leverage,
  capital-flows and market-stress domains.

For Docker, copy `deploy/.env.example` to `deploy/.env` and fill in the values.

## Quickstart (local)

```sh
# Bring up PostgreSQL and run migrations.
make up

# Build all services.
make build-all

# Run a single domain service once (one-shot tick).
./bin/liquidity-service -config configs/liquidity.yaml

# Integration tests (require Docker).
make test-integration
```

## Run the full stack (Docker)

```sh
cp deploy/.env.example deploy/.env   # then set COINGLASS_API_KEY
docker compose -f deploy/docker-compose.yml up --build
```

## Layout

```
cmd/            one main per service (domain services, gateway, orchestrator, ...)
internal/
  api/          REST handlers
  config/       YAML loader + config versioning
  domain/       shared domain types
  storage/      pgx pool + bitemporal repositories
  outbox/       transactional outbox
  providers/    external data-source clients
  regime/       aggregation core
  backtest/     replay and sensitivity tooling
  liquidity/ leverage/ capitalflows/ marketstress/ volatility/   domain logic
configs/        per-service YAML configuration
deploy/         Dockerfile, docker-compose, env template
migrations/     SQL migrations
web/            React + TypeScript dashboard
```
