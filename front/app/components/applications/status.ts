/**
 * Status filter of the application lists: short URL values (`?status=pending`) ↔ API enum.
 */
import { ApplicationStatus } from '~~/api/types';

export type StatusFilterValue = '' | 'pending' | 'accepted' | 'declined' | 'withdrawn';

export interface StatusFilterOption {
  value: StatusFilterValue
  label: string
}

const TO_ENUM: Record<Exclude<StatusFilterValue, ''>, ApplicationStatus> = {
  pending: ApplicationStatus.APPLICATION_STATUS_PENDING,
  accepted: ApplicationStatus.APPLICATION_STATUS_ACCEPTED,
  declined: ApplicationStatus.APPLICATION_STATUS_DECLINED,
  withdrawn: ApplicationStatus.APPLICATION_STATUS_WITHDRAWN,
};

// A Set, not `in TO_ENUM`: `in` also matches prototype keys (`?status=constructor`).
const KNOWN_VALUES = new Set<string>(Object.keys(TO_ENUM));

export const STATUS_FILTER_OPTIONS: StatusFilterOption[] = [
  { value: '', label: 'Все' },
  { value: 'pending', label: 'Ожидающие' },
  { value: 'accepted', label: 'Принятые' },
  { value: 'declined', label: 'Отклонённые' },
  { value: 'withdrawn', label: 'Отозванные' },
];

/** `route.query.status` → filter value; anything unknown means «Все». */
export function parseStatusQuery(raw: unknown): StatusFilterValue {
  const value = Array.isArray(raw) ? raw[0] : raw;
  if (typeof value !== 'string') {
    return '';
  }
  const normalized = value.toLowerCase();
  return KNOWN_VALUES.has(normalized) ? (normalized as StatusFilterValue) : '';
}

/** Filter value → API enum (undefined = no filter). */
export function statusFilterToEnum(value: StatusFilterValue): ApplicationStatus | undefined {
  return value ? TO_ENUM[value] : undefined;
}

/** Label of a filter value, e.g. for the empty state. */
export function statusFilterLabel(value: StatusFilterValue): string {
  return STATUS_FILTER_OPTIONS.find(o => o.value === value)?.label ?? '';
}
