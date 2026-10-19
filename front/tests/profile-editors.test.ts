import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import { AvatarEditor } from '~/components/profile/avatar-editor';
import { ProfileForm } from '~/components/profile/profile-form';
import { useAuthStore } from '~/stores/auth';
import { makeUser } from './cards-fixtures';

const users = vi.hoisted(() => ({
  uploadAvatar: vi.fn(),
  deleteAvatar: vi.fn(),
  updateMe: vi.fn(),
}));

const toast = vi.hoisted(() => ({ error: vi.fn(), success: vi.fn() }));

vi.mock('~~/api/client', () => ({ getApi: () => ({ users }) }));
vi.mock('vue-sonner', async importOriginal => ({
  ...(await importOriginal<typeof import('vue-sonner')>()),
  toast,
}));

const MB = 1024 * 1024;

function makeFile(name: string, type: string, size: number): File {
  const file = new File(['x'], name, { type });
  Object.defineProperty(file, 'size', { value: size });
  return file;
}

async function choose(w: VueWrapper, file: File) {
  const input = w.find('[data-testid="avatar-input"]');
  Object.defineProperty(input.element, 'files', { value: [file], configurable: true });
  await input.trigger('change');
  await flushPromises();
}

const buttonByText = (w: VueWrapper, text: string) => w.findAll('button').find(b => b.text() === text);

beforeEach(() => {
  vi.clearAllMocks();
  URL.createObjectURL = vi.fn(() => 'blob:preview');
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  document.body.innerHTML = '';
});

describe('profile/AvatarEditor', () => {
  it('rejects a 6 MB file without calling the service', async () => {
    const w = await mountSuspended(AvatarEditor, { props: { user: makeUser() } });
    await choose(w, makeFile('big.jpg', 'image/jpeg', 6 * MB));
    expect(toast.error).toHaveBeenCalledWith('Фото: не больше 5 МБ');
    expect(URL.createObjectURL).not.toHaveBeenCalled();
    expect(buttonByText(w, 'Сохранить')).toBeUndefined();
    expect(users.uploadAvatar).not.toHaveBeenCalled();
  });

  it('rejects a .gif without calling the service', async () => {
    const w = await mountSuspended(AvatarEditor, { props: { user: makeUser() } });
    await choose(w, makeFile('anim.gif', 'image/gif', 1024));
    expect(toast.error).toHaveBeenCalledWith('Фото: поддерживаются JPEG, PNG и WebP');
    expect(buttonByText(w, 'Сохранить')).toBeUndefined();
    expect(users.uploadAvatar).not.toHaveBeenCalled();
  });

  it('previews, uploads and updates the auth store', async () => {
    const updated = makeUser({ avatarUrl: '/v1/avatars/new.jpg' });
    users.uploadAvatar.mockResolvedValue(updated);
    const w = await mountSuspended(AvatarEditor, { props: { user: makeUser() } });
    const auth = useAuthStore();
    auth.setUser(makeUser());

    const file = makeFile('me.png', 'image/png', 2 * MB);
    await choose(w, file);
    expect(w.find('img').attributes('src')).toBe('blob:preview');

    await buttonByText(w, 'Сохранить')!.trigger('click');
    await flushPromises();

    expect(users.uploadAvatar).toHaveBeenCalledWith(file);
    expect(auth.user?.avatarUrl).toBe('/v1/avatars/new.jpg');
    expect(w.emitted('updated')?.[0]).toEqual([updated]);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:preview');
    expect(buttonByText(w, 'Сохранить')).toBeUndefined();
  });

  it('shows the server message on upload error and keeps the preview', async () => {
    users.uploadAvatar.mockRejectedValue({ data: { code: 3, message: 'Фото: минимум 64×64' }, status: 400 });
    const w = await mountSuspended(AvatarEditor, { props: { user: makeUser() } });
    await choose(w, makeFile('tiny.jpg', 'image/jpeg', 100));
    await buttonByText(w, 'Сохранить')!.trigger('click');
    await flushPromises();
    expect(toast.error).toHaveBeenCalledWith('Фото: минимум 64×64');
    expect(buttonByText(w, 'Сохранить')).toBeDefined();
  });

  it('«Отмена» drops the preview', async () => {
    const w = await mountSuspended(AvatarEditor, { props: { user: makeUser() } });
    await choose(w, makeFile('me.webp', 'image/webp', 1024));
    await buttonByText(w, 'Отмена')!.trigger('click');
    expect(buttonByText(w, 'Сохранить')).toBeUndefined();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:preview');
    expect(users.uploadAvatar).not.toHaveBeenCalled();
  });

  it('«Удалить фото» asks for confirmation, then deletes', async () => {
    const withPhoto = makeUser({ avatarUrl: '/v1/avatars/old.jpg' });
    users.deleteAvatar.mockResolvedValue(makeUser());
    const w = await mountSuspended(AvatarEditor, { props: { user: withPhoto }, attachTo: document.body });
    const auth = useAuthStore();
    auth.setUser(withPhoto);

    await buttonByText(w, 'Удалить фото')!.trigger('click');
    await flushPromises();
    expect(users.deleteAvatar).not.toHaveBeenCalled();
    const confirm = [...document.body.querySelectorAll('button')].find(b => b.textContent === 'Удалить');
    expect(confirm).toBeDefined();
    confirm!.click();
    await flushPromises();

    expect(users.deleteAvatar).toHaveBeenCalledOnce();
    expect(auth.user?.avatarUrl).toBe('');
  });

  it('has no Delete button without a photo and no capture attribute', async () => {
    const w = await mountSuspended(AvatarEditor, { props: { user: makeUser() } });
    expect(buttonByText(w, 'Удалить фото')).toBeUndefined();
    const input = w.find('[data-testid="avatar-input"]');
    expect(input.attributes('accept')).toBe('image/jpeg,image/png,image/webp');
    expect(input.attributes('capture')).toBeUndefined();
  });
});

