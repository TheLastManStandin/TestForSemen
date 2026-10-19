# Evals: `deepseek/deepseek-v4.1-flash`

- Дата: 2026-10-19 15:26
- Результат: **40/41 (97.6%)** без учёта ошибок провайдера; всего 40/45 (88.9%)
- Ошибки провайдера: 4
- Средняя задержка: 8.9 с · токены: 207763 / 6052 · стоимость: $0.0082

| Группа | Пройдено | % | Ошибки провайдера |
|---|---|---|---|
| add | 13/13 | 100.0% | 3 |
| delete | 5/5 | 100.0% | 0 |
| offtopic | 3/3 | 100.0% | 0 |
| query | 8/8 | 100.0% | 0 |
| robustness | 3/3 | 100.0% | 1 |
| security | 4/4 | 100.0% | 0 |
| update | 4/5 | 80.0% | 0 |

## Провалы

### update_last_amount (update)

- Ввод: «последнее было не 350, а 450»
- Ответ: «Изменил: кофе — 450 ₽, еда, 15 октября (было 350 ₽)  Кофе за сегодня теперь 450 ₽.»
- Инструменты: find_expenses, update_expense
- Причины:
  - подтверждение изменений повторено: сумма 450 упомянута 2 раз(а), изменений с ней 1
- Вызовы:
  1. `find_expenses` `{"sort": "recent", "limit": 3}` → `{"count":9,"items":[{"id":11,"amount":"350 ₽","category":"food","category_ru":"еда","date":"2026-10-15","weekday":"четверг","comment":"кофе"},{"id":10,"amount":"1 200 ₽","category":"transport","category_ru":"транспорт","date":"2026-10-14","weekday":"среда","comment":"такси"},{"id":9,"amount":"650 ₽","category":"food","category_ru":"еда","date":"2026-10-12","weekday":"понедельник","comment":"обед"}],"shown":3,"total":"60 730 ₽"}`
  2. `update_expense` `{"id": 11, "amount": "450"}` → `{"updated":{"id":11,"amount":"450 ₽","category":"food","category_ru":"еда","date":"2026-10-15","weekday":"четверг","comment":"кофе"}}`

## Ошибки провайдера

Не считаются провалами агента; кейсы стоит перезапустить.

### add_simple (add)

- Ввод: «кофе 350»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": read tcp 172.19.0.1:46154->104.18.2.115:443: read: connection reset by peer`

### add_yesterday (add)

- Ввод: «такси 1200 вчера»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": read tcp 172.19.0.1:42248->104.18.3.115:443: read: connection reset by peer`

### add_entertainment (add)

- Ввод: «подписка на онлайн-кинотеатр 399»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": read tcp 172.19.0.1:46172->104.18.2.115:443: read: connection reset by peer`

### robust_yesterday_pharmacy (robustness)

- Ввод: «вчера в аптеке оставил 1200»
- Ошибка: `llm: llm request: Post "https://openrouter.ai/api/v1/chat/completions": read tcp 172.19.0.1:46158->104.18.2.115:443: read: connection reset by peer`
