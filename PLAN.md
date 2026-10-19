# PLAN — граф задач и порядок запуска агентов

Каждая задача в `tasks/<ID>-*.md` рассчитана на одного агента (который внутри может
раздать подзадачи субагентам — в каждом файле есть раздел «Параллелизация внутри»).
Отчёт агента кладётся в `tasks/reports/<ID>.md`, его читают зависимые задачи.

## Зависимости

```
B0 ──┬── B1 users ────────┐
     ├── B2 listings ─────┤
     ├── B3 applications ─┼── B5 assembly + seed + e2e ──┐
     └── B4 bff ──────────┘                               │
                                                          ├── I1 smoke + QA ── I2 README
F0 ──┬── F1 feed ─────────┐                               │
     ├── F2 details ──────┤                               │
     ├── F3 listing form ─┼── F6 polish ──────────────────┘
     ├── F4 applications ─┤
     └── F5 profile ──────┘
F7 blob-avatar (без зависимостей; F0/F6 подключают) ──────┘
B0 + F0 (Dockerfile'ы) ── I0 compose + observability stack ──────────────────────┘
```

## Волны

| Волна | Параллельно | Условие старта |
|---|---|---|
| 0 | **B0**, **F0**, **F7** | сразу (контракт `back/api/hbc/v1/hbc.proto` готов); F7 — чистая библиотека, ничего не ждёт |
| 1 | **B1 B2 B3 B4**, **F1 F2 F3 F4 F5**, **I0** | B* — после отчёта B0; F* — после отчёта F0; I0 — после обоих |
| 2 | **B5**, затем **F6** | B5 — после B1–B4; F6 — после F1–F5 и B5 (проверка на живом бэке) |
| 3 | **I1** → **I2** | после B5, F6, I0 |

Критический путь: B0 → B4 → B5 → I1 → I2. B4 стартует сразу после B0 на фейках портов,
не дожидаясь B1–B3.

## Задачи

| ID | Файл | Суть | Выход |
|---|---|---|---|
| B0 | `tasks/B0-backend-foundation.md` | модуль, генерация, platform (config/logger/telemetry/db/blob/server/auth/apperr), порты, BFF-скелет с ping, Makefile, Dockerfile | запускаемый бинарник: ping, /metrics, /docs, health, трейсы, S3-стор |
| B1 | `tasks/B1-users-context.md` | контекст users, фото профиля (нормализация + S3) | реализация `users.Service` + миграции + тесты |
| B2 | `tasks/B2-listings-context.md` | контекст listings | реализация `listings.Service` + миграции + тесты |
| B3 | `tasks/B3-applications-context.md` | контекст applications | реализация `applications.Service` + миграции + тесты |
| B4 | `tasks/B4-bff.md` | реализация `hbc.v1` поверх портов на фейках | все 22 RPC + unit-тесты |
| B5 | `tasks/B5-assembly-seed-e2e.md` | сборка, адаптеры, сид, e2e-тесты по HTTP | зелёный `make test-e2e`, swagger сверен |
| F0 | `tasks/F0-frontend-foundation.md` | шаблон, клиент, прокси + cookie-auth, mock API, стор, layout/навигация, UI-кит (incl. `avatar` с blobatar), доменные карточки, login/register | приложение с входом и пустыми страницами-заглушками |
| F1 | `tasks/F1-feed.md` | лента | `/` |
| F2 | `tasks/F2-listing-details.md` | подробности + отклик | `/listings/:id` |
| F3 | `tasks/F3-listing-form-and-my-listings.md` | форма публикации, мои публикации | `/listings/new`, `/listings/:id/edit`, `/me/listings` |
| F4 | `tasks/F4-applications.md` | отклики автора и мои отклики, бейдж | `/listings/:id/applications`, `/me/applications` |
| F5 | `tasks/F5-profile.md` | профиль с загрузкой фото, публичный профиль, 404 | `/me`, `/users/:id`, error page |
| F6 | `tasks/F6-polish.md` | a11y, тёмная тема, адаптив-QA на живом бэке, покрытие | отчёт QA + фиксы |
| F7 | `tasks/F7-blob-avatar.md` | своя библиотека `@fe/blob-avatar`: детерминированный SVG-аватар с лицом из строки, TS + Vue, без зависимостей | пакет `front/lib/blob-avatar` с тестами и галереей |
| I0 | `tasks/I0-compose.md` | docker-compose (db, minio, back, front), env, профиль observability, корневой Makefile | `docker compose up` |
| I1 | `tasks/I1-smoke-and-qa.md` | smoke-скрипт, сквозная ручная проверка, фиксы на стыке | `make smoke` зелёный |
| I2 | `tasks/I2-readme.md` | корневой README, скриншоты, TODO для ручного заполнения | README |

## Как запускать

1. Каждому агенту — свой git worktree и ветка `task/<ID>`:
   `git worktree add ../hbc-<ID> -b task/<ID>`.
   Промпт агенту: «Выполни задачу `tasks/<ID>-….md` в этом репозитории. Перед началом
   прочитай `CLAUDE.md`, `SPEC.md`, `back/ARCHITECTURE.md` (для B*) и отчёты зависимостей в
   `tasks/reports/`. По завершении напиши отчёт в `tasks/reports/<ID>.md`».
2. После отчёта — ревью диффа по критериям приёмки из задачи, merge в `main`
   (`git merge --no-ff task/<ID>`), удалить worktree.
3. В каждой задаче есть раздел **Владение** — какие директории агент может менять. Правки
   вне своей зоны допускаются только с записью в отчёт; это минимизирует конфликты при
   merge параллельных веток.
4. Конфликты по общим файлам (`go.mod`, `package.json`, `labels.ts`) разрешаются при merge
   вручную; задачи спроектированы так, чтобы таких файлов было мало.

## Ручная работа пользователя

- README §«Пример проверки решения ИИ» и §«Потраченное время» — только вручную (ТЗ).
- Учёт времени по каждой волне.
- Решение о деплое (бонус) — отдельная задача по запросу.
