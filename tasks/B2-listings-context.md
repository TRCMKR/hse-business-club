# B2 — Контекст listings

**Зависимости:** B0. **Блокирует:** B5.
**Читать:** `SPEC.md` §2.2–2.4, §3 (таблица listings), §4.1 (сортировки); `back/ARCHITECTURE.md` §1–2;
`tasks/reports/B0.md`; `back/internal/listings/listings.go`.

## Что сделать

1. `internal/listings/domain`: валидация `CreateParams`/`UpdateParams` по SPEC §2.4 (kind
   обязателен; stage только для startup, compensation только для vacancy; tags trim/dedupe
   ≤ 10), валидация `ListFilter` (page ≥ 1, limit 1..50 с дефолтами 1/20, query ≤ 100),
   правила переходов (`CanEdit`: только active; `Close`: идемпотентно), ошибки `apperr`
   с русскими сообщениями. Табличные тесты.
2. `internal/listings/postgres`: миграция `migrations/00001_listings.sql` (схема `listings`,
   таблица и индексы по SPEC §3; `author_id uuid NOT NULL` без FK), репозиторий: `Create`,
   `GetByID`, `GetMany`, `ListActive(filter)` → items + total (один запрос с
   `COUNT(*) OVER()` либо два — на выбор, но без N+1), поиск `ILIKE '%' || $q || '%'` по
   title и description с экранированием `%`/`_`; `ListByAuthor(authorID, status)`; `Update`;
   `Close` (`UPDATE ... SET status='closed', closed_at=now() WHERE id=$1 AND status='active'`).
3. `internal/listings/service`: реализация `listings.Service`; `Update`/`Close` проверяют
   `actorID == AuthorID` (иначе `KindForbidden` «Недостаточно прав»), `Update` на closed →
   `KindPrecondition` «Публикация закрыта»; `Close` на closed возвращает текущее состояние.
4. Тесты: domain unit; repo integration (`TEST_POSTGRES_DSN`): фильтры kind/query,
   пагинация и total, сортировка по `created_at DESC`, closed не в ленте;
   service unit с фейковым репо: права, переходы, идемпотентность.

## Выходы

Пакеты `domain`, `postgres`, `service` в `internal/listings`; миграция; тесты.

## Владение

`back/internal/listings/**` (кроме `listings.go` — только с отчётом).

## Параллелизация внутри

domain ‖ postgres → service.

## Критерии приёмки

- [ ] Unit и integration тесты зелёные; `make lint` чисто.
- [ ] `ListActive` с `limit=50`, `page=1000` возвращает пустой `Items` и корректный `Total`.
- [ ] Поиск по `%` и `_` не ломается и не матчит всё подряд.

## Отчёт `tasks/reports/B2.md`

Конструкторы, SQL-особенности, отклонения.
