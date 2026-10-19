import { defineComponent, ref } from 'vue';
import { toast } from 'vue-sonner';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import { getErrorMessage } from '~/utils/errors';
import type { ListingStats } from '~~/api/types';
import styles from './index.module.css';

export const CLOSE_SUCCESS_MESSAGE = 'Публикация закрыта';

const EMPTY_STATS: ListingStats = { total: 0, pending: 0, accepted: 0, declined: 0 };

const props = {
  listingId: prop<string>().required(),
  /** `listing.stats`; withdrawn applications are not counted (SPEC §4.2). */
  stats: prop<ListingStats | null>().required(),
  canEdit: prop<boolean>().optional(false),
  canClose: prop<boolean>().optional(false),
  /** `api.listings.close(id)` — idempotent; called after confirmation. */
  close: prop<() => Promise<unknown>>().required(),
};

export type OwnerPanelProps = ExtractProps<typeof props>;

/**
 * Author's panel: stats (ожидают · приняты · отклонены), «Отклики (N)» →
 * `/listings/:id/applications`, «Редактировать» (active only), «Закрыть» behind a
 * confirmation. Emits `closed` after a successful close (a repeated close is a 200 too).
 */
export const OwnerPanel = defineComponent((props: OwnerPanelProps, { emit }) => {
  const confirmOpen = ref(false);
  const loading = ref(false);

  const onConfirm = async () => {
    if (loading.value) {
      return;
    }
    loading.value = true;
    try {
      await props.close();
      confirmOpen.value = false;
      toast.success(CLOSE_SUCCESS_MESSAGE);
      emit('closed');
    } catch (e) {
      confirmOpen.value = false;
      toast.error(getErrorMessage(e));
      emit('failed', e);
    } finally {
      loading.value = false;
    }
  };

  return () => {
    const s = props.stats ?? EMPTY_STATS;
    const items = [
      { key: 'pending', value: s.pending, label: 'ожидают', cls: styles.pending },
      { key: 'accepted', value: s.accepted, label: 'приняты', cls: styles.accepted },
      { key: 'declined', value: s.declined, label: 'отклонены', cls: styles.declined },
    ];

    return (
      <Card as={'section'} class={styles.panel} data-testid={'owner-panel'} aria-labelledby={'owner-panel-title'}>
        <h2 id={'owner-panel-title'} class={styles.title}>{'Отклики'}</h2>

        <dl class={styles.stats} data-testid={'owner-stats'}>
          {items.map(item => (
            <div key={item.key} class={styles.stat}>
              <dt class={styles.statLabel}>{item.label}</dt>
              <dd class={[styles.statValue, item.value > 0 && item.cls]}>{item.value}</dd>
            </div>
          ))}
        </dl>

        <div class={styles.actions}>
          <Button to={`/listings/${props.listingId}/applications`} block icon={'inbox'} data-testid={'owner-applications'}>
            {`Отклики (${s.total})`}
          </Button>
          {props.canEdit && (
            <Button to={`/listings/${props.listingId}/edit`} variant={'secondary'} block icon={'edit'}>
              {'Редактировать'}
            </Button>
          )}
          {props.canClose && (
            <Button
              variant={'ghost'}
              block
              loading={loading.value}
              onClick={() => (confirmOpen.value = true)}
              data-testid={'owner-close'}
            >
              {'Закрыть публикацию'}
            </Button>
          )}
        </div>

        <ConfirmDialog
          open={confirmOpen.value}
          title={'Закрыть публикацию?'}
          message={'Она пропадёт из ленты и перестанет принимать отклики. Редактировать её будет нельзя, открыть снова — тоже.'}
          confirmText={'Закрыть'}
          tone={'danger'}
          loading={loading.value}
          onUpdate:open={(v: boolean) => (confirmOpen.value = v)}
          onConfirm={onConfirm}
        />
      </Card>
    );
  };
}, {
  props,
  name: 'OwnerPanel',
  emits: {
    closed: () => true,
    failed: (_e: unknown) => true,
  },
});
