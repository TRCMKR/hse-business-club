// @vitest-environment node
import { describe, expect, it } from 'vitest';
import type { AnySchema } from 'yup';
import { ListingKind, StartupStage } from '~~/api/types';
import {
  applicationFormSchema,
  displayNameSchema,
  headlineSchema,
  listingFormSchema,
  loginFormSchema,
  messageSchema,
  passwordSchema,
  profileFormSchema,
  querySchema,
  registerFormSchema,
  skillsSchema,
  tagsSchema,
  telegramSchema,
  titleSchema,
  usernameSchema,
} from '~/utils/validation';

/** Returns the first validation message or null when valid. */
async function errorOf(schema: AnySchema, value: unknown): Promise<string | null> {
  try {
    await schema.validate(value);
    return null;
  } catch (error) {
    return (error as Error).message;
  }
}

async function errorsOf(schema: AnySchema, value: unknown): Promise<Record<string, string>> {
  try {
    await schema.validate(value, { abortEarly: false });
    return {};
  } catch (error) {
    const inner = (error as { inner: { path: string, message: string }[] }).inner;
    return Object.fromEntries(inner.map(e => [e.path, e.message]));
  }
}

describe('username', () => {
  it.each([
    ['ab', false],
    ['abc', true],
    ['a'.repeat(32), true],
    ['a'.repeat(33), false],
    ['anna_99', true],
    ['anna-99', false],
    ['анна', false],
    ['an na', false],
    ['', false],
  ])('%s → valid=%s', async (value, valid) => {
    expect(await errorOf(usernameSchema, value) === null).toBe(valid);
  });

  it('normalizes to lower-case and trims', async () => {
    expect(await usernameSchema.validate('  Anna_K ')).toBe('anna_k');
  });

  it('server-style message', async () => {
    expect(await errorOf(usernameSchema, 'ab')).toBe('Логин: от 3 до 32 символов, латиница, цифры и _');
  });
});

describe('password', () => {
  it.each([
    [7, false],
    [8, true],
    [72, true],
    [73, false],
  ])('length %i → valid=%s', async (n, valid) => {
    expect(await errorOf(passwordSchema, 'x'.repeat(n)) === null).toBe(valid);
  });
});

describe('simple text fields', () => {
  it('title 3–120', async () => {
    expect(await errorOf(titleSchema, 'ab')).toBe('Название: от 3 до 120 символов');
    expect(await errorOf(titleSchema, '  ab  ')).toBe('Название: от 3 до 120 символов');
    expect(await errorOf(titleSchema, 'abc')).toBeNull();
    expect(await errorOf(titleSchema, 'a'.repeat(120))).toBeNull();
    expect(await errorOf(titleSchema, 'a'.repeat(121))).toBe('Название: от 3 до 120 символов');
  });

  it('counts code points, not UTF-16 units', async () => {
    expect(await errorOf(titleSchema, '🚀'.repeat(120))).toBeNull();
  });

  it('displayName required', async () => {
    expect(await errorOf(displayNameSchema, '')).not.toBeNull();
    expect(await errorOf(displayNameSchema, 'А')).toBeNull();
  });

  it('optional fields accept empty', async () => {
    expect(await errorOf(headlineSchema, '')).toBeNull();
    expect(await errorOf(headlineSchema, 'a'.repeat(121))).toBe('Коротко о себе: не больше 120 символов');
    expect(await errorOf(querySchema, '')).toBeNull();
    expect(await errorOf(querySchema, 'a'.repeat(101))).not.toBeNull();
  });

  it('message 1–2000', async () => {
    expect(await errorOf(messageSchema, '   ')).not.toBeNull();
    expect(await errorOf(messageSchema, 'a'.repeat(2000))).toBeNull();
    expect(await errorOf(messageSchema, 'a'.repeat(2001))).toBe('Сообщение: от 1 до 2000 символов');
  });
});

