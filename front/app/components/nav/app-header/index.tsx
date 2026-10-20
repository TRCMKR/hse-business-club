import { computed, defineComponent } from 'vue';
import { NuxtLink } from '#components';
import { Avatar } from '~/components/ui/avatar';
import { Button } from '~/components/ui/button';
import { Icon } from '~/components/ui/icon';
import { useApplicationsBadge } from '~/composables/useApplicationsBadge';
import { useAuthStore } from '~/stores/auth';
import { isNavItemActive, NAV_ITEMS } from '../items';
import styles from './index.module.css';

/** Desktop top bar (≥ 768px): logo, nav items, «Создать публикацию», user + logout. */
export const AppHeader = defineComponent(() => {
  const route = useRoute();
  const auth = useAuthStore();
  const badge = useApplicationsBadge();

  const onLogout = async () => {
    await auth.logout();
    await navigateTo('/login');
  };

  const user = computed(() => auth.user);

  return () => (
    <header class={styles.header}>
      <div class={styles.inner}>
        <NuxtLink to={'/'} class={styles.logo} aria-label={'Бизнес-клуб — на главную'}>
          <span class={styles.logoMark} aria-hidden={'true'}>{'БК'}</span>
          <span class={styles.logoText}>
            <span class={styles.logoTitle}>{'Бизнес-клуб'}</span>
            <span class={styles.logoSubtitle}>{'Стартапы и вакансии'}</span>
          </span>
        </NuxtLink>

        <nav class={styles.nav} aria-label={'Основная навигация'}>
          {NAV_ITEMS.map(item => (
            <NuxtLink
              key={item.to}
              to={item.to}
              class={[styles.link, isNavItemActive(item, route.path) && styles.active]}
              aria-current={isNavItemActive(item, route.path) ? 'page' : undefined}
            >
              {item.label}
              {item.badge && badge.value > 0 && (
                <>
                  <span class={styles.counter} aria-hidden={'true'}>{badge.value}</span>
                  <span class={'visually-hidden'}>{`, новых: ${badge.value}`}</span>
                </>
              )}
            </NuxtLink>
          ))}
        </nav>

        <div class={styles.actions}>
          <Button to={'/listings/new'} size={'sm'} aria-label={'Создать публикацию'}>
            <Icon name={'plus'} size={18} />
            <span class={styles.createLabel}>{'Создать публикацию'}</span>
          </Button>
          {user.value && (
            <NuxtLink to={'/me'} class={styles.me} aria-label={'Профиль'} title={user.value.displayName}>
              <Avatar user={user.value} size={32} />
            </NuxtLink>
          )}
          <button type={'button'} class={styles.logout} onClick={onLogout} aria-label={'Выйти'} title={'Выйти'}>
            <Icon name={'logout'} size={20} />
          </button>
        </div>
      </div>
    </header>
  );
}, { name: 'AppHeader' });
