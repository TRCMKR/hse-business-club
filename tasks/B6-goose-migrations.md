# B6 — Миграции через goose CLI из `back/migrations/`

**Зависимости:** B5, I1 (всё в main). **Блокирует:** финальную выкладку I3 (образ бэка).
**Читать:** `tasks/DECISIONS.md` (05:30), `back/ARCHITECTURE.md` §1 п.2, §4, §6 (уже обновлены),
`SPEC.md` §3, `back/internal/platform/db/{migrator.go,dbtest}`, `back/internal/*/postgres/migrations/*.sql`,
`back/internal/app/app.go`, `back/Makefile`, `back/Dockerfile`, `back/tests/e2e/harness_test.go`,
`back/README.md`. Референс соглашения: `/Users/trcmkr/Проекты/bot-aller/{migrations,Makefile}`,
`/Users/trcmkr/Проекты/matcher/matcher-back/{migrations,Makefile}`.

## Что сделать

1. `back/migrations/`: перенести три миграции контекстов в одну папку с timestamp-именами
   (`20261020000001_users.sql`, `…0002_listings.sql`, `…0003_applications.sql` — или через
   `goose create`), в каждой — `CREATE SCHEMA IF NOT EXISTS <ctx>;` + объекты с
   полностью квалифицированными именами; `-- +goose Down` с `DROP SCHEMA … CASCADE`.
   Порядок users → listings → applications сохранить номерами.
2. Удалить `platform/db.Migrator`, `Migrator`-переменные и `embed` в `internal/*/postgres`,
   `app.Migrators()` и вызов миграций из `app.New` (приложение стартует, считая схему
   готовой; `/readyz` — только `pool.Ping` + `blob.Ping`). Советующий лок/проверка пустоты в
   сиде — без изменений.
3. `platform/db/dbtest`: `Migrate(ctx, pool)` → goose как библиотека (`goose.SetBaseFS(nil)`,
   `goose.Up(db, "<abs path to back/migrations>")`, путь вычислять от `runtime.Caller`/модуля,
   не от cwd), `Reset` → `DROP SCHEMA users, listings, applications CASCADE` + `DROP TABLE
   public.goose_db_version`. Integration-тесты контекстов и e2e-harness переключить на него.
4. `back/Makefile`: по соглашению других проектов — `MIGRATION_FOLDER=$(CURDIR)/migrations`,
   цели `migration-create` (`goose -dir … create $(name) sql`), `migration-up`, `migration-down`,
   `migration-status` с DSN из `POSTGRES_DSN` (дефолт — тестовый compose). goose CLI —
   через `go run github.com/pressly/goose/v3/cmd/goose@<версия из go.mod>` или `go tool`,
   чтобы не требовать глобальной установки.
5. `back/Dockerfile`: собрать `goose` в builder-стадии (`go build` `github.com/pressly/goose/v3/cmd/goose`
   той же версии), скопировать бинарь и `migrations/` в runtime-образ; `entrypoint.sh`:
   `goose -dir /app/migrations postgres "$POSTGRES_DSN" up` (с повтором ожидания БД до ~30 с)
   → `exec hbc`. Образ остаётся non-root, alpine, компактным (<50 МБ).
6. `docker-compose.yaml` — без изменений (entrypoint делает всё сам); проверить `make reset
   && make up` и `make smoke` через override портов. `back/README.md`: раздел «Миграции»
   (как создать, применить, откатить; как это работает в контейнере). Корневой README:
   упоминания «миграции применяются при старте» поправить на новое поведение.
7. Тесты: `make lint`, `make test`, `make test-all` зелёные; e2e-сценарий сида и повторного
   старта проходит; integration-тесты не оставляют мусора между пакетами (одна БД, `Reset`).

## Владение

`back/**` (кроме `back/api/**`), корневой `README.md` (только абзацы про миграции),
`docker-compose*.yaml` при необходимости. Контракт proto и порты не менять.

## Критерии приёмки

- [ ] В коде нет `Migrator`, `embed` миграций и вызова goose из `app.New`; `platform/db` — только pool/WithTx/dbtest.
- [ ] `back/migrations/` — единственный источник схемы; `make migration-status` показывает три применённые.
- [ ] `docker build ./back` → контейнер на пустой БД применяет миграции и стартует; повторный старт — `no migrations to run`.
- [ ] `make lint`, `make test`, `make test-all` зелёные с нуля; `make reset && make up && make smoke` (через override портов) зелёный.

## Отчёт `tasks/reports/B6.md`

Что перенесено, как работает entrypoint, команды, правки в чужих зонах.