describe('tags / skills', () => {
  const tags = (n: number) => Array.from({ length: n }, (_, i) => `tag${i}`);

  it('≤ 10 tags', async () => {
    expect(await errorOf(tagsSchema, tags(10))).toBeNull();
    expect(await errorOf(tagsSchema, tags(11))).toBe('Теги: не больше 10');
  });

  it('rejects duplicates after trim', async () => {
    expect(await errorOf(tagsSchema, ['Go', ' Go '])).toBe('Теги: без повторов');
  });

  it('item length 1–40', async () => {
    expect(await errorOf(tagsSchema, ['  '])).toBe('Теги: каждый от 1 до 40 символов');
    expect(await errorOf(tagsSchema, ['a'.repeat(41)])).toBe('Теги: каждый от 1 до 40 символов');
    expect(await tagsSchema.validate([' Go ', 'Vue'])).toEqual(['Go', 'Vue']);
  });

  it('≤ 20 skills', async () => {
    expect(await errorOf(skillsSchema, tags(20))).toBeNull();
    expect(await errorOf(skillsSchema, tags(21))).toBe('Навыки: не больше 20');
  });
});

describe('telegram', () => {
  it('drops leading @ and allows empty', async () => {
    expect(await telegramSchema.validate('@anna')).toBe('anna');
    expect(await telegramSchema.validate('')).toBe('');
  });

  it('≤ 64 without @', async () => {
    expect(await errorOf(telegramSchema, `@${'a'.repeat(64)}`)).toBeNull();
    expect(await errorOf(telegramSchema, 'a'.repeat(65))).toBe('Telegram: не больше 64 символов');
  });
});

describe('forms', () => {
  const listing = {
    title: 'Платформа для менторства',
    description: 'Ищем людей в команду стартапа',
    lookingFor: 'Frontend-разработчик',
    tags: ['Vue'],
  };

  it('startup requires stage and forbids compensation', async () => {
    const base = { ...listing, kind: ListingKind.LISTING_KIND_STARTUP };
    expect(await errorsOf(listingFormSchema, { ...base, stage: StartupStage.STARTUP_STAGE_UNSPECIFIED, compensation: '' }))
      .toHaveProperty('stage', 'Стадия: выберите стадию стартапа');
    expect(await errorsOf(listingFormSchema, { ...base, stage: StartupStage.STARTUP_STAGE_MVP, compensation: 'Доля' }))
      .toEqual({ compensation: 'Условия: только для вакансий' });
    expect(await errorsOf(listingFormSchema, { ...base, stage: StartupStage.STARTUP_STAGE_MVP, compensation: '' }))
      .toEqual({});
  });

  it('vacancy forbids stage', async () => {
    const base = { ...listing, kind: ListingKind.LISTING_KIND_VACANCY, compensation: '50 000 ₽' };
    expect(await errorsOf(listingFormSchema, { ...base, stage: StartupStage.STARTUP_STAGE_IDEA }))
      .toEqual({ stage: 'Стадия: только для стартапов' });
    expect(await errorsOf(listingFormSchema, { ...base, stage: StartupStage.STARTUP_STAGE_UNSPECIFIED }))
      .toEqual({});
    expect(await errorsOf(listingFormSchema, { ...base, stage: '' })).toEqual({});
  });

  it('kind is required', async () => {
    expect(await errorsOf(listingFormSchema, { ...listing, kind: ListingKind.LISTING_KIND_UNSPECIFIED }))
      .toHaveProperty('kind');
  });

  it('register: passwords must match', async () => {
    const value = { username: 'anna', password: 'password1', passwordConfirm: 'password2', displayName: 'Анна' };
    expect(await errorsOf(registerFormSchema, value)).toEqual({ passwordConfirm: 'Пароли не совпадают' });
    expect(await errorsOf(registerFormSchema, { ...value, passwordConfirm: 'password1' })).toEqual({});
  });

  it('login: presence only', async () => {
    expect(await errorsOf(loginFormSchema, { username: '', password: '' }))
      .toEqual({ username: 'Введите логин', password: 'Введите пароль' });
    expect(await loginFormSchema.validate({ username: ' Anna ', password: 'x' })).toEqual({ username: 'anna', password: 'x' });
  });

  it('profile and application', async () => {
    expect(await profileFormSchema.validate({
      displayName: 'Анна',
      headline: '',
      about: '',
      skills: [],
      telegram: '@anna',
    })).toMatchObject({ telegram: 'anna' });
    expect(await errorsOf(applicationFormSchema, { message: '' })).toHaveProperty('message');
  });
});
