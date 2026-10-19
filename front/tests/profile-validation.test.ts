// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { ValidationError } from 'yup';
import { profileFormSchema } from '~/utils/validation';
import {
  AVATAR_MAX_BYTES,
  AVATAR_SIZE_MESSAGE,
  AVATAR_TYPE_MESSAGE,
  validateAvatarFile,
} from '~/components/profile/avatar-editor/validate';

const valid = {
  displayName: 'Анна Смирнова',
  headline: 'Продакт-менеджер',
  about: 'Ищу команду',
  skills: ['Product', 'SQL'],
  telegram: 'anna_s',
};

async function errorsOf(values: Record<string, unknown>): Promise<Record<string, string>> {
  try {
    await profileFormSchema.validate(values, { abortEarly: false });
    return {};
  } catch (e) {
    if (!(e instanceof ValidationError)) {
      throw e;
    }
    return Object.fromEntries(e.inner.map(err => [err.path ?? '', err.message]));
  }
}

describe('profileFormSchema', () => {
  it('accepts a valid profile and normalises values', async () => {
    const out = await profileFormSchema.validate({
      ...valid,
      displayName: '  Анна  ',
      skills: [' Product ', '', 'SQL'],
      telegram: '@anna_s',
    });
    expect(out).toEqual({ ...valid, displayName: 'Анна' });
  });

  it('allows empty optional fields', async () => {
    expect(await errorsOf({ displayName: 'A', headline: '', about: '', skills: [], telegram: '' })).toEqual({});
  });

  it('uses SPEC §2.4 field labels', async () => {
    const errors = await errorsOf({
      displayName: '   ',
      headline: 'a'.repeat(121),
      about: 'a'.repeat(2001),
      skills: Array.from({ length: 21 }, (_, i) => `s${i}`),
      telegram: 'a'.repeat(65),
    });
    expect(errors).toEqual({
      displayName: 'Имя: от 1 до 80 символов',
      headline: 'Кратко о себе: не больше 120 символов',
      about: 'О себе: не больше 2000 символов',
      skills: 'Навыки: не больше 20',
      telegram: 'Telegram: не больше 64 символов',
    });
  });

  it('rejects case-insensitive skill duplicates and too long skills', async () => {
    expect((await errorsOf({ ...valid, skills: ['SQL', 'sql'] })).skills).toBe('Навыки: без повторов');
    expect((await errorsOf({ ...valid, skills: ['a'.repeat(41)] })).skills).toBe('Навыки: каждый от 1 до 40 символов');
  });

  it('counts the leading @ out of the Telegram limit', async () => {
    expect(await errorsOf({ ...valid, telegram: `@${'a'.repeat(64)}` })).toEqual({});
  });
});

describe('validateAvatarFile', () => {
  const file = (name: string, type: string, size = 1024) => ({ name, type, size });

  it('accepts JPEG, PNG and WebP up to 5 MB', () => {
    expect(validateAvatarFile(file('a.jpg', 'image/jpeg'))).toBeNull();
    expect(validateAvatarFile(file('a.png', 'image/png'))).toBeNull();
    expect(validateAvatarFile(file('a.webp', 'image/webp', AVATAR_MAX_BYTES))).toBeNull();
  });

  it('rejects other types with the SPEC message', () => {
    expect(validateAvatarFile(file('a.gif', 'image/gif'))).toBe(AVATAR_TYPE_MESSAGE);
    expect(validateAvatarFile(file('a.pdf', 'application/pdf'))).toBe(AVATAR_TYPE_MESSAGE);
    expect(AVATAR_TYPE_MESSAGE).toBe('Фото: поддерживаются JPEG, PNG и WebP');
  });

  it('falls back to the extension when the type is empty', () => {
    expect(validateAvatarFile(file('photo.JPEG', ''))).toBeNull();
    expect(validateAvatarFile(file('photo.heic', ''))).toBe(AVATAR_TYPE_MESSAGE);
  });

  it('rejects files over 5 MB', () => {
    expect(validateAvatarFile(file('a.jpg', 'image/jpeg', AVATAR_MAX_BYTES + 1))).toBe(AVATAR_SIZE_MESSAGE);
    expect(AVATAR_SIZE_MESSAGE).toBe('Фото: не больше 5 МБ');
  });
});
