import { defineComponent, ref } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import styles from './index.module.css';

export interface TabItem {
  value: string
  label: string
  /** Counter badge; hidden when undefined. */
  count?: number
}

const props = {
  modelValue: prop<string>().required(),
  tabs: prop<TabItem[]>().required(),
  /**
   * When set, tab ids are `${idPrefix}-tab-${value}` and the selected tab's aria-controls
   * points to `${idPrefix}-panel-${value}` — give the page's (single) panel that id and
   * `role="tabpanel"` + `aria-labelledby` = tab id. Inactive tabs have no aria-controls:
   * pages render only the active panel, and a reference to a missing id is invalid.
   */
  idPrefix: prop<string>().optional(),
  ariaLabel: prop<string>().optional(),
};

export type TabsProps = ExtractProps<typeof props>;

export const Tabs = defineComponent((props: TabsProps, { emit }) => {
  const root = ref<HTMLElement | null>(null);

  const onKeydown = (e: KeyboardEvent) => {
    const idx = props.tabs.findIndex(t => t.value === props.modelValue);
    const n = props.tabs.length;
    let next = -1;
    if (e.key === 'ArrowRight') {
      next = (idx + 1) % n;
    } else if (e.key === 'ArrowLeft') {
      next = (idx - 1 + n) % n;
    } else if (e.key === 'Home') {
      next = 0;
    } else if (e.key === 'End') {
      next = n - 1;
    }
    if (next < 0) {
      return;
    }
    e.preventDefault();
    emit('update:modelValue', props.tabs[next]!.value);
    root.value?.querySelectorAll<HTMLElement>('[role="tab"]')[next]?.focus();
  };

  return () => (
    <div ref={root} class={styles.tabs} role={'tablist'} aria-label={props.ariaLabel} onKeydown={onKeydown}>
      {props.tabs.map((t) => {
        const selected = t.value === props.modelValue;
        return (
          <button
            key={t.value}
            type={'button'}
            role={'tab'}
            id={props.idPrefix ? `${props.idPrefix}-tab-${t.value}` : undefined}
            aria-controls={props.idPrefix && selected ? `${props.idPrefix}-panel-${t.value}` : undefined}
            aria-selected={selected ? 'true' : 'false'}
            tabindex={selected ? 0 : -1}
            class={[styles.tab, selected && styles.active]}
            onClick={() => selected || emit('update:modelValue', t.value)}
          >
            <span>{t.label}</span>
            {t.count !== undefined && <span class={styles.count}>{t.count}</span>}
          </button>
        );
      })}
    </div>
  );
}, {
  props,
  name: 'Tabs',
  emits: {
    'update:modelValue': (_v: string) => true,
  },
});
