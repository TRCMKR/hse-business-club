import { computed, defineComponent, onBeforeUnmount, ref, useId, watch } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Icon } from '~/components/ui/icon';
import { Input } from '~/components/ui/input';
import { Segmented } from '~/components/ui/segmented';
import { listingKindOptions } from '~/utils/labels';
import type { ListingKind } from '~~/api/types';
import { FEED_QUERY_MAX_LENGTH, buildFeedQuery, normalizeFeedFilters, parseFeedQuery, sameFeedFilters } from '../query';
import type { FeedFilters as Filters } from '../query';
import styles from './index.module.css';

/** Delay between the last keystroke and the URL update. */
export const FEED_SEARCH_DEBOUNCE_MS = 300;

const props = {
  debounce: prop<number>().optional(FEED_SEARCH_DEBOUNCE_MS),
};

export type FeedFiltersProps = ExtractProps<typeof props>;

/**
 * Kind switch + search. The URL is the single source of truth: the component reads
 * `kind`/`q` from `route.query` and writes them back with `router.replace`
 * (search — debounced, kind and «clear» — immediately), so a link reproduces the feed.
 */
export const FeedFilters = defineComponent((props: FeedFiltersProps) => {
  const route = useRoute();
  const router = useRouter();
  const inputId = useId();

  const filters = computed(() => parseFeedQuery(route.query));
  /** Raw input value; may contain spaces the URL doesn't have. */
  const search = ref(filters.value.q);
  let timer: ReturnType<typeof setTimeout> | undefined;

  const cancel = () => {
    if (timer !== undefined) {
      clearTimeout(timer);
      timer = undefined;
    }
  };

  const apply = (patch: Partial<Filters>) => {
    cancel();
    const next = normalizeFeedFilters({ ...filters.value, ...patch });
    if (sameFeedFilters(next, filters.value)) {
      return;
    }
    void router.replace({ query: buildFeedQuery(next, route.query) });
  };

  // External URL changes (back/forward, «Сбросить фильтры») update the input,
  // unless it already shows the same query (e.g. with a trailing space being typed).
  watch(() => filters.value.q, (q) => {
    if (normalizeFeedFilters({ q: search.value }).q !== q) {
      cancel();
      search.value = q;
    }
  });

  const onInput = (value: string) => {
    search.value = value;
    cancel();
    timer = setTimeout(() => apply({ q: value }), props.debounce ?? FEED_SEARCH_DEBOUNCE_MS);
  };

  const clear = () => {
    search.value = '';
    apply({ q: '' });
    document.getElementById(inputId)?.focus();
  };

  onBeforeUnmount(cancel);

  return () => (
    <div class={styles.filters}>
      <Segmented
        class={styles.kind}
        modelValue={filters.value.kind}
        options={listingKindOptions}
        ariaLabel={'Тип публикаций'}
        onUpdate:modelValue={(kind: string) => apply({ kind: kind as ListingKind })}
      />
      <div class={styles.search} role={'search'}>
        <label class={'visually-hidden'} for={inputId}>{'Поиск по публикациям'}</label>
        <Icon name={'search'} size={18} class={styles.searchIcon} />
        <Input
          id={inputId}
          type={'search'}
          inputmode={'search'}
          autocomplete={'off'}
          placeholder={'Поиск по названию и описанию'}
          maxlength={FEED_QUERY_MAX_LENGTH}
          modelValue={search.value}
          onUpdate:modelValue={onInput}
          onEnter={() => apply({ q: search.value })}
        />
        {search.value !== '' && (
          <button
            type={'button'}
            class={styles.clear}
            aria-label={'Очистить поиск'}
            data-testid={'feed-search-clear'}
            onClick={clear}
          >
            <Icon name={'close'} size={18} />
          </button>
        )}
      </div>
    </div>
  );
}, { props, name: 'FeedFilters' });
