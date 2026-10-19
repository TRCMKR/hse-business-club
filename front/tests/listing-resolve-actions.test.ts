// @vitest-environment node
import { describe, expect, it } from 'vitest';
import { resolveActions } from '~/components/listing/resolve-actions';
import type { ListingActions } from '~/components/listing/resolve-actions';
import { ApplicationStatus, ListingStatus } from '~~/api/types';

const ACTIVE = ListingStatus.LISTING_STATUS_ACTIVE;
const CLOSED = ListingStatus.LISTING_STATUS_CLOSED;
const { APPLICATION_STATUS_PENDING: PENDING, APPLICATION_STATUS_ACCEPTED: ACCEPTED, APPLICATION_STATUS_DECLINED: DECLINED, APPLICATION_STATUS_WITHDRAWN: WITHDRAWN } = ApplicationStatus;

const NONE: ListingActions = {
  ownerPanel: false,
  canEdit: false,
  canClose: false,
  myApplication: false,
  canWithdraw: false,
  apply: 'hidden',
};

type Row = [name: string, isMine: boolean, status: ListingStatus, app: ApplicationStatus | null, expected: Partial<ListingActions>];

const rows: Row[] = [
  ['owner, active', true, ACTIVE, null, { ownerPanel: true, canEdit: true, canClose: true }],
  ['owner, closed', true, CLOSED, null, { ownerPanel: true }],
  ['visitor, active, no application', false, ACTIVE, null, { apply: 'enabled' }],
  ['visitor, closed, no application', false, CLOSED, null, { apply: 'disabled' }],
  ['applicant, active, pending', false, ACTIVE, PENDING, { myApplication: true, canWithdraw: true }],
  ['applicant, active, accepted', false, ACTIVE, ACCEPTED, { myApplication: true }],
  ['applicant, active, declined', false, ACTIVE, DECLINED, { myApplication: true }],
  ['applicant, closed, pending', false, CLOSED, PENDING, { myApplication: true, canWithdraw: true }],
  ['applicant, closed, accepted', false, CLOSED, ACCEPTED, { myApplication: true }],
  ['withdrawn (defensive), active → apply again', false, ACTIVE, WITHDRAWN, { apply: 'enabled' }],
  ['withdrawn (defensive), closed → disabled', false, CLOSED, WITHDRAWN, { apply: 'disabled' }],
];

describe('resolveActions', () => {
  it.each(rows)('%s', (_name, isMine, status, app, expected) => {
    const actual = resolveActions({ isMine, status, myApplication: app ? { status: app } : null });
    expect(actual).toEqual({ ...NONE, ...expected });
  });
});
