/**
 * Request parsing and SPEC §2.4 validation. Messages are user-facing (Russian),
 * format «<Поле>: <правило>».
 */
import { invalidArgument } from './errors';
import type { ApplicationStatus, ListingKind, ListingStatus, StartupStage } from './state';
import { APPLICATION_STATUSES, LISTING_KINDS, LISTING_STATUSES, STARTUP_STAGES } from './state';

/** Field labels used in error messages. */
export const LABELS = {
  username: 'Логин',
  password: 'Пароль',
  displayName: 'Имя',
  headline: 'Кратко о себе',
  about: 'О себе',
  skills: 'Навыки',
  telegram: 'Telegram',
  avatar: 'Фото',
  kind: 'Тип',
  title: 'Название',
  description: 'Описание',
  lookingFor: 'Кого ищем',
  tags: 'Теги',
  stage: 'Стадия',
  compensation: 'Условия',
  message: 'Сообщение',
  query: 'Поиск',
  page: 'Страница',
  limit: 'Количество',
  status: 'Статус',
} as const;

type Field = keyof typeof LABELS;

/** All user-facing validation messages in one place. */
export const MESSAGES = {
  badRequest: 'Некорректный запрос',
  length: (field: Field, min: number, max: number) =>
    min > 0 ? `${LABELS[field]}: от ${min} до ${max} символов` : `${LABELS[field]}: не больше ${max} символов`,
  listCount: (field: Field, max: number) => `${LABELS[field]}: не больше ${max}`,
  listItem: (field: Field, max: number) => `${LABELS[field]}: каждый элемент — до ${max} символов`,
  badEnum: (field: Field) => `${LABELS[field]}: недопустимое значение`,
  usernameChars: `${LABELS.username}: только латинские буквы, цифры и _`,
  kindRequired: `${LABELS.kind}: выберите стартап или вакансию`,
  stageRequired: `${LABELS.stage}: выберите стадию стартапа`,
  stageStartupOnly: `${LABELS.stage}: только для стартапов`,
  compensationVacancyOnly: `${LABELS.compensation}: только для вакансий`,
  pageMin: `${LABELS.page}: не меньше 1`,
  pageNumber: `${LABELS.page}: должна быть числом`,
  limitRange: `${LABELS.limit}: от 1 до 50`,
  avatarType: `${LABELS.avatar}: поддерживаются JPEG, PNG и WebP`,
  avatarSize: `${LABELS.avatar}: не больше 5 МБ`,
} as const;

// ---------- raw body access ----------

export type Body = Record<string, unknown>;

export function asBody(body: unknown): Body {
  if (body === undefined || body === null || body === '') {
    return {};
  }
  if (typeof body !== 'object' || Array.isArray(body)) {
    throw invalidArgument(MESSAGES.badRequest);
  }
  return body as Body;
}

/** protojson semantics: absent/null → default, wrong JSON type → 400. */
export function readString(body: Body, key: string): string {
  const v = body[key];
  if (v === undefined || v === null) {
    return '';
  }
  if (typeof v !== 'string') {
    throw invalidArgument(MESSAGES.badRequest);
  }
  return v;
}

export function readStringList(body: Body, key: string): string[] {
  const v = body[key];
  if (v === undefined || v === null) {
    return [];
  }
  if (!Array.isArray(v) || v.some(item => typeof item !== 'string')) {
    throw invalidArgument(MESSAGES.badRequest);
  }
  return v as string[];
}

/** Enum by string name or numeric value; absent → UNSPECIFIED (index 0). */
export function parseEnum<T extends string>(value: unknown, values: readonly T[], field: Field): T {
  if (value === undefined || value === null || value === '') {
    return values[0]!;
  }
  if (typeof value === 'string' && (values as readonly string[]).includes(value)) {
    return value as T;
  }
  const n = typeof value === 'number' ? value : typeof value === 'string' && /^\d+$/.test(value) ? Number(value) : Number.NaN;
  if (Number.isInteger(n) && n >= 0 && n < values.length) {
    return values[n]!;
  }
  throw invalidArgument(MESSAGES.badEnum(field));
}

export const parseListingKind = (v: unknown) => parseEnum<ListingKind>(v, LISTING_KINDS, 'kind');
export const parseListingStatus = (v: unknown) => parseEnum<ListingStatus>(v, LISTING_STATUSES, 'status');
export const parseStartupStage = (v: unknown) => parseEnum<StartupStage>(v, STARTUP_STAGES, 'stage');
export const parseApplicationStatus = (v: unknown) => parseEnum<ApplicationStatus>(v, APPLICATION_STATUSES, 'status');

// ---------- rules ----------

/** Length in code points (as the Go backend counts runes). */
const runeLength = (s: string) => [...s].length;

function checkLength(field: Field, value: string, min: number, max: number): string {
  const len = runeLength(value);
  if (len < min || len > max) {
    throw invalidArgument(MESSAGES.length(field, min, max));
  }
  return value;
}

