import { ref } from 'vue';
import type { Ref } from 'vue';

/**
 * Number shown on the «Отклики» nav item (pending received applications).
 * F0 stub: always 0. Implemented in F4.
 */
export const useApplicationsBadge = (): Ref<number> => {
  return ref(0);
};
