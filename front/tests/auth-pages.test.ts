import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { mountSuspended } from '@nuxt/test-utils/runtime';
import LoginPage from '~/pages/login';
import RegisterPage from '~/pages/register';
import { useAuthStore } from '~/stores/auth';
import { makeUser } from './cards-fixtures';

// The store module is already loaded by the Nuxt test app, so its actions are replaced
// on the store instance instead of mocking `~~/api/client`.
const auth = { login: vi.fn(), register: vi.fn() };

const pending = <T>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
};

const buttonByText = (w: VueWrapper, text: string) => w.findAll('button').find(b => b.text() === text)!;

beforeEach(() => {
  vi.clearAllMocks();
  const store = useAuthStore();
  store.setUser(null);
  store.login = auth.login;
  store.register = auth.register;
});

afterEach(() => {
  useAuthStore().setUser(null);
});

describe('pages/login', () => {
  it('validates empty fields without calling the API', async () => {
    const w = await mountSuspended(LoginPage, { route: '/login' });
    await w.find('form').trigger('submit');
    await vi.waitFor(() => expect(w.text()).toContain('Введите логин'));
    expect(w.text()).toContain('Введите пароль');
    expect(auth.login).not.toHaveBeenCalled();
  });

  it('shows the server message on wrong credentials', async () => {
    auth.login.mockRejectedValue({ status: 401, data: { code: 16, message: 'Неверный логин или пароль' } });
    const w = await mountSuspended(LoginPage, { route: '/login' });
    await w.find('input[name="username"]').setValue('  Anna ');
    await w.find('input[name="password"]').setValue('wrong-pass');
    await w.find('form').trigger('submit');
    await vi.waitFor(() => expect(w.find('[role="alert"]').exists()).toBe(true));
    expect(w.find('[role="alert"]').text()).toBe('Неверный логин или пароль');
    expect(auth.login).toHaveBeenCalledWith({ username: 'anna', password: 'wrong-pass' });
  });

  it('demo button signs in once, even on a double click', async () => {
    const request = pending<ReturnType<typeof makeUser>>();
    auth.login.mockReturnValue(request.promise);
    const w = await mountSuspended(LoginPage, { route: '/login' });
    const demo = buttonByText(w, 'Войти как Борис');
    await demo.trigger('click');
    await buttonByText(w, 'Войти как Анна').trigger('click');
    await w.find('form').trigger('submit');
    await flushPromises();
    expect(auth.login).toHaveBeenCalledTimes(1);
    expect(auth.login).toHaveBeenCalledWith({ username: 'boris', password: 'demo1234' });
    request.resolve(makeUser({ username: 'boris' }));
    await flushPromises();
  });
});

describe('pages/register', () => {
  const fill = async (w: VueWrapper, values: Record<string, string>) => {
    for (const [name, value] of Object.entries(values)) {
      await w.find(`input[name="${name}"]`).setValue(value);
    }
  };

  it('checks the password confirmation on the client', async () => {
    const w = await mountSuspended(RegisterPage, { route: '/register' });
    await fill(w, { username: 'new_user', displayName: 'Новый', password: 'secret123', passwordConfirm: 'secret124' });
    await w.find('form').trigger('submit');
    await vi.waitFor(() => expect(w.text()).toContain('Пароли не совпадают'));
    expect(auth.register).not.toHaveBeenCalled();
  });

  it('shows «Логин уже занят» from the server and ignores repeated submits', async () => {
    const request = pending<never>();
    auth.register.mockReturnValue(request.promise);
    const w = await mountSuspended(RegisterPage, { route: '/register' });
    await fill(w, { username: 'anna', displayName: 'Анна', password: 'secret123', passwordConfirm: 'secret123' });
    await w.find('form').trigger('submit');
    await vi.waitFor(() => expect(auth.register).toHaveBeenCalledTimes(1));
    await w.find('form').trigger('submit');
    await flushPromises();
    expect(auth.register).toHaveBeenCalledTimes(1);
    expect(auth.register).toHaveBeenCalledWith({ username: 'anna', password: 'secret123', displayName: 'Анна' });

    // Reject through a fresh mock: the pending promise above stays unresolved.
    auth.register.mockRejectedValue({ status: 409, data: { code: 6, message: 'Логин уже занят' } });
    const w2 = await mountSuspended(RegisterPage, { route: '/register' });
    await fill(w2, { username: 'anna', displayName: 'Анна', password: 'secret123', passwordConfirm: 'secret123' });
    await w2.find('form').trigger('submit');
    await vi.waitFor(() => expect(w2.find('[role="alert"]').text()).toBe('Логин уже занят'));
  });
});
