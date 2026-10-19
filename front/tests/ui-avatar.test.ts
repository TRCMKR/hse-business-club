import { describe, expect, it } from 'vitest';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { Avatar } from '~/components/ui/avatar';

const anna = { username: 'anna', displayName: 'Анна Смирнова', avatarUrl: '' };

describe('ui/Avatar', () => {
  it('renders <img> when avatarUrl is set', async () => {
    const w = await mountSuspended(Avatar, {
      props: { user: { ...anna, avatarUrl: '/v1/avatars/x.jpg' }, size: 48 },
    });
    const img = w.find('img');
    expect(img.exists()).toBe(true);
    expect(img.attributes('src')).toBe('/api/v1/avatars/x.jpg');
    expect(img.attributes('alt')).toBe('Анна Смирнова');
    expect(img.attributes('loading')).toBe('lazy');
    expect(w.find('svg').exists()).toBe(false);
  });

  it('renders an SVG without avatarUrl', async () => {
    const w = await mountSuspended(Avatar, { props: { user: anna } });
    expect(w.find('img').exists()).toBe(false);
    const svg = w.find('svg[role="img"]');
    expect(svg.exists()).toBe(true);
    expect(svg.attributes('aria-label')).toBe('Анна Смирнова');
  });

  it('is deterministic per username', async () => {
    const a1 = await mountSuspended(Avatar, { props: { user: anna, size: 64 } });
    const a2 = await mountSuspended(Avatar, { props: { user: { ...anna }, size: 64 } });
    const b = await mountSuspended(Avatar, { props: { user: { ...anna, username: 'boris' }, size: 64 } });
    expect(a1.html()).toBe(a2.html());
    expect(a1.find('svg').html()).not.toBe(b.find('svg').html());
  });

  it('falls back to SVG when the image fails to load', async () => {
    const w = await mountSuspended(Avatar, {
      props: { user: { ...anna, avatarUrl: '/v1/avatars/broken.jpg' } },
    });
    await w.find('img').trigger('error');
    expect(w.find('img').exists()).toBe(false);
    expect(w.find('svg').exists()).toBe(true);
  });
});
