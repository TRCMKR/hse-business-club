import { computed, defineComponent } from 'vue';
import { NuxtLink } from '#components';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Avatar } from '~/components/ui/avatar';
import { Chip } from '~/components/ui/chip';
import { displayTelegram, telegramUrl } from '~/utils/format';
import type { User } from '~~/api/types';
import styles from './index.module.css';

export type UserSummarySize = 'sm' | 'md' | 'lg';

const AVATAR_SIZE: Record<UserSummarySize, number> = { sm: 40, md: 56, lg: 96 };

const props = {
  user: prop<User>().required(),
  size: prop<UserSummarySize>().optional('md'),
  /** When set, the display name is a link. */
  to: prop<string>().optional(),
  /** Skill chips; defaults to `true` for `lg` and `false` otherwise. */
  showSkills: prop<boolean>().optional(),
};

export type UserSummaryProps = ExtractProps<typeof props>;

/** Avatar + name + headline (+ Telegram and skills for `lg`). */
export const UserSummary = defineComponent((props: UserSummaryProps) => {
  const size = computed<UserSummarySize>(() => props.size ?? 'md');
  const withSkills = computed(() => props.showSkills ?? size.value === 'lg');

  return () => {
    const u = props.user;
    const tg = size.value === 'lg' ? telegramUrl(u.telegram) : null;
    return (
      <div class={[styles.summary, styles[size.value]]} data-testid={'user-summary'}>
        <Avatar user={u} size={AVATAR_SIZE[size.value]} />
        <div class={styles.body}>
          {props.to
            ? <NuxtLink to={props.to} class={[styles.name, styles.nameLink]}>{u.displayName}</NuxtLink>
            : <span class={styles.name}>{u.displayName}</span>}
          {u.headline && <span class={styles.headline}>{u.headline}</span>}
          {tg && (
            <a class={styles.telegram} href={tg} target={'_blank'} rel={'noopener noreferrer'}>
              {displayTelegram(u.telegram)}
            </a>
          )}
          {withSkills.value && u.skills.length > 0 && (
            <div class={styles.skills}>
              {u.skills.map(skill => <Chip key={skill} size={'sm'} label={skill} />)}
            </div>
          )}
        </div>
      </div>
    );
  };
}, { props, name: 'UserSummary' });
