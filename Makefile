# COMPOSE_FILES: extra compose files, e.g. a local override of the ports when 8080/3000
# are busy: make up smoke COMPOSE_FILES=ports.override.yaml API_URL=http://localhost:18080
# FRONT_URL=http://localhost:13000. docker compose also honours COMPOSE_FILE and
# COMPOSE_PROJECT_NAME from the environment.
COMPOSE_FILES ?=
COMPOSE ?= docker compose $(if $(COMPOSE_FILES),-f docker-compose.yaml $(addprefix -f ,$(COMPOSE_FILES)))
API_URL ?= http://localhost:8080
FRONT_URL ?= http://localhost:3000

.PHONY: help
## prints help about all targets
help:
	@echo "Usage: make <target>"
	@echo ""
	@awk '/^## /{c=substr($$0,4);next} /^[a-zA-Z0-9_-]+:/{t=$$1;sub(":","",t);if(c!=""){printf "  %-10s %s\n",t,c;c=""}}' $(MAKEFILE_LIST)

.PHONY: up
## builds images and starts db, minio, back, front (front :3000, API :8080, debug :8081)
up:
	$(COMPOSE) up -d --build --wait

.PHONY: obs
## same as up plus Jaeger :16686, Prometheus :9090, MinIO console :9001; back exports spans
obs:
	OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4317 $(COMPOSE) --profile observability up -d --build --wait

.PHONY: down
## stops and removes containers (volumes are kept)
down:
	$(COMPOSE) --profile observability down

.PHONY: reset
## stops containers and removes volumes (database and uploaded photos)
reset:
	$(COMPOSE) --profile observability down -v

.PHONY: logs
## follows logs of all services
logs:
	$(COMPOSE) --profile observability logs -f --tail=100

.PHONY: test
## back: unit + integration + e2e (starts and stops its own test env); front: vitest
test:
	$(MAKE) -C back test-all
	cd front && npm run test

.PHONY: lint
## back: buf lint + golangci-lint; front: eslint + stylelint + tsc
lint:
	$(MAKE) -C back lint
	cd front && npm run lint

.PHONY: smoke
## smoke test of the running stack: main scenario, photo via MinIO, login via the front (API_URL, FRONT_URL)
smoke:
	@API_URL="$(API_URL)" FRONT_URL="$(FRONT_URL)" scripts/smoke.sh
