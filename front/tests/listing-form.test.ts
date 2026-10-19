// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { ValidationError } from 'yup';
import { ListingKind, StartupStage } from '~~/api/types';
import {
  emptyListingForm,
  fromListing,
  kindFromQuery,
  toCreateRequest,
  toUpdateRequest,
} from '~/components/listing-form/mapping';
import type { ListingFormState } from '~/components/listing-form/mapping';
import { listingFormSchema } from '~/utils/validation';
import { makeListing, makeVacancy } from './cards-fixtures';

const startup = (over: Partial<ListingFormState> = {}): ListingFormState => ({
  kind: ListingKind.LISTING_KIND_STARTUP,
  title: 'Сервис аренды',
  description: 'Подробное описание проекта',
  lookingFor: 'CTO',
  tags: ['Go', 'Vue'],
  stage: StartupStage.STARTUP_STAGE_MVP,
  compensation: '',
  ...over,
});

const vacancy = (over: Partial<ListingFormState> = {}): ListingFormState => ({
  ...startup(),
  kind: ListingKind.LISTING_KIND_VACANCY,
  stage: StartupStage.STARTUP_STAGE_UNSPECIFIED,
  compensation: '80 000 ₽, удалёнка',
  ...over,
});

async function errorsOf(value: unknown): Promise<Record<string, string>> {
  try {
    await listingFormSchema.validate(value, { abortEarly: false });
    return {};
  } catch (error) {
    if (!(error instanceof ValidationError)) {
      throw error;
    }
    return Object.fromEntries(error.inner.map(e => [e.path ?? '', e.message]));
  }
}

describe('listing form schema', () => {
  it('accepts a valid startup and a valid vacancy', async () => {
    expect(await errorsOf(startup())).toEqual({});
    expect(await errorsOf(vacancy())).toEqual({});
  });

  it('stage is required for a startup', async () => {
    expect(await errorsOf(startup({ stage: StartupStage.STARTUP_STAGE_UNSPECIFIED })))
      .toEqual({ stage: 'Стадия: укажите стадию' });
    expect(await errorsOf({ ...startup(), stage: '' })).toEqual({ stage: 'Стадия: укажите стадию' });
  });

  it('stage is forbidden for a vacancy', async () => {
    expect(await errorsOf(vacancy({ stage: StartupStage.STARTUP_STAGE_IDEA })))
      .toEqual({ stage: 'Стадия: только для стартапа' });
  });

  it('compensation only for a vacancy, at most 120 characters', async () => {
    expect(await errorsOf(startup({ compensation: 'Доля' }))).toEqual({ compensation: 'Условия: только для вакансий' });
    expect(await errorsOf(vacancy({ compensation: 'я'.repeat(120) }))).toEqual({});
    expect(await errorsOf(vacancy({ compensation: 'я'.repeat(121) })))
      .toEqual({ compensation: 'Условия: не больше 120 символов' });
  });

  it('tags: at most 10, no case-insensitive duplicates, each 1–40', async () => {
    const ten = Array.from({ length: 10 }, (_, i) => `tag${i}`);
    expect(await errorsOf(startup({ tags: ten }))).toEqual({});
    expect(await errorsOf(startup({ tags: [...ten, 'tag10'] }))).toEqual({ tags: 'Теги: не больше 10' });
    expect(await errorsOf(startup({ tags: ['Go', 'go'] }))).toEqual({ tags: 'Теги: без повторов' });
    expect(await errorsOf(startup({ tags: ['x'.repeat(41)] }))).toEqual({ tags: 'Теги: каждый от 1 до 40 символов' });
    expect(await errorsOf(startup({ tags: [] }))).toEqual({});
  });

  it('length limits of text fields (trimmed, in code points)', async () => {
    expect(await errorsOf(startup({ title: '  ab  ' }))).toEqual({ title: 'Название: от 3 до 120 символов' });
    expect(await errorsOf(startup({ title: 'я'.repeat(121) }))).toEqual({ title: 'Название: от 3 до 120 символов' });
    expect(await errorsOf(startup({ title: '🚀🚀🚀' }))).toEqual({});
    expect(await errorsOf(startup({ description: 'коротко' }))).toEqual({ description: 'Описание: от 10 до 5000 символов' });
    expect(await errorsOf(startup({ description: 'я'.repeat(5001) }))).toEqual({ description: 'Описание: от 10 до 5000 символов' });
    expect(await errorsOf(startup({ lookingFor: '   ' }))).toEqual({ lookingFor: 'Кого ищете: от 1 до 1000 символов' });
    expect(await errorsOf(startup({ lookingFor: 'я'.repeat(1001) }))).toEqual({ lookingFor: 'Кого ищете: от 1 до 1000 символов' });
  });
});

