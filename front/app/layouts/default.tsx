import { defineComponent } from 'vue';
import { AppHeader } from '~/components/nav/app-header';
import { CreateFab } from '~/components/nav/create-fab';
import { TabBar } from '~/components/nav/tab-bar';
import styles from './default.module.css';

/** Authenticated shell: header ≥ 768px, tab bar + FAB < 768px, content ≤ 1040px. */
export default defineComponent({
  name: 'DefaultLayout',
  setup(_, { slots }) {
    return () => (
      <div class={styles.shell}>
        <AppHeader />
        <main class={styles.main}>
          <div class={styles.container}>{slots.default?.()}</div>
        </main>
        <CreateFab />
        <TabBar />
      </div>
    );
  },
});
