import { defineComponent } from 'vue';
import { prop } from '@fe/prop-types';
import type { ExtractProps } from '@fe/prop-types';
import { Card } from '~/components/ui/card';
import { Chip } from '~/components/ui/chip';
import { startupStageLabel } from '~/utils/labels';
import { ListingKind, StartupStage } from '~~/api/types';
import type { Listing } from '~~/api/types';
import styles from './index.module.css';

const props = {
  listing: prop<Listing>().required(),
};

export type ListingBodyProps = ExtractProps<typeof props>;

/**
 * Description (line breaks preserved), «Кого ищем» with tags, and the kind-specific
 * field: stage for startups, compensation for vacancies.
 */
export const ListingBody = defineComponent((props: ListingBodyProps) => {
  return () => {
    const l = props.listing;
    const isStartup = l.kind === ListingKind.LISTING_KIND_STARTUP;
    const hasStage = isStartup && l.stage !== StartupStage.STARTUP_STAGE_UNSPECIFIED;
    const hasCompensation = l.kind === ListingKind.LISTING_KIND_VACANCY && l.compensation !== '';

    return (
      <Card as={'section'} class={styles.body} data-testid={'listing-body'}>
        <section class={styles.section} aria-labelledby={'listing-description'}>
          <h2 id={'listing-description'} class={styles.heading}>{'Описание'}</h2>
          <p class={styles.text}>{l.description}</p>
        </section>

        <section class={styles.section} aria-labelledby={'listing-looking-for'}>
          <h2 id={'listing-looking-for'} class={styles.heading}>{'Кого ищем'}</h2>
          <p class={styles.text}>{l.lookingFor}</p>
          {l.tags.length > 0 && (
            <div class={styles.tags}>
              {l.tags.map(tag => <Chip key={tag} label={tag} />)}
            </div>
          )}
        </section>

        {hasStage && (
          <section class={styles.section} data-testid={'listing-stage'}>
            <h2 class={styles.heading}>{'Стадия'}</h2>
            <p class={styles.value}>{startupStageLabel[l.stage]}</p>
          </section>
        )}

        {hasCompensation && (
          <section class={styles.section} data-testid={'listing-compensation'}>
            <h2 class={styles.heading}>{'Условия'}</h2>
            <p class={[styles.value, styles.text]}>{l.compensation}</p>
          </section>
        )}
      </Card>
    );
  };
}, { props, name: 'ListingBody' });
