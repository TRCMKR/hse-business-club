/**
 * Which action blocks the listing details page shows for the current viewer.
 * Pure function of the `GET /v1/listings/{id}` response (SPEC §2.3), table-tested.
 */
import { ApplicationStatus, ListingStatus } from '~~/api/types';
import type { Listing } from '~~/api/types';

/**
 * State of the «Откликнуться» button:
 * - `enabled` — someone else's active listing without my application;
 * - `disabled` — someone else's closed listing without my application
 *   (button disabled with the caption «Публикация закрыта»);
 * - `hidden` — my own listing, or I already have a non-withdrawn application.
 */
export type ApplyState = 'enabled' | 'disabled' | 'hidden';

export interface ListingActions {
  /** Viewer is the author: stats, «Отклики (N)», edit/close. */
  ownerPanel: boolean
  /** «Редактировать»: author and the listing is active. */
  canEdit: boolean
  /** «Закрыть»: author and the listing is active. */
  canClose: boolean
  /** Viewer has a non-withdrawn application (`myApplication`). */
  myApplication: boolean
  /** «Отозвать»: my application is pending. */
  canWithdraw: boolean
  apply: ApplyState
}

type ListingForActions = Pick<Listing, 'isMine' | 'status'> & {
  myApplication: Pick<NonNullable<Listing['myApplication']>, 'status'> | null
};

export function resolveActions(listing: ListingForActions): ListingActions {
  const active = listing.status === ListingStatus.LISTING_STATUS_ACTIVE;

  if (listing.isMine) {
    return {
      ownerPanel: true,
      canEdit: active,
      canClose: active,
      myApplication: false,
      canWithdraw: false,
      apply: 'hidden',
    };
  }

  // The API never returns a withdrawn application here, but be defensive: a withdrawn
  // one means the viewer may apply again (SPEC §2.3).
  const app = listing.myApplication;
  const hasApplication = app !== null && app.status !== ApplicationStatus.APPLICATION_STATUS_WITHDRAWN;

  if (hasApplication) {
    return {
      ownerPanel: false,
      canEdit: false,
      canClose: false,
      myApplication: true,
      canWithdraw: app.status === ApplicationStatus.APPLICATION_STATUS_PENDING,
      apply: 'hidden',
    };
  }

  return {
    ownerPanel: false,
    canEdit: false,
    canClose: false,
    myApplication: false,
    canWithdraw: false,
    apply: active ? 'enabled' : 'disabled',
  };
}
