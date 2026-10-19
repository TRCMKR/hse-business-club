import { defineComponent } from 'vue';
import { NuxtLayout, NuxtPage } from '#components';

export default defineComponent({
  setup() {
    return () => (
      <NuxtLayout>
        <NuxtPage />
      </NuxtLayout>
    );
  },
});
