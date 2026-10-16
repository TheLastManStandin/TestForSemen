# Evals: `deepseek/deepseek-v4.1-flash`

- Дата: 2026-10-16 13:57
- Результат: **0/1 (0.0%)**
- Средняя задержка: 7.5 с · токены: 14987 / 436 · стоимость: $0.0005

| Группа | Пройдено | % |
|---|---|---|
| update | 0/1 | 0.0% |

## Провалы

### update_yesterday_taxi (update)

- Ввод: «вчерашнее такси было 1300»
- Ответ: «Не получилось разобраться с запросом, попробуй сформулировать иначе»
- Инструменты: find_expenses, update_expense, update_expense, update_expense, update_expense, update_expense
- Причины:
  - изменено трат: 0, ожидалось 1
- Вызовы:
  1. `find_expenses` `{"category":"transport","text":"такси","to":"2026-10-14"}` → `{"count":2,"items":[{"id":10,"amount":"1 200 ₽","category":"transport","category_ru":"транспорт","date":"2026-10-14","weekday":"среда","comment":"такси"},{"id":4,"amount":"950 ₽","category":"transport","category_ru":"транспорт","date":"2026-09-28","weekday":"понедельник","comment":"такси"}],"shown":2,"total":"2 150 ₽"}`
  2. `update_expense` `{"id":10}` → `{"error":"не передано ни одного поля для изменения (fields)"}`
  3. `update_expense` `{"id":10}` → `{"error":"не передано ни одного поля для изменения (fields)"}`
  4. `update_expense` `{"id":10}` → `{"error":"не передано ни одного поля для изменения (fields)"}`
  5. `update_expense` `{"id":10}` → `{"error":"не передано ни одного поля для изменения (fields)"}`
  6. `update_expense` `{"id":10}` → `{"error":"не передано ни одного поля для изменения (fields)"}`
