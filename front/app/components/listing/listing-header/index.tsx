import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Badge } from '~/components/ui/badge';
import { UserSummary } from '~/components/cards/user-summary';
import { formatDateTime, formatRelative } from '~/utils/dates';
import { listingKindLabel, listingKindTone, startupStageLabel } from '~/utils/labels';
import { ListingKind, ListingStatus, StartupStage } from '~~/api/types';
import type { Listing } from '~~/api/types';
import styles from './index.module.css';

const props = {
  listing: prop<Listing>().required(),
};

export type ListingHeaderProps = ExtractProps<typeof props>;

/** Kind / stage / status badges, title (h1), author and relative publication date. */
export const ListingHeader = defineComponent((props: ListingHeaderProps) => {
  return () => {
    const l = props.listing;
    const closed = l.status === ListingStatus.LISTING_STATUS_CLOSED;
    const hasStage = l.kind === ListingKind.LISTING_KIND_STARTUP
      && l.stage !== StartupStage.STARTUP_STAGE_UNSPECIFIED;

    return (
      <header class={styles.header} data-testid={'listing-header'}>
        <div class={styles.badges}>
          <Badge tone={listingKindTone[l.kind]}>{listingKindLabel[l.kind]}</Badge>
          {hasStage && <Badge tone={'neutral'}>{startupStageLabel[l.stage]}</Badge>}
          {l.isMine && <Badge tone={'accent'} dot>{'Ваша публикация'}</Badge>}
        </div>

        {closed && (
          <div class={styles.closed} role={'status'} data-testid={'listing-closed'}>
            <span class={styles.closedTitle}>{'Публикация закрыта'}</span>
            {l.closedAt && (
              <time class={styles.closedAt} datetime={l.closedAt} title={formatDateTime(l.closedAt)}>
                {formatRelative(l.closedAt)}
              </time>
            )}
          </div>
        )}

        <h1 class={styles.title}>{l.title}</h1>

        <div class={styles.meta}>
          <UserSummary user={l.author} size={'sm'} to={`/users/${l.author.id}`} />
          <time class={styles.date} datetime={l.createdAt} title={formatDateTime(l.createdAt)}>
            {formatRelative(l.createdAt)}
          </time>
        </div>
      </header>
    );
  };
}, { props, name: 'ListingHeader' });
