# STT spike — OpenRouter transcription of voice notes

- Date: 2026-10-16
- Endpoint: `POST https://openrouter.ai/api/v1/audio/transcriptions`
- Request: JSON `{"model":"openai/whisper-large-v3","language":"ru","input_audio":{"data":"<base64>","format":"ogg"}}`, headers `Authorization: Bearer <key>`, `Content-Type: application/json`.
- Sample: NOT a Telegram voice note (nobody could send one during the spike). It was a public-domain Russian recording downloaded from Wikimedia Commons ("Lenin - about work for transport", 1920s, noisy), cut to the first 10 s and converted to Telegram's format with `ffmpeg -i in.ogg -t 10 -ac 1 -ar 48000 -c:a libopus -b:a 32k voice.ogg` (OGG/Opus, mono, 48 kHz, 39 KB).
- Opus-in-OGG was accepted as `format: "ogg"` without further conversion. HTTP 200, one call, no multipart fallback needed.
- Response: `{"text":"…","usage":{"seconds":10,"cost":0.000075}}`
- Result (curl time_total 2.30 s) transcript, verbatim:
  ` О работе для транспорта. Слово товарища Ленина.  Продолжение следует...  Продолжение следует...  Продолжение следует...  Давай!  Продолжение следует...  Делите победу!`
  The first sentence is correct; the rest is Whisper hallucination on the noisy archival audio, so quality on clean speech is unverified. Text has leading/double spaces, so the client trims it.
- Decision: implement `stt.OpenRouter` with the JSON format above. Re-verify with a real Telegram voice note when one is available.
