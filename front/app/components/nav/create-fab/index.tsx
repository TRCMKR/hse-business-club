import { defineComponent } from 'vue';
import { NuxtLink } from '#components';
import { Icon } from '~/components/ui/icon';
import styles from './index.module.css';

/** Mobile floating «Создать» button above the tab bar (< 768px). Hidden on /listings/new. */
export const CreateFab = defineComponent(() => {
  const route = useRoute();

  return () => (route.path === '/listings/new'
    ? null
    : (
      <NuxtLink to={'/listings/new'} class={styles.fab} aria-label={'Создать публикацию'}>
        <Icon name={'plus'} size={24} />
        <span class={styles.label}>{'Создать'}</span>
      </NuxtLink>
    ));
}, { name: 'CreateFab' });
