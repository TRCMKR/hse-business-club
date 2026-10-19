package seed

import "github.com/trcmkr/hse-business-club/back/internal/listings"

// Password is the password of every demo account (SPEC §6).
const Password = "demo1234"

type userSeed struct {
	Username, DisplayName, Headline, About, Telegram string
	Skills                                           []string
}

type listingSeed struct {
	Author       string
	Kind         listings.Kind
	Title        string
	Description  string
	LookingFor   string
	Tags         []string
	Stage        listings.Stage
	Compensation string
	Closed       bool
}

type applicationSeed struct {
	Applicant string
	Listing   int // index in demoListings
	Message   string
	Outcome   string // pending | accepted | declined | withdrawn
}

// Users, listings and applications of SPEC §6. Stable keys are username and title.
var demoUsers = []userSeed{
	{
		Username:    "anna",
		DisplayName: "Анна Соколова",
		Headline:    "Фаундер FinTrack, ex-продакт в финтехе",
		About:       "Три года вела продукт в банковском приложении, теперь делаю FinTrack — сервис учёта личных финансов для студентов. Ищу команду, которая любит простые и полезные продукты.",
		Skills:      []string{"Продукт", "Финтех", "Питчинг"},
		Telegram:    "anna_sokolova",
	},
	{
		Username:    "boris",
		DisplayName: "Борис Климов",
		Headline:    "Backend-разработчик (Go, Python), ищу проект",
		About:       "Пишу бэкенды на Go и Python, люблю PostgreSQL и аккуратные API. Хочу присоединиться к стартапу на ранней стадии.",
		Skills:      []string{"Go", "Python", "PostgreSQL"},
		Telegram:    "boris_dev",
	},
	{
		Username:    "vera",
		DisplayName: "Вера Лисицына",
		Headline:    "Продуктовый дизайнер, UX-исследования",
		About:       "Проектирую интерфейсы и провожу UX-исследования: интервью, юзабилити-тесты, CJM. Работала с финтехом и EdTech.",
		Skills:      []string{"Figma", "UX", "Прототипирование"},
		Telegram:    "vera_design",
	},
	{
		Username:    "dmitry",
		DisplayName: "Дмитрий Орлов",
		Headline:    "Маркетолог, growth и комьюнити",
		About:       "Запускаю growth-эксперименты и строю комьюнити вокруг продуктов. Делаю Volunteer Hub — платформу волонтёрских проектов ВШЭ.",
		Skills:      []string{"Маркетинг", "Growth", "SMM"},
		Telegram:    "dmitry_growth",
	},
}

var demoListings = []listingSeed{
	{ // 1
		Author:      "anna",
		Kind:        listings.KindStartup,
		Title:       "FinTrack — учёт личных финансов для студентов",
		Description: "Мобильное приложение, которое помогает студентам вести бюджет: стипендия, подработки, общие траты с соседями. Есть MVP и первые 300 пользователей из ВШЭ, ищем техническое усиление перед запуском на другие вузы.",
		LookingFor:  "Технический сооснователь (Go/Python), готовый взять на себя бэкенд и инфраструктуру",
		Tags:        []string{"Go", "Backend", "CTO"},
		Stage:       listings.StageMVP,
	},
	{ // 2
		Author:       "anna",
		Kind:         listings.KindVacancy,
		Title:        "Backend-разработчик в FinTrack",
		Description:  "Нужен разработчик для API FinTrack: синхронизация трат, категории, напоминания. Стек — Go, PostgreSQL, gRPC. Удалённо, гибкий график, созвон раз в неделю.",
		LookingFor:   "Разработчик на Go или Python на 15–20 часов в неделю",
		Tags:         []string{"Go", "PostgreSQL", "API"},
		Compensation: "Доля 3% + стипендия 40 000 ₽",
	},
	{ // 3
		Author:      "boris",
		Kind:        listings.KindStartup,
		Title:       "CampusEats — доставка еды по кампусу Покровки",
		Description: "Идея: студенты заказывают еду из кафе рядом с кампусом, а доставляют другие студенты между парами. Техническую часть беру на себя, нужен человек на маркетинг и операционку.",
		LookingFor:  "Сооснователь по маркетингу и операционке",
		Tags:        []string{"Маркетинг", "Операции", "Сооснователь"},
		Stage:       listings.StageIdea,
	},
	{ // 4
		Author:       "vera",
		Kind:         listings.KindVacancy,
		Title:        "UX-исследователь на EdTech-проект",
		Description:  "Исследуем, как студенты готовятся к экзаменам, чтобы спроектировать сервис подготовки. Нужна помощь с интервью, синтезом и отчётами. Научу методологии.",
		LookingFor:   "Студент с интересом к UX-исследованиям на 2 месяца",
		Tags:         []string{"UX", "Исследования", "EdTech"},
		Compensation: "Pro bono, кейс в портфолио",
	},
	{ // 5
		Author:      "dmitry",
		Kind:        listings.KindStartup,
		Title:       "Volunteer Hub — платформа волонтёрских проектов ВШЭ",
		Description: "Единое место, где студенческие организации публикуют волонтёрские проекты, а студенты находят их и записываются. Есть кликабельный прототип и договорённости с тремя организациями.",
		LookingFor:  "Frontend-разработчик и дизайнер",
		Tags:        []string{"Frontend", "Vue", "Дизайн"},
		Stage:       listings.StagePrototype,
	},
	{ // 6 (closed)
		Author:       "dmitry",
		Kind:         listings.KindVacancy,
		Title:        "Frontend-разработчик в Volunteer Hub",
		Description:  "Нужен разработчик, чтобы собрать веб-версию Volunteer Hub по готовому прототипу: каталог проектов, запись, личный кабинет.",
		LookingFor:   "Vue/React-разработчик",
		Tags:         []string{"Vue", "Frontend"},
		Compensation: "Без оплаты, доля",
		Closed:       true,
	},
}

const (
	outcomePending   = "pending"
	outcomeAccepted  = "accepted"
	outcomeDeclined  = "declined"
	outcomeWithdrawn = "withdrawn"
)

var demoApplications = []applicationSeed{
	{"boris", 0, "Привет! Делаю бэкенды на Go 3 года, финтех интересен. Готов обсудить роль CTO.", outcomePending},
	{"vera", 0, "Могу помочь с продуктом и UX на старте, есть опыт в финтех-дизайне.", outcomeAccepted},
	{"dmitry", 1, "Не разработчик, но могу помочь с ростом — вдруг актуально.", outcomeDeclined},
	{"anna", 2, "Интересная идея, могу помочь с продуктом и питчем инвесторам.", outcomePending},
	{"boris", 5, "Готов взять фронт на Vue.", outcomeAccepted},
	{"vera", 4, "Хотела помочь с дизайном, но пока нет времени.", outcomeWithdrawn},
}
