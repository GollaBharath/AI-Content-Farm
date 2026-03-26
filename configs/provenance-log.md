# Provenance Log

## 2026-03-26

- Initialized clean-room Go repository structure.
- Implemented fresh HTTP API and async job runner from written spec.
- Added stub trends, TTS, and video layers with new interfaces.
- Added Docker and Compose setup for API + external TTS service integration.
- Replaced mock adapters with HTTP-based TTS synthesis and FFmpeg rendering.
- Added TTS runtime configuration (`TTS_SYNTH_PATH`, `TTS_TIMEOUT_SECONDS`).
- Added 24/7 operational components: persistent job store, autopilot scheduler, container restart policy, and health checks.
- Removed trends/subtitle dependence from the active runtime architecture.
- Added OpenAI-compatible LLM script generation flow.
- Added built-in mobile-first web control panel served by the Go API.
- Migrated job persistence to SQLite.
- Added SQLite-backed runtime settings API (`/api/settings`).
- Added script approval flow (`/v1/scripts/generate` + `script_override`).
- Added background video library API (`/api/videos`, `/api/videos/upload`).
- Added orientation and custom dimension render controls to FFmpeg pipeline.