/** Trim, drop empty items and duplicates (case-insensitive, first wins), then check limits. */
export function normalizeList(field: Field, items: string[], maxCount: number, maxItem: number): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of items) {
    const item = raw.trim();
    const key = item.toLowerCase();
    if (!item || seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push(item);
  }
  if (out.length > maxCount) {
    throw invalidArgument(MESSAGES.listCount(field, maxCount));
  }
  if (out.some(item => runeLength(item) > maxItem)) {
    throw invalidArgument(MESSAGES.listItem(field, maxItem));
  }
  return out;
}

export function validateUsername(raw: string): string {
  const username = raw.trim().toLowerCase();
  checkLength('username', username, 3, 32);
  if (!/^[a-z0-9_]+$/.test(username)) {
    throw invalidArgument(MESSAGES.usernameChars);
  }
  return username;
}

export const validatePassword = (raw: string) => checkLength('password', raw, 8, 72);
export const validateDisplayName = (raw: string) => checkLength('displayName', raw.trim(), 1, 80);

export interface ProfileFields {
  displayName: string
  headline: string
  about: string
  skills: string[]
  telegram: string
}

export function validateProfile(body: Body): ProfileFields {
  return {
    displayName: validateDisplayName(readString(body, 'displayName')),
    headline: checkLength('headline', readString(body, 'headline').trim(), 0, 120),
    about: checkLength('about', readString(body, 'about').trim(), 0, 2000),
    skills: normalizeList('skills', readStringList(body, 'skills'), 20, 40),
    telegram: checkLength('telegram', readString(body, 'telegram').trim().replace(/^@+/, ''), 0, 64),
  };
}

export interface ListingFields {
  title: string
  description: string
  lookingFor: string
  tags: string[]
  stage: StartupStage
  compensation: string
}

/** Validates the editable listing fields against the (existing or requested) kind. */
export function validateListingFields(body: Body, kind: ListingKind): ListingFields {
  const fields: ListingFields = {
    title: checkLength('title', readString(body, 'title').trim(), 3, 120),
    description: checkLength('description', readString(body, 'description').trim(), 10, 5000),
    lookingFor: checkLength('lookingFor', readString(body, 'lookingFor').trim(), 1, 1000),
    tags: normalizeList('tags', readStringList(body, 'tags'), 10, 40),
    stage: parseStartupStage(body.stage),
    compensation: checkLength('compensation', readString(body, 'compensation').trim(), 0, 120),
  };
  if (kind === 'LISTING_KIND_STARTUP') {
    if (fields.stage === 'STARTUP_STAGE_UNSPECIFIED') {
      throw invalidArgument(MESSAGES.stageRequired);
    }
    if (fields.compensation) {
      throw invalidArgument(MESSAGES.compensationVacancyOnly);
    }
  } else if (fields.stage !== 'STARTUP_STAGE_UNSPECIFIED') {
    throw invalidArgument(MESSAGES.stageStartupOnly);
  }
  return fields;
}

export function validateKind(value: unknown): ListingKind {
  const kind = parseListingKind(value);
  if (kind === 'LISTING_KIND_UNSPECIFIED') {
    throw invalidArgument(MESSAGES.kindRequired);
  }
  return kind;
}

export const validateMessage = (body: Body) => checkLength('message', readString(body, 'message').trim(), 1, 2000);

export interface FeedParams {
  kind: ListingKind
  query: string
  page: number
  limit: number
}

function parseInt32(value: string | undefined, onNaN: string): number {
  if (value === undefined || value.trim() === '') {
    return 0;
  }
  if (!/^-?\d+$/.test(value.trim())) {
    throw invalidArgument(onNaN);
  }
  return Number(value);
}

/** Query params of `GET /v1/listings`. 0 / absent page and limit mean defaults (proto3 semantics). */
export function validateFeedParams(query: Record<string, string | undefined>): FeedParams {
  const kind = parseListingKind(query.kind);
  const q = checkLength('query', (query.query ?? '').trim(), 0, 100);
  const page = parseInt32(query.page, MESSAGES.pageNumber);
  if (page < 0) {
    throw invalidArgument(MESSAGES.pageMin);
  }
  const limit = parseInt32(query.limit, MESSAGES.limitRange);
  if (limit < 0 || limit > 50) {
    throw invalidArgument(MESSAGES.limitRange);
  }
  return { kind, query: q, page: page || 1, limit: limit || 20 };
}

// ---------- avatar ----------

export const AVATAR_MAX_BYTES = 5 * 1024 * 1024;

/** Detects the image type by magic bytes; null — not JPEG/PNG/WebP. */
export function detectImageType(bytes: Uint8Array): string | null {
  const b = bytes;
  if (b.length >= 3 && b[0] === 0xFF && b[1] === 0xD8 && b[2] === 0xFF) {
    return 'image/jpeg';
  }
  if (b.length >= 8 && b[0] === 0x89 && b[1] === 0x50 && b[2] === 0x4E && b[3] === 0x47
    && b[4] === 0x0D && b[5] === 0x0A && b[6] === 0x1A && b[7] === 0x0A) {
    return 'image/png';
  }
  const ascii = (from: number, to: number) => String.fromCharCode(...b.subarray(from, to));
  if (b.length >= 12 && ascii(0, 4) === 'RIFF' && ascii(8, 12) === 'WEBP') {
    return 'image/webp';
  }
  return null;
}
