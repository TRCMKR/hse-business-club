// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { formatDate, formatDateTime, formatRelative } from '~/utils/dates';

// Local-time constructors keep the test independent of the machine's time zone.
const now = new Date(2026, 9, 20, 15, 30, 0);
const ago = (ms: number) => new Date(now.getTime() - ms).toISOString();
const MIN = 60_000;
const HOUR = 60 * MIN;

describe('formatRelative', () => {
  it.each([
    ['30 seconds', ago(30_000), 'только что'],
    ['1 minute', ago(MIN), '1 минуту назад'],
    ['5 minutes', ago(5 * MIN), '5 минут назад'],
    ['22 minutes', ago(22 * MIN), '22 минуты назад'],
    ['2 hours', ago(2 * HOUR), '2 часа назад'],
    ['yesterday', new Date(2026, 9, 19, 14, 5).toISOString(), 'вчера в 14:05'],
    ['3 days', new Date(2026, 9, 17, 10, 0).toISOString(), '3 дня назад'],
    ['6 days', new Date(2026, 9, 14, 10, 0).toISOString(), '6 дней назад'],
    ['this year', new Date(2026, 9, 12, 10, 0).toISOString(), '12 октября'],
    ['last year', new Date(2025, 9, 12, 10, 0).toISOString(), '12 октября 2025'],
  ])('%s', (_, iso, expected) => {
    expect(formatRelative(iso, now)).toBe(expected);
  });

  it('empty and invalid input → empty string', () => {
    expect(formatRelative(null, now)).toBe('');
    expect(formatRelative(undefined, now)).toBe('');
    expect(formatRelative('', now)).toBe('');
    expect(formatRelative('not a date', now)).toBe('');
  });

  it('minutes win over «вчера» right after midnight', () => {
    const justAfterMidnight = new Date(2026, 9, 20, 0, 10);
    expect(formatRelative(new Date(2026, 9, 19, 23, 50).toISOString(), justAfterMidnight)).toBe('20 минут назад');
  });
});

describe('formatDate / formatDateTime', () => {
  const iso = new Date(2026, 9, 12, 14, 5).toISOString();

  it('formats in Russian', () => {
    expect(formatDate(iso)).toBe('12 октября 2026');
    expect(formatDateTime(iso)).toBe('12 октября 2026, 14:05');
  });

  it('empty and invalid input → empty string', () => {
    expect(formatDate(null)).toBe('');
    expect(formatDateTime('garbage')).toBe('');
  });
});
