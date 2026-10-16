# Семейный учёт расходов: Telegram-бот с ИИ-агентом и веб-панель — дизайн

- **Дата:** 2026-10-15
- **Статус:** утверждена; уточнена при проверке планов реализации (см. «Изменения после планирования»)
- **Исходное ТЗ:** тестовое задание «ИИ-бот учёта расходов» (5 рабочих дней)

## 1. Цель и критерии успеха

Сервис учёта семейных расходов: член семьи пишет боту в Telegram текстом или голосом («кофе 350»), трата сохраняется с автоматически определёнными суммой, категорией и датой. Глава семьи видит и редактирует траты всех в веб-панели.

**Требования ТЗ (обязательные):**

- Telegram-бот с ИИ-агентом: CRUD над своими тратами на естественном языке, без кнопок и команд. Чужие траты недоступны.
- Веб-панель: CRUD над тратами всех членов семьи, фильтры по члену семьи, периоду и категории. Работает через REST API. Авторизация не нужна.
- Бэкенд на Go, фронтенд на любом стеке.
- Запуск со всеми зависимостями одним `docker compose up`.
- Доступ к LLM настраивается через конфиг.

**Наша цель сверх ТЗ:** ревьюер за 20–40 минут видит production-подход: изоляция данных на уровне архитектуры, детерминизм там, где он нужен, измеримое качество агента (evals), живая панель, прозрачный процесс работы с ИИ-ассистентом.

**Критерии успеха:**

1. `cp .env.example .env`, заполнить токен бота, ключ OpenRouter и ID членов семьи, `docker compose up` — бот отвечает, панель открывается.
2. Все сценарии из ТЗ («кофе 350», «такси 1200 вчера», «сколько потратил на еду в этом месяце», «покажи траты за неделю», «последнее было не 350, а 450», «удали такси») работают текстом и голосом.
3. Ни через бота, ни через prompt injection нельзя получить или изменить чужую трату.
4. Трата, добавленная в боте, появляется в панели без перезагрузки.
5. Eval-набор проходит с точностью ≥ 90% на модели по умолчанию; результаты — в README.
6. CI зелёный: линтеры, юнит- и интеграционные тесты, сборка.

**Вне скоупа:** авторизация в панели, мультивалютность, несколько семей, распознавание чеков по фото, мягкое удаление/undo, webhook-режим бота, e2e-тесты фронтенда.

## 2. Архитектура

Один Go-бинарь с двумя адаптерами (REST API и Telegram-бот) над общим ядром бизнес-логики.

```
docker compose:
  postgres  (16, healthcheck)
  backend   (Go: HTTP :8080 + Telegram long polling)
  frontend  (nginx: статика React + proxy /api → backend)
```

```mermaid
flowchart LR
  TG[Telegram] -- long polling --> BOT[bot]
  BOT -- voice --> STT[stt: OpenRouter transcriptions]
  STT -- transcript --> BOT
  BOT -- text / transcript --> AGENT[agent: tool-calling loop]
  AGENT -- chat completions --> LLM[llm: OpenRouter]
  AGENT -- tools --> SVC[expense.Service]
  WEB[React panel] -- REST / SSE --> HTTP[httpapi]
  HTTP --> SVC
  SVC --> STORE[store/postgres]
  SVC -- events --> BUS[events bus]
  BUS -- SSE --> HTTP
  STORE --> PG[(Postgres)]
```

### Структура репозитория

```
api/openapi.yaml              источник правды REST-контракта
backend/
  cmd/server/                 main: конфиг, сборка зависимостей, запуск HTTP и бота
  internal/
    config/                   загрузка env-конфига и валидация
    domain/                   Expense, Category, Money, Period, доменные ошибки
    expense/                  Service: бизнес-логика, валидация, scoping по user_id
    store/postgres/           pgx + sqlc; репозитории expenses, members, chat, traces
    agent/                    цикл tool calling, инструменты, системный промпт, история
    llm/                      интерфейс Client + OpenAI-совместимая реализация
    stt/                      интерфейс Transcriber + реализация OpenRouter
    bot/                      Telegram-адаптер, whitelist, обработка text/voice
    httpapi/                  strict-server хендлеры (oapi-codegen) + SSE
    events/                   in-process pub/sub
  migrations/                 goose, применяются при старте
frontend/                     React + Vite + TypeScript
  evals/                      cases.yaml и отчёты (внутри модуля backend — раннеру нужны internal-пакеты)
  cmd/eval/                   CLI прогона evals
docs/
  superpowers/specs|plans/    спеки и планы
  adr/                        architecture decision records
  ai-workflow.md              как использовался ИИ-ассистент
docker-compose.yml
Makefile
.env.example
AGENTS.md
README.md
```

