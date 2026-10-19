import { defineComponent } from 'vue';
import { Card } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import styles from './index.module.css';

/** Placeholder of the listing details page while the first request is running. */
export const ListingSkeleton = defineComponent(() => {
  return () => (
    <div class={styles.skeleton} aria-busy={'true'} aria-label={'Загрузка публикации'} data-testid={'listing-skeleton'}>
      <div class={styles.main}>
        <div class={styles.badges}>
          <Skeleton width={72} height={24} radius={999} />
          <Skeleton width={56} height={24} radius={999} />
        </div>
        <Skeleton height={32} width={'80%'} />
        <div class={styles.author}>
          <Skeleton circle width={40} height={40} />
          <Skeleton width={160} height={16} />
        </div>
        <Card>
          <Skeleton lines={5} />
          <div class={styles.gap} />
          <Skeleton lines={2} />
        </Card>
      </div>
      <div>
        <Card>
          <Skeleton height={44} radius={12} />
        </Card>
      </div>
    </div>
  );
}, { name: 'ListingSkeleton' });
