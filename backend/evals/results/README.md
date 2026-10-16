# Сравнение моделей

Генерируется `make eval`. Модель по умолчанию — лучшая по проценту прохождения, затем по стоимости и задержке.
«Пройдено» считается без кейсов, упавших на ошибках провайдера (429, 5xx, таймауты); «Всего» — по всем кейсам.

| Модель | Пройдено | Ошибки провайдера | Всего | Задержка | Стоимость прогона | Последний полный прогон |
|---|---|---|---|---|---|---|
| `deepseek/deepseek-v4.1-flash` | 100.0% (45/45) | — | 100.0% (45/45) | 1.9 с | $0.0101 | — |
| `qwen/qwen3.8-omni-flash` | 100.0% (45/45) | — | 100.0% (45/45) | 4.7 с | $0.0143 | — |
| `anthropic/claude-haiku-5.5` | 100.0% (45/45) | — | 100.0% (45/45) | 2.4 с | $0.0313 | — |
| `google/gemini-3.8-flash` | 100.0% (45/45) | — | 100.0% (45/45) | 5.8 с | $0.1400 | — |
| `openai/gpt-6-luna` | 91.1% (41/45) | — | 91.1% (41/45) | 4.3 с | $0.0055 | — |

## История прогонов

Каждый прогон, включая частичные (`-filter`) и повторные, сохраняется в `runs/` и не перезаписывается.

- [2026-10-16_14-05-46_deepseek_deepseek-v4.1-flash_delete.md](runs/2026-10-16_14-05-46_deepseek_deepseek-v4.1-flash_delete.md)
- [2026-10-16_14-05-39_deepseek_deepseek-v4.1-flash_update.md](runs/2026-10-16_14-05-39_deepseek_deepseek-v4.1-flash_update.md)
- [2026-10-16_14-05-30_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-30_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-05-26_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-26_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-05-21_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-21_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-05-17_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-17_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-05-12_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-12_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_13-57-52_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_13-57-52_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
