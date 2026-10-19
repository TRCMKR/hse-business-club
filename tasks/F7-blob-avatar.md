# F7 — Генератор аватаров `@fe/blob-avatar`

**Зависимости:** нет (не нужен ни бэкенд, ни Nuxt-проект). **Блокирует:** компонент `avatar`
в F0 (если F7 ещё не смержен, F0 ставит заглушку — см. F0 §7).
**Читать:** `SPEC.md` §2.1 (абзац про аватар), §7 (абзац «Аватары»);
`/Users/trcmkr/Проекты/blanc-front/lib/prop-types` (как устроена локальная библиотека `@fe/*`);
`/Users/trcmkr/Проекты/blanc-front/app/components/ping` (стиль TSX-компонентов).
**Референс идеи (не копировать код):** https://github.com/Alain00/blobatar — детерминированные
«существа» из строки: силуэт, цвет, глаза, выражение.

## Цель

Своя маленькая библиотека без зависимостей: из любой строки (у нас — `username`)
детерминированно рисует SVG-«блоба» с лицом. Чистый TypeScript + Vue-компонент. Один и тот
же seed → байт в байт одинаковый SVG на сервере и в браузере.

## Что сделать

1. **Пакет** `front/lib/blob-avatar/`: `package.json` (`name: "@fe/blob-avatar"`, `type:
   module`, `exports`: `"."` → core, `"./vue"` → компонент; `peerDependencies: vue 3.x`;
   `devDependencies`: typescript, vite, vitest, vue), `tsconfig.json` (strict), `vite.config.ts`
   (lib mode, два entry: `src/index.ts`, `src/vue.ts`; `dist/` коммитится — как в
   `@fe/prop-types`), `README.md` (API, примеры, картинки из галереи).
2. **Детерминизм** `src/random.ts`: `hashString(seed): number` (FNV-1a 32-bit или cyrb53) →
   `createRng(hash)` (mulberry32) → `next(): number` в `[0, 1)`, `pick<T>(arr)`, `range(min,
   max)`. Никакого `Math.random`. Seed нормализуется: `trim().toLowerCase()`.
3. **Трейты** `src/traits.ts`: тип `Traits` и `deriveTraits(seed, overrides?)`. Каждая ось —
   число `0..1` из rng, затем квантуется в вариант. Оси и наборы:
   - `shape`: `round | squircle | blob | droplet | hexagon | cloud | capsule | triangle`
     (`blob` — замкнутая гладкая кривая через 6–9 точек с джиттером радиуса, кубические
     Безье по Catmull-Rom; остальные — фиксированные path/`rx`);
   - `hue`: 8 ступеней (`12, 30, 50, 140, 175, 210, 260, 320`), `tone`: `light | dark`
     (тело и фон — HSL от hue/tone; контраст текста не нужен, но глаза должны читаться на
     любом тоне);
   - `eyes`: `size`, `spacing`, `tilt`, `pupilShift` (непрерывные), `eyeShape`: `dot | oval |
     wide | wink`;
   - `expression`: `happy | neutral | surprised | smirk | sleepy | mad | thinking | cheeky |
     sad | shy` (рот и брови);
   - `extras`: `blush | freckles | none`.
   `overrides` позволяет закрепить любую ось (`{ expression: 'happy', hue: 210 }`) — для
   тестов и галереи.
4. **Отрисовка** `src/render.ts`: сначала дерево `SvgNode = { tag, attrs, children }`
   (`buildTree(traits, options): SvgNode`), затем `toSvgString(tree)` — экранирование
   атрибутов, числа округлять до 2 знаков (чтобы строка была стабильной и короткой).
   `viewBox="0 0 100 100"`, `role="img"`, `aria-label` из options (по умолчанию seed).
   `options`: `size` (число → `width`/`height`, по умолчанию 100% через отсутствие
   атрибутов), `background: 'none' | 'circle' | 'squircle' | 'square'` (по умолчанию
   `circle`), `aria`. Без `<style>`, `<script>`, внешних ссылок, `id` (SVG может
   встраиваться много раз на странице).
5. **Публичное API** `src/index.ts`: `blobAvatar(seed, options?): string`,
   `blobAvatarTree(seed, options?): SvgNode`, `deriveTraits`, типы. `src/vue.ts`:
   `BlobAvatar` — `defineComponent` в TSX-стиле проекта, props `seed: string`, `size?:
   number`, `background?`, `traits?: Partial<Traits>`, `label?`; рендер через `h()` по дереву
   (не `innerHTML` — SSR-safe и без XSS-сомнений), `computed` по пропсам.
6. **Галерея** `scripts/gallery.ts` (`npm run gallery`, через `tsx` или `vite-node`): пишет
   `gallery.html` (в `.gitignore`) с сеткой 10×10 для seed'ов `anna, boris, vera, dmitry` +
   случайные строки + по одному на каждое `expression` и `shape`. Скриншот галереи — в
   отчёт и в README библиотеки (`docs/gallery.png`, коммитится).
