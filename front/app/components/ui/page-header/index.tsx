import { defineComponent } from 'vue';
import { useRouter } from 'vue-router';
import { NuxtLink } from '#components';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Icon } from '../icon';
import styles from './index.module.css';

const props = {
  title: prop<string>().required(),
  subtitle: prop<string>().optional(),
  /** Back link: a path, or `true` for history back (falls back to `/`). */
  back: prop<string | boolean>().optional(),
  backLabel: prop<string>().optional('Назад'),
};

export type PageHeaderProps = ExtractProps<typeof props>;

/** Page title (h1) with optional back link and an `actions` slot on the right. */
export const PageHeader = defineComponent((props: PageHeaderProps, { slots }) => {
  const router = useRouter();

  const goBack = () => {
    if (typeof window !== 'undefined' && window.history.state?.back) {
      router.back();
    } else {
      router.push('/');
    }
  };

  return () => {
    const backContent = (
      <>
        <Icon name={'chevron-left'} size={20} />
        <span>{props.backLabel}</span>
      </>
    );
    return (
      <header class={styles.header}>
        {typeof props.back === 'string' && props.back && (
          <NuxtLink to={props.back} class={styles.back}>{backContent}</NuxtLink>
        )}
        {props.back === true && (
          <button type={'button'} class={styles.back} onClick={goBack}>{backContent}</button>
        )}
        <div class={styles.row}>
          <div class={styles.text}>
            <h1 class={styles.title}>{props.title}</h1>
            {props.subtitle && <p class={styles.subtitle}>{props.subtitle}</p>}
          </div>
          {slots.actions && <div class={styles.actions}>{slots.actions()}</div>}
        </div>
      </header>
    );
  };
}, { props, name: 'PageHeader' });
