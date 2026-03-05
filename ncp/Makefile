.PHONY: up down migrate-up migrate-status run stop wait-api logs test-e2e test-e2e-all logs-summary demo env-copy

RUN_DIR := .run
API_PID := $(RUN_DIR)/api.pid
WORKER_PID := $(RUN_DIR)/worker.pid
API_LOG := $(RUN_DIR)/api.log
WORKER_LOG := $(RUN_DIR)/worker.log

SCENARIOS ?= happy,ttl-delete,ttl-rollback,idempotency
DEMO_SCENARIOS ?= ttl-delete,ttl-rollback

up:
	@docker compose -f deploy/docker-compose.yaml up -d postgres

down: stop
	@docker compose -f deploy/docker-compose.yaml down

migrate-up:
	@go run ./cmd/ncp-migrate up

migrate-status:
	@go run ./cmd/ncp-migrate status

run: migrate-up
	@mkdir -p $(RUN_DIR)
	@if [ -f $(API_PID) ] && kill -0 $$(cat $(API_PID)) 2>/dev/null; then \
		echo "api already running (pid=$$(cat $(API_PID)))"; \
	else \
		nohup go run ./cmd/ncp-api > $(API_LOG) 2>&1 & echo $$! > $(API_PID); \
		echo "api started (pid=$$(cat $(API_PID)))"; \
	fi
	@if [ -f $(WORKER_PID) ] && kill -0 $$(cat $(WORKER_PID)) 2>/dev/null; then \
		echo "worker already running (pid=$$(cat $(WORKER_PID)))"; \
	else \
		nohup go run ./cmd/ncp-worker > $(WORKER_LOG) 2>&1 & echo $$! > $(WORKER_PID); \
		echo "worker started (pid=$$(cat $(WORKER_PID)))"; \
	fi
	@$(MAKE) wait-api

wait-api:
	@API_URL=$${NCP_API_URL:-http://127.0.0.1:8080}; \
	for i in $$(seq 1 60); do \
		if curl -fsS "$$API_URL/health" >/dev/null 2>&1; then \
			echo "api ready: $$API_URL"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "api is not ready"; \
	exit 1

stop:
	@if [ -f $(API_PID) ] && kill -0 $$(cat $(API_PID)) 2>/dev/null; then \
		kill $$(cat $(API_PID)); \
		echo "api stopped"; \
	fi
	@if [ -f $(WORKER_PID) ] && kill -0 $$(cat $(WORKER_PID)) 2>/dev/null; then \
		kill $$(cat $(WORKER_PID)); \
		echo "worker stopped"; \
	fi
	@rm -f $(API_PID) $(WORKER_PID)

logs:
	@touch $(API_LOG) $(WORKER_LOG)
	@tail -n 200 -f $(API_LOG) $(WORKER_LOG)

test-e2e:
	@go run ./cmd/ncp-e2e run -scenarios="$(SCENARIOS)"

test-e2e-all:
	@$(MAKE) test-e2e SCENARIOS="happy,ttl-delete,ttl-rollback,idempotency"

logs-summary:
	@go run ./cmd/ncp-e2e summary -limit=40

demo: up run
	@$(MAKE) test-e2e SCENARIOS="$(DEMO_SCENARIOS)"
	@$(MAKE) logs-summary
	@API_URL=$${NCP_API_URL:-http://127.0.0.1:8080}; \
	echo "demo finished"; \
	echo "api: $$API_URL"; \
	echo "openapi: $$API_URL/openapi.yaml"; \
	echo "health: $$API_URL/health"; \
	echo "secure endpoints require Authorization: Bearer $${NCP_DEV_TOKEN:-devtoken}"

env-copy:
	@if [ -f .env ]; then \
		echo ".env already exists"; \
	else \
		cp .env.example .env; \
		echo "created .env from .env.example"; \
	fi
