import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { ListingCard } from '~/components/cards/listing-card';
import { EmptyState } from '~/components/states/empty-state';
import { ErrorState } from '~/components/states/error-state';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Icon } from '~/components/ui/icon';
import { Skeleton } from '~/components/ui/skeleton';
import { getErrorMessage } from '~/utils/errors';
import type { Listing } from '~~/api/types';
import styles from './index.module.css';

/** Number of skeleton cards while the first page loads. */
export const FEED_SKELETON_COUNT = 3;

const props = {
  items: prop<Listing[]>().required(),
  /** First page is loading → skeletons. */
  loading: prop<boolean>().optional(false),
  /** First page failed → error state with «Повторить». */
  error: prop<unknown>().optional(),
  /** Retry in progress (spinner on «Повторить»). */
  retrying: prop<boolean>().optional(false),
  hasMore: prop<boolean>().optional(false),
  loadingMore: prop<boolean>().optional(false),
  /** Any filter is set: changes the empty state («Ничего не нашлось» + reset). */
  filtered: prop<boolean>().optional(false),
};

export type FeedListProps = ExtractProps<typeof props>;

const SkeletonCard = () => (
  <Card class={styles.skeleton} data-testid={'feed-skeleton'}>
    <Skeleton width={84} height={20} radius={999} />
    <Skeleton height={22} width={'70%'} />
    <Skeleton lines={2} />
    <div class={styles.skeletonAuthor}>
      <Skeleton circle width={24} />
      <Skeleton width={120} height={14} />
    </div>
  </Card>
);

/** Feed body: skeletons / error / empty states / cards grid + «Показать ещё». */
export const FeedList = defineComponent((props: FeedListProps, { emit }) => {
  return () => {
    if (props.error) {
      return (
        <ErrorState
          message={getErrorMessage(props.error)}
          retry
          retrying={props.retrying}
          onRetry={() => emit('retry')}
        />
      );
    }

    if (props.loading) {
      return (
        <div class={styles.grid} aria-busy={'true'} aria-label={'Загрузка публикаций'}>
          {Array.from({ length: FEED_SKELETON_COUNT }, (_, i) => <SkeletonCard key={i} />)}
        </div>
      );
    }

    if (!props.items.length) {
      return props.filtered
        ? (
          <EmptyState
            title={'Ничего не нашлось'}
            description={'Попробуйте изменить запрос или выбрать другой тип публикаций.'}
            v-slots={{
              icon: () => <Icon name={'search'} size={32} />,
              actions: () => (
                <Button variant={'secondary'} onClick={() => emit('reset')}>{'Сбросить фильтры'}</Button>
              ),
            }}
          />
        )
        : (
          <EmptyState
            title={'Пока нет предложений'}
            description={'Расскажите о своём стартапе или вакансии — участники клуба откликнутся.'}
            v-slots={{
              icon: () => <Icon name={'rocket'} size={32} />,
              actions: () => (
                <Button to={'/listings/new'} icon={'plus'}>{'Создать публикацию'}</Button>
              ),
            }}
          />
        );
    }

    return (
      <div class={styles.list}>
        <ul class={styles.grid} aria-label={'Публикации'}>
          {props.items.map(l => (
            <li key={l.id} class={styles.item}>
              <ListingCard listing={l} />
            </li>
          ))}
        </ul>
        <div class={styles.footer}>
          {props.hasMore
            ? (
              <Button
                variant={'secondary'}
                loading={props.loadingMore}
                data-testid={'feed-load-more'}
                onClick={() => emit('loadMore')}
              >
                {'Показать ещё'}
              </Button>
            )
            : <p class={styles.end} data-testid={'feed-end'}>{'Это всё'}</p>}
        </div>
      </div>
    );
  };
}, {
  props,
  name: 'FeedList',
  emits: {
    loadMore: () => true,
    retry: () => true,
    reset: () => true,
  },
});
