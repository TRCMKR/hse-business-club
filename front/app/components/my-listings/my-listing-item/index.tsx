import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { ListingCard } from '~/components/cards/listing-card';
import { Button } from '~/components/ui/button';
import { ListingStatus } from '~~/api/types';
import type { Listing } from '~~/api/types';
import styles from './index.module.css';

const props = {
  listing: prop<Listing>().required(),
  /** Disables «Закрыть» while a close request for this listing is in flight. */
  closing: prop<boolean>().optional(false),
};

export type MyListingItemProps = ExtractProps<typeof props>;

/**
 * Card in «Мои публикации»: listing with stats and actions — «Отклики» always,
 * «Изменить» and «Закрыть» for active listings. Emits `close`; confirmation is up to
 * the parent.
 */
export const MyListingItem = defineComponent((props: MyListingItemProps, { emit }) => {
  return () => {
    const l = props.listing;
    const active = l.status === ListingStatus.LISTING_STATUS_ACTIVE;
    return (
      <ListingCard listing={l} showAuthor={false} data-testid={'my-listing'}>
        {{
          actions: () => (
            <div class={styles.actions}>
              <Button size={'sm'} variant={'secondary'} icon={'inbox'} to={`/listings/${l.id}/applications`}>
                {'Отклики'}
              </Button>
              {active && (
                <>
                  <Button size={'sm'} variant={'ghost'} icon={'edit'} to={`/listings/${l.id}/edit`}>
                    {'Изменить'}
                  </Button>
                  <Button
                    size={'sm'}
                    variant={'ghost'}
                    icon={'close'}
                    class={styles.close}
                    loading={props.closing}
                    data-testid={'my-listing-close'}
                    onClick={() => emit('close', l)}
                  >
                    {'Закрыть'}
                  </Button>
                </>
              )}
            </div>
          ),
        }}
      </ListingCard>
    );
  };
}, {
  props,
  name: 'MyListingItem',
  emits: {
    close: (_listing: Listing) => true,
  },
});
