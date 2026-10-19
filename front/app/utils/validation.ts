/**
 * Client-side validation mirroring the server rules (SPEC §2.4). The server is the
 * source of truth; messages follow its style: «Название: от 3 до 120 символов».
 *
 * Exported are plain yup schemas; wrap them with `toTypedSchema` from
 * `@vee-validate/yup` at the call site. String lengths are counted in Unicode code
 * points (as the Go backend counts runes), values are trimmed on cast.
 */
import * as yup from 'yup';
import { ListingKind, StartupStage } from '~~/api/types';

interface RangeLimit {
  min: number
  max: number
}

interface ListLimit {
  maxItems: number
  item: RangeLimit
}

/** Field limits from SPEC §2.4 — use them for `maxlength` and textarea counters. */
export const LIMITS = {
  username: { min: 3, max: 32 },
  password: { min: 8, max: 72 },
  displayName: { min: 1, max: 80 },
  headline: { min: 0, max: 120 },
  about: { min: 0, max: 2000 },
  skills: { maxItems: 20, item: { min: 1, max: 40 } },
  telegram: { min: 0, max: 64 },
  title: { min: 3, max: 120 },
  description: { min: 10, max: 5000 },
  lookingFor: { min: 1, max: 1000 },
  tags: { maxItems: 10, item: { min: 1, max: 40 } },
  compensation: { min: 0, max: 120 },
  message: { min: 1, max: 2000 },
  query: { min: 0, max: 100 },
} as const satisfies Record<string, RangeLimit | ListLimit>;

export const USERNAME_PATTERN = /^[a-z0-9_]+$/;

/** Length in code points (matches Go `utf8.RuneCountInString`). */
export function textLength(value: string | null | undefined): number {
  return value ? [...value].length : 0;
}

function rangeMessage(label: string, { min, max }: RangeLimit): string {
  return min > 0 ? `${label}: от ${min} до ${max} символов` : `${label}: не больше ${max} символов`;
}

/** Trimmed string with a code-point length check. Empty is allowed when `min` is 0. */
function textSchema(label: string, limit: RangeLimit, message = rangeMessage(label, limit)) {
  let schema = yup.string().trim().default('');
  if (limit.min > 0) {
    schema = schema.required(message);
  }
  return schema.test('length', message, (value) => {
    const n = textLength(value);
    return (limit.min === 0 && n === 0) || (n >= limit.min && n <= limit.max);
  });
}

function stringListSchema(label: string, limit: ListLimit) {
  return yup
    .array()
    .of(yup.string().defined())
    .default([])
    .transform((value: unknown) => Array.isArray(value)
      ? value.map(item => (typeof item === 'string' ? item.trim() : item))
      : value)
    .max(limit.maxItems, `${label}: не больше ${limit.maxItems}`)
    .test(
      'item-length',
      `${label}: каждый от ${limit.item.min} до ${limit.item.max} символов`,
      list => (list ?? []).every((item) => {
        const n = textLength(item);
        return n >= limit.item.min && n <= limit.item.max;
      }),
    )
    .test('unique', `${label}: без повторов`, (list) => {
      const items = list ?? [];
      return new Set(items).size === items.length;
    });
}

// Users

const USERNAME_MESSAGE = 'Логин: от 3 до 32 символов, латиница, цифры и _';

/** Lower-cased and trimmed on cast. */
export const usernameSchema = yup
  .string()
  .transform((value: unknown) => (typeof value === 'string' ? value.trim().toLowerCase() : value))
  .required(USERNAME_MESSAGE)
  .min(LIMITS.username.min, USERNAME_MESSAGE)
  .max(LIMITS.username.max, USERNAME_MESSAGE)
  .matches(USERNAME_PATTERN, USERNAME_MESSAGE);

const PASSWORD_MESSAGE = rangeMessage('Пароль', LIMITS.password);

/** Not trimmed: spaces are part of the password. */
export const passwordSchema = yup
  .string()
  .required(PASSWORD_MESSAGE)
  .min(LIMITS.password.min, PASSWORD_MESSAGE)
  .max(LIMITS.password.max, PASSWORD_MESSAGE);

export const displayNameSchema = textSchema('Имя', LIMITS.displayName);

export const headlineSchema = textSchema('Коротко о себе', LIMITS.headline);

export const aboutSchema = textSchema('О себе', LIMITS.about);

export const skillsSchema = stringListSchema('Навыки', LIMITS.skills);

