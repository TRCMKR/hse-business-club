import { defineComponent } from 'vue';
import { Toaster } from 'vue-sonner';
import 'vue-sonner/style.css';
import { NuxtLayout, NuxtPage, NuxtRouteAnnouncer } from '#components';

export default defineComponent({
  setup() {
    return () => (
      <>
        <NuxtRouteAnnouncer />
        <NuxtLayout>
          <NuxtPage />
        </NuxtLayout>
        <Toaster position={'top-center'} richColors closeButton />
      </>
    );
  },
});
