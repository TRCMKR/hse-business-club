/**
 * Domain types used by the app. The backend serializes with protojson
 * `EmitUnpopulated: true` (SPEC §4.2), so every scalar/array field is always present
 * and only nested messages / timestamps may be `null`. The generated types mark
 * everything optional; here we tighten them once so pages and components don't have
 * to null-check every field.
 */
import type {
  RpcStatus,
  V1Application,
  V1Listing,
  V1ListingStats,
  V1ListingSummary,
  V1User,
  V1V1AuthResponse,
} from './generated/hbc';
import {
  V1ApplicationStatus,
  V1ListingKind,
  V1ListingStatus,
  V1StartupStage,
} from './generated/hbc';

export {
  V1ApplicationStatus as ApplicationStatus,
  V1ListingKind as ListingKind,
  V1ListingStatus as ListingStatus,
  V1StartupStage as StartupStage,
};

export type User = Required<V1User>;

export type ListingStats = Required<V1ListingStats>;

export type ListingSummary = Required<Omit<V1ListingSummary, 'author'>> & {
  author: User
};

export type Application = Required<Omit<V1Application, 'listing' | 'applicant' | 'decidedAt'>> & {
  listing: ListingSummary
  applicant: User
  /** RFC 3339; null for pending and withdrawn. */
  decidedAt: string | null
};

export type Listing = Required<
  Omit<V1Listing, 'author' | 'myApplication' | 'stats' | 'closedAt'>
> & {
  author: User
  /** Current user's non-withdrawn application, or null. */
  myApplication: Application | null
  /** Only for isMine = true, otherwise null. */
  stats: ListingStats | null
  /** RFC 3339; null while active. */
  closedAt: string | null
};

export interface ListingsPage {
  items: Listing[]
  total: number
  page: number
  limit: number
}

/** grpc-gateway error body (SPEC §4.2). */
export type ApiErrorBody = Required<Pick<RpcStatus, 'code' | 'message'>> & Pick<RpcStatus, 'details'>;

/**
 * `V1Login` / `V1Register` / `V1Refresh` response. Server-only: the Nitro routes keep the
 * tokens in httpOnly cookies and hand the browser only `{ user }`.
 */
export type AuthResponse = Required<Omit<V1V1AuthResponse, 'user'>> & { user: User };

export interface ListingInput {
  kind: V1ListingKind
  title: string
  description: string
  lookingFor: string
  tags: string[]
  /** STARTUP only; UNSPECIFIED for vacancies. */
  stage: V1StartupStage
  /** VACANCY only; '' for startups. */
  compensation: string
}

export type ListingUpdateInput = Omit<ListingInput, 'kind'>;

export interface ProfileInput {
  displayName: string
  headline: string
  about: string
  skills: string[]
  telegram: string
}

export interface LoginInput {
  username: string
  password: string
}

export interface RegisterInput extends LoginInput {
  displayName: string
}
