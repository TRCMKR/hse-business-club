import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Card } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import styles from './index.module.css';

const props = {
  count: prop<number>().optional(3),
};

export type MyListingsSkeletonProps = ExtractProps<typeof props>;

/** Placeholder cards while «Мои публикации» load. */
export const MyListingsSkeleton = defineComponent((props: MyListingsSkeletonProps) => {
  return () => (
    <div class={styles.list} aria-busy={'true'} data-testid={'my-listings-skeleton'}>
      {Array.from({ length: props.count ?? 3 }, (_, i) => (
        <Card key={i} class={styles.card}>
          <Skeleton width={120} height={20} radius={10} />
          <Skeleton width={'70%'} height={22} />
          <Skeleton lines={2} />
          <Skeleton width={160} height={14} />
        </Card>
      ))}
    </div>
  );
}, { props, name: 'MyListingsSkeleton' });
