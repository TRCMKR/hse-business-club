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
	@awk '/^## /{c=substr($$0,4);next} /^[a-zA-Z0-9_-]+:/{t=$$1;sub(":","",t);if(c!=""){printf "  %-16s %s\n",t,c;c=""}}' $(MAKEFILE_LIST)

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

# ---------- Deploy: Kamal 2, stand https://hbc.trcmkr.ru (README §Деплой) ----------
# Two Kamal configs (monorepo, two images); the order is always back (with accessories
# db + minio) → front, and the reverse for removal.
# Secrets: .env.deploy (gitignored, template .env.deploy.example) is exported into the
# environment; .kamal/secrets maps the variables to the containers. Docker Hub credentials
# come from DOCKERHUB_USERNAME / DOCKERHUB_PASSWORD: already exported (e.g. a login zsh
# that sourced ~/.zprofile), set in .env.deploy, or read from ~/.zprofile with zsh as a
# fallback. Only those two variables are taken from the profile, so it never has to be
# sourced by /bin/sh.
KAMAL ?= kamal
DEPLOY_ENV_FILE ?= .env.deploy
# Thor parses -c only after the (sub)command: `kamal deploy -c …`, not `kamal -c … deploy`.
BACK_CFG = -c config/deploy.back.yml
FRONT_CFG = -c config/deploy.front.yml
# deploy-logs / deploy-rollback: which service (back|front); deploy-rollback: VERSION=<git sha>
SERVICE ?= front
VERSION ?=

DEPLOY_ENV = set -e; \
	if [ ! -f "$(DEPLOY_ENV_FILE)" ]; then echo "$(DEPLOY_ENV_FILE) not found: cp .env.deploy.example $(DEPLOY_ENV_FILE) and fill it in" >&2; exit 1; fi; \
	set -a; . "$(abspath $(DEPLOY_ENV_FILE))"; set +a; \
	if [ -z "$${DOCKERHUB_USERNAME:-}" ] && [ -r "$$HOME/.zprofile" ] && command -v zsh >/dev/null 2>&1; then \
		DOCKERHUB_USERNAME="$$(zsh -c '. "$$HOME/.zprofile" >/dev/null 2>&1; print -r -- "$$DOCKERHUB_USERNAME"')"; \
		DOCKERHUB_PASSWORD="$$(zsh -c '. "$$HOME/.zprofile" >/dev/null 2>&1; print -r -- "$$DOCKERHUB_PASSWORD"')"; \
		export DOCKERHUB_USERNAME DOCKERHUB_PASSWORD; \
	fi; \
	if [ -z "$${DOCKERHUB_USERNAME:-}" ] || [ -z "$${DOCKERHUB_PASSWORD:-}" ]; then echo "DOCKERHUB_USERNAME / DOCKERHUB_PASSWORD are not set" >&2; exit 1; fi; \
	for v in HBC_JWT_SECRET HBC_POSTGRES_PASSWORD HBC_S3_ACCESS_KEY HBC_S3_SECRET_KEY; do \
		if [ -z "$$(printenv "$$v")" ]; then echo "$$v is empty in $(DEPLOY_ENV_FILE)" >&2; exit 1; fi; \
	done; \
	if [ "$$(printf %s "$$HBC_JWT_SECRET" | wc -c)" -lt 32 ]; then echo "HBC_JWT_SECRET must be at least 32 characters" >&2; exit 1; fi;

.PHONY: deploy-config
## checks .env.deploy (all HBC_* set, JWT >= 32 chars) and that both Kamal configs parse; no server access
deploy-config:
	@$(DEPLOY_ENV) $(KAMAL) config $(BACK_CFG) >/dev/null && $(KAMAL) config $(FRONT_CFG) >/dev/null && echo "kamal config: ok"

.PHONY: deploy-setup
## first deploy: back + db + minio (kamal setup), then front; needs .env.deploy
deploy-setup:
	@$(DEPLOY_ENV) $(KAMAL) setup $(BACK_CFG) && $(KAMAL) setup $(FRONT_CFG)

.PHONY: deploy
## builds amd64 images, pushes them to Docker Hub and deploys back, then front
deploy:
	@$(DEPLOY_ENV) $(KAMAL) deploy $(BACK_CFG) && $(KAMAL) deploy $(FRONT_CFG)

.PHONY: deploy-logs
## follows logs of the deployed SERVICE=front|back (default front)
deploy-logs:
	@$(DEPLOY_ENV) $(KAMAL) app logs -f -c config/deploy.$(SERVICE).yml

.PHONY: deploy-rollback
## SERVICE=front|back VERSION=<sha>: rolls back to a retained version (no VERSION: lists them)
deploy-rollback:
	@$(DEPLOY_ENV) if [ -z "$(VERSION)" ]; then \
		$(KAMAL) app containers -c config/deploy.$(SERVICE).yml; \
		echo "pick a version and run: make deploy-rollback SERVICE=$(SERVICE) VERSION=<version>" >&2; exit 1; \
	fi; \
	$(KAMAL) rollback "$(VERSION)" -c config/deploy.$(SERVICE).yml

.PHONY: deploy-reset
## wipes stand data (db + photos) and reseeds demo data; the app images stay
deploy-reset:
	@$(DEPLOY_ENV) $(KAMAL) accessory remove all -y $(BACK_CFG) && $(KAMAL) accessory boot all $(BACK_CFG) && $(KAMAL) app boot $(BACK_CFG)

.PHONY: deploy-remove
## removes the stand: front, back, db + minio with their data (kamal-proxy stays)
deploy-remove:
	@$(DEPLOY_ENV) $(KAMAL) app remove -y $(FRONT_CFG) && $(KAMAL) app remove -y $(BACK_CFG) && $(KAMAL) accessory remove all -y $(BACK_CFG)
