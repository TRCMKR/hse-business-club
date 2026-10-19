// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { avatarSrc } from '~/utils/avatar';
import { displayTelegram, telegramUrl } from '~/utils/format';

describe('avatarSrc', () => {
  it('prefixes avatarUrl with the proxy path', () => {
    expect(avatarSrc({ avatarUrl: '/v1/avatars/abc.jpg' })).toBe('/api/v1/avatars/abc.jpg');
  });

  it('returns null without a photo', () => {
    expect(avatarSrc({ avatarUrl: '' })).toBeNull();
    expect(avatarSrc(null)).toBeNull();
  });
});

describe('telegram', () => {
  it('formats handles', () => {
    expect(displayTelegram('anna')).toBe('@anna');
    expect(displayTelegram('@anna')).toBe('@anna');
    expect(displayTelegram('')).toBe('');
    expect(telegramUrl('anna')).toBe('https://t.me/anna');
    expect(telegramUrl('')).toBeNull();
  });
});
