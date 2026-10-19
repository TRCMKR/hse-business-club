import type { IconName } from '~/components/ui/icon';

export interface NavItem {
  to: string
  label: string
  /** Short label for the mobile tab bar. */
  shortLabel: string
  icon: IconName
  /** Shows the pending-applications counter. */
  badge?: boolean
}

export const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Лента', shortLabel: 'Лента', icon: 'home' },
  { to: '/me/listings', label: 'Мои публикации', shortLabel: 'Публикации', icon: 'list' },
  { to: '/me/applications', label: 'Отклики', shortLabel: 'Отклики', icon: 'inbox', badge: true },
  { to: '/me', label: 'Профиль', shortLabel: 'Профиль', icon: 'user' },
];

/** `/` is active only on the feed itself (and listing pages opened from it). */
export const isNavItemActive = (item: NavItem, path: string): boolean => {
  if (item.to === '/') {
    return path === '/' || (path.startsWith('/listings/') && !path.startsWith('/listings/new'));
  }
  if (item.to === '/me') {
    return path === '/me';
  }
  return path === item.to || path.startsWith(`${item.to}/`);
};
