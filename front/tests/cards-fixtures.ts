/** Hand-built API DTOs for card component tests (shape as in server/mock/serialize.ts). */
import { ApplicationStatus, ListingKind, ListingStatus, StartupStage } from '~~/api/types';
import type { Application, Listing, ListingSummary, User } from '~~/api/types';

export function makeUser(over: Partial<User> = {}): User {
  return {
    id: 'u-anna',
    username: 'anna',
    displayName: 'Анна Смирнова',
    headline: 'Продакт-менеджер',
    about: '',
    skills: ['Product', 'SQL'],
    telegram: 'anna_s',
    createdAt: '2026-10-01T10:00:00Z',
    avatarUrl: '',
    ...over,
  };
}

export const boris = (): User => makeUser({
  id: 'u-boris',
  username: 'boris',
  displayName: 'Борис Иванов',
  headline: 'Backend-разработчик',
  telegram: 'boris',
});

export function makeSummary(over: Partial<ListingSummary> = {}): ListingSummary {
  return {
    id: 'l-1',
    kind: ListingKind.LISTING_KIND_STARTUP,
    title: 'Сервис аренды самокатов',
    status: ListingStatus.LISTING_STATUS_ACTIVE,
    author: makeUser(),
    ...over,
  };
}

export function makeApplication(over: Partial<Application> = {}): Application {
  return {
    id: 'a-1',
    listing: makeSummary(),
    applicant: boris(),
    message: 'Привет!\nХочу в команду.',
    status: ApplicationStatus.APPLICATION_STATUS_PENDING,
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
    decidedAt: null,
    ...over,
  };
}

export function makeListing(over: Partial<Listing> = {}): Listing {
  return {
    id: 'l-1',
    kind: ListingKind.LISTING_KIND_STARTUP,
    title: 'Сервис аренды самокатов',
    description: 'Описание',
    lookingFor: 'Backend-разработчика',
    tags: ['go', 'mobility'],
    stage: StartupStage.STARTUP_STAGE_MVP,
    compensation: '',
    status: ListingStatus.LISTING_STATUS_ACTIVE,
    author: makeUser(),
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
    closedAt: null,
    isMine: false,
    myApplication: null,
    stats: null,
    ...over,
  };
}

export function makeVacancy(over: Partial<Listing> = {}): Listing {
  return makeListing({
    id: 'l-2',
    kind: ListingKind.LISTING_KIND_VACANCY,
    title: 'Аналитик данных',
    stage: StartupStage.STARTUP_STAGE_UNSPECIFIED,
    compensation: 'от 120 000 ₽',
    ...over,
  });
}
