import { defineComponent } from 'vue';
import { NuxtLink } from '#components';
import { Icon } from '~/components/ui/icon';
import { useApplicationsBadge } from '~/composables/useApplicationsBadge';
import { isNavItemActive, NAV_ITEMS } from '../items';
import styles from './index.module.css';

/** Mobile bottom navigation (< 768px), respects the bottom safe area. */
export const TabBar = defineComponent(() => {
  const route = useRoute();
  const badge = useApplicationsBadge();

  return () => (
    <nav class={styles.bar} aria-label={'Основная навигация'}>
      {NAV_ITEMS.map((item) => {
        const active = isNavItemActive(item, route.path);
        return (
          <NuxtLink
            key={item.to}
            to={item.to}
            class={[styles.item, active && styles.active]}
            aria-current={active ? 'page' : undefined}
          >
            <span class={styles.icon}>
              <Icon name={item.icon} size={24} />
              {item.badge && badge.value > 0 && (
                <span class={styles.counter} aria-label={`новых: ${badge.value}`}>{badge.value}</span>
              )}
            </span>
            <span class={styles.label}>{item.shortLabel}</span>
          </NuxtLink>
        );
      })}
    </nav>
  );
}, { name: 'TabBar' });