/** Leading `@` is dropped on cast; empty string is allowed. */
export const telegramSchema = yup
  .string()
  .default('')
  .transform((value: unknown) => (typeof value === 'string' ? value.trim().replace(/^@+/, '') : value))
  .test('length', rangeMessage('Telegram', LIMITS.telegram), value => textLength(value) <= LIMITS.telegram.max);

// Listings

export const titleSchema = textSchema('Название', LIMITS.title);

export const descriptionSchema = textSchema('Описание', LIMITS.description);

export const lookingForSchema = textSchema('Кого ищем', LIMITS.lookingFor);

export const tagsSchema = stringListSchema('Теги', LIMITS.tags);

export const compensationSchema = textSchema('Условия', LIMITS.compensation);

const STAGE_VALUES = Object.values(StartupStage);

/** Any stage value; empty → UNSPECIFIED. Kind-specific rules live in `listingFormSchema`. */
export const stageSchema = yup
  .mixed<StartupStage>()
  .transform((value: unknown) => (value === '' || value === null ? StartupStage.STARTUP_STAGE_UNSPECIFIED : value))
  .oneOf(STAGE_VALUES, 'Стадия: выберите стадию стартапа')
  .default(StartupStage.STARTUP_STAGE_UNSPECIFIED);

export const kindSchema = yup
  .mixed<ListingKind>()
  .oneOf([ListingKind.LISTING_KIND_STARTUP, ListingKind.LISTING_KIND_VACANCY], 'Тип: выберите стартап или вакансию')
  .required('Тип: выберите стартап или вакансию');

export const querySchema = textSchema('Поиск', LIMITS.query);

// Applications

export const messageSchema = textSchema('Сообщение', LIMITS.message);

// Forms

/** Login form: only presence is checked, the server answers 401 for wrong credentials. */
export const loginFormSchema = yup.object({
  username: yup
    .string()
    .transform((value: unknown) => (typeof value === 'string' ? value.trim().toLowerCase() : value))
    .required('Введите логин'),
  password: yup.string().required('Введите пароль'),
});

export const registerFormSchema = yup.object({
  username: usernameSchema,
  password: passwordSchema,
  passwordConfirm: yup
    .string()
    .required('Повторите пароль')
    .oneOf([yup.ref('password')], 'Пароли не совпадают'),
  displayName: displayNameSchema,
});

export const profileFormSchema = yup.object({
  displayName: displayNameSchema,
  headline: headlineSchema,
  about: aboutSchema,
  skills: skillsSchema,
  telegram: telegramSchema,
});

const isStartup = (kind: unknown) => kind === ListingKind.LISTING_KIND_STARTUP;
const isVacancy = (kind: unknown) => kind === ListingKind.LISTING_KIND_VACANCY;

/**
 * Create/edit listing form. `stage` is required for startups and must be UNSPECIFIED for
 * vacancies; `compensation` is allowed only for vacancies.
 */
export const listingFormSchema = yup.object({
  kind: kindSchema,
  title: titleSchema,
  description: descriptionSchema,
  lookingFor: lookingForSchema,
  tags: tagsSchema,
  // yup's oneOf/notOneOf merge with the base whitelist, so kind rules use explicit tests.
  stage: stageSchema
    .when('kind', {
      is: isStartup,
      then: schema => schema.test(
        'startup-stage',
        'Стадия: выберите стадию стартапа',
        value => value !== undefined && value !== StartupStage.STARTUP_STAGE_UNSPECIFIED,
      ),
    })
    .when('kind', {
      is: isVacancy,
      then: schema => schema.test(
        'vacancy-stage',
        'Стадия: только для стартапов',
        value => value === undefined || value === StartupStage.STARTUP_STAGE_UNSPECIFIED,
      ),
    }),
  compensation: compensationSchema.when('kind', {
    is: isStartup,
    then: schema => schema.test(
      'startup-empty',
      'Условия: только для вакансий',
      value => textLength(value) === 0,
    ),
  }),
});

export const applicationFormSchema = yup.object({
  message: messageSchema,
});

export type LoginFormValues = yup.InferType<typeof loginFormSchema>;
export type RegisterFormValues = yup.InferType<typeof registerFormSchema>;
export type ProfileFormValues = yup.InferType<typeof profileFormSchema>;
export type ListingFormValues = yup.InferType<typeof listingFormSchema>;
export type ApplicationFormValues = yup.InferType<typeof applicationFormSchema>;
