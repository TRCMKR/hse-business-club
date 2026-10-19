import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { ApplicationCard } from '~/components/cards/application-card';
import type { ApplicationCardMode } from '~/components/cards/application-card';
import { Skeleton } from '~/components/ui/skeleton';
import { Card } from '~/components/ui/card';
import { ApplicationStatus } from '~~/api/types';
import type { Application } from '~~/api/types';
import { ApplicationActions } from '../application-actions';
import styles from './index.module.css';

const props = {
  items: prop<Application[]>().required(),
  mode: prop<ApplicationCardMode>().required(),
};

export type ApplicationListProps = ExtractProps<typeof props>;

/** Application cards with actions for pending ones; re-emits `updated` / `stale` of the actions. */
export const ApplicationList = defineComponent((props: ApplicationListProps, { emit }) => {
  return () => (
    <ul class={styles.list}>
      {props.items.map(a => (
        <li key={a.id} class={styles.item}>
          <ApplicationCard
            application={a}
            mode={props.mode}
            v-slots={a.status === ApplicationStatus.APPLICATION_STATUS_PENDING
              ? {
                actions: () => (
                  <ApplicationActions
                    application={a}
                    mode={props.mode}
                    onUpdated={(next: Application) => emit('updated', next)}
                    onStale={(id: string) => emit('stale', id)}
                  />
                ),
              }
              : undefined}
          />
        </li>
      ))}
    </ul>
  );
}, {
  props,
  name: 'ApplicationList',
  emits: {
    updated: (_application: Application) => true,
    stale: (_id: string) => true,
  },
});

/** Placeholder while the first page of applications is loading. */
const skeletonProps = {
  count: prop<number>().optional(3),
};

export const ApplicationListSkeleton = defineComponent((props: ExtractProps<typeof skeletonProps>) => {
  return () => (
    <ul class={styles.list} aria-busy={'true'} aria-label={'Загрузка откликов'}>
      {Array.from({ length: props.count ?? 3 }, (_, i) => (
        <li key={i} class={styles.item}>
          <Card class={styles.skeleton}>
            <div class={styles.skeletonHead}>
              <Skeleton circle width={40} height={40} />
              <Skeleton width={'40%'} height={18} />
            </div>
            <Skeleton lines={2} />
          </Card>
        </li>
      ))}
    </ul>
  );
}, { props: skeletonProps, name: 'ApplicationListSkeleton' });

/**
 * Replaces one application in a list with the server's new state, keeping nested
 * objects the response may omit.
 */
export function patchApplication(items: Application[] | null | undefined, next: Application): Application[] {
  return (items ?? []).map(a => (a.id === next.id
    ? { ...a, ...next, listing: next.listing ?? a.listing, applicant: next.applicant ?? a.applicant }
    : a));
}
