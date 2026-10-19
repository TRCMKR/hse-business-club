/**
 * Russian labels and badge tones for every domain enum (SPEC §2, §7).
 * Import explicitly: `import { listingKindLabel } from '~/utils/labels'`.
 */
import { ApplicationStatus, ListingKind, ListingStatus, StartupStage } from '~~/api/types';

/** Tone values understood by the `Badge` UI component. */
export type BadgeTone = 'neutral' | 'accent' | 'success' | 'warning' | 'danger' | 'info';

export interface EnumOption<T extends string> {
  value: T
  label: string
}

// Listing kind

export const listingKindLabel: Record<ListingKind, string> = {
  [ListingKind.LISTING_KIND_UNSPECIFIED]: 'Все',
  [ListingKind.LISTING_KIND_STARTUP]: 'Стартап',
  [ListingKind.LISTING_KIND_VACANCY]: 'Вакансия',
};

/** Options for the feed segmented control (UNSPECIFIED = no filter). */
export const listingKindOptions: EnumOption<ListingKind>[] = [
  { value: ListingKind.LISTING_KIND_UNSPECIFIED, label: 'Все' },
  { value: ListingKind.LISTING_KIND_STARTUP, label: 'Стартапы' },
  { value: ListingKind.LISTING_KIND_VACANCY, label: 'Вакансии' },
];

/** Options for choosing the kind of a new listing (no UNSPECIFIED). */
export const listingKindCreateOptions: EnumOption<ListingKind>[] = [
  { value: ListingKind.LISTING_KIND_STARTUP, label: 'Стартап' },
  { value: ListingKind.LISTING_KIND_VACANCY, label: 'Вакансия' },
];

export const listingKindTone: Record<ListingKind, BadgeTone> = {
  [ListingKind.LISTING_KIND_UNSPECIFIED]: 'neutral',
  [ListingKind.LISTING_KIND_STARTUP]: 'accent',
  [ListingKind.LISTING_KIND_VACANCY]: 'info',
};

// Startup stage

export const startupStageLabel: Record<StartupStage, string> = {
  [StartupStage.STARTUP_STAGE_UNSPECIFIED]: 'Не указана',
  [StartupStage.STARTUP_STAGE_IDEA]: 'Идея',
  [StartupStage.STARTUP_STAGE_PROTOTYPE]: 'Прототип',
  [StartupStage.STARTUP_STAGE_MVP]: 'MVP',
  [StartupStage.STARTUP_STAGE_LAUNCHED]: 'Запущен',
};

/** Options for the stage select (no UNSPECIFIED). */
export const startupStageOptions: EnumOption<StartupStage>[] = [
  StartupStage.STARTUP_STAGE_IDEA,
  StartupStage.STARTUP_STAGE_PROTOTYPE,
  StartupStage.STARTUP_STAGE_MVP,
  StartupStage.STARTUP_STAGE_LAUNCHED,
].map(value => ({ value, label: startupStageLabel[value] }));

export const startupStageTone: Record<StartupStage, BadgeTone> = {
  [StartupStage.STARTUP_STAGE_UNSPECIFIED]: 'neutral',
  [StartupStage.STARTUP_STAGE_IDEA]: 'neutral',
  [StartupStage.STARTUP_STAGE_PROTOTYPE]: 'neutral',
  [StartupStage.STARTUP_STAGE_MVP]: 'neutral',
  [StartupStage.STARTUP_STAGE_LAUNCHED]: 'neutral',
};

// Listing status

export const listingStatusLabel: Record<ListingStatus, string> = {
  [ListingStatus.LISTING_STATUS_UNSPECIFIED]: 'Неизвестно',
  [ListingStatus.LISTING_STATUS_ACTIVE]: 'Активна',
  [ListingStatus.LISTING_STATUS_CLOSED]: 'Закрыта',
};

export const listingStatusTone: Record<ListingStatus, BadgeTone> = {
  [ListingStatus.LISTING_STATUS_UNSPECIFIED]: 'neutral',
  [ListingStatus.LISTING_STATUS_ACTIVE]: 'success',
  [ListingStatus.LISTING_STATUS_CLOSED]: 'neutral',
};

// Application status

/** Status of an application as seen by the listing author or in lists. */
export const applicationStatusLabel: Record<ApplicationStatus, string> = {
  [ApplicationStatus.APPLICATION_STATUS_UNSPECIFIED]: 'Неизвестно',
  [ApplicationStatus.APPLICATION_STATUS_PENDING]: 'Ожидает',
  [ApplicationStatus.APPLICATION_STATUS_ACCEPTED]: 'Принят',
  [ApplicationStatus.APPLICATION_STATUS_DECLINED]: 'Отклонён',
  [ApplicationStatus.APPLICATION_STATUS_WITHDRAWN]: 'Отозван',
};

/** Badge on a listing card: status of the current user's own application. */
export const myApplicationStatusLabel: Record<ApplicationStatus, string> = {
  [ApplicationStatus.APPLICATION_STATUS_UNSPECIFIED]: 'Вы откликнулись',
  [ApplicationStatus.APPLICATION_STATUS_PENDING]: 'Вы откликнулись',
  [ApplicationStatus.APPLICATION_STATUS_ACCEPTED]: 'Вас приняли',
  [ApplicationStatus.APPLICATION_STATUS_DECLINED]: 'Отказ',
  [ApplicationStatus.APPLICATION_STATUS_WITHDRAWN]: 'Отклик отозван',
};

export const applicationStatusTone: Record<ApplicationStatus, BadgeTone> = {
  [ApplicationStatus.APPLICATION_STATUS_UNSPECIFIED]: 'neutral',
  [ApplicationStatus.APPLICATION_STATUS_PENDING]: 'warning',
  [ApplicationStatus.APPLICATION_STATUS_ACCEPTED]: 'success',
  [ApplicationStatus.APPLICATION_STATUS_DECLINED]: 'danger',
  [ApplicationStatus.APPLICATION_STATUS_WITHDRAWN]: 'neutral',
};

// Plurals

/** Russian plural forms: [one, few, many], e.g. ['отклик', 'отклика', 'откликов']. */
export type PluralForms = readonly [string, string, string];

export const PLURAL_APPLICATIONS: PluralForms = ['отклик', 'отклика', 'откликов'];
export const PLURAL_LISTINGS: PluralForms = ['публикация', 'публикации', 'публикаций'];
export const PLURAL_TAGS: PluralForms = ['тег', 'тега', 'тегов'];

/** Picks the Russian plural form for `n`: 1 отклик, 2 отклика, 5 откликов, 21 отклик. */
export function pluralize(n: number, forms: PluralForms): string {
  const abs = Math.abs(Math.trunc(n));
  const mod10 = abs % 10;
  const mod100 = abs % 100;
  if (mod10 === 1 && mod100 !== 11) {
    return forms[0];
  }
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) {
    return forms[1];
  }
  return forms[2];
}

/** `formatCount(3, PLURAL_APPLICATIONS)` → «3 отклика». */
export function formatCount(n: number, forms: PluralForms): string {
  return `${n} ${pluralize(n, forms)}`;
}
