/**
 * Demo data from SPEC §6. UUIDs are generated on every state build; stable identifiers
 * for tests are `username` and `title`.
 */
import type {
  ApplicationStatus,
  ListingKind,
  ListingStatus,
  MockState,
  StartupStage,
} from './state';
import { createEmptyState, findUserByUsername, newId, nextSeq } from './state';

/** Password of every demo account. */
export const DEMO_PASSWORD = 'demo1234';

const at = (iso: string) => new Date(iso);

interface UserFixture {
  username: string
  displayName: string
  headline: string
  about: string
  skills: string[]
  telegram: string
  createdAt: string
}

const USERS: UserFixture[] = [
  {
    username: 'anna',
    displayName: 'Анна Соколова',
    headline: 'Фаундер FinTrack, ex-продакт в финтехе',
    about: 'Три года была продактом в банке, теперь делаю FinTrack — приложение для учёта студенческих финансов. Ищу команду.',
    skills: ['Продукт', 'Финтех', 'Питчинг'],
    telegram: 'anna_sokolova',
    createdAt: '2026-09-20T10:00:00Z',
  },
  {
    username: 'boris',
    displayName: 'Борис Климов',
    headline: 'Backend-разработчик (Go, Python), ищу проект',
    about: 'Пишу бэкенды на Go и Python, люблю PostgreSQL и понятную архитектуру. Хочу в стартап на роль техлида.',
    skills: ['Go', 'Python', 'PostgreSQL'],
    telegram: 'boris_dev',
    createdAt: '2026-09-21T11:30:00Z',
  },
  {
    username: 'vera',
    displayName: 'Вера Лисицына',
    headline: 'Продуктовый дизайнер, UX-исследования',
    about: 'Проектирую интерфейсы и провожу UX-исследования. Помогаю командам находить, что на самом деле нужно пользователям.',
    skills: ['Figma', 'UX', 'Прототипирование'],
    telegram: 'vera_design',
    createdAt: '2026-09-22T09:15:00Z',
  },
  {
    username: 'dmitry',
    displayName: 'Дмитрий Орлов',
    headline: 'Маркетолог, growth и комьюнити',
    about: 'Запускал студенческие сообщества и growth-эксперименты. Сейчас развиваю Volunteer Hub.',
    skills: ['Маркетинг', 'Growth', 'SMM'],
    telegram: 'dmitry_growth',
    createdAt: '2026-09-23T14:45:00Z',
  },
];

interface ListingFixture {
  author: string
  kind: ListingKind
  title: string
  description: string
  lookingFor: string
  tags: string[]
  stage: StartupStage
  compensation: string
  status: ListingStatus
  createdAt: string
  closedAt: string | null
}

const LISTINGS: ListingFixture[] = [
  {
    author: 'anna',
    kind: 'LISTING_KIND_STARTUP',
    title: 'FinTrack — учёт личных финансов для студентов',
    description: 'Мобильное приложение, которое помогает студентам планировать бюджет, копить на цели и не уходить в минус к концу месяца. Есть MVP и первые 300 пользователей.',
    lookingFor: 'Технический сооснователь (Go/Python), готовый взять на себя бэкенд и инфраструктуру',
    tags: ['Go', 'Backend', 'CTO'],
    stage: 'STARTUP_STAGE_MVP',
    compensation: '',
    status: 'LISTING_STATUS_ACTIVE',
    createdAt: '2026-10-01T12:00:00Z',
    closedAt: null,
  },
  {
    author: 'anna',
    kind: 'LISTING_KIND_VACANCY',
    title: 'Backend-разработчик в FinTrack',
    description: 'Нужен разработчик для API синхронизации транзакций и интеграции с банками. Удалённо, гибкий график.',
    lookingFor: 'Разработчик на Go или Python на 15–20 часов в неделю',
    tags: ['Go', 'PostgreSQL', 'API'],
    stage: 'STARTUP_STAGE_UNSPECIFIED',
    compensation: 'Доля 3% + стипендия 40 000 ₽',
    status: 'LISTING_STATUS_ACTIVE',
    createdAt: '2026-10-03T12:00:00Z',
    closedAt: null,
  },
  {
    author: 'boris',
    kind: 'LISTING_KIND_STARTUP',
    title: 'CampusEats — доставка еды по кампусу Покровки',
    description: 'Хотим сделать доставку из кафе рядом с кампусом прямо к аудитории за 15 минут. Пока идея и опрос студентов.',
    lookingFor: 'Сооснователь по маркетингу и операционке',
    tags: ['Маркетинг', 'Операции', 'Сооснователь'],
    stage: 'STARTUP_STAGE_IDEA',
    compensation: '',
    status: 'LISTING_STATUS_ACTIVE',
    createdAt: '2026-10-05T12:00:00Z',
    closedAt: null,
  },
  {
    author: 'vera',
    kind: 'LISTING_KIND_VACANCY',
    title: 'UX-исследователь на EdTech-проект',
    description: 'Исследуем, как студенты готовятся к экзаменам: интервью, анализ, карта пути пользователя. Научу методологии.',
    lookingFor: 'Студент с интересом к UX-исследованиям на 2 месяца',
    tags: ['UX', 'Исследования', 'EdTech'],
    stage: 'STARTUP_STAGE_UNSPECIFIED',
    compensation: 'Pro bono, кейс в портфолио',
    status: 'LISTING_STATUS_ACTIVE',
    createdAt: '2026-10-07T12:00:00Z',
    closedAt: null,
  },
  {
    author: 'dmitry',
    kind: 'LISTING_KIND_STARTUP',
    title: 'Volunteer Hub — платформа волонтёрских проектов ВШЭ',
    description: 'Единое место, где студенческие организации публикуют волонтёрские задачи, а студенты находят их и копят часы. Есть кликабельный прототип.',
    lookingFor: 'Frontend-разработчик и дизайнер',
    tags: ['Frontend', 'Vue', 'Дизайн'],
    stage: 'STARTUP_STAGE_PROTOTYPE',
    compensation: '',
    status: 'LISTING_STATUS_ACTIVE',
    createdAt: '2026-10-09T12:00:00Z',
    closedAt: null,
  },
  {
    author: 'dmitry',
    kind: 'LISTING_KIND_VACANCY',
    title: 'Frontend-разработчик в Volunteer Hub',
    description: 'Нужно собрать веб-версию платформы по готовому прототипу. Вакансия закрыта — разработчик найден.',
    lookingFor: 'Vue/React-разработчик',
    tags: ['Vue', 'Frontend'],
    stage: 'STARTUP_STAGE_UNSPECIFIED',
    compensation: 'Без оплаты, доля',
    status: 'LISTING_STATUS_CLOSED',
    createdAt: '2026-10-11T12:00:00Z',
    closedAt: '2026-10-15T18:00:00Z',
  },
];