### Ключевые решения

| Решение | Почему |
|---|---|
| Один бинарь, in-process шина событий | Минимум инфраструктуры; шина за интерфейсом, заменяема на Postgres LISTEN/NOTIFY |
| Telegram long polling | Ревьюеру не нужен публичный URL / ngrok |
| Postgres + sqlc, без ORM | Явный SQL, типобезопасный сгенерированный код, `timestamptz`/`date` |
| OpenAPI-first, oapi-codegen + openapi-typescript | Один контракт, типы от БД до UI |
| Конфиг только через env | 12-factor, просто для compose |
| Свой цикл tool calling, без фреймворка | Полный контроль, тестируемость, понятность на ревью |

## 3. Модель данных

```sql
CREATE TABLE members (
  telegram_id BIGINT PRIMARY KEY,
  name        TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE expenses (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES members(telegram_id),
  amount      BIGINT NOT NULL CHECK (amount > 0),          -- копейки
  category    TEXT   NOT NULL CHECK (category IN
                ('food','transport','housing','entertainment','health','shopping','other')),
  spent_at    DATE   NOT NULL,
  comment     TEXT   NOT NULL DEFAULT '',            -- пустая строка = нет комментария
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  version     BIGINT NOT NULL DEFAULT 1                -- +1 при каждом изменении (миграция 00002)
);
CREATE INDEX expenses_user_spent_idx ON expenses (user_id, spent_at DESC);
CREATE INDEX expenses_spent_idx      ON expenses (spent_at DESC);

CREATE TABLE chat_messages (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES members(telegram_id),
  content     JSONB  NOT NULL,          -- сообщение в формате chat completions (role, content, tool_calls)
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_user_idx ON chat_messages (user_id, created_at DESC);

CREATE TABLE agent_traces (
  id                BIGSERIAL PRIMARY KEY,
  user_id           BIGINT NOT NULL REFERENCES members(telegram_id),
  source            TEXT   NOT NULL,     -- text | voice
  input             TEXT   NOT NULL,     -- текст или транскрипт
  output            TEXT,                -- финальный ответ бота
  steps             JSONB  NOT NULL,     -- [{tool, args, result, ms}]
  model             TEXT   NOT NULL,
  prompt_tokens     INT,
  completion_tokens INT,
  stt_ms            INT,
  latency_ms        INT    NOT NULL,
  error             TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX agent_traces_created_idx ON agent_traces (created_at DESC);
```

**Правила:**

- **Деньги** — `int64` копеек в домене и БД. В API поле `amount` — строка-десятичное `"350.00"`. Парсер принимает `350`, `350.5`, `350,50`, `1 200`. Валюта одна — рубли.
- **`spent_at`** — `DATE`. «Сегодня/вчера» вычисляются в таймзоне `APP_TIMEZONE` (по умолчанию `Europe/Moscow`).
- **Категории** — английские коды в БД и API; русские названия — в UI и в промпте агента. Маппинг: food=еда, transport=транспорт, housing=жильё, entertainment=развлечения, health=здоровье, shopping=покупки, other=другое.
- **`updated_at`** и **`version`** выставляет сам SQL `UPDATE` (`updated_at = now()`, `version = version + 1`) при каждом изменении, а не код сервиса.
- **Обновление — частичное и атомарное.** Один `UPDATE` меняет только переданные поля (`COALESCE(arg, col)`), в том же `WHERE` проверяет владельца и (если передана) ожидаемую `version`. Патч без полей (`{}` или только `version`) сервис отклоняет ошибкой валидации (422, поле `fields`) без записи и события. Сервис читает строку только для валидации итогового состояния. Поэтому одновременные правки разных полей (бот меняет сумму, панель — комментарий) обе сохраняются, а устаревшая версия даёт `ErrConflict` без записи.
- **Удаление** — жёсткое.
- **История диалога** — в контекст агента попадают последние 20 сообщений пользователя не старше 6 часов. Старые записи не удаляются (YAGNI на очистку).

## 4. Члены семьи и доступ

