# AI-Content-Farm

Clean-room Go implementation for generating short-form AI video jobs.

## Goals

- Brand-new repository and codebase with no inherited git history.
- Modular pipeline architecture in Go.
- External AI services integrated through interfaces.

## Current Status

- HTTP API implemented (`/healthz`, `/v1/jobs`, `/v1/jobs/{id}`).
- Async in-memory job runner implemented.
- Real TTS HTTP adapter and FFmpeg video renderer implemented.
- Docker and Docker Compose setup included.

## Project Structure

```text
cmd/api/                 # API entrypoint
internal/config/         # Environment configuration
internal/httpserver/     # HTTP handlers and server wiring
internal/pipeline/       # Job models and async runner
internal/storage/        # In-memory job store
internal/trends/         # Trends provider interface and mock impl
internal/tts/            # TTS client interface and HTTP adapter
internal/video/          # Video builder interface and FFmpeg adapter
configs/                 # Clean-room spec and provenance logs
```

## Runtime Requirements

- `ffmpeg` available in `PATH` for local runs.
- A TTS HTTP service compatible with `POST /api/tts` that returns WAV bytes.

## Quick Start (Local)

```bash
cp .env.example .env
go run ./cmd/api
```

Health check:

```bash
curl http://localhost:8080/healthz
```

Create a job:

```bash
curl -X POST http://localhost:8080/v1/jobs \
	-H "Content-Type: application/json" \
	-d '{
		"category": "technology",
		"voice": "en-us",
		"target_seconds": 45,
		"country_code": "US"
	}'
```

List jobs:

```bash
curl http://localhost:8080/v1/jobs
```

## Docker

```bash
docker compose up --build
```

## Next Milestones

- Replace static trends provider with live data integration.
- Add persistent storage for jobs and artifact metadata.
- Add automated tests and CI workflow.
