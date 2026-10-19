import { computed, defineComponent } from 'vue';
import type { NuxtError } from '#app';
import { NuxtLayout } from '#components';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Button } from '~/components/ui/button';
import { Icon } from '~/components/ui/icon';
import { getUserStatusMessage } from '~/utils/errors';
import styles from './error.module.css';

const props = {
  error: prop<NuxtError>().required(),
};

type Props = ExtractProps<typeof props>;

/**
 * Global error page inside the default layout. 404 (pages throw
 * `createError({ statusCode: 404, statusMessage })` for missing objects, Nuxt — for
 * unknown routes) — the page's `statusMessage` or «Страница не найдена» + «В ленту»;
 * anything else — «Что-то пошло не так» with «Повторить» (full reload) and «В ленту».
 */
export default defineComponent((props: Props) => {
  const is404 = computed(() => props.error.statusCode === 404);
  const title = computed(() => getUserStatusMessage(props.error)
    ?? (is404.value ? 'Страница не найдена' : 'Что-то пошло не так'));

  useHead({ title: computed(() => (is404.value ? title.value : 'Ошибка')) });

  const goFeed = () => clearError({ redirect: '/' });
  const retry = () => window.location.reload();

  return () => (
    <NuxtLayout name={'default'}>
      <div class={styles.page} role={is404.value ? undefined : 'alert'} data-testid={'error-page'}>
        <div class={styles.icon} aria-hidden={'true'}>
          <Icon name={is404.value ? 'search' : 'alert'} size={32} />
        </div>
        <div class={styles.code}>{props.error.statusCode || 500}</div>
        <h1 class={styles.title}>{title.value}</h1>
        <p class={styles.text}>
          {is404.value
            ? 'Возможно, ссылка устарела или в ней опечатка.'
            : 'Мы уже знаем о проблеме. Попробуйте ещё раз чуть позже.'}
        </p>
        <div class={styles.actions}>
          {!is404.value && <Button onClick={retry}>{'Повторить'}</Button>}
          <Button variant={is404.value ? 'primary' : 'secondary'} onClick={goFeed}>{'В ленту'}</Button>
        </div>
      </div>
    </NuxtLayout>
  );
}, { props, name: 'ErrorPage' });
