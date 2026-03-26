# Provenance Log

## 2026-03-26

- Initialized clean-room Go repository structure.
- Implemented fresh HTTP API and async job runner from written spec.
- Added stub trends, TTS, and video layers with new interfaces.
- Added Docker and Compose setup for API + external TTS service integration.
- Replaced mock adapters with HTTP-based TTS synthesis and FFmpeg rendering.
- Added TTS runtime configuration (`TTS_SYNTH_PATH`, `TTS_TIMEOUT_SECONDS`).
