# AI-Content-Farm

Clean-room Go content pipeline with phone-first control UI, LLM script generation, manual script approval, TTS, FFmpeg rendering, and SQLite persistence.

## Included Features

- Prompt-based script generation via OpenAI-compatible API.
- Manual script approval/edit before render (`script_override`).
- Orientation control: `portrait`, `landscape`, `square`, `original`, `custom`.
- Background video library selection and upload from UI.
- Runtime settings API (`/api/settings`) persisted in SQLite.
- Job history persisted in SQLite.
- Mobile-first web UI served at `/`.

## Core Endpoints

- `GET /healthz`
- `POST /v1/scripts/generate`
- `POST /v1/jobs`
- `GET /v1/jobs`
- `GET /v1/jobs/{id}`
- `GET /api/settings`
- `PUT /api/settings`
- `GET /api/videos`
- `POST /api/videos/upload`

## Quick Start

```bash
cp .env.example .env
docker compose up -d --build
```

Open: `http://localhost:8080`

## Environment

See `.env.example` for all options. Important keys:

- `DB_PATH` SQLite DB path for jobs/settings.
- `INPUT_VIDEOS_DIR` folder containing source/background videos.
- `OUTPUT_VIDEOS_DIR` folder where rendered videos are written.
- `LLM_API_KEY` key for script generation.

## Notes

- If `LLM_API_KEY` is empty, the app uses fallback local script generation.
- Generated videos are accessible via `/outputs/<filename>`.
