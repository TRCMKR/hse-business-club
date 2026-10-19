import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Button } from '~/components/ui/button';
import type { ApplyState } from '../resolve-actions';
import styles from './index.module.css';

const props = {
  /** From `resolveActions(listing).apply`; `hidden` renders nothing. */
  apply: prop<ApplyState>().required(),
  /** Blocks the button while the apply sheet is open / a request is running. */
  loading: prop<boolean>().optional(false),
};

export type ListingActionBarProps = ExtractProps<typeof props>;

/**
 * «Откликнуться» for someone else's listing (the page makes it sticky above the tab bar
 * below 768px; on desktop it sits in the side column). For a closed listing the button is
 * disabled with the caption «Публикация закрыта».
 */
export const ListingActionBar = defineComponent((props: ListingActionBarProps, { emit }) => {
  return () => {
    if (props.apply === 'hidden') {
      return null;
    }
    const disabled = props.apply === 'disabled';
    return (
      <div class={styles.bar} data-testid={'listing-action-bar'}>
        <Button
          block
          icon={'send'}
          disabled={disabled}
          loading={props.loading}
          onClick={() => emit('apply')}
          data-testid={'apply'}
        >
          {'Откликнуться'}
        </Button>
        <p class={[styles.caption, disabled && styles.closed]}>
          {disabled ? 'Публикация закрыта' : 'Автор увидит ваше сообщение и профиль'}
        </p>
      </div>
    );
  };
}, {
  props,
  name: 'ListingActionBar',
  emits: {
    apply: () => true,
  },
});
