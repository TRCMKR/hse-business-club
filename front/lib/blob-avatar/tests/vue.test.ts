import { mount } from '@vue/test-utils';
import { describe, expect, it } from 'vitest';
import { h } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { blobAvatar } from '../src/index';
import { BlobAvatar } from '../src/vue';

function canonical(markup: string): string {
  const doc = new DOMParser().parseFromString(markup, 'image/svg+xml');
  return new XMLSerializer().serializeToString(doc.documentElement);
}

describe('BlobAvatar', () => {
  it('mounts and renders an accessible svg', () => {
    const wrapper = mount(BlobAvatar, { props: { seed: 'anna', size: 40 } });
    const svg = wrapper.find('svg');
    expect(svg.exists()).toBe(true);
    expect(svg.attributes('role')).toBe('img');
    expect(svg.attributes('aria-label')).toBe('anna');
    expect(svg.attributes('width')).toBe('40');
    expect(svg.element.namespaceURI).toBe('http://www.w3.org/2000/svg');
  });

  it('reacts to prop changes', async () => {
    const wrapper = mount(BlobAvatar, { props: { seed: 'anna' } });
    const before = wrapper.html();
    await wrapper.setProps({ seed: 'boris', label: 'Борис' });
    expect(wrapper.html()).not.toBe(before);
    expect(wrapper.find('svg').attributes('aria-label')).toBe('Борис');
  });

  it('server-renders the same markup as blobAvatar()', async () => {
    const options = { size: 64, background: 'squircle' as const, traits: { accent: 'overlap' as const, palette: 5 } };
    const ssr = await renderToString(h(BlobAvatar, { seed: 'vera', ...options }));
    expect(ssr).not.toContain('innerHTML');
    expect(canonical(ssr)).toBe(canonical(blobAvatar('vera', options)));
  });
});
