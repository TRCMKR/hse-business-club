import { computed, defineComponent } from 'vue';
import { NuxtLink } from '#components';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Avatar } from '~/components/ui/avatar';
import { Badge } from '~/components/ui/badge';
import { Card } from '~/components/ui/card';
import { Chip } from '~/components/ui/chip';
import { formatRelative } from '~/utils/dates';
import {
  PLURAL_APPLICATIONS,
  applicationStatusTone,
  formatCount,
  listingKindLabel,
  listingKindTone,
  myApplicationStatusLabel,
  pluralize,
  startupStageLabel,
} from '~/utils/labels';
import { ListingKind, ListingStatus, StartupStage } from '~~/api/types';
import type { Listing } from '~~/api/types';
import type { PluralForms } from '~/utils/labels';
import styles from './index.module.css';

const PLURAL_NEW: PluralForms = ['новый', 'новых', 'новых'];

/** Max tag chips shown on a card; the rest collapse into «+N». */
export const LISTING_CARD_MAX_TAGS = 3;

const props = {
  listing: prop<Listing>().required(),
  /** Target of the card link; defaults to `/listings/<id>`. */
  to: prop<string>().optional(),
  showAuthor: prop<boolean>().optional(true),
  /** Hides `lookingFor` and compensation, tighter padding. */
  compact: prop<boolean>().optional(false),
};

export type ListingCardProps = ExtractProps<typeof props>;

/**
 * Listing in feeds and «Мои публикации». The title link is stretched over the whole
 * card (`::after`), so the card is clickable while the `actions` slot stays outside
 * the `<a>` and receives its own clicks.
 */
export const ListingCard = defineComponent((props: ListingCardProps, { slots }) => {
  const href = computed(() => props.to ?? `/listings/${props.listing.id}`);
  const tags = computed(() => props.listing.tags.slice(0, LISTING_CARD_MAX_TAGS));
  const hiddenTags = computed(() => Math.max(0, props.listing.tags.length - LISTING_CARD_MAX_TAGS));

  return () => {
    const l = props.listing;
    const isStartup = l.kind === ListingKind.LISTING_KIND_STARTUP;
    const isVacancy = l.kind === ListingKind.LISTING_KIND_VACANCY;
    const closed = l.status === ListingStatus.LISTING_STATUS_CLOSED;
    const hasStage = isStartup && l.stage && l.stage !== StartupStage.STARTUP_STAGE_UNSPECIFIED;
    const actions = slots.actions?.();

    return (
      <Card
        as={'article'}
        interactive
        padding={props.compact ? 'sm' : 'md'}
        class={[styles.card, closed && styles.closed]}
        data-testid={'listing-card'}
      >
        <div class={styles.badges}>
          <Badge tone={listingKindTone[l.kind]} size={'sm'}>{listingKindLabel[l.kind]}</Badge>
          {hasStage && <Badge tone={'neutral'} size={'sm'}>{startupStageLabel[l.stage]}</Badge>}
          {l.isMine && <Badge tone={'accent'} size={'sm'} dot>{'Ваша публикация'}</Badge>}
          {l.myApplication && (
            <Badge tone={applicationStatusTone[l.myApplication.status]} size={'sm'} dot>
              {myApplicationStatusLabel[l.myApplication.status]}
            </Badge>
          )}
          {closed && <Badge tone={'neutral'} size={'sm'}>{'Закрыта'}</Badge>}
        </div>

        <h3 class={styles.title}>
          <NuxtLink to={href.value} class={styles.link}>{l.title}</NuxtLink>
        </h3>

        {!props.compact && l.lookingFor && (
          <p class={styles.lookingFor}>
            <span class={styles.label}>{'Ищем: '}</span>
            {l.lookingFor}
          </p>
        )}
        {!props.compact && isVacancy && l.compensation && (
          <p class={styles.compensation}>{l.compensation}</p>
        )}

        {l.tags.length > 0 && (
          <div class={styles.tags}>
            {tags.value.map(tag => <Chip key={tag} size={'sm'} label={tag} />)}
            {hiddenTags.value > 0 && <Chip size={'sm'} label={`+${hiddenTags.value}`} />}
          </div>
        )}

        {l.stats && (
          <p class={styles.stats} data-testid={'listing-stats'}>
            {formatCount(l.stats.total, PLURAL_APPLICATIONS)}
            {l.stats.pending > 0 && (
              <>
                {' · '}
                <span class={styles.pending}>{`${l.stats.pending} ${pluralize(l.stats.pending, PLURAL_NEW)}`}</span>
              </>
            )}
          </p>
        )}

        <div class={styles.footer}>
          {props.showAuthor && l.author && (
            <span class={styles.author}>
              <Avatar user={l.author} size={24} />
              <span class={styles.authorName}>{l.author.displayName}</span>
            </span>
          )}
          <time class={styles.time} datetime={l.createdAt}>{formatRelative(l.createdAt)}</time>
        </div>

        {actions && <div class={styles.actions}>{actions}</div>}
      </Card>
    );
  };
}, { props, name: 'ListingCard' });
