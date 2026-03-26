# Clean-Room Specification (Initial)

This repository is an independent implementation for generating short-form AI video content.

## Scope

- Provide an HTTP API to create asynchronous generation jobs.
- Resolve script text from explicit topic/prompt input.
- Synthesize voiceover from generated script text.
- Render final short video output.
- Persist jobs and app settings in SQLite.

## Current Adapter Behavior

- TTS adapter calls configurable endpoint (`TTS_BASE_URL` + `TTS_SYNTH_PATH`) using query-parameter POST first (Coqui-compatible) and JSON POST fallback, then persists returned WAV bytes.
- Video adapter invokes `ffmpeg` to combine generated audio with a vertical 1080x1920 background into an MP4 output.
- Script generator calls an OpenAI-compatible chat-completions endpoint.
- Job metadata persists in SQLite (`DB_PATH`) for restart resilience.
- Runtime settings persist in SQLite and are editable via API.
- Autopilot mode can queue generation jobs on a fixed interval.

## Input Contract

- Endpoint: `POST /v1/jobs`
- JSON fields:
  - `topic` (optional string)
  - `prompt` (optional string)
  - `script_override` (optional string)
  - `voice` (optional string)
  - `orientation` (`portrait`/`landscape`/`square`/`original`/`custom`)
  - `custom_width` (optional int)
  - `custom_height` (optional int)
  - `background_video` (optional string)

## Output Contract

- Endpoint: `GET /v1/jobs/{id}` returns status and output path.
- Status values: `queued`, `running`, `failed`, `completed`.

## Implementation Rules

- Do not import or copy source files from predecessor repositories.
- Implement services from this specification and fresh design decisions.
- Keep provenance notes for major subsystems.
