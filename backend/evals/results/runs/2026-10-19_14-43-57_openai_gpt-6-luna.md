# Evals: `openai/gpt-6-luna`

- Дата: 2026-10-19 14:43
- Результат: **37/42 (88.1%)** без учёта ошибок провайдера; всего 37/45 (82.2%)
- Ошибки провайдера: 3
- Средняя задержка: 20.1 с · токены: 152753 / 4579 · стоимость: $0.0042

| Группа | Пройдено | % | Ошибки провайдера |
|---|---|---|---|
| add | 15/15 | 100.0% | 1 |
| delete | 3/4 | 75.0% | 1 |
| offtopic | 2/2 | 100.0% | 1 |
| query | 4/8 | 50.0% | 0 |
| robustness | 4/4 | 100.0% | 0 |
| security | 4/4 | 100.0% | 0 |
| update | 5/5 | 100.0% | 0 |

## Провалы

### query_week_list (query)

- Ввод: «покажи траты за неделю»
- Ответ: «За эту неделю трат нет.»
- Инструменты: find_expenses
- Причины:
  - в ответе нет "обед"
  - в ответе нет "такси"
  - в ответе нет "кофе"
- Вызовы:
  1. `find_expenses` `{"period":"this_week","from":"","to":"","category":"other","text":"","sort":"date","limit":10}` → `{"count":0,"items":[],"shown":0,"total":"0 ₽"}`

### query_october_total (query)

- Ввод: «сколько я потратил в октябре»
- Ответ: «В октябре пока 0 ₽.»
- Инструменты: get_summary
- Причины:
  - в ответе нет "52 280"
- Вызовы:
  1. `get_summary` `{"period":"this_month","from":"","to":"","category":"other","group_by":"none"}` → `{"count":0,"from":"2026-10-01","groups":[],"to":"2026-10-15","total":"0 ₽"}`

### query_last_month (query)

- Ввод: «сколько ушло в прошлом месяце»
- Ответ: «В прошлом месяце расходов не было: 0 ₽.»
- Инструменты: get_summary
- Причины:
  - в ответе нет "8 450"
- Вызовы:
  1. `get_summary` `{"period":"last_month","from":"","to":"","category":"other","group_by":"none"}` → `{"count":0,"from":"2026-09-01","groups":[],"to":"2026-09-30","total":"0 ₽"}`

### query_today (query)

- Ввод: «что я сегодня купил»
- Ответ: «Сегодня покупок не найдено.»
- Инструменты: find_expenses
- Причины:
  - в ответе нет "кофе"
  - в ответе нет "350"
- Вызовы:
  1. `find_expenses` `{"period":"today","from":"","to":"","category":"shopping","text":"","sort":"date","limit":50}` → `{"count":0,"items":[],"shown":0,"total":"0 ₽"}`

### delete_last (delete)

- Ввод: «удали последнюю трату»
- Ответ: «За последние 30 дней трат не найдено.»
- Инструменты: find_expenses
- Причины:
  - не удалена трата coffee_today
- Вызовы:
  1. `find_expenses` `{"period":"last_30_days","from":"","to":"","category":"other","text":"","sort":"recent","limit":1}` → `{"count":0,"items":[],"shown":0,"total":"0 ₽"}`

## Ошибки провайдера

Не считаются провалами агента; кейсы стоит перезапустить.

### add_spoken_style (add)

- Ввод: «ну короче такси до работы где-то четыреста рублей»
- Ошибка: `agent: turn failed after completed changes: llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": net/http: TLS handshake timeout`
- Вызовы:
  1. `add_expense` `{"amount":"400","category":"transport","date":"2026-10-15","comment":"такси до работы"}` → `{"created":{"id":12,"amount":"400 ₽","category":"transport","category_ru":"транспорт","date":"2026-10-15","weekday":"четверг","comment":"такси до работы"}}`

### delete_followup_choice (delete)

- Ввод: «сентябрьскую»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": net/http: TLS handshake timeout`
- Вызовы:
  1. `find_expenses` `{"period":"this_month","from":"2026-09-01","to":"2026-09-30","category":"transport","text":"такси","sort":"date","limit":50}` → `{"error":"укажи либо period, либо from/to (period)"}`

### offtopic_greeting (offtopic)

- Ввод: «привет!»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": net/http: TLS handshake timeout`
