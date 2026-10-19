/**
 * Date formatting in Russian (date-fns + ru locale).
 *
 * All functions take RFC 3339 strings from the API (UTC) and format them in the
 * **local time zone of the runtime**. During SSR that is the server's zone, on the client
 * the browser's, so a server-rendered relative time («вчера в 14:05») may differ from the
 * hydrated one by the zone offset. This is accepted: render relative times inside
 * `<ClientOnly>` or re-render on mount where the mismatch matters.
 *
 * `null`, empty and invalid input always produce `''`.
 */
import {
  differenceInCalendarDays,
  differenceInMinutes,
  format,
  isSameDay,
  isSameYear,
  isValid,
  parseISO,
  subDays,
} from 'date-fns';
import { ru } from 'date-fns/locale/ru';
import { pluralize } from '~/utils/labels';

type DateInput = string | Date | null | undefined;

function toDate(value: DateInput): Date | null {
  if (!value) {
    return null;
  }
  const date = typeof value === 'string' ? parseISO(value) : value;
  return isValid(date) ? date : null;
}

/** «12 октября 2026». */
export function formatDate(iso: DateInput): string {
  const date = toDate(iso);
  return date ? format(date, 'd MMMM yyyy', { locale: ru }) : '';
}

/** «12 октября 2026, 14:05». */
export function formatDateTime(iso: DateInput): string {
  const date = toDate(iso);
  return date ? format(date, 'd MMMM yyyy, HH:mm', { locale: ru }) : '';
}

/**
 * Human relative time:
 * «только что» (< 1 min) · «5 минут назад» · «2 часа назад» (today) ·
 * «вчера в 14:05» · «3 дня назад» (< 7 days) · «12 октября» (this year) ·
 * «12 октября 2025» (other years and future dates).
 *
 * @param iso RFC 3339 timestamp (or Date).
 * @param now reference point, injectable for tests.
 */
export function formatRelative(iso: DateInput, now: Date = new Date()): string {
  const date = toDate(iso);
  if (!date) {
    return '';
  }
  const minutes = differenceInMinutes(now, date);
  // Small clock skew between server and client: treat near-future as «just now».
  if (minutes < 1 && minutes > -2) {
    return 'только что';
  }
  if (minutes < 0) {
    return format(date, isSameYear(date, now) ? 'd MMMM' : 'd MMMM yyyy', { locale: ru });
  }
  if (minutes < 60) {
    return `${minutes} ${pluralize(minutes, ['минуту', 'минуты', 'минут'])} назад`;
  }
  if (isSameDay(date, now)) {
    const hours = Math.floor(minutes / 60);
    return `${hours} ${pluralize(hours, ['час', 'часа', 'часов'])} назад`;
  }
  if (isSameDay(date, subDays(now, 1))) {
    return `вчера в ${format(date, 'HH:mm')}`;
  }
  const days = differenceInCalendarDays(now, date);
  if (days < 7) {
    return `${days} ${pluralize(days, ['день', 'дня', 'дней'])} назад`;
  }
  return format(date, isSameYear(date, now) ? 'd MMMM' : 'd MMMM yyyy', { locale: ru });
}
