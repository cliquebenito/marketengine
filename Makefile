.PHONY: tidy build build-all build-orchestrator build-gateway run-gateway vet test test-integration \
        up down logs ps migrate ingest regime backtest backtest-metrics ingest-mcap stack clean \
        demo demo-down demo-logs

GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
COMPOSE := docker compose -f deploy/docker-compose.yml

# ----------------------------------------------------------------------------
# Локальная сборка
# ----------------------------------------------------------------------------

tidy:
	go mod tidy

build:
	CGO_ENABLED=0 go build -ldflags "-X main.GitSHA=$(GIT_SHA)" -o ./bin/liquidity-service ./cmd/liquidity-service

build-all:
	@for svc in liquidity-service leverage-service market-stress-service capital-flows-service \
	            volatility-service regime-engine gateway orchestrator backtest-runner \
	            backtest-metrics ingest-mcap migrate; do \
	  echo ">> building $$svc"; \
	  CGO_ENABLED=0 go build -ldflags "-X main.GitSHA=$(GIT_SHA)" -o ./bin/$$svc ./cmd/$$svc || exit 1; \
	done

build-orchestrator:
	CGO_ENABLED=0 go build -ldflags "-X main.GitSHA=$(GIT_SHA)" -o ./bin/orchestrator ./cmd/orchestrator

build-gateway:
	CGO_ENABLED=0 go build -ldflags "-X main.GitSHA=$(GIT_SHA)" -o ./bin/gateway ./cmd/gateway

run-gateway: build-gateway
	./bin/gateway -config configs/gateway.yaml

vet:
	go vet ./...

test:
	go test ./...

# Интеграционные тесты требуют Docker (testcontainers-go).
test-integration:
	go test -tags=integration ./test/integration/...

# ----------------------------------------------------------------------------
# Docker compose: одно-кнопочный запуск
# ----------------------------------------------------------------------------

# `make stack` — поднять всё с нуля: миграции → postgres+gateway → ingest → regime.
# Использовать после первой checkout/clone, чтобы не вспоминать порядок профилей.
stack: migrate up ingest regime
	@echo
	@echo "✅ Стек запущен. Gateway на http://localhost:8080"

# Поднять core: postgres + gateway (read-path всегда крутится).
up:
	$(COMPOSE) up -d

# Остановить и удалить контейнеры (volumes сохраняются).
down:
	$(COMPOSE) down

# Полный сброс: контейнеры + volumes (включая pgdata!). Осторожно: дропнет БД.
clean:
	$(COMPOSE) down -v

# Логи всех сервисов в follow-режиме.
logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

# Применить goose-миграции (профиль init, one-shot).
migrate:
	$(COMPOSE) --profile init run --rm migrate

# Запустить orchestrator (профиль ingest, one-shot).
# Тянет данные провайдеров за вчерашний день и считает все 5 доменов.
ingest:
	$(COMPOSE) --profile ingest run --rm orchestrator

# Запустить regime-engine: пересчёт агрегации режимов.
# Использует value_date по умолчанию из конфига; для backfill смотри cmd/regime-engine/main.go.
regime:
	$(COMPOSE) --profile regime run --rm regime-engine

# ----------------------------------------------------------------------------
# Backtest: replay/sensitivity sweep
# ----------------------------------------------------------------------------

# `make backtest FROM=2023-01-01 TO=2026-04-25 [MODE=replay|sensitivity] [SWEEP=transition.baseline=0.4,0.5,0.6]`
# Дефолтный mode — replay, дефолтный диапазон — последние 30 дней (вычисляется в самом cmd).
FROM ?=
TO   ?=
MODE ?= replay
SWEEP ?=

backtest:
	@FROM_FLAG=$$( [ -n "$(FROM)" ] && echo "-from $(FROM)" || echo ); \
	 TO_FLAG=$$(   [ -n "$(TO)"   ] && echo "-to $(TO)"     || echo ); \
	 SWEEP_FLAG=$$([ -n "$(SWEEP)" ] && echo "-sweep $(SWEEP)" || echo ); \
	 $(COMPOSE) --profile backtest run --rm backtest-runner \
	   -config /configs/regime-engine.docker.yaml \
	   -mode $(MODE) \
	   $$FROM_FLAG $$TO_FLAG $$SWEEP_FLAG

# `make backtest-metrics RUN=<run_id>` — пересчитать метрики для существующего run.
RUN ?=
backtest-metrics:
	@if [ -z "$(RUN)" ]; then echo "Usage: make backtest-metrics RUN=<run_id>"; exit 1; fi
	$(COMPOSE) --profile backtest run --rm backtest-metrics -run $(RUN)

# Глубокий backfill market-cap (CoinGlass /coin/market-data-history). Один раз.
ingest-mcap:
	$(COMPOSE) --profile ingest-mcap run --rm ingest-mcap

# ----------------------------------------------------------------------------
# `make demo` — одна кнопка: всё в фоне с авто-обновлением.
#
# Поднимает: postgres, gateway, orchestrator-daemon (тикает каждые 4 ч),
# regime-engine-daemon (тикает каждые 24 ч), web (UI на :4173).
# Перед стартом применяет миграции и делает первый прогон ingest+regime,
# чтобы UI открылся уже с данными.
#
# Требует переменную окружения COINGLASS_API_KEY (Startup tier+).
#
# UI:      http://localhost:4173
# API:     http://localhost:8080
# Postgres: localhost:5432  (regime/regime)
# ----------------------------------------------------------------------------
demo: migrate
	$(COMPOSE) --profile ingest run --rm orchestrator
	$(COMPOSE) --profile demo up -d --build
	@echo
	@echo "✅ Demo стенд запущен:"
	@echo "   UI       — http://localhost:4173"
	@echo "   Gateway  — http://localhost:8080"
	@echo "   Логи     — make demo-logs"
	@echo "   Стоп     — make demo-down"

# Логи long-running сервисов в follow-режиме.
demo-logs:
	$(COMPOSE) --profile demo logs -f gateway orchestrator-daemon regime-engine-daemon web

# Остановить demo-стенд (postgres остаётся, чтобы данные не терялись).
demo-down:
	$(COMPOSE) --profile demo down
