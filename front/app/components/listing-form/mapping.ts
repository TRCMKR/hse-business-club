/**
 * Listing form state and its mapping to API requests (SPEC §2.4, §4.1).
 * Pure functions: no Vue, no API calls — covered by `tests/listing-form.test.ts`.
 */
import { ListingKind, StartupStage } from '~~/api/types';
import type { Listing, ListingInput, ListingUpdateInput } from '~~/api/types';

/** Values edited by `ListingForm`. `stage` is UNSPECIFIED until chosen. */
export interface ListingFormState {
  kind: ListingKind
  title: string
  description: string
  lookingFor: string
  tags: string[]
  stage: StartupStage
  compensation: string
}

/** Blank form for a new listing of the given kind. */
export function emptyListingForm(kind: ListingKind = ListingKind.LISTING_KIND_STARTUP): ListingFormState {
  return {
    kind,
    title: '',
    description: '',
    lookingFor: '',
    tags: [],
    stage: StartupStage.STARTUP_STAGE_UNSPECIFIED,
    compensation: '',
  };
}

/** Prefill for the edit form. */
export function fromListing(listing: Listing): ListingFormState {
  const isStartup = listing.kind === ListingKind.LISTING_KIND_STARTUP;
  return {
    kind: listing.kind,
    title: listing.title,
    description: listing.description,
    lookingFor: listing.lookingFor,
    tags: [...listing.tags],
    stage: isStartup ? listing.stage : StartupStage.STARTUP_STAGE_UNSPECIFIED,
    compensation: isStartup ? '' : listing.compensation,
  };
}

/** Kind of a new listing from `?kind=startup|vacancy`; startup by default. */
export function kindFromQuery(value: unknown): ListingKind {
  return value === 'vacancy' || value === ListingKind.LISTING_KIND_VACANCY
    ? ListingKind.LISTING_KIND_VACANCY
    : ListingKind.LISTING_KIND_STARTUP;
}

function cleanTags(tags: readonly string[]): string[] {
  return tags.map(tag => tag.trim()).filter(tag => tag !== '');
}

/**
 * Body of `PUT /v1/listings/{id}`: trimmed strings, no empty tags; fields of the other
 * kind are reset (stage → UNSPECIFIED for vacancies, compensation → '' for startups),
 * as the server requires.
 */
export function toUpdateRequest(values: ListingFormState): ListingUpdateInput {
  const isStartup = values.kind === ListingKind.LISTING_KIND_STARTUP;
  return {
    title: values.title.trim(),
    description: values.description.trim(),
    lookingFor: values.lookingFor.trim(),
    tags: cleanTags(values.tags),
    stage: isStartup ? values.stage : StartupStage.STARTUP_STAGE_UNSPECIFIED,
    compensation: isStartup ? '' : values.compensation.trim(),
  };
}

/** Body of `POST /v1/listings` — same normalization as `toUpdateRequest` plus `kind`. */
export function toCreateRequest(values: ListingFormState): ListingInput {
  return { kind: values.kind, ...toUpdateRequest(values) };
}
