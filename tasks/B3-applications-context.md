# B3 — Контекст applications

**Зависимости:** B0. **Блокирует:** B5.
**Читать:** `SPEC.md` §2.2–2.4, §3 (таблица applications), §4.1 (сортировки); `back/ARCHITECTURE.md` §1–2
(особенно правило 3 о денормализации); `tasks/reports/B0.md`; `back/internal/applications/applications.go`.

## Что сделать

1. `internal/applications/domain`: валидация message (1–2000), машина состояний:
   `Transition(from Status, action) (to Status, noop bool, err)` для accept/decline/withdraw
   по SPEC §2.2–2.3 (повтор того же перехода — `noop=true`; из финального в другой финальный
   — `KindPrecondition` «Решение по отклику уже принято»). Табличные тесты на все 4×3 пары.
2. `internal/applications/postgres`: миграция `migrations/00001_applications.sql` (схема
   `applications`, таблица по SPEC §3 **плюс** колонка `listing_author_id uuid NOT NULL` и
   индекс `(listing_author_id, status, created_at DESC)`; partial unique index как в SPEC).
   Репозиторий: `Create` (`23505` → `KindConflict` «Вы уже откликались на эту публикацию»),
   `GetByID`, `GetForUpdate(tx, id)`, `UpdateStatus(tx, id, status, decidedAt)`,
   `ListByListing`, `ListByApplicant`, `ListByListingAuthor`, `GetMineForListings`
   (`status <> 'withdrawn' AND listing_id = ANY($2)`), `StatsForListings` (`GROUP BY
   listing_id, status` → агрегировать в Go, отсутствующие — нулевые).
3. `internal/applications/service`: `New(repo, listingReader)`. `Apply`: `ListingReader.
   GetListing` → свои/закрытые → `KindPrecondition`; создаёт с `ListingAuthorID` из
   `ListingInfo`. `Accept`/`Decline`: в транзакции `GetForUpdate`, проверка
   `actorID == ListingAuthorID` (иначе `KindNotFound` — не раскрываем), переход через
   domain, при `noop` — вернуть как есть без записи. `Withdraw`: аналогично с
   `ApplicantID`. `Get`: автор публикации или откликнувшийся, иначе `KindNotFound`.
   `ListByListing`: право автора проверяй через `ListingReader.GetListing` (а не по
   `listing_author_id` откликов — при нуле откликов проверить нечего); чужому —
   `KindForbidden` «Недостаточно прав».
4. Тесты: domain unit (полная таблица переходов); repo integration (`TEST_POSTGRES_DSN`):
   unique partial index — второй pending → conflict, после withdrawn — можно; сортировки
   (pending первыми); `GetMineForListings` игнорирует withdrawn; `StatsForListings` с нулями;
   service unit с фейковыми repo и `ListingReader`: все правила из SPEC §2.3.

## Выходы

Пакеты `domain`, `postgres`, `service` в `internal/applications`; миграция; тесты.

## Владение

`back/internal/applications/**` (кроме `applications.go` — только с отчётом).

## Параллелизация внутри

domain ‖ postgres → service.

## Критерии приёмки

- [ ] Unit и integration тесты зелёные; `make lint` чисто.
- [ ] Гонка двух `Apply` одного пользователя (два горутины на реальной БД) даёт ровно одну запись и один `KindConflict`.
- [ ] Никаких импортов `internal/listings/...` — только `applications.ListingReader`.

## Отчёт `tasks/reports/B3.md`

Конструкторы, решение по денормализации, отклонения.