- `FAMILY_MEMBERS=123456:Папа,789012:Мама` — при старте upsert в `members` (имя обновляется, лишние не удаляются, чтобы не терять траты).
- Бот обрабатывает сообщения только от `from.id`, присутствующих в `members`. Остальным отвечает: «Этот бот семейный. Попросите главу семьи добавить ваш ID: 123456789» — и больше ничего не делает (LLM не вызывается).
- Веб-панель без авторизации (по ТЗ). В README: в проде закрыть basic auth / VPN.

## 5. Сервисный слой

`expense.Service` — единственное место бизнес-логики, используется и REST, и агентом.

```go
type Scope struct{ UserID *int64 } // nil = все пользователи (панель), не nil = только свои (бот)

Create(ctx, userID int64, in CreateInput) (Expense, error)
Get(ctx, scope Scope, id int64) (Expense, error)
List(ctx, scope Scope, f Filter) (Page, error)       // Filter: UserID, From, To, Category, Text, Sort, Limit, Offset
Summary(ctx, scope Scope, f SummaryFilter) (Summary, error) // group_by: category | day | member | none
Update(ctx, scope Scope, id int64, p Patch) (Expense, error)
Delete(ctx, scope Scope, id int64) (Expense, error)  // возвращает удалённую — для ответа бота и события
```

- Scoping реализован в SQL: `WHERE id = $1 AND ($2::bigint IS NULL OR user_id = $2)`. Чужая трата → `ErrNotFound` (не `Forbidden`, чтобы не раскрывать существование).
- Валидация: сумма > 0 и ≤ 100 000 000 ₽, категория из enum, дата не позже чем завтра (защита от опечаток в году), комментарий ≤ 500 символов, `user_id` существует в `members`.
- После успешной мутации публикует событие `expense.created|updated|deleted` в шину.

## 6. ИИ-агент

### Цикл

1. Бот передаёт `Run(ctx, userID, input, source)`.
2. Собираются сообщения: системный промпт + история (≤ 20, ≤ 6 ч) + новое сообщение.
3. Запрос в LLM с инструментами. Если в ответе `tool_calls` — каждый вызов исполняется (параллельные вызовы поддерживаются), результаты добавляются как `tool`-сообщения, повтор.
4. Ответ без `tool_calls` — финальный текст.
5. Лимит 6 итераций; при превышении — «Не получилось разобраться с запросом, попробуй сформулировать иначе» + ошибка в трейсе.
6. Новые сообщения (user, assistant с tool_calls, tool, финальный assistant) сохраняются в `chat_messages`; трейс — в `agent_traces`.

`userID` передаётся в исполнитель инструментов через замыкание/контекст и **отсутствует в JSON-схемах инструментов**.

### Инструменты

| Инструмент | Аргументы | Результат |
|---|---|---|
| `add_expense` | `amount` (string, обяз.), `category` (enum, обяз.), `date` (YYYY-MM-DD, опц., по умолчанию сегодня), `comment` (опц.) | созданная трата |
| `find_expenses` | `period` (enum, опц.) **или** `from`/`to` (опц.), `category` (опц.), `text` (поиск по комментарию, ILIKE, опц.), `sort` (`recent` по created_at / `date` по spent_at, по умолчанию `date`), `limit` (1–50, по умолчанию 10) | список трат с `id` + общая сумма |
| `get_summary` | `period` **или** `from`/`to`, `category` (опц.), `group_by` (`category` / `day` / `none`) | итоги |
| `update_expense` | `id` (обяз.), любые из `amount`, `category`, `date`, `comment` | обновлённая трата |
| `delete_expense` | `id` (обяз.) | удалённая трата |

`period` enum: `today`, `yesterday`, `this_week`, `last_week`, `this_month`, `last_month`, `last_7_days`, `last_30_days`. Границы вычисляет Go (`domain.ResolvePeriod(period, now, tz)`), неделя начинается с понедельника. Если переданы и `period`, и `from`/`to` — ошибка валидации, возвращаемая модели.

Результаты инструментов — компактный JSON (суммы уже отформатированы строками, категории — кодами и русскими названиями), чтобы модель не пересчитывала.

### Системный промпт (содержание)