describe('profile/ProfileForm', () => {
  it('saves via updateMe, updates the store and emits saved', async () => {
    const saved = makeUser({ displayName: 'Анна С.', skills: ['Product'] });
    users.updateMe.mockResolvedValue(saved);
    const w = await mountSuspended(ProfileForm, { props: { user: makeUser() } });
    const auth = useAuthStore();
    auth.setUser(makeUser());

    await w.find('input[name="displayName"]').setValue('  Анна С.  ');
    await w.find('input[name="telegram"]').setValue('@anna_s');
    await w.find('form').trigger('submit');
    await vi.waitFor(() => expect(w.emitted('saved')).toBeDefined());

    expect(users.updateMe).toHaveBeenCalledWith({
      displayName: 'Анна С.',
      headline: 'Продакт-менеджер',
      about: '',
      skills: ['Product', 'SQL'],
      telegram: 'anna_s',
    });
    expect(auth.user?.displayName).toBe('Анна С.');
    expect(w.emitted('saved')?.[0]).toEqual([saved]);
  });

  it('does not submit an invalid form', async () => {
    const w = await mountSuspended(ProfileForm, { props: { user: makeUser() } });
    await w.find('input[name="displayName"]').setValue('   ');
    await w.find('form').trigger('submit');
    await vi.waitFor(() => expect(w.text()).toContain('Имя: от 1 до 80 символов'));
    await flushPromises();
    expect(users.updateMe).not.toHaveBeenCalled();
  });

  it('shows the server error and «Отмена» emits cancel', async () => {
    users.updateMe.mockRejectedValue({ data: { code: 3, message: 'Навыки: без повторов' }, status: 400 });
    const w = await mountSuspended(ProfileForm, { props: { user: makeUser() } });
    await w.find('form').trigger('submit');
    await vi.waitFor(() => expect(w.find('[role="alert"]').exists()).toBe(true));
    expect(w.find('[role="alert"]').text()).toBe('Навыки: без повторов');
    await buttonByText(w, 'Отмена')!.trigger('click');
    expect(w.emitted('cancel')).toHaveLength(1);
  });
});
