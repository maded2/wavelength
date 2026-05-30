# Release Notes — v0.2.0

## New Features

### Voice Input with Whisper Support
- **whisper.cpp server support** — Transcribe voice input using a local [whisper.cpp](https://github.com/ggerganov/whisper.cpp) server (`/inference` endpoint)
- **OpenAI-compatible Whisper** — Use any OpenAI-compatible `/v1/audio/transcriptions` endpoint (OpenAI API, Open WebUI, LiteLLM, vLLM)
- **Configurable whisper endpoint** — Set `voice.whisper_url` to point to a separate Whisper server, independent of the LLM endpoint
- **Server type selection** — Choose between `openai` and `whispercpp` via `voice.whisper_type` config field
- **Audio format conversion** — Automatic WebM→WAV conversion via ffmpeg for whisper.cpp compatibility
- **Voice status on landing page** — Voice availability shown alongside LLM status on the main page
- **Startup health check** — Probes the Whisper endpoint at startup with detailed `[VOICE]` logging

### Improved Developer Experience
- **Detailed startup logging** — `[VOICE]` prefixed logs show whisper type, URL, and check status
- **Friendly error messages** — User-friendly transcription error messages that clear on new input or recording

## Bug Fixes
- Handle whisper.cpp string response format (`{"text": "..."}`) in addition to array format
- Clear voice error status when user starts typing or begins a new recording

## Documentation
- Updated README with whisper.cpp configuration examples
- Added voice input server type comparison table
- Updated project structure to reflect current codebase

## Dependencies
- Added `github.com/go-audio/wav` for proper WAV file encoding

## Configuration Changes

New config fields:
```json
{
  "voice": {
    "whisper_url": "http://192.168.8.5:8085",
    "whisper_type": "whispercpp"
  }
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `voice.whisper_url` | string | `llm.endpoint` | Base URL for the transcription API |
| `voice.whisper_type` | string | `"openai"` | Server type: `openai` or `whispercpp` |

## Requirements
- For whisper.cpp: `ffmpeg` must be installed on the server
