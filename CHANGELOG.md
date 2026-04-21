# Changelog

## 0.1.6

### Fixed

- Release workflow now uses dynamic version from git tag instead of hardcoded value.

## 0.1.5

### Added

- `onboard` command for interactive global configuration setup.
- Saves default embedding settings to `~/.codeindex/default_settings.json`.
- Supports providers: openai, ollama, openrouter, mistral, gemini, lmstudio, llamacpp, openai-compatible.
- CLI flags mode: `--provider`, `--model`, `--base-url`, `--api-key-env`.

## 0.1.4

### Changed

- Bumped the project version to 0.1.4.

## 0.1.3

### Added

- Configurable rate limiting for embedding API requests to prevent throttling.
- Configurable HTTP timeout for embedding requests.
- Retry system with exponential backoff for transient errors (429, 502, 503, timeouts).
- Automatic skip to next file when all retries fail during indexing.
- New configuration fields: `rate_limit`, `timeout`, `max_retries`, `retry_initial_delay`, `retry_max_delay`.
- Hybrid search combining vector and keyword matching (TF-IDF).
- Improved CLI output with better progress feedback.

### Changed

- Enhanced error handling for embedding providers.
- Updated documentation with rate limiting, timeout, and retry configuration examples.
- Added 23 new unit tests for retry and rate limiting logic.

## 0.1.2

### Changed

- `init` now seeds project settings from the user's home default settings when available.
- The indexer reads nested `.gitignore` files and supports configurable worker/checkpoint settings.
- Default index output is quieter unless `--verbose` is used.

## 0.1.1

### Changed

- Improved installation flow for Unix and Windows users.
- Added release packaging for `.tar.gz` and `.zip` assets.
- Improved incremental indexing feedback and terminal progress display.

## 0.1.0

### Added

- Go rewrite of cocoindex-code.
- Cloud-only embeddings via configurable API providers.
- Support for OpenAI, OpenAI-compatible, OpenRouter, Mistral, Gemini, Ollama, and LM Studio.
- Incremental indexing with per-file change detection.
- Per-file progress output, spinner, and release packaging workflow.
- Search, status, init, and release binary generation in Go.
