import { describe, expect, it, vi } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { ListingForm } from '~/components/listing-form';
import type { ListingFormState } from '~/components/listing-form';
import { emptyListingForm, fromListing } from '~/components/listing-form/mapping';
import { ListingKind, StartupStage } from '~~/api/types';
import { makeVacancy } from './cards-fixtures';

type Submit = (values: ListingFormState) => Promise<void>;

const mountForm = (submit: Submit, initialValues = emptyListingForm(), mode: 'create' | 'edit' = 'create') =>
  mountSuspended(ListingForm, { props: { mode, submit, initialValues } });

type Wrapper = Awaited<ReturnType<typeof mountForm>>;

async function fillCommon(w: Wrapper) {
  await w.find('input[name="title"]').setValue('Сервис аренды');
  await w.find('textarea[name="description"]').setValue('Подробное описание проекта');
  await w.find('textarea[name="lookingFor"]').setValue('CTO');
}

/** yup + vee-validate resolve over several macrotasks. */
async function settle() {
  for (let i = 0; i < 5; i++) {
    await flushPromises();
    await new Promise(resolve => setTimeout(resolve, 0));
  }
}

const radio = (w: Wrapper, label: string) => w.findAll('[role="radio"]').find(b => b.text() === label)!;

describe('ListingForm', () => {
  it('startup without a stage is not submitted', async () => {
    const submit = vi.fn<Submit>().mockResolvedValue();
    const w = await mountForm(submit);
    await fillCommon(w);
    await w.find('form').trigger('submit');
    await settle();
    expect(submit).not.toHaveBeenCalled();
    expect(w.text()).toContain('Стадия: укажите стадию');

    await w.find('select').setValue(StartupStage.STARTUP_STAGE_IDEA);
    await w.find('form').trigger('submit');
    await settle();
    expect(submit).toHaveBeenCalledTimes(1);
    expect(submit.mock.calls[0]![0]).toMatchObject({
      kind: ListingKind.LISTING_KIND_STARTUP,
      title: 'Сервис аренды',
      stage: StartupStage.STARTUP_STAGE_IDEA,
      compensation: '',
    });
  });

  it('vacancy: no stage select, shows compensation', async () => {
    const submit = vi.fn<Submit>().mockResolvedValue();
    const w = await mountForm(submit);
    expect(w.find('select').exists()).toBe(true);
    expect(w.find('input[name="compensation"]').exists()).toBe(false);

    await radio(w, 'Вакансия').trigger('click');
    expect(w.find('select').exists()).toBe(false);
    expect(w.find('input[name="compensation"]').exists()).toBe(true);

    await fillCommon(w);
    await w.find('input[name="compensation"]').setValue('Удалёнка');
    await w.find('form').trigger('submit');
    await settle();
    expect(submit).toHaveBeenCalledTimes(1);
    expect(submit.mock.calls[0]![0]).toMatchObject({
      kind: ListingKind.LISTING_KIND_VACANCY,
      stage: StartupStage.STARTUP_STAGE_UNSPECIFIED,
      compensation: 'Удалёнка',
    });
  });

  it('a stage chosen before switching to vacancy does not block submit and comes back', async () => {
    const submit = vi.fn<Submit>().mockResolvedValue();
    const w = await mountForm(submit);
    await w.find('select').setValue(StartupStage.STARTUP_STAGE_MVP);
    await radio(w, 'Вакансия').trigger('click');
    await fillCommon(w);
    await w.find('form').trigger('submit');
    await settle();
    expect(submit).toHaveBeenCalledTimes(1);

    await radio(w, 'Стартап').trigger('click');
    expect((w.find('select').element as HTMLSelectElement).value).toBe(StartupStage.STARTUP_STAGE_MVP);
  });

  it('double submit sends one request', async () => {
    let resolve!: () => void;
    const submit = vi.fn<Submit>(() => new Promise<void>((r) => {
      resolve = r;
    }));
    const w = await mountForm(submit);
    await fillCommon(w);
    await w.find('select').setValue(StartupStage.STARTUP_STAGE_IDEA);

    const form = w.find('form');
    await form.trigger('submit');
    await form.trigger('submit');
    await settle();
    await form.trigger('submit');
    await w.find('[data-testid="listing-form-submit"]').trigger('click');
    await settle();
    expect(submit).toHaveBeenCalledTimes(1);
    expect(w.find('[data-testid="listing-form-submit"]').attributes('aria-busy')).toBe('true');

    resolve();
    await settle();
    await form.trigger('submit');
    await settle();
    expect(submit).toHaveBeenCalledTimes(1);
  });

  it('server error is shown under the form and the form can be resubmitted', async () => {
    const submit = vi.fn<Submit>()
      .mockRejectedValueOnce({ status: 400, data: { code: 3, message: 'Теги: без повторов', details: [] } })
      .mockResolvedValueOnce();
    const w = await mountForm(submit);
    await fillCommon(w);
    await w.find('select').setValue(StartupStage.STARTUP_STAGE_IDEA);
    await w.find('form').trigger('submit');
    await settle();
    expect(w.find('[data-testid="listing-form-error"]').text()).toBe('Теги: без повторов');

    await w.find('form').trigger('submit');
    await settle();
    expect(submit).toHaveBeenCalledTimes(2);
  });

  it('edit mode: kind is a locked badge, fields are prefilled', async () => {
    const listing = makeVacancy();
    const w = await mountForm(vi.fn<Submit>().mockResolvedValue(), fromListing(listing), 'edit');
    expect(w.find('[role="radiogroup"]').exists()).toBe(false);
    expect(w.find('[data-testid="listing-form-kind-badge"]').text()).toBe('Вакансия');
    expect((w.find('input[name="title"]').element as HTMLInputElement).value).toBe(listing.title);
    expect((w.find('input[name="compensation"]').element as HTMLInputElement).value).toBe(listing.compensation);
    expect(w.find('[data-testid="listing-form-submit"]').text()).toBe('Сохранить');
  });
});
