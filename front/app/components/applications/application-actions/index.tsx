import { computed, defineComponent, ref } from 'vue';
import { toast } from 'vue-sonner';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Button } from '~/components/ui/button';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import type { ApplicationCardMode } from '~/components/cards/application-card';
import { useApplicationsBadgeControl } from '~/composables/useApplicationsBadge';
import { getErrorMessage, getErrorStatus } from '~/utils/errors';
import { getApi } from '~~/api/client';
import { ApplicationStatus } from '~~/api/types';
import type { Application } from '~~/api/types';
import styles from './index.module.css';

export type ApplicationAction = 'accept' | 'decline' | 'withdraw';

/** Statuses after which the list is out of date and must be reloaded (decided elsewhere, gone, no rights). */
const STALE_STATUSES = new Set([400, 403, 404, 409]);

const SUCCESS_MESSAGE: Record<ApplicationAction, string> = {
  accept: 'Отклик принят',
  decline: 'Отклик отклонён',
  withdraw: 'Отклик отозван',
};

const props = {
  application: prop<Application>().required(),
  /** `incoming` — «Принять» / «Отклонить» (listing author); `outgoing` — «Отозвать» (applicant). */
  mode: prop<ApplicationCardMode>().required(),
};

export type ApplicationActionsProps = ExtractProps<typeof props>;

/**
 * Action buttons for a pending application (renders nothing for other statuses).
 * «Принять» runs at once, «Отклонить» and «Отозвать» ask for confirmation. While a request
 * is running all buttons are blocked. Emits `updated` with the new state on success and
 * `stale` when the server says the list is out of date (400 «Решение по отклику уже
 * принято», 403, 404) — the error is shown as a toast, the parent reloads the list.
 * Accept/decline also force a reload of the nav badge.
 */
export const ApplicationActions = defineComponent((props: ApplicationActionsProps, { emit }) => {
  const api = getApi();
  const badge = useApplicationsBadgeControl();
  const busy = ref<ApplicationAction | null>(null);
  const confirming = ref<'decline' | 'withdraw' | null>(null);

  const isPending = computed(() => props.application.status === ApplicationStatus.APPLICATION_STATUS_PENDING);

  const run = async (action: ApplicationAction) => {
    if (busy.value) {
      return;
    }
    busy.value = action;
    const id = props.application.id;
    try {
      const updated = await api.applications[action](id);
      toast.success(SUCCESS_MESSAGE[action]);
      emit('updated', updated);
    } catch (error) {
      toast.error(getErrorMessage(error));
      const status = getErrorStatus(error);
      if (status !== undefined && STALE_STATUSES.has(status)) {
        emit('stale', id);
      }
    } finally {
      busy.value = null;
      confirming.value = null;
      if (action !== 'withdraw') {
        void badge.refresh({ force: true });
      }
    }
  };

  const ask = (action: 'decline' | 'withdraw') => {
    if (!busy.value) {
      confirming.value = action;
    }
  };

  return () => {
    if (!isPending.value) {
      return null;
    }
    const blocked = busy.value !== null;
    const dialog = (
      <ConfirmDialog
        open={confirming.value !== null}
        title={confirming.value === 'withdraw' ? 'Отозвать отклик?' : 'Отклонить отклик?'}
        message={confirming.value === 'withdraw'
          ? 'Автор публикации больше не увидит его на рассмотрении. Откликнуться заново можно будет позже.'
          : 'Решение окончательное: изменить его будет нельзя.'}
        confirmText={confirming.value === 'withdraw' ? 'Отозвать' : 'Отклонить'}
        tone={'danger'}
        loading={blocked}
        onConfirm={() => confirming.value && run(confirming.value)}
        onUpdate:open={(open: boolean) => {
          if (!open && !blocked) {
            confirming.value = null;
          }
        }}
      />
    );

    if (props.mode === 'outgoing') {
      return (
        <div class={styles.actions}>
          <Button
            variant={'secondary'}
            size={'sm'}
            loading={busy.value === 'withdraw'}
            disabled={blocked}
            data-testid={'application-withdraw'}
            onClick={() => ask('withdraw')}
          >
            {'Отозвать'}
          </Button>
          {dialog}
        </div>
      );
    }

    return (
      <div class={styles.actions}>
        <Button
          size={'sm'}
          icon={'check'}
          loading={busy.value === 'accept'}
          disabled={blocked}
          data-testid={'application-accept'}
          onClick={() => run('accept')}
        >
          {'Принять'}
        </Button>
        <Button
          variant={'secondary'}
          size={'sm'}
          loading={busy.value === 'decline'}
          disabled={blocked}
          data-testid={'application-decline'}
          onClick={() => ask('decline')}
        >
          {'Отклонить'}
        </Button>
        {dialog}
      </div>
    );
  };
}, {
  props,
  name: 'ApplicationActions',
  emits: {
    updated: (_application: Application) => true,
    stale: (_id: string) => true,
  },
});