interface ApplicationFixture {
  applicant: string
  /** 1-based number of the listing in LISTINGS. */
  listing: number
  status: ApplicationStatus
  message: string
  createdAt: string
  /** accept/decline/withdraw moment. */
  changedAt: string | null
}

const APPLICATIONS: ApplicationFixture[] = [
  {
    applicant: 'boris',
    listing: 1,
    status: 'APPLICATION_STATUS_PENDING',
    message: 'Привет! Делаю бэкенды на Go 3 года, финтех интересен. Готов обсудить роль CTO.',
    createdAt: '2026-10-13T09:00:00Z',
    changedAt: null,
  },
  {
    applicant: 'vera',
    listing: 1,
    status: 'APPLICATION_STATUS_ACCEPTED',
    message: 'Могу помочь с продуктом и UX на старте, есть опыт в финтех-дизайне.',
    createdAt: '2026-10-12T15:00:00Z',
    changedAt: '2026-10-14T10:00:00Z',
  },
  {
    applicant: 'dmitry',
    listing: 2,
    status: 'APPLICATION_STATUS_DECLINED',
    message: 'Не разработчик, но могу помочь с ростом — вдруг актуально.',
    createdAt: '2026-10-14T11:00:00Z',
    changedAt: '2026-10-15T09:00:00Z',
  },
  {
    applicant: 'anna',
    listing: 3,
    status: 'APPLICATION_STATUS_PENDING',
    message: 'Интересная идея, могу помочь с продуктом и питчем инвесторам.',
    createdAt: '2026-10-15T14:00:00Z',
    changedAt: null,
  },
  {
    applicant: 'boris',
    listing: 6,
    status: 'APPLICATION_STATUS_ACCEPTED',
    message: 'Готов взять фронт на Vue.',
    createdAt: '2026-10-12T16:00:00Z',
    changedAt: '2026-10-13T12:00:00Z',
  },
  {
    applicant: 'vera',
    listing: 5,
    status: 'APPLICATION_STATUS_WITHDRAWN',
    message: 'Хотела помочь с дизайном, но пока нет времени.',
    createdAt: '2026-10-16T10:00:00Z',
    changedAt: '2026-10-17T08:30:00Z',
  },
];

export function buildFixtureState(): MockState {
  const state = createEmptyState();

  for (const u of USERS) {
    const id = newId();
    state.users.set(id, {
      id,
      username: u.username,
      password: DEMO_PASSWORD,
      displayName: u.displayName,
      headline: u.headline,
      about: u.about,
      skills: [...u.skills],
      telegram: u.telegram,
      avatarKey: '',
      createdAt: at(u.createdAt),
      updatedAt: at(u.createdAt),
    });
  }

  const listingIds: string[] = [];
  for (const l of LISTINGS) {
    const id = newId();
    listingIds.push(id);
    state.listings.set(id, {
      id,
      seq: nextSeq(state),
      kind: l.kind,
      title: l.title,
      description: l.description,
      lookingFor: l.lookingFor,
      tags: [...l.tags],
      stage: l.stage,
      compensation: l.compensation,
      status: l.status,
      authorId: findUserByUsername(state, l.author)!.id,
      createdAt: at(l.createdAt),
      updatedAt: at(l.closedAt ?? l.createdAt),
      closedAt: l.closedAt ? at(l.closedAt) : null,
    });
  }

  for (const a of APPLICATIONS) {
    const id = newId();
    const decided = a.status === 'APPLICATION_STATUS_ACCEPTED' || a.status === 'APPLICATION_STATUS_DECLINED';
    state.applications.set(id, {
      id,
      seq: nextSeq(state),
      listingId: listingIds[a.listing - 1]!,
      applicantId: findUserByUsername(state, a.applicant)!.id,
      message: a.message,
      status: a.status,
      createdAt: at(a.createdAt),
      updatedAt: at(a.changedAt ?? a.createdAt),
      decidedAt: decided && a.changedAt ? at(a.changedAt) : null,
    });
  }

  return state;
}
