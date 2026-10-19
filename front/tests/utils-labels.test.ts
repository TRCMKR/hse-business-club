// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { ApplicationStatus, ListingKind, ListingStatus, StartupStage } from '~~/api/types';
import {
  applicationStatusLabel,
  applicationStatusTone,
  formatCount,
  listingKindLabel,
  listingKindOptions,
  listingKindTone,
  listingStatusLabel,
  listingStatusTone,
  myApplicationStatusLabel,
  PLURAL_APPLICATIONS,
  pluralize,
  startupStageLabel,
  startupStageOptions,
  startupStageTone,
} from '~/utils/labels';

const TONES = ['neutral', 'accent', 'success', 'warning', 'danger', 'info'];

const cases: [string, object, Record<string, string>, Record<string, string>][] = [
  ['ListingKind', ListingKind, listingKindLabel, listingKindTone],
  ['ListingStatus', ListingStatus, listingStatusLabel, listingStatusTone],
  ['StartupStage', StartupStage, startupStageLabel, startupStageTone],
  ['ApplicationStatus', ApplicationStatus, applicationStatusLabel, applicationStatusTone],
  ['ApplicationStatus (mine)', ApplicationStatus, myApplicationStatusLabel, applicationStatusTone],
];

describe('labels', () => {
  it.each(cases)('%s: every value has a label and a tone', (_, values, labels, tones) => {
    for (const value of Object.values(values)) {
      expect(labels[value]?.trim()).toBeTruthy();
      expect(TONES).toContain(tones[value]);
    }
  });

  it('feed options start with «Все» and cover both kinds', () => {
    expect(listingKindOptions.map(o => o.value)).toEqual([
      ListingKind.LISTING_KIND_UNSPECIFIED,
      ListingKind.LISTING_KIND_STARTUP,
      ListingKind.LISTING_KIND_VACANCY,
    ]);
    expect(listingKindOptions.map(o => o.label)).toEqual(['Все', 'Стартапы', 'Вакансии']);
  });

  it('stage options exclude UNSPECIFIED', () => {
    expect(startupStageOptions.map(o => o.label)).toEqual(['Идея', 'Прототип', 'MVP', 'Запущен']);
  });

  it('application tones follow the spec', () => {
    expect(applicationStatusTone[ApplicationStatus.APPLICATION_STATUS_PENDING]).toBe('warning');
    expect(applicationStatusTone[ApplicationStatus.APPLICATION_STATUS_ACCEPTED]).toBe('success');
    expect(applicationStatusTone[ApplicationStatus.APPLICATION_STATUS_DECLINED]).toBe('danger');
    expect(applicationStatusTone[ApplicationStatus.APPLICATION_STATUS_WITHDRAWN]).toBe('neutral');
    expect(listingStatusTone[ListingStatus.LISTING_STATUS_CLOSED]).toBe('neutral');
  });
});

describe('pluralize', () => {
  it.each([
    [0, 'откликов'],
    [1, 'отклик'],
    [2, 'отклика'],
    [4, 'отклика'],
    [5, 'откликов'],
    [11, 'откликов'],
    [12, 'откликов'],
    [14, 'откликов'],
    [21, 'отклик'],
    [22, 'отклика'],
    [25, 'откликов'],
    [101, 'отклик'],
    [111, 'откликов'],
  ])('%i → %s', (n, form) => {
    expect(pluralize(n, PLURAL_APPLICATIONS)).toBe(form);
  });

  it('formatCount', () => {
    expect(formatCount(3, PLURAL_APPLICATIONS)).toBe('3 отклика');
    expect(formatCount(21, ['участник', 'участника', 'участников'])).toBe('21 участник');
  });
});
