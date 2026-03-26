# Clean-Room Specification (Initial)

This repository is an independent implementation for generating short-form AI video content.

## Scope

- Provide an HTTP API to create asynchronous generation jobs.
- Resolve a topic from explicit input or a trends provider.
- Synthesize voiceover from generated script text.
- Render final short video output.

## Current Adapter Behavior

- TTS adapter sends `POST` JSON to configurable endpoint (`TTS_BASE_URL` + `TTS_SYNTH_PATH`) and persists returned WAV bytes.
- Video adapter invokes `ffmpeg` to combine generated audio with a vertical 1080x1920 background into an MP4 output.

## Input Contract

- Endpoint: `POST /v1/jobs`
- JSON fields:
  - `topic` (optional string)
  - `category` (optional string)
  - `voice` (optional string)
  - `target_seconds` (optional int)
  - `country_code` (optional string)

## Output Contract

- Endpoint: `GET /v1/jobs/{id}` returns status and output path.
- Status values: `queued`, `running`, `failed`, `completed`.

## Implementation Rules

- Do not import or copy source files from predecessor repositories.
- Implement services from this specification and fresh design decisions.
- Keep provenance notes for major subsystems.
