# Сравнение моделей

Генерируется `make eval`. Строки отсортированы сначала по проценту от всех кейсов («Всего»), затем по «Пройдено», стоимости и задержке.
«Пройдено» считается без кейсов, упавших на ошибках провайдера (429, 5xx, таймауты); «Всего» — по всем кейсам.
Прогон с ошибками провайдера помечен «неполный прогон» и не может быть моделью по умолчанию: ею становится верхняя строка без этой пометки.
Строки со значением «—» в «Ошибки провайдера» перенесены из старого формата таблицы: их итоги могли включать инфраструктурные ошибки как провалы.

| Модель | Пройдено | Ошибки провайдера | Всего | Задержка | Стоимость прогона | Последний полный прогон |
|---|---|---|---|---|---|---|
| `deepseek/deepseek-v4.1-flash` | 100.0% (45/45) | 0 | 100.0% (45/45) | 3.4 с | $0.0087 | [отчёт](runs/2026-10-19_15-34-43_deepseek_deepseek-v4.1-flash.md) |
| `qwen/qwen3.8-omni-flash` | 100.0% (45/45) | 0 | 100.0% (45/45) | 6.1 с | $0.0152 | [отчёт](runs/2026-10-19_20-47-29_qwen_qwen3.8-omni-flash.md) |
| `anthropic/claude-haiku-5.5` | 100.0% (45/45) | 0 | 100.0% (45/45) | 2.6 с | $0.0312 | [отчёт](runs/2026-10-19_15-31-31_anthropic_claude-haiku-5.5.md) |
| `google/gemini-3.8-flash` | 100.0% (45/45) | 0 | 100.0% (45/45) | 9.0 с | $0.1813 | [отчёт](runs/2026-10-19_20-46-17_google_gemini-3.8-flash.md) |
| `openai/gpt-6-luna` | 93.3% (42/45) | 0 | 93.3% (42/45) | 4.9 с | $0.0050 | [отчёт](runs/2026-10-19_15-32-36_openai_gpt-6-luna.md) |

## История прогонов

Каждый прогон, включая частичные (`-filter`) и повторные, сохраняется в `runs/` и не перезаписывается.

