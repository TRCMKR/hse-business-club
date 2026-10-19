import { defineComponent } from 'vue';
import styles from './auth.module.css';

/** Centered card layout for /login and /register. */
export default defineComponent({
  name: 'AuthLayout',
  setup(_, { slots }) {
    return () => (
      <div class={styles.shell}>
        <div class={styles.brand}>
          <span class={styles.mark}>{'ВШЭ'}</span>
          <div>
            <div class={styles.title}>{'Бизнес-клуб'}</div>
            <div class={styles.subtitle}>{'Стартапы и вакансии'}</div>
          </div>
        </div>
        <main class={styles.card}>{slots.default?.()}</main>
      </div>
    );
  },
});
