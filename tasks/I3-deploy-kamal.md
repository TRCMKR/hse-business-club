# I3 — Деплой стенда через Kamal (бонус, по запросу пользователя)

**Зависимости:** I2. **Блокирует:** ничего.
**Читать:** `README.md` (§Запуск, §Безопасность), `docker-compose.yaml`, `.env.example`,
`back/Dockerfile`, `front/Dockerfile`, `back/ARCHITECTURE.md` §5 (env), `SPEC.md` §5,
`tasks/DECISIONS.md`. Референс стиля Kamal — `/Users/trcmkr/Проекты/blanc-back/config/deploy.yml`,
`/Users/trcmkr/Проекты/blanc-front/config/deploy.yml`, их `.kamal/secrets` (читать, не копировать креды).

## Целевой стенд (факты, проверены координатором 2026-10-20)

- Сервер `72.4.68.165` (root по SSH-ключу, ключ хоста уже в `known_hosts`), Ubuntu 24.04,
  Docker 29.1, **1 CPU, 961 МБ RAM, диск 9,8 ГБ (свободно ~1,8 ГБ)**. На сервере уже
  работает `kamal-proxy` v0.9.2 (порты 80/443) с приложениями `fs-front`, `trcmkr-front` и
  посторонние контейнеры `amnezia-*` — **ничего чужого не трогать**, прокси общий.
- DNS: wildcard `*.trcmkr.ru → 72.4.68.165`; домен стенда — `hbc.trcmkr.ru` (DNS менять не нужно).
- Registry: Docker Hub, образы `trcmkr/hbc-back` и `trcmkr/hbc-front`; креды — переменные
  `DOCKERHUB_USERNAME`/`DOCKERHUB_PASSWORD` из `~/.zprofile` (в `.kamal/secrets` маппить
  `KAMAL_REGISTRY_USERNAME=$DOCKERHUB_USERNAME` и т. п., значения в репозиторий не попадают).
- `kamal` 2.11 установлен локально; локальный buildx-builder есть (`kamal-local-docker-container`).
- Адрес `77.91.77.13` из старых deploy.yml blanc устарел — не использовать.

## Что сделать

1. **Конфигурация Kamal** (монорепо, два образа → два конфига):
   - `config/deploy.back.yml`: `service: hbc-back`, `image: trcmkr/hbc-back`, `servers.web.hosts: [72.4.68.165]`,
     `options.network-alias: hbc-back`, **без публичного прокси** (`proxy: false` на роли —
     наружу смотрит только фронт, SPEC §5); `builder.arch: amd64`, `builder.dockerfile: back/Dockerfile`,
     `builder.context: back`; env `clear`: `APP_ENV=production`, `HTTP_ADDR=:8080`,
     `DEBUG_ADDR=:8081`, `GRPC_ADDR=127.0.0.1:8082`, `POSTGRES_DSN` на accessory `hbc-db`,
     `S3_ENDPOINT=hbc-minio:9000`, `S3_BUCKET=hbc-avatars`, `CORS_ORIGINS=https://hbc.trcmkr.ru`,
     `SEED_DEMO_DATA=true`, `LOG_LEVEL=info`; env `secret`: `JWT_SECRET`, `S3_ACCESS_KEY`,
     `S3_SECRET_KEY`, `POSTGRES_PASSWORD` (внутри DSN — через `POSTGRES_DSN` как secret целиком,
     если проще); `accessories`: `db` (`postgres:17-alpine`, `directories: data:/var/lib/postgresql/data`,
     `network-alias: hbc-db`, env `POSTGRES_USER/DB=hbc`, пароль из secret, без публикации портов),
     `minio` (`cgr.dev/chainguard/minio@sha256:…` — тот же digest, что в `docker-compose.yaml`,
     `cmd: server /data`, `directories: data:/data`, `network-alias: hbc-minio`, креды из secret,
     без портов наружу). Healthcheck бэка — из Dockerfile (`HEALTHCHECK` уже есть).
   - `config/deploy.front.yml`: `service: hbc-front`, `image: trcmkr/hbc-front`, тот же хост,
     `builder.dockerfile: front/Dockerfile`, `builder.context: front`, `proxy: ssl: true,
     host: hbc.trcmkr.ru, app_port: 3000, healthcheck.path: /login`; env: `NUXT_API_BASE_URL=http://hbc-back:8080`,
     `NUXT_MOCK_API=false`, `NUXT_COOKIE_SECURE=true` (за HTTPS-прокси — см. DECISIONS 01:36).
   - `.kamal/secrets`: только ссылки на переменные окружения (`$DOCKERHUB_USERNAME`,
     `$HBC_JWT_SECRET`, …) — файл безопасен для git. Значения живут в **`.env.deploy`**
     (в `.gitignore`!), шаблон — `.env.deploy.example` с инструкцией генерации
     (`openssl rand -hex 32` для JWT ≥ 32 байт, пароли БД/MinIO).
   - `retain_containers: 2` (диск маленький), `ssh.user: root`.