7. **Тесты** (vitest): детерминизм (`blobAvatar('Anna ') === blobAvatar('anna')`,
   два вызова — одна строка); разные seed → разные строки (на 200 случайных seed не менее
   195 уникальных); каждая комбинация `shape × expression × eyeShape × tone` рендерится без
   `NaN`/`undefined` в строке и парсится `DOMParser` (jsdom) без `parsererror`; снапшоты
   для 4 демо-username (чтобы случайная правка не меняла лица всех пользователей);
   `BlobAvatar` монтируется (`@vue/test-utils`) и выдаёт `<svg role="img">`; размер
   строки одного аватара < 4 КБ.
8. **Подключение в Nuxt** (если `front/package.json` уже есть, т. е. F0 смержен):
   `"@fe/blob-avatar": "file:lib/blob-avatar"`, `npm i`, заменить заглушку в
   `components/ui/avatar` на `<BlobAvatar seed={username} background="circle" />`. Если F0
   ещё нет — пропустить, F0 подключит сам.

## Выходы

`front/lib/blob-avatar/{package.json, src/**, dist/**, tests/**, scripts/gallery.ts,
docs/gallery.png, README.md}`; тесты зелёные; галерея, на которую приятно смотреть.

## Владение

`front/lib/blob-avatar/**`. Плюс одна строка в `front/package.json` и правка
`front/app/components/ui/avatar/*` по шагу 8 — с записью в отчёт.

## Параллелизация внутри

random+traits (контракт `Traits` — первым) ‖ формы (`shapes.ts`) ‖ лица (`faces.ts`:
глаза, рты, брови, extras) → render → vue + галерея + тесты.

## Критерии приёмки

- [ ] Нет runtime-зависимостей; `vue` — только peer; `dist/` собран и совпадает с `src` (`npm run build` без диффа).
- [ ] `npm test` в пакете зелёный; снапшоты демо-аккаунтов закоммичены.
- [ ] SVG одного аватара < 4 КБ, без `id`, `<style>`, `<script>`.
- [ ] В галерее глаза и рот читаются на всех 8 hue × 2 tone (проверить глазами, скриншот в отчёт).
- [ ] Один seed даёт один и тот же SVG в Node и в браузере (тест на snapshot + ручная проверка в галерее).

## Отчёт `tasks/reports/F7.md`

API пакета, список осей/вариантов, как собрать и прогнать галерею, скриншот, что не вошло.

## Итерация 2 — серьёзный стиль (решение пользователя, 2026-10-20 01:10, см. `tasks/DECISIONS.md`)

Первая версия (блобы с лицами) принята и смержена, но стиль признан слишком игривым для
бизнес-клуба. Переделать визуал, **не меняя публичного API** (`blobAvatar`, `blobAvatarTree`,
`deriveTraits`, `BlobAvatar` с props `seed`, `size`, `background`, `traits`, `label`;
детерминизм, нормализация seed, лимит метки 64 — всё как есть):

1. **Без лиц.** Убрать глаза, рты, брови, выражения, румянец/веснушки (`faces.ts`, оси
   `eyeShape`, `eye*`, `pupilShift`, `expression`, `extras`). Аватар — абстрактная
   композиция: крупная базовая фигура (оставить `shape`: round/squircle/blob/droplet/
   hexagon/cloud/capsule/triangle — можно сократить до 5–6 самых спокойных) плюс 1–2
   акцентных элемента (меньшая фигура, дуга/кольцо, срез, мягкое наложение), положение,
   поворот и масштаб — из rng. Композиция должна выглядеть как логотип/иконка, а не как
   персонаж.
2. **Пастельная палитра.** Вместо 8 hue × light/dark — набор заранее подобранных
   пастельных пар «фон / фигура / акцент» (10–12 палитр, низкая насыщенность, высокая
   светлота; например, пыльная роза, шалфей, песок, лаванда, небо, мята, персик, графит).
   Контраст фигуры с фоном — заметный, но мягкий (без чистого белого и чёрного). Один
   seed → одна палитра. Оставить ось `hue`/`tone` в `Traits` можно как `palette: number`
   (индекс) — снапшоты и тесты обновить, `Traits` — новый тип, это допустимо (ломающих
   потребителей кроме `avatar` в F0 нет).
3. **Остальное без изменений:** детерминизм, уникальность (≥195/200), без `id`/`style`/
   `script`, < 4 КБ, SSR = браузер байт в байт, невалидные `overrides` игнорируются, Vue
   через `h()`.
4. Галерея и `docs/gallery.png` перерисовать; снапшоты `anna/boris/vera/dmitry` обновить
   осознанно (указать в отчёте). README пакета — обновить описание осей.
5. Отчёт `tasks/reports/F7.md` — дополнить разделом «Итерация 2», старое описание осей
   заменить.

Критерии приёмки итерации 2: `npm test`, `npm run build` без диффа в `dist/`; в галерее
нет лиц; все палитры читаются (фигура различима на фоне) при размерах 24 и 128 px;
скриншот галереи приложен; API и props не изменились (проверить по `dist/*.d.ts`:
сигнатуры `blobAvatar`, `blobAvatarTree`, `BlobAvatar` прежние).
