import { defineComponent, ref } from 'vue';
import { toast } from 'vue-sonner';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import { formatDateTime, formatRelative } from '~/utils/dates';
import { getErrorMessage } from '~/utils/errors';
import { displayTelegram, telegramUrl } from '~/utils/format';
import { applicationStatusTone } from '~/utils/labels';
import { ApplicationStatus } from '~~/api/types';
import type { Application, User } from '~~/api/types';
import styles from './index.module.css';

/** Status of the viewer's own application on the details page («Вы откликнулись · Ожидает»). */
export const MY_APPLICATION_STATUS: Record<ApplicationStatus, string> = {
  [ApplicationStatus.APPLICATION_STATUS_UNSPECIFIED]: 'Неизвестно',
  [ApplicationStatus.APPLICATION_STATUS_PENDING]: 'Ожидает',
  [ApplicationStatus.APPLICATION_STATUS_ACCEPTED]: 'Принят',
  [ApplicationStatus.APPLICATION_STATUS_DECLINED]: 'Отклонён',
  [ApplicationStatus.APPLICATION_STATUS_WITHDRAWN]: 'Отозван',
};

export const WITHDRAW_SUCCESS_MESSAGE = 'Отклик отозван';

const props = {
  application: prop<Application>().required(),
  /** Listing author: on `accepted` the block suggests contacting them in Telegram. */
  author: prop<User>().optional(),
  /** Show «Отозвать» (pending only, see `resolveActions`). */
  canWithdraw: prop<boolean>().optional(false),
  /** `api.applications.withdraw(application.id)`; called after confirmation. */
  withdraw: prop<() => Promise<unknown>>().required(),
};

export type MyApplicationBlockProps = ExtractProps<typeof props>;

/**
 * The viewer's non-withdrawn application: status badge, own message, dates; for pending —
 * «Отозвать» behind a `ConfirmDialog`. Emits `withdrawn` after success (the page refreshes
 * and shows «Откликнуться» again).
 */
export const MyApplicationBlock = defineComponent((props: MyApplicationBlockProps, { emit }) => {
  const confirmOpen = ref(false);
  const loading = ref(false);

  const onConfirm = async () => {
    if (loading.value) {
      return;
    }
    loading.value = true;
    try {
      await props.withdraw();
      confirmOpen.value = false;
      toast.success(WITHDRAW_SUCCESS_MESSAGE);
      emit('withdrawn');
    } catch (e) {
      toast.error(getErrorMessage(e));
      confirmOpen.value = false;
      emit('failed', e);
    } finally {
      loading.value = false;
    }
  };

  return () => {
    const a = props.application;
    const accepted = a.status === ApplicationStatus.APPLICATION_STATUS_ACCEPTED;
    const tg = accepted && props.author ? telegramUrl(props.author.telegram) : null;

    return (
      <Card as={'section'} class={styles.block} data-testid={'my-application'} aria-labelledby={'my-application-title'}>
        <div class={styles.head}>
          <h2 id={'my-application-title'} class={styles.title}>{'Вы откликнулись'}</h2>
          <Badge tone={applicationStatusTone[a.status]} dot data-testid={'my-application-status'}>
            {MY_APPLICATION_STATUS[a.status]}
          </Badge>
        </div>

        {a.message && <p class={styles.message}>{a.message}</p>}

        <p class={styles.dates}>
          <time datetime={a.createdAt} title={formatDateTime(a.createdAt)}>
            {`Отправлен ${formatRelative(a.createdAt)}`}
          </time>
          {a.decidedAt && (
            <>
              {' · '}
              <time datetime={a.decidedAt} title={formatDateTime(a.decidedAt)}>
                {`решение ${formatRelative(a.decidedAt)}`}
              </time>
            </>
          )}
        </p>

        {accepted && (
          <p class={styles.hint}>
            {'Автор принял ваш отклик'}
            {tg && props.author
              ? (
                <>
                  {' — напишите ему в Telegram: '}
                  <a href={tg} target={'_blank'} rel={'noopener noreferrer'}>{displayTelegram(props.author.telegram)}</a>
                </>
              )
              : '.'}
          </p>
        )}

        {props.canWithdraw && (
          <Button
            variant={'secondary'}
            block
            loading={loading.value}
            onClick={() => (confirmOpen.value = true)}
            data-testid={'withdraw'}
          >
            {'Отозвать'}
          </Button>
        )}

        <ConfirmDialog
          open={confirmOpen.value}
          title={'Отозвать отклик?'}
          message={'Автор больше не увидит его в списке. Позже можно откликнуться заново.'}
          confirmText={'Отозвать'}
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
  name: 'MyApplicationBlock',
  emits: {
    withdrawn: () => true,
    failed: (_e: unknown) => true,
  },
});
