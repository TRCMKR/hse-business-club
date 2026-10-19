import { computed, defineComponent, onBeforeUnmount, ref } from 'vue';
import { toast } from 'vue-sonner';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { getApi } from '~~/api/client';
import type { User } from '~~/api/types';
import { Avatar } from '~/components/ui/avatar';
import { Button } from '~/components/ui/button';
import { ConfirmDialog } from '~/components/ui/confirm-dialog';
import { Icon } from '~/components/ui/icon';
import { useAuthStore } from '~/stores/auth';
import { getErrorMessage } from '~/utils/errors';
import { AVATAR_ACCEPT, validateAvatarFile } from './validate';
import styles from './index.module.css';

const props = {
  user: prop<User>().required(),
  /** Avatar size in px (24–128). */
  size: prop<number>().optional(112),
};

export type AvatarEditorProps = ExtractProps<typeof props>;

/**
 * Photo editor for the current user: pick a file (click on the avatar or «Загрузить
 * фото»), check type and size on the client, preview, then «Сохранить» uploads it;
 * «Удалить фото» asks for confirmation. The returned `User` goes to `auth.setUser`,
 * so every `Avatar` (header included) updates at once. Emits `updated(user)`.
 */
export const AvatarEditor = defineComponent((props: AvatarEditorProps, { emit }) => {
  const api = getApi();
  const auth = useAuthStore();

  const input = ref<HTMLInputElement | null>(null);
  const pending = ref<File | null>(null);
  const previewUrl = ref<string | null>(null);
  const uploading = ref(false);
  const confirmOpen = ref(false);
  const deleting = ref(false);

  const size = computed(() => Math.min(128, Math.max(24, props.size ?? 112)));
  const hasPhoto = computed(() => Boolean(props.user.avatarUrl));
  const busy = computed(() => uploading.value || deleting.value);

  const clearPreview = () => {
    if (previewUrl.value) {
      URL.revokeObjectURL(previewUrl.value);
    }
    previewUrl.value = null;
    pending.value = null;
  };

  onBeforeUnmount(clearPreview);

  const pick = () => {
    if (!busy.value) {
      input.value?.click();
    }
  };

  const onFileChange = (e: Event) => {
    const el = e.target as HTMLInputElement;
    const file = el.files?.[0];
    // Reset so that choosing the same file again fires `change`.
    el.value = '';
    if (!file) {
      return;
    }
    const error = validateAvatarFile(file);
    if (error) {
      toast.error(error);
      return;
    }
    clearPreview();
    pending.value = file;
    previewUrl.value = URL.createObjectURL(file);
  };

  const apply = (user: User) => {
    auth.setUser(user);
    emit('updated', user);
  };

  const save = async () => {
    if (!pending.value || uploading.value) {
      return;
    }
    uploading.value = true;
    try {
      const user = await api.users.uploadAvatar(pending.value);
      clearPreview();
      apply(user);
      toast.success('Фото обновлено');
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      uploading.value = false;
    }
  };

  const remove = async () => {
    deleting.value = true;
    try {
      apply(await api.users.deleteAvatar());
      confirmOpen.value = false;
      toast.success('Фото удалено');
    } catch (error) {
      toast.error(getErrorMessage(error));
    } finally {
      deleting.value = false;
    }
  };

  return () => {
    const style = { height: `${size.value}px`, width: `${size.value}px` };
    return (
      <div class={styles.editor} data-testid={'avatar-editor'}>
        <button
          type={'button'}
          class={styles.picker}
          style={style}
          onClick={pick}
          disabled={busy.value}
          aria-label={hasPhoto.value ? 'Заменить фото' : 'Загрузить фото'}
        >
          {previewUrl.value
            ? <img class={styles.preview} style={style} src={previewUrl.value} alt={'Превью нового фото'} />
            : <Avatar user={props.user} size={size.value} />}
          <span class={styles.overlay} aria-hidden={'true'}>
            <Icon name={'camera'} size={24} />
          </span>
        </button>

        <input
          ref={input}
          class={styles.file}
          type={'file'}
          accept={AVATAR_ACCEPT}
          tabindex={-1}
          aria-hidden={'true'}
          data-testid={'avatar-input'}
          onChange={onFileChange}
        />

        <div class={styles.buttons}>
          {pending.value
            ? (
              <>
                <Button size={'sm'} loading={uploading.value} onClick={save}>{'Сохранить'}</Button>
                <Button size={'sm'} variant={'secondary'} disabled={uploading.value} onClick={clearPreview}>{'Отмена'}</Button>
              </>
            )
            : (
              <>
                <Button size={'sm'} variant={'secondary'} icon={'camera'} disabled={busy.value} onClick={pick}>
                  {hasPhoto.value ? 'Заменить фото' : 'Загрузить фото'}
                </Button>
                {hasPhoto.value && (
                  <Button size={'sm'} variant={'ghost'} icon={'trash'} disabled={busy.value} onClick={() => (confirmOpen.value = true)}>
                    {'Удалить фото'}
                  </Button>
                )}
              </>
            )}
        </div>

        <ConfirmDialog
          open={confirmOpen.value}
          title={'Удалить фото?'}
          message={'Вместо фото будет показан сгенерированный аватар.'}
          confirmText={'Удалить'}
          tone={'danger'}
          loading={deleting.value}
          onConfirm={remove}
          onUpdate:open={(v: boolean) => (confirmOpen.value = v)}
        />
      </div>
    );
  };
}, {
  props,
  name: 'AvatarEditor',
  emits: {
    updated: (_user: User) => true,
  },
});
