import { describe, expect, it } from 'vitest';
import { h } from 'vue';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { ProfileCard } from '~/components/profile/profile-card';
import { makeUser } from './cards-fixtures';

describe('profile/ProfileCard', () => {
  it('renders name, @username, headline, about, skills and a Telegram link', async () => {
    const w = await mountSuspended(ProfileCard, {
      props: { user: makeUser({ about: 'Ищу команду для стартапа' }) },
    });
    expect(w.find('h2').text()).toBe('Анна Смирнова');
    expect(w.text()).toContain('@anna');
    expect(w.find('[data-testid="profile-headline"]').text()).toBe('Продакт-менеджер');
    expect(w.find('[data-testid="profile-about"]').text()).toContain('Ищу команду для стартапа');
    const chips = w.find('[data-testid="profile-skills"]').text();
    expect(chips).toContain('Product');
    expect(chips).toContain('SQL');

    const tg = w.find('[data-testid="profile-telegram"]');
    expect(tg.attributes('href')).toBe('https://t.me/anna_s');
    expect(tg.attributes('target')).toBe('_blank');
    expect(tg.attributes('rel')).toContain('noopener');
    expect(tg.text()).toContain('@anna_s');
  });

  it('does not render sections for empty fields', async () => {
    const w = await mountSuspended(ProfileCard, {
      props: { user: makeUser({ headline: '', about: '  ', skills: [], telegram: '' }) },
    });
    expect(w.find('[data-testid="profile-headline"]').exists()).toBe(false);
    expect(w.find('[data-testid="profile-about"]').exists()).toBe(false);
    expect(w.find('[data-testid="profile-skills"]').exists()).toBe(false);
    expect(w.find('[data-testid="profile-telegram"]').exists()).toBe(false);
    expect(w.text()).not.toContain('О себе');
    expect(w.text()).not.toContain('Навыки');
  });

  it('shows the uploaded photo, and slots replace avatar and add actions', async () => {
    const photo = await mountSuspended(ProfileCard, {
      props: { user: makeUser({ avatarUrl: '/v1/avatars/x.jpg' }) },
    });
    expect(photo.find('img').attributes('src')).toBe('/api/v1/avatars/x.jpg');

    const slotted = await mountSuspended(ProfileCard, {
      props: { user: makeUser() },
      slots: {
        avatar: () => h('div', { 'data-test': 'editor' }),
        actions: () => h('button', 'Редактировать'),
      },
    });
    expect(slotted.find('[data-test="editor"]').exists()).toBe(true);
    expect(slotted.find('img').exists()).toBe(false);
    expect(slotted.find('button').text()).toBe('Редактировать');
  });
});
