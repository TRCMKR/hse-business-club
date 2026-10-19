import { computed, defineComponent } from 'vue';
import { toast } from 'vue-sonner';
import { FeedFilters } from '~/components/feed/feed-filters';
import { FeedList } from '~/components/feed/feed-list';
import { EMPTY_FEED_FILTERS, buildFeedQuery, parseFeedQuery } from '~/components/feed/query';
import { PageHeader } from '~/components/ui/page-header';
import { useFeed } from '~/composables/useFeed';
import { getErrorMessage } from '~/utils/errors';
import styles from './index.module.css';

/** `/` — feed of active listings with kind/search filters kept in the URL. */
export default defineComponent({
  name: 'FeedPage',
  setup() {
    useHead({ title: 'Лента' });

    const route = useRoute();
    const router = useRouter();
    const filters = computed(() => parseFeedQuery(route.query));
    const feed = useFeed(filters);

    const loadMore = async () => {
      await feed.loadMore();
      if (feed.loadMoreError.value) {
        toast.error(getErrorMessage(feed.loadMoreError.value));
      }
    };

    const resetFilters = () => router.replace({ query: buildFeedQuery(EMPTY_FEED_FILTERS, route.query) });

    return () => (
      <div class={styles.page}>
        <PageHeader title={'Лента'} subtitle={'Стартапы и вакансии участников клуба'} />
        <FeedFilters />
        <FeedList
          items={feed.items.value}
          loading={feed.loading.value}
          error={feed.error.value ?? undefined}
          retrying={feed.status.value === 'pending'}
          hasMore={feed.hasMore.value}
          loadingMore={feed.loadingMore.value}
          filtered={feed.filtered.value}
          onLoadMore={loadMore}
          onRetry={() => void feed.refresh()}
          onReset={resetFilters}
        />
      </div>
    );
  },
});
