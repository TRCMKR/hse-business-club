import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Avatar } from '~/components/ui/avatar';
import { Card } from '~/components/ui/card';
import { Chip } from '~/components/ui/chip';
import { displayTelegram, telegramUrl } from '~/utils/format';
import type { User } from '~~/api/types';
import styles from './index.module.css';

const props = {
  user: prop<User>().required(),
};

export type ProfileCardProps = ExtractProps<typeof props>;

/**
 * Read-only profile: avatar, name, @username, headline, Telegram link, «О себе» and
 * «Навыки». Empty fields render nothing. Slots: `avatar` replaces the avatar (the photo
 * editor on /me), `actions` — buttons under the header (e.g. «Редактировать»).
 */
export const ProfileCard = defineComponent((props: ProfileCardProps, { slots }) => {
  return () => {
    const u = props.user;
    const tg = telegramUrl(u.telegram);
    const about = u.about.trim();
    return (
      <Card class={styles.card} data-testid={'profile-card'}>
        <div class={styles.head}>
          <div class={styles.avatar}>
            {slots.avatar ? slots.avatar() : <Avatar user={u} size={112} />}
          </div>
          <div class={styles.identity}>
            <h2 class={styles.name}>{u.displayName}</h2>
            <span class={styles.username}>{`@${u.username}`}</span>
            {u.headline && <p class={styles.headline} data-testid={'profile-headline'}>{u.headline}</p>}
            {tg && (
              <a
                class={styles.telegram}
                href={tg}
                target={'_blank'}
                rel={'noopener noreferrer'}
                data-testid={'profile-telegram'}
              >
                {`Telegram: ${displayTelegram(u.telegram)}`}
              </a>
            )}
            {slots.actions && <div class={styles.actions}>{slots.actions()}</div>}
          </div>
        </div>

        {about && (
          <section class={styles.section} data-testid={'profile-about'}>
            <h3 class={styles.sectionTitle}>{'О себе'}</h3>
            <p class={styles.about}>{about}</p>
          </section>
        )}

        {u.skills.length > 0 && (
          <section class={styles.section} data-testid={'profile-skills'}>
            <h3 class={styles.sectionTitle}>{'Навыки'}</h3>
            <div class={styles.skills}>
              {u.skills.map(skill => <Chip key={skill} label={skill} />)}
            </div>
          </section>
        )}
      </Card>
    );
  };
}, { props, name: 'ProfileCard' });
