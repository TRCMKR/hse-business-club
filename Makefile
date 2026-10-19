COMPOSE ?= docker compose

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
## back: unit + integration + e2e; front: vitest
test:
	$(MAKE) -C back test
	$(MAKE) -C back test-env-up
	$(MAKE) -C back test-integration test-e2e; rc=$$?; $(MAKE) -C back test-env-down; exit $$rc
	cd front && npm run test

.PHONY: lint
## back: buf lint + golangci-lint; front: eslint + stylelint + tsc
lint:
	$(MAKE) -C back lint
	cd front && npm run lint

.PHONY: smoke
## end-to-end smoke test against the running stack (implemented in I1)
smoke:
	@if [ -x scripts/smoke.sh ]; then scripts/smoke.sh; \
	else echo "scripts/smoke.sh is not implemented yet (I1)"; fi