describe('toCreateRequest', () => {
  it('maps a startup: trims, drops empty tags, clears compensation', () => {
    expect(toCreateRequest(startup({
      title: '  Сервис аренды  ',
      description: ' Подробное описание проекта\n',
      lookingFor: ' CTO ',
      tags: [' Go ', '', '  ', 'Vue'],
      compensation: 'остаток от вакансии',
    }))).toEqual({
      kind: ListingKind.LISTING_KIND_STARTUP,
      title: 'Сервис аренды',
      description: 'Подробное описание проекта',
      lookingFor: 'CTO',
      tags: ['Go', 'Vue'],
      stage: StartupStage.STARTUP_STAGE_MVP,
      compensation: '',
    });
  });

  it('maps a vacancy: stage is always UNSPECIFIED', () => {
    expect(toCreateRequest(vacancy({ stage: StartupStage.STARTUP_STAGE_IDEA, compensation: ' Доля 5% ' }))).toEqual({
      kind: ListingKind.LISTING_KIND_VACANCY,
      title: 'Сервис аренды',
      description: 'Подробное описание проекта',
      lookingFor: 'CTO',
      tags: ['Go', 'Vue'],
      stage: StartupStage.STARTUP_STAGE_UNSPECIFIED,
      compensation: 'Доля 5%',
    });
  });

  it('toUpdateRequest has no kind', () => {
    const body = toUpdateRequest(vacancy());
    expect(body).not.toHaveProperty('kind');
    const { kind: _kind, ...rest } = toCreateRequest(startup());
    expect(toUpdateRequest(startup())).toEqual(rest);
  });
});

describe('form state helpers', () => {
  it('emptyListingForm', () => {
    expect(emptyListingForm()).toEqual({
      kind: ListingKind.LISTING_KIND_STARTUP,
      title: '',
      description: '',
      lookingFor: '',
      tags: [],
      stage: StartupStage.STARTUP_STAGE_UNSPECIFIED,
      compensation: '',
    });
    expect(emptyListingForm(ListingKind.LISTING_KIND_VACANCY).kind).toBe(ListingKind.LISTING_KIND_VACANCY);
  });

  it('fromListing round-trips through toUpdateRequest', () => {
    const l = makeListing();
    const state = fromListing(l);
    expect(state.kind).toBe(l.kind);
    expect(toUpdateRequest(state)).toEqual({
      title: l.title,
      description: l.description,
      lookingFor: l.lookingFor,
      tags: l.tags,
      stage: l.stage,
      compensation: '',
    });
    expect(state.tags).not.toBe(l.tags);

    const v = makeVacancy();
    expect(fromListing(v)).toMatchObject({ stage: StartupStage.STARTUP_STAGE_UNSPECIFIED, compensation: v.compensation });
  });

  it('kindFromQuery', () => {
    expect(kindFromQuery('vacancy')).toBe(ListingKind.LISTING_KIND_VACANCY);
    expect(kindFromQuery(ListingKind.LISTING_KIND_VACANCY)).toBe(ListingKind.LISTING_KIND_VACANCY);
    expect(kindFromQuery('startup')).toBe(ListingKind.LISTING_KIND_STARTUP);
    expect(kindFromQuery(undefined)).toBe(ListingKind.LISTING_KIND_STARTUP);
    expect(kindFromQuery(['vacancy'])).toBe(ListingKind.LISTING_KIND_STARTUP);
  });
});
