import { defineComponent } from 'vue';
import { NuxtLink } from '#components';
import { Icon } from '~/components/ui/icon';
import styles from './index.module.css';

/**
 * Mobile floating «Создать» button above the tab bar (< 768px). Hidden on /listings/new
 * and on pages with `definePageMeta({ hideCreateFab: true })` (they have their own
 * sticky action bar in that spot).
 */
export const CreateFab = defineComponent(() => {
  const route = useRoute();

  return () => (route.path === '/listings/new' || route.meta.hideCreateFab
    ? null
    : (
      <NuxtLink to={'/listings/new'} class={styles.fab} aria-label={'Создать публикацию'}>
        <Icon name={'plus'} size={24} />
        <span class={styles.label}>{'Создать'}</span>
      </NuxtLink>
    ));
}, { name: 'CreateFab' });
