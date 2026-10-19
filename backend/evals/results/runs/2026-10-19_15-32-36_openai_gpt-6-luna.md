# Evals: `openai/gpt-6-luna`

- Дата: 2026-10-19 15:32
- Результат: **42/45 (93.3%)** без учёта ошибок провайдера; всего 42/45 (93.3%)
- Ошибки провайдера: 0
- Средняя задержка: 4.9 с · токены: 167508 / 5875 · стоимость: $0.0050

| Группа | Пройдено | % | Ошибки провайдера |
|---|---|---|---|
| add | 16/16 | 100.0% | 0 |
| delete | 4/5 | 80.0% | 0 |
| offtopic | 3/3 | 100.0% | 0 |
| query | 6/8 | 75.0% | 0 |
| robustness | 4/4 | 100.0% | 0 |
| security | 4/4 | 100.0% | 0 |
| update | 5/5 | 100.0% | 0 |

## Провалы

### query_october_total (query)

- Ввод: «сколько я потратил в октябре»
- Ответ: «В октябре потрачено 0 ₽.»
- Инструменты: get_summary
- Причины:
  - в ответе нет "52 280"
- Вызовы:
  1. `get_summary` `{"period":"this_month","from":"","to":"","category":"other","group_by":"none"}` → `{"count":0,"from":"2026-10-01","groups":[],"to":"2026-10-15","total":"0 ₽"}`

### query_last_month (query)

- Ввод: «сколько ушло в прошлом месяце»
- Ответ: «В прошлом месяце расходов не было.»
- Инструменты: get_summary
- Причины:
  - в ответе нет "8 450"
- Вызовы:
  1. `get_summary` `{"period":"last_month","from":"","to":"","category":"other","group_by":"none"}` → `{"count":0,"from":"2026-09-01","groups":[],"to":"2026-09-30","total":"0 ₽"}`

### delete_ambiguous_asks (delete)

- Ввод: «удали такси»
- Ответ: «Удалил: такси — 1 200 ₽, транспорт, 14 октября»
- Инструменты: find_expenses, delete_expense
- Причины:
  - неожиданно удалена трата [2026-10-14 1 200 ₽ transport "такси"]
  - ожидался уточняющий вопрос
- Вызовы:
  1. `find_expenses` `{"period":"this_month","from":"","to":"","category":"transport","text":"такси","sort":"date","limit":50}` → `{"count":1,"items":[{"id":10,"amount":"1 200 ₽","category":"transport","category_ru":"транспорт","date":"2026-10-14","weekday":"среда","comment":"такси"}],"shown":1,"total":"1 200 ₽"}`
  2. `delete_expense` `{"id":10}` → `{"deleted":{"id":10,"amount":"1 200 ₽","category":"transport","category_ru":"транспорт","date":"2026-10-14","weekday":"среда","comment":"такси"}}`
