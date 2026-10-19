import { defineComponent } from 'vue';
import styles from './auth.module.css';

/** /login and /register: navy club header, form on a light card. */
export default defineComponent({
  name: 'AuthLayout',
  setup(_, { slots }) {
    return () => (
      <div class={styles.shell}>
        <header class={styles.hero}>
          <div class={styles.brand}>
            <span class={styles.mark} aria-hidden={'true'}>{'БК'}</span>
            <div>
              <div class={styles.title}>{'Бизнес-клуб'}</div>
              <div class={styles.subtitle}>{'Стартапы и вакансии'}</div>
            </div>
          </div>
        </header>
        <main class={styles.card}>{slots.default?.()}</main>
      </div>
    );
  },
});
