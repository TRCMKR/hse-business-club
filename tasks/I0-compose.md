# I0 — Docker Compose и окружение

**Зависимости:** B0 и F0 (Dockerfile'ы и env-контракты). **Блокирует:** I1.
**Читать:** `back/ARCHITECTURE.md` §4–5, `back/README.md`, `front/README.md`, отчёты B0, F0.

## Что сделать

1. `docker-compose.yaml` в корне:
   - `db`: `postgres:17-alpine`, `hbc/hbc/hbc`, volume `db_data`, healthcheck
     `pg_isready -U hbc -d hbc` (5s × 10), порт наружу не публикуется;
   - `minio`: `cgr.dev/chainguard/minio` (образ `minio/minio` больше не публикуется; тот же,
     что в `back/docker-compose.test.yaml` из B0 — закрепить digest или тег), `server /data
     --console-address :9001`, `MINIO_ROOT_USER/PASSWORD` из `${S3_ACCESS_KEY:-minioadmin}`
     / `${S3_SECRET_KEY:-minioadmin}`, volume `minio_data`, healthcheck как в
     `back/docker-compose.test.yaml` (в образе chainguard нет `mc`; 5s × 10), порты наружу не публикуются (консоль 9001 — только в профиле
     `observability`);
   - `back`: build `./back`; env по ARCHITECTURE §5 (`POSTGRES_DSN` на `db`, `S3_ENDPOINT=minio:9000`,
     `S3_ACCESS_KEY`/`S3_SECRET_KEY` те же, `S3_BUCKET=hbc-avatars`,
     `JWT_SECRET=${JWT_SECRET:-dev-only-secret-change-me-0123456789abcdef}`,
     `SEED_DEMO_DATA=${SEED_DEMO_DATA:-true}`, `CORS_ORIGINS=http://localhost:3000`,
     `APP_ENV=production`, `OTEL_EXPORTER_OTLP_ENDPOINT=${OTEL_EXPORTER_OTLP_ENDPOINT:-}`);
     `depends_on: db, minio: service_healthy`; порты `8080:8080` (API) и `8081:8081` (docs,
     metrics — с пометкой в README, что в проде не публикуется); healthcheck `/readyz`;
   - `front`: build `./front`; `NUXT_API_BASE_URL=http://back:8080`; `depends_on: back:
     service_healthy`; порты `3000:8080`.
   - профиль `observability`: `jaeger` (`jaegertracing/all-in-one:1.60`, OTLP 4317, UI
     16686), `prometheus` (`prom/prometheus`, конфиг `deploy/prometheus.yml` со scrape
     `back:8081/metrics`, UI 9090). При профиле `back` получает
     `OTEL_EXPORTER_OTLP_ENDPOINT=jaeger:4317`.
2. `docker-compose.test.yaml` (override): публикует `db` на `5433` и `minio` на `9000`
   для `make test-integration`/`test-e2e` бэка; заменить `back/docker-compose.test.yaml` из B0/B5 на использование корневого
   (`docker compose -f ../docker-compose.yaml -f ../docker-compose.test.yaml up -d db`),
   удалить старый файл, обновить `back/Makefile`.
3. `.env.example` в корне: `JWT_SECRET`, `SEED_DEMO_DATA`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`,
   `OTEL_EXPORTER_OTLP_ENDPOINT`.
4. Корневой `Makefile`: `up` (`--build`), `down`, `reset` (`down -v`), `logs`, `obs`
   (`--profile observability up`), `test` (бэк unit+integration+e2e, фронт vitest),
   `lint` (оба), `smoke` (заглушка до I1).
5. Проверка с нуля: `make reset && make up` → ≤ 90с фронт на `:3000` отвечает, `:8081/docs/`
   открыт, `:8081/metrics` есть, `:8080/readyz` → `{"db":"ok","blob":"ok"}`; с `make obs`
   спаны видны в Jaeger `:16686`, консоль MinIO на `:9001`.

## Владение

`docker-compose*.yaml`, `.env.example`, `Makefile`, `deploy/**` в корне; `back/Makefile`
(только цели миграций/тестов), `back/.dockerignore`, `front/.dockerignore`.

## Критерии приёмки

- [ ] `docker compose config` валиден; `make up` с чистого состояния без `.env` работает.
- [ ] Образы собираются без SSH/приватных модулей; в образах нет `.env`.
- [ ] В отчёте — время холодного старта и размеры образов.

## Отчёт `tasks/reports/I0.md`

## Примечания координатора (2026-10-20)

- Порты 8080–8082 на машине разработчика сейчас заняты посторонним процессом (`go run ./cmd`
  из blanc-back). Если `make up` падает на публикации портов — сообщить в резюме, не менять
  порты в compose (контракт README: фронт `:3000`, API `:8080`, debug `:8081`).
- Диск Docker VM — 7,8 ГБ, свободно мало. Образы держать компактными (alpine/distroless),
  не тянуть лишнего; при ENOSPC — сообщить в резюме, не делать `docker system prune -a`.
- Фронт: cookie `Secure` отключается через `NUXT_COOKIE_SECURE=false` (см. отчёт F0) — в
  compose фронт открывается по `http://localhost:3000`, задать соответствующе.