2. **Makefile** (корень): цели `deploy-setup` (`kamal setup` для back, затем front),
   `deploy` (`kamal deploy` back → front), `deploy-logs`, `deploy-rollback`, `deploy-remove`;
   все грузят `.env.deploy` и `~/.zprofile`-переменные (`set -a; . .env.deploy; set +a`).
   Порядок всегда: back (с accessories) → front.
3. **Локальная проверка без выкладки**: `kamal config -c config/deploy.back.yml` и `-c
   config/deploy.front.yml` валидны; кросс-сборка `docker buildx build --platform linux/amd64`
   обоих образов проходит локально (без push; если сборка фронта под QEMU слишком тяжела для
   диска Docker VM 7,8 ГБ — зафиксируй и предложи `builder.remote` на сервере, но помни про
   1 ГБ RAM там). `kamal server bootstrap`/`setup`/`deploy` **НЕ запускать** без явной команды
   координатора.
4. **README** — раздел «Деплой»: заменить «не делался» на: куда (стенд `https://hbc.trcmkr.ru`),
   что нужно (SSH-доступ root по ключу, Docker Hub, `.env.deploy`), команды `make deploy-setup`
   / `make deploy` / `make deploy-logs` / `make deploy-rollback`, как сбросить данные стенда,
   ограничения (1 CPU / 1 ГБ RAM, общий kamal-proxy, API не публикуется — Swagger недоступен
   снаружи, сид включён). Секции README «Пример проверки решения ИИ» и «Потраченное время» не трогать.
5. **Ресурсы сервера**: оценить память четырёх контейнеров (back ~40 МБ, front ~120 МБ,
   postgres ~60 МБ, minio ~100 МБ) против ~600 МБ доступных; задать `options.memory` лимиты
   в конфигах, чтобы стенд не уронил чужие приложения; `shared_preload_libraries`/`shm` для
   Postgres не нужны. Диск: образы ~700 МБ + тома — с запасом ~1 ГБ, `retain_containers: 2`.

## Владение

`config/deploy*.yml`, `.kamal/**`, `.env.deploy.example`, `.gitignore` (строка для
`.env.deploy`), корневой `Makefile` (новые цели), `README.md` §«Деплой». Код back/front не
менять; если Dockerfile требует правки для прокси (например, `trust proxy` для `Secure`
cookie за kamal-proxy) — минимально и в отчёт.

## Критерии приёмки

- [ ] `kamal config -c …` валидны для обоих конфигов; `.kamal/secrets` без значений; `.env.deploy` в `.gitignore`.
- [ ] Кросс-сборка amd64 обоих образов проходит локально (или задокументирована альтернатива).
- [ ] Makefile-цели работают до шага `kamal …` (dry-run: `make -n deploy`).
- [ ] README §«Деплой» описывает всё, что нужно человеку, чтобы выполнить `make deploy-setup` с нуля.
- [ ] Выкладка на сервер — только после подтверждения пользователя (отдельный шаг, фиксируется в отчёте с результатом: URL, `curl -I https://hbc.trcmkr.ru/login`, логин через фронт).

## Отчёт `tasks/reports/I3.md`

Что настроено, как выкладывать, что проверено локально, оценка ресурсов, что осталось сделать руками.
