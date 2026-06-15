# deploy/

Docker-compose stack for the marketengine Go services.

## One-time setup

```sh
cp deploy/.env.example deploy/.env
# edit deploy/.env and set COINGLASS_API_KEY=<real value>
```

## Bring up the read path (postgres + gateway)

```sh
docker compose -f deploy/docker-compose.yml up -d postgres gateway
```

Gateway listens on host port `8080` (inside container: `0.0.0.0:8080`).

## Apply migrations (profile `init`)

```sh
docker compose -f deploy/docker-compose.yml --profile init run --rm migrate
```

Forward-only goose migrations from `migrations/` against the compose postgres.

## Trigger the daily orchestrator manually (profile `manual`)

```sh
docker compose -f deploy/docker-compose.yml --profile manual run --rm orchestrator
```

One-shot: runs all 6 domain services + regime engine sequentially for
`value_date = yesterday UTC` (main.go default). In production the host cron
or k8s CronJob invokes this — it is NOT started by `docker compose up` by
design (profile gate).

## Config file layout

- `configs/*.yaml` — for running binaries on the host (points at
  `localhost:5432`).
- `configs/*.docker.yaml` — container-world copies (hostname swapped to the
  compose service name `postgres`). These are mounted read-only at
  `/configs` in every service container.

Keep both in sync when tweaking params.