- Роль: помощник учёта семейных расходов; отвечает по-русски, кратко.
- Текущая дата, день недели, таймзона; календарь последних 14 дней (дата → день недели) для «в понедельник», «12 октября».
- Категории с русскими названиями и примерами синонимов (продукты/кафе/кофе → food, такси/метро/бензин → transport, аренда/коммуналка → housing, кино/игры → entertainment, аптека/врач → health, одежда/техника → shopping).
- Правила:
  - Не выдумывать суммы; если суммы нет — спросить.
  - Изменение/удаление — только после `find_expenses` или по `id` из истории. Если подходит больше одной траты и пользователь не уточнил — перечислить кандидатов и спросить, какую.
  - Для периодов использовать `period`, когда он подходит.
  - Числа словами («триста пятьдесят», «полторы тыщи») переводить в цифры.
  - Запросы не про расходы — вежливо отказать одним предложением.
  - Никогда не утверждать, что видишь чужие траты; инструменты возвращают только траты текущего пользователя.
- Формат ответа: «Записал: такси — 1 200 ₽, транспорт, 14 октября»; суммы с пробелом-разделителем тысяч и «₽».

### LLM-клиент

- Интерфейс `llm.Client{ Chat(ctx, ChatRequest) (ChatResponse, error) }` — свои типы, не типы SDK.
- Реализация — OpenAI-совместимый HTTP (chat completions с `tools`), конфиг: `LLM_BASE_URL` (по умолчанию `https://openrouter.ai/api/v1`), `LLM_API_KEY`, `LLM_MODEL`, `LLM_TIMEOUT` (30s).
- Ретраи: 2 повтора с экспоненциальным backoff на 429 и 5xx.
- Модель по умолчанию выбирается по результатам evals (3–4 кандидата с хорошим tool calling на OpenRouter); выбор и таблица — в README.

### Конкурентность

Сообщения одного пользователя обрабатываются последовательно (per-user mutex), разных пользователей — параллельно.

## 7. Telegram-бот

- Библиотека: `github.com/go-telegram/bot`, long polling.
- Обрабатываемые типы: `text`, `voice`. Остальное — «Я понимаю текст и голосовые 🙂».
- Пока агент работает — `sendChatAction(typing)`.
- Ответ — plain text (без Markdown-парсинга, чтобы не ломаться на спецсимволах из комментариев).
- Ошибки инфраструктуры (LLM недоступна, БД) → «Что-то пошло не так, попробуй ещё раз через минуту» + лог + трейс с `error`.

### Голосовые

- Скачивание через `getFile`; отклоняется, если `duration > STT_MAX_SECONDS` (60).
- `stt.Transcriber{ Transcribe(ctx, audio []byte, format string) (Transcript, error) }`; реализация — OpenRouter `POST /api/v1/audio/transcriptions`, JSON-тело `{model, input_audio: {data: <base64>, format: "ogg"}, language: "ru"}`. Конфиг: `STT_ENABLED` (true), `STT_MODEL` (`openai/whisper-large-v3`), `STT_MAX_SECONDS`.
- Транскрипт передаётся в тот же агент с `source=voice`.
- Ответ: первой строкой «🎤 «кофе триста пятьдесят»», далее ответ агента.
- Пустой транскрипт / ошибка STT → «Не разобрал голосовое 🙏 Попробуй ещё раз или напиши текстом».
- **Риск:** формат запроса не подтверждён на живом API — первая задача плана — spike с реальным голосовым.

## 8. REST API

