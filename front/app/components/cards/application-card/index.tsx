import { defineComponent } from 'vue';
import { NuxtLink } from '#components';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Avatar } from '~/components/ui/avatar';
import { Badge } from '~/components/ui/badge';
import { Card } from '~/components/ui/card';
import { formatRelative } from '~/utils/dates';
import {
  applicationStatusLabel,
  applicationStatusTone,
  listingKindLabel,
  listingKindTone,
} from '~/utils/labels';
import { ListingStatus } from '~~/api/types';
import type { Application } from '~~/api/types';
import styles from './index.module.css';

export type ApplicationCardMode = 'incoming' | 'outgoing';

const props = {
  application: prop<Application>().required(),
  /**
   * `incoming` — the viewer is the listing author looking at someone's application;
   * `outgoing` — the viewer's own application.
   */
  mode: prop<ApplicationCardMode>().required(),
};

export type ApplicationCardProps = ExtractProps<typeof props>;

export const ApplicationCard = defineComponent((props: ApplicationCardProps, { slots }) => {
  const renderTime = (a: Application) => (
    <span class={styles.time}>
      <time datetime={a.createdAt}>{formatRelative(a.createdAt)}</time>
      {a.decidedAt && (
        <>
          {' · '}
          <time datetime={a.decidedAt}>{`решение ${formatRelative(a.decidedAt)}`}</time>
        </>
      )}
    </span>
  );

  const renderMessage = (a: Application) => (
    a.message ? <p class={styles.message}>{a.message}</p> : null
  );

  return () => {
    const a = props.application;
    const listing = a.listing;
    const actions = slots.actions?.();
    const status = (
      <Badge tone={applicationStatusTone[a.status]} size={'sm'} dot>{applicationStatusLabel[a.status]}</Badge>
    );

    if (props.mode === 'incoming') {
      return (
        <Card as={'article'} class={styles.card} data-testid={'application-card'}>
          <div class={styles.head}>
            <Avatar user={a.applicant} size={40} />
            <div class={styles.who}>
              <NuxtLink to={`/users/${a.applicant.id}`} class={styles.name}>{a.applicant.displayName}</NuxtLink>
              {a.applicant.headline && <span class={styles.headline}>{a.applicant.headline}</span>}
            </div>
          </div>
          {listing && (
            <NuxtLink to={`/listings/${listing.id}`} class={styles.listingRef}>{listing.title}</NuxtLink>
          )}
          {renderMessage(a)}
          <div class={styles.meta}>
            {status}
            {renderTime(a)}
          </div>
          {actions && <div class={styles.actions}>{actions}</div>}
        </Card>
      );
    }

    const closed = listing?.status === ListingStatus.LISTING_STATUS_CLOSED;
    return (
      <Card as={'article'} class={styles.card} data-testid={'application-card'}>
        {listing && (
          <>
            <div class={styles.badges}>
              <Badge tone={listingKindTone[listing.kind]} size={'sm'}>{listingKindLabel[listing.kind]}</Badge>
              {closed && <Badge tone={'neutral'} size={'sm'}>{'Публикация закрыта'}</Badge>}
            </div>
            <h3 class={styles.title}>
              <NuxtLink to={`/listings/${listing.id}`} class={styles.titleLink}>{listing.title}</NuxtLink>
            </h3>
            {listing.author && (
              <span class={styles.author}>
                <Avatar user={listing.author} size={24} />
                <span class={styles.authorName}>{listing.author.displayName}</span>
              </span>
            )}
          </>
        )}
        {renderMessage(a)}
        <div class={styles.meta}>
          {status}
          {renderTime(a)}
        </div>
        {actions && <div class={styles.actions}>{actions}</div>}
      </Card>
    );
  };
}, { props, name: 'ApplicationCard' });
