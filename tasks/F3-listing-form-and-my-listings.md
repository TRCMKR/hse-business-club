# F3 — Форма публикации и «Мои публикации»

**Зависимости:** F0. **Блокирует:** F6.
**Читать:** `SPEC.md` §2.4, §4.1 (`V1CreateListing`, `V1UpdateListing`, `V1CloseListing`, `V1ListMyListings`), §7; `tasks/reports/F0.md`.

## Что сделать

1. `components/listing-form/`: vee-validate + схемы из `utils/validation.ts`. Сегмент вида
   (создание) / заблокированный бейдж вида (редактирование); поля: заголовок, описание,
   «Кого ищете», теги (`chips-input` ≤ 10), для стартапа — `select` стадии (обязательна),
   для вакансии — условия (≤ 120). Ошибки под полями; серверная 400 — toast + текст под
   формой; `loading` на submit; предупреждение при уходе с несохранённой формой
   (`onBeforeRouteLeave` + confirm).
2. `pages/listings/new.tsx` → после создания `/listings/:id` + toast.
3. `pages/listings/[id]/edit.tsx`: загрузка, предзаполнение; не автор или closed →
   редирект на `/listings/:id` с toast «Редактирование недоступно».
4. `pages/me/listings.tsx` + `components/my-listings/*`: вкладки Активные / Закрытые
   (`tabs`, query `status`), `listing-card` со `stats` и действиями «Отклики», «Закрыть»
   (confirm; после — карточка переезжает во вкладку закрытых). Пусто: «У вас нет
   публикаций» + CTA; ошибка; скелетоны.
5. Тесты: схема формы (стадия обязательна для стартапа и запрещена для вакансии; теги ≤
   10; длины), маппинг формы → запрос (`toCreateRequest`), вкладки.

## Владение

`front/app/pages/listings/new.tsx`, `front/app/pages/listings/[id]/edit.tsx`,
`front/app/pages/me/listings.tsx`, `front/app/components/listing-form/**`,
`front/app/components/my-listings/**`.

## Параллелизация внутри

форма ‖ мои публикации; страницы new/edit — после формы.

## Критерии приёмки

- [ ] Создание стартапа без стадии не отправляется; создание вакансии не показывает стадию.
- [ ] На mock API: создать → открылась страница публикации; редактировать → изменения видны; закрыть из «Мои публикации» → во вкладке «Закрытые».
- [ ] Двойной клик по «Создать» делает один запрос.
- [ ] `npm run lint`, `npm run test` зелёные.

## Отчёт `tasks/reports/F3.md`