- [2026-10-19_20-47-29_qwen_qwen3.8-omni-flash.md](runs/2026-10-19_20-47-29_qwen_qwen3.8-omni-flash.md)
- [2026-10-19_20-46-17_google_gemini-3.8-flash.md](runs/2026-10-19_20-46-17_google_gemini-3.8-flash.md)
- [2026-10-19_15-34-43_deepseek_deepseek-v4.1-flash.md](runs/2026-10-19_15-34-43_deepseek_deepseek-v4.1-flash.md)
- [2026-10-19_15-32-36_openai_gpt-6-luna.md](runs/2026-10-19_15-32-36_openai_gpt-6-luna.md)
- [2026-10-19_15-31-31_anthropic_claude-haiku-5.5.md](runs/2026-10-19_15-31-31_anthropic_claude-haiku-5.5.md)
- [2026-10-19_15-27-35_qwen_qwen3.8-omni-flash.md](runs/2026-10-19_15-27-35_qwen_qwen3.8-omni-flash.md)
- [2026-10-19_15-26-07_deepseek_deepseek-v4.1-flash.md](runs/2026-10-19_15-26-07_deepseek_deepseek-v4.1-flash.md)
- [2026-10-19_15-24-25_google_gemini-3.8-flash.md](runs/2026-10-19_15-24-25_google_gemini-3.8-flash.md)
- [2026-10-19_15-20-58_openai_gpt-6-luna.md](runs/2026-10-19_15-20-58_openai_gpt-6-luna.md)
- [2026-10-19_15-19-29_anthropic_claude-haiku-5.5.md](runs/2026-10-19_15-19-29_anthropic_claude-haiku-5.5.md)
- [2026-10-19_15-13-56_qwen_qwen3.8-omni-flash.md](runs/2026-10-19_15-13-56_qwen_qwen3.8-omni-flash.md)
- [2026-10-19_15-12-35_deepseek_deepseek-v4.1-flash.md](runs/2026-10-19_15-12-35_deepseek_deepseek-v4.1-flash.md)
- [2026-10-19_15-12-00_google_gemini-3.8-flash.md](runs/2026-10-19_15-12-00_google_gemini-3.8-flash.md)
- [2026-10-19_15-08-48_openai_gpt-6-luna.md](runs/2026-10-19_15-08-48_openai_gpt-6-luna.md)
- [2026-10-19_15-07-55_anthropic_claude-haiku-5.5.md](runs/2026-10-19_15-07-55_anthropic_claude-haiku-5.5.md)
- [2026-10-19_15-06-28_qwen_qwen3.8-omni-flash.md](runs/2026-10-19_15-06-28_qwen_qwen3.8-omni-flash.md)
- [2026-10-19_15-05-14_deepseek_deepseek-v4.1-flash.md](runs/2026-10-19_15-05-14_deepseek_deepseek-v4.1-flash.md)
- [2026-10-19_15-04-41_google_gemini-3.8-flash.md](runs/2026-10-19_15-04-41_google_gemini-3.8-flash.md)
- [2026-10-19_15-01-41_openai_gpt-6-luna.md](runs/2026-10-19_15-01-41_openai_gpt-6-luna.md)
- [2026-10-19_15-00-26_anthropic_claude-haiku-5.5.md](runs/2026-10-19_15-00-26_anthropic_claude-haiku-5.5.md)
- [2026-10-19_14-48-14_qwen_qwen3.8-omni-flash.md](runs/2026-10-19_14-48-14_qwen_qwen3.8-omni-flash.md)
- [2026-10-19_14-46-58_deepseek_deepseek-v4.1-flash.md](runs/2026-10-19_14-46-58_deepseek_deepseek-v4.1-flash.md)
- [2026-10-19_14-46-21_google_gemini-3.8-flash.md](runs/2026-10-19_14-46-21_google_gemini-3.8-flash.md)
- [2026-10-19_14-43-57_openai_gpt-6-luna.md](runs/2026-10-19_14-43-57_openai_gpt-6-luna.md)
- [2026-10-19_14-39-54_anthropic_claude-haiku-5.5.md](runs/2026-10-19_14-39-54_anthropic_claude-haiku-5.5.md)
- [2026-10-19_14-21-24_qwen_qwen3.8-omni-flash.md](runs/2026-10-19_14-21-24_qwen_qwen3.8-omni-flash.md)
- [2026-10-19_14-19-47_deepseek_deepseek-v4.1-flash.md](runs/2026-10-19_14-19-47_deepseek_deepseek-v4.1-flash.md)
- [2026-10-19_14-19-13_google_gemini-3.8-flash.md](runs/2026-10-19_14-19-13_google_gemini-3.8-flash.md)
- [2026-10-19_14-16-43_openai_gpt-6-luna.md](runs/2026-10-19_14-16-43_openai_gpt-6-luna.md)
- [2026-10-19_14-15-48_anthropic_claude-haiku-5.5.md](runs/2026-10-19_14-15-48_anthropic_claude-haiku-5.5.md)
- [2026-10-19_14-14-23_qwen_qwen3.8-omni-flash.md](runs/2026-10-19_14-14-23_qwen_qwen3.8-omni-flash.md)
- [2026-10-19_14-12-52_deepseek_deepseek-v4.1-flash.md](runs/2026-10-19_14-12-52_deepseek_deepseek-v4.1-flash.md)
- [2026-10-19_14-12-22_google_gemini-3.8-flash.md](runs/2026-10-19_14-12-22_google_gemini-3.8-flash.md)
- [2026-10-19_14-10-28_openai_gpt-6-luna.md](runs/2026-10-19_14-10-28_openai_gpt-6-luna.md)
- [2026-10-19_14-09-26_anthropic_claude-haiku-5.5.md](runs/2026-10-19_14-09-26_anthropic_claude-haiku-5.5.md)
- [2026-10-19_11-59-57_qwen_qwen3.8-omni-flash.md](runs/2026-10-19_11-59-57_qwen_qwen3.8-omni-flash.md)
- [2026-10-19_11-58-13_deepseek_deepseek-v4.1-flash.md](runs/2026-10-19_11-58-13_deepseek_deepseek-v4.1-flash.md)
- [2026-10-19_11-57-43_google_gemini-3.8-flash.md](runs/2026-10-19_11-57-43_google_gemini-3.8-flash.md)
- [2026-10-19_11-55-51_openai_gpt-6-luna.md](runs/2026-10-19_11-55-51_openai_gpt-6-luna.md)
- [2026-10-19_11-55-02_anthropic_claude-haiku-5.5.md](runs/2026-10-19_11-55-02_anthropic_claude-haiku-5.5.md)
- [2026-10-16_14-20-56_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-56_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-52_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-52_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-47_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-47_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-43_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-43_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-39_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-39_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-35_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-35_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-32_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-32_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-28_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-28_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-24_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-24_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-20_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-20-20_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-20-12_openai_gpt-6-luna.md](runs/2026-10-16_14-20-12_openai_gpt-6-luna.md)
- [2026-10-16_14-19-21_google_gemini-3.8-flash.md](runs/2026-10-16_14-19-21_google_gemini-3.8-flash.md)
- [2026-10-16_14-18-11_anthropic_claude-haiku-5.5.md](runs/2026-10-16_14-18-11_anthropic_claude-haiku-5.5.md)
- [2026-10-16_14-17-35_qwen_qwen3.8-omni-flash.md](runs/2026-10-16_14-17-35_qwen_qwen3.8-omni-flash.md)
- [2026-10-16_14-16-47_qwen_qwen3.8-omni-flash.md](runs/2026-10-16_14-16-47_qwen_qwen3.8-omni-flash.md)
- [2026-10-16_14-15-56_deepseek_deepseek-v4.1-flash.md](runs/2026-10-16_14-15-56_deepseek_deepseek-v4.1-flash.md)
- [2026-10-16_14-15-29_deepseek_deepseek-v4.1-flash.md](runs/2026-10-16_14-15-29_deepseek_deepseek-v4.1-flash.md)
- [2026-10-16_14-15-05_deepseek_deepseek-v4.1-flash.md](runs/2026-10-16_14-15-05_deepseek_deepseek-v4.1-flash.md)
- [2026-10-16_14-05-46_deepseek_deepseek-v4.1-flash_delete.md](runs/2026-10-16_14-05-46_deepseek_deepseek-v4.1-flash_delete.md)
- [2026-10-16_14-05-39_deepseek_deepseek-v4.1-flash_update.md](runs/2026-10-16_14-05-39_deepseek_deepseek-v4.1-flash_update.md)
- [2026-10-16_14-05-30_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-30_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-05-26_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-26_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-05-21_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-21_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-05-17_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-17_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_14-05-12_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_14-05-12_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
- [2026-10-16_13-57-52_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md](runs/2026-10-16_13-57-52_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md)
