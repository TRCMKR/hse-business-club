# F4 — Отклики

**Зависимости:** F0. **Блокирует:** F6.
**Читать:** `SPEC.md` §2.2–2.3, §4.1 (методы applications), §7; `tasks/reports/F0.md`.

## Что сделать

1. `pages/listings/[id]/applications.tsx` (автор): список `application-card` в режиме
   `incoming`, фильтр статуса чипами (query `status`), «Принять» (без confirm) / «Отклонить»
   (confirm) для pending; после действия — refresh; идемпотентный повтор без ошибки;
   400 «Решение уже принято» → toast + refresh. Не автор (403) → редирект на
   `/listings/:id` с toast. Пусто: «Откликов пока нет».
2. `pages/me/applications.tsx`: `tabs` «Отправленные» / «Полученные» (query `tab`), фильтр
   статуса. Отправленные — `application-card` `outgoing` (публикация, автор, мой текст,
   статус; «Отозвать» с confirm для pending; клик по публикации → детали). Полученные —
   `incoming` с «Принять»/«Отклонить» и ссылкой на публикацию. Пустые состояния для
   каждой вкладки.
3. `composables/useApplicationsBadge.ts` — заменить заглушку F0: число pending в
   полученных (`received?status=PENDING`), обновляется после входа и после действий
   (простой `refresh` через общий `useState`), не дёргается чаще раза в 30с.
4. Тесты: `useApplicationsBadge` (инвалидация), действия карточки (блокировка повторного
   клика, обработка 400/404 → toast + refresh), фильтры.

## Владение

`front/app/pages/listings/[id]/applications.tsx`, `front/app/pages/me/applications.tsx`,
`front/app/components/applications/**`, `front/app/composables/useApplicationsBadge.ts`.

## Параллелизация внутри

страница публикации ‖ страница «мои отклики» ‖ бейдж.

## Критерии приёмки

- [ ] На mock API от Анны: бейдж «1», в «Полученных» pending от Бориса; принять → бейдж «0», статус «Принят»; от Бориса в «Отправленных» — «Принят».
- [ ] От Веры: отозванный отклик виден в «Отправленных» со статусом «Отозван» без кнопок.
- [ ] `npm run lint`, `npm run test` зелёные.

## Отчёт `tasks/reports/F4.md`