Описан в `api/openapi.yaml` (OpenAPI 3.0). Ошибки — RFC 7807 `application/problem+json`.

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/members` | список членов семьи |
| GET | `/api/expenses` | `member_id`, `from`, `to`, `category`, `limit` (≤ 200, по умолчанию 50), `offset` → `{items, total, sum}` |
| POST | `/api/expenses` | `{user_id, amount, category, spent_at, comment?}` |
| GET | `/api/expenses/{id}` | одна трата |
| PATCH | `/api/expenses/{id}` | частичное обновление (включая смену `user_id`): меняются только переданные поля; необязательный `version` — ожидаемая версия, при несовпадении 409 без записи; без полей — 422 |
| DELETE | `/api/expenses/{id}` | 204 |
| GET | `/api/stats` | `from`, `to`, `member_id` → `{total, count, by_category[], by_member[], by_day[]}` и `prev_total` за предыдущий период той же длины |
| GET | `/api/events` | SSE: `expense.created`, `expense.updated`, `expense.deleted` (payload — трата), heartbeat каждые 25 с |
| GET | `/api/traces` | `member_id`, `limit` (≤ 100), `offset` |
| GET | `/healthz` | проверка БД |

Объект траты: `{id, user_id, member_name, amount, category, spent_at, comment, created_at, updated_at, version}`.

Панель отправляет в PATCH только изменённые поля и `version` открытой траты; на 409 показывает уведомление с кнопкой «Обновить», которая подгружает свежую трату и сохраняет введённые пользователем правки для повторной отправки. Бот (агент) `version` не передаёт.

## 9. Веб-панель

Стек: React 19, Vite, TypeScript, TanStack Query, shadcn/ui (Tailwind), Recharts, react-hook-form + zod, клиент `openapi-fetch` с типами из `openapi-typescript`.

**Вкладка «Траты»**

- Фильтры: период (пресеты: эта неделя, этот месяц, прошлый месяц, 30 дней, произвольный диапазон), член семьи (все / один), категория (все / одна). Состояние фильтров — в URL.
- KPI: сумма за период, количество трат, среднее в день, изменение к прошлому периоду.
- Графики: горизонтальные столбцы по категориям (один цвет, подписи значений — для 7 классов donut нечитаем); столбцы по дням с разбивкой по членам семьи (категориальная палитра в фиксированном порядке, легенда, тултип).
- Таблица: дата, член семьи, категория (цветной бейдж), сумма, комментарий, действия; сортировка по дате; пагинация.
- Создание/редактирование — модальное окно; удаление — с подтверждением.
- SSE: на событие — инвалидация запросов, toast «Мама: кофе — 350 ₽», подсветка строки; автопереподключение.

**Вкладка «Журнал агента»**

- Список трейсов: член семьи, источник (текст/🎤), вход, ответ, модель, токены, задержка, ошибка.
- Раскрытие — цепочка шагов: вызов инструмента → аргументы → результат.
- Фильтр по члену семьи.

**Общее:** светлая/тёмная тема, адаптивная вёрстка, скелетоны, пустые состояния, ошибки API — toast с текстом из `problem+json`.

## 10. Конфигурация

`.env.example`:

```
TELEGRAM_TOKEN=
FAMILY_MEMBERS=123456789:Папа,987654321:Мама
LLM_API_KEY=
LLM_BASE_URL=https://openrouter.ai/api/v1
LLM_MODEL=           # рекомендуемое значение вписывается после прогона evals
LLM_TIMEOUT=30s
STT_ENABLED=true
STT_MODEL=openai/whisper-large-v3
STT_MAX_SECONDS=60
APP_TIMEZONE=Europe/Moscow
DATABASE_URL=postgres://expenses:expenses@postgres:5432/expenses?sslmode=disable
HTTP_ADDR=:8080
LOG_LEVEL=info
SEED_DEMO=false
```

Обязательные: `TELEGRAM_TOKEN`, `LLM_API_KEY`, `LLM_MODEL`, `FAMILY_MEMBERS`. `LLM_MODEL` намеренно без дефолта в коде: рекомендуемое значение фиксируется в `.env.example` и README по итогам evals. При отсутствии — понятная ошибка при старте. `SEED_DEMO=true` наполняет БД демо-тратами за 60 дней для членов из `FAMILY_MEMBERS` (только если таблица `expenses` пуста).

## 11. Обработка ошибок и наблюдаемость

- Доменные ошибки: `ErrNotFound`, `ErrValidation{Field, Msg}`, `ErrConflict` (устаревшая `version`). Маппинг: HTTP 404/422/409 + problem+json; для агента — `{"error": "..."}` в результате инструмента.
- Логи: `log/slog` JSON, поля `update_id` / `request_id`, `user_id`, длительности.
- Graceful shutdown: SIGTERM → остановка polling, дренаж HTTP (10 с), закрытие пула БД.
- Миграции применяются при старте; при ошибке — выход с ненулевым кодом.

## 12. Тестирование

- **Юнит:** парсинг/форматирование `Money`; `ResolvePeriod` (табличные тесты, фиксированное время, границы недели/месяца, таймзона); цикл агента с фейковым `llm.Client`, выдающим заданную последовательность ответов (диспетчеризация, параллельные вызовы, лимит итераций, ошибки инструментов возвращаются модели, `user_id` из контекста); валидация аргументов инструментов; фейковый `Transcriber` для пути голосовых в боте.
- **Интеграционные (testcontainers, Postgres):** store; `expense.Service`; HTTP-хендлеры через `httptest`; **тест изоляции** — пользователь A не может прочитать/изменить/удалить трату B через сервис со scope и через инструменты агента.
- **Фронтенд:** Vitest для чистой логики (форматирование денег и дат, пресеты периодов, состояние фильтров в URL), `tsc`, ESLint, `vite build`.

## 13. Evals

- `backend/evals/cases.yaml`, 45 кейсов: `{name, group, history?, input, expect}`. Проверяется **результат**, а не форма вызова: `expect` описывает, какие траты созданы/изменены/удалены (по ключам фикстуры), что должно и не должно быть в ответе, `ask: true` — уточняющий вопрос. Отсутствие ожидаемых изменений = любые изменения запрещены. Мир кейса: свежий in-memory репозиторий, фиксированное «сейчас» 2026-10-15 12:00 МСК, траты двух членов семьи.
- Группы: добавление, просмотр и сводки, изменение, удаление, неоднозначность, числа словами, prompt injection / чужие данные, нерелевантные запросы.
- Раннер: `make eval MODEL=...` — реальная LLM через OpenRouter. Отчёт — `backend/evals/results/<model>.md` (точность по группам, средняя задержка, токены/стоимость, разбор провалов) и сводная таблица моделей `backend/evals/results/README.md`.
- Не входит в CI (стоит денег); запуск вручную или `workflow_dispatch`.

## 14. CI

GitHub Actions на push/PR:

- backend: `go vet`, golangci-lint, `go test ./...` (с testcontainers), проверка актуальности сгенерированного кода (`sqlc generate`, `oapi-codegen` → `git diff --exit-code`);
- frontend: lint, typecheck, build, актуальность сгенерированных типов API;
- `docker compose build`.

## 15. Документация и подача

- **README:** GIF-демо (бот + живая панель), быстрый старт в 3 шага, mermaid-диаграмма, ключевые решения, таблица evals, безопасность, известные ограничения.
- **`docs/adr/`:** tool calling vs извлечение намерений; периоды вычисляет код; Postgres; деньги в копейках; long polling.
- **`docs/ai-workflow.md`:** процесс (спека → план → TDD через субагентов → ревью → evals для итераций промпта), где ИИ ошибался и как это ловилось.
- **`AGENTS.md`:** конвенции репозитория.
- **`Makefile`:** `up`, `down`, `test`, `lint`, `gen`, `eval`, `seed`.
- **Коммиты:** Conventional Commits.

## 16. Риски

| Риск | Митигация |
|---|---|
| Формат STT-запроса OpenRouter отличается от документации | Spike первой задачей; Transcriber за интерфейсом |
| Выбранная модель плохо делает tool calling на русском | Evals на 3–4 моделях до фиксации дефолта |
| Не уложиться в 5 дней | Порядок работ: ядро и бот → панель → голосовые → evals-отчёт → полировка; голосовые и журнал агента отрезаемы без поломки остального |
| Ревьюер без ключа OpenRouter | Чёткий README; при наличии времени — опциональный профиль Ollama (вне базового скоупа) |

## Изменения после планирования

Код планов был собран и прогнан до реализации; по итогам спека уточнена:

- React 19 вместо 18 (актуальная версия на момент планирования).
- График категорий — горизонтальные столбцы вместо donut (правила dataviz: donut — не более 6 сегментов и не для близких значений).
- `expenses.comment` — `NOT NULL DEFAULT ''` вместо NULL; `chat_messages` хранит роль внутри JSON.
- Evals проверяют результат (дифф состояния и ответ), а не точные аргументы инструментов; 45 кейсов; находятся в `backend/evals`.
- Фронтенд покрыт юнит-тестами Vitest для чистой логики.

## Изменения после ревью: одновременные правки

- Потерянные обновления: раньше `Service.Update` читал строку и перезаписывал все поля, так что одновременные правки бота и панели молча затирали друг друга. Теперь `Repository.Update(id, Patch, owner)` пишет только переданные поля одним атомарным `UPDATE` с проверкой владельца.
- Оптимистическая блокировка для панели: колонка `expenses.version` (миграция 00002), поле `version` в объекте траты и необязательный `version` в PATCH; несовпадение — `ErrConflict` → 409 «Трата была изменена другим пользователем. Обновите данные и повторите.»
- Поведение закреплено общим контрактом `expensetest` (Postgres через testcontainers и memrepo).
