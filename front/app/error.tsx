import { computed, defineComponent } from 'vue';
import type { NuxtError } from '#app';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Button } from '~/components/ui/button';
import styles from './error.module.css';

const props = {
  error: prop<NuxtError>().required(),
};

type Props = ExtractProps<typeof props>;

/** Global error page: 404 and 5xx. Pages throw `createError({ statusCode: 404 })` for missing objects. */
export default defineComponent((props: Props) => {
  const is404 = computed(() => props.error.statusCode === 404);

  useHead({ title: is404.value ? 'Страница не найдена' : 'Ошибка' });

  const goHome = () => clearError({ redirect: '/' });

  return () => (
    <div class={styles.page}>
      <div class={styles.code}>{props.error.statusCode || 500}</div>
      <h1 class={styles.title}>{is404.value ? 'Страница не найдена' : 'Что-то пошло не так'}</h1>
      <p class={styles.text}>
        {is404.value
          ? 'Возможно, ссылка устарела или публикация недоступна.'
          : 'Попробуйте обновить страницу чуть позже.'}
      </p>
      <Button onClick={goHome}>{'На главную'}</Button>
    </div>
  );
}, { props, name: 'ErrorPage' });
