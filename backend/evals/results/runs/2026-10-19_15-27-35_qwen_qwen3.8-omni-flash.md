# Evals: `qwen/qwen3.8-omni-flash`

- Дата: 2026-10-19 15:27
- Результат: **41/42 (97.6%)** без учёта ошибок провайдера; всего 41/45 (91.1%)
- Ошибки провайдера: 3
- Средняя задержка: 7.7 с · токены: 196729 / 8059 · стоимость: $0.0150

| Группа | Пройдено | % | Ошибки провайдера |
|---|---|---|---|
| add | 15/15 | 100.0% | 1 |
| delete | 5/5 | 100.0% | 0 |
| offtopic | 3/3 | 100.0% | 0 |
| query | 7/7 | 100.0% | 1 |
| robustness | 3/3 | 100.0% | 1 |
| security | 4/4 | 100.0% | 0 |
| update | 4/5 | 80.0% | 0 |

## Провалы

### update_yesterday_taxi (update)

- Ввод: «вчерашнее такси было 1300»
- Ответ: «Изменил: такси — 1 300 ₽, транспорт, 14 октября (было 1 200 ₽)  Записал вчерашнее такси как 1 300 ₽ — остальное не трогал.»
- Инструменты: find_expenses, update_expense
- Причины:
  - подтверждение изменений повторено: сумма 1300 упомянута 2 раз(а), изменений с ней 1
- Вызовы:
  1. `find_expenses` `{"period": "yesterday", "text": "такси"}` → `{"count":1,"items":[{"id":10,"amount":"1 200 ₽","category":"transport","category_ru":"транспорт","date":"2026-10-14","weekday":"среда","comment":"такси"}],"shown":1,"total":"1 200 ₽"}`
  2. `update_expense` `{"id": 10, "amount": "1300"}` → `{"updated":{"id":10,"amount":"1 300 ₽","category":"transport","category_ru":"транспорт","date":"2026-10-14","weekday":"среда","comment":"такси"}}`

## Ошибки провайдера

Не считаются провалами агента; кейсы стоит перезапустить.

### add_entertainment (add)

- Ввод: «подписка на онлайн-кинотеатр 399»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": EOF`

### query_october_total (query)

- Ввод: «сколько я потратил в октябре»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": EOF`

### robust_yesterday_pharmacy (robustness)

- Ввод: «вчера в аптеке оставил 1200»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": EOF`
