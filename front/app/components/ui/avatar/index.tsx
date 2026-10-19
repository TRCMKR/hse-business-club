import { computed, defineComponent, ref, watch } from 'vue';
import { BlobAvatar } from '@fe/blob-avatar/vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import type { User } from '~~/api/types';
import styles from './index.module.css';

export type AvatarUser = Pick<User, 'username' | 'avatarUrl' | 'displayName'>;

const props = {
  user: prop<AvatarUser>().required(),
  /** Pixels, clamped to 24–128. */
  size: prop<number>().optional(40),
};

export type AvatarProps = ExtractProps<typeof props>;

/**
 * User avatar: the uploaded photo when `avatarUrl` is set (served via the `/api`
 * proxy), otherwise — and if the photo fails to load — a deterministic blob avatar
 * seeded by `username`, identical in SSR, on the client and in mock mode.
 */
export const Avatar = defineComponent((props: AvatarProps) => {
  const failed = ref(false);
  watch(() => props.user.avatarUrl, () => {
    failed.value = false;
  });

  const size = computed(() => Math.min(128, Math.max(24, Math.round(props.size ?? 40))));
  const label = computed(() => props.user.displayName || props.user.username);

  return () => {
    const style = { height: `${size.value}px`, width: `${size.value}px` };
    if (props.user.avatarUrl && !failed.value) {
      return (
        <img
          class={styles.avatar}
          style={style}
          src={`/api${props.user.avatarUrl}`}
          alt={label.value}
          width={size.value}
          height={size.value}
          loading={'lazy'}
          decoding={'async'}
          onError={() => {
            failed.value = true;
          }}
        />
      );
    }
    return (
      <span class={styles.avatar} style={style}>
        <BlobAvatar seed={props.user.username} size={size.value} background={'circle'} label={label.value} />
      </span>
    );
  };
}, { props, name: 'Avatar' });
