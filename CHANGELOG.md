# Changelog

All notable changes to seshat are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).  
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [1.1.0] — 2026-08-01

### Added
- `excel_edit` tool: create/edit `.xlsx` cells and formulas natively (excelize), with a read-before-write gate matching `write_file`/`edit_file`.
- `docx_edit` tool: create Word documents from plain/markdown-ish text (`#`.."######" → real Word heading styles) or find/replace edit existing ones, with fuzzy-match diagnostics on a near-miss.
- `write_pdf` tool: create/append/delete-pages PDFs via `go-pdf/fpdf` + `pdfcpu` — no headless-browser dependency.
- `search_start` tool: cancellable, streaming background content search (reuses `job_output`/`job_kill`) that also searches inside `.docx`/`.pptx`/`.xlsx` content, which ripgrep can't see.
- `get_config` tool (read-only): exposes the effective security policy — denied command fragments/patterns, commands requiring approval, read/write-denied path prefixes, file-read limits, sandbox availability, default shell.
- `internal/tools/files/docling` package: `read_document_url` (moved from `files/read_url`) plus new `docling_convert` for explicit local-file conversion (OCR, complex slide decks, audio transcription) — FileRead's own automatic docling fallback is unchanged.
- `rag_delete` tool: delete an entire corpus or a single file's chunks — previously unreachable capability (`Service.DeleteNamespace`/`DeleteFileChunks` existed but no tool exposed them).
- `rag_ingest` gains an optional `file_id` param (defaults to `filename`) for idempotent re-ingest — re-ingesting the same file now replaces its chunks instead of accumulating duplicates.
- **Vectorless RAG**: `rag_ingest`/`rag_search` now work without any embedding provider configured, via pure BM25/keyword ranking (real BM25 through SQLite FTS5; keyword-overlap scoring on HNSW/Memory). Previously RAG was hard-disabled with no embedder at all.
- RAG now falls back to the SQLite vector backend when the embedded HNSW backend is unavailable — notably on Windows, where `github.com/coder/hnsw`'s atomic-write dependency doesn't build, which previously disabled RAG entirely regardless of configuration.
- Reranking now actually runs: `SetReranker` was implemented but never wired up; RAG search now reranks results when a reranker is configured.
- Reranker generalized beyond LangSearch's hosted API into `HTTPReranker`, additionally speaking HuggingFace TEI's shape — point `RAG_RERANK_URL` at a self-hosted TEI/vLLM instance (e.g. `BAAI/bge-reranker-v2-m3`) for a free alternative requiring no API key.
- `SemanticChunker` (embedding-similarity sentence grouping) is now actually used by the CLI's RAG wiring when an embedder is configured, instead of always falling back to the naive `ParagraphChunker`.
- No-API-key DuckDuckGo fallback provider for `web_search`.
- `pkg/rag/embedder`: `FromEnv()`/`NewFromEnv()` (parity with `pkg/rag/reranker`).
- `pkg/web/search/providers`: `NewDuckDuckGoProvider()` and `ProviderModeDuckDuckGo` (parity with the other providers in the same file).
- `pkg/rag`: `ParagraphChunker`, `SemanticChunker`, `DefaultChunker()`, `NewSemanticChunker()`, `ArtifactKey()` — `NewService` requires a `Chunker` argument, but no implementation was previously reachable from the public API.
- `pkg/image` + `pkg/image/providers` (OpenAI DALL-E, Google Imagen) and `pkg/audio/stt` + `pkg/audio/tts` + `pkg/audio/providers` (OpenAI Whisper/TTS): standalone client libraries, mirroring `pkg/docling` — a host application can now call image/speech generation directly (e.g. a UI where a human types a prompt) without going through the agent's LLM tool-calling loop for a deterministic action. The same provider implementations power both the direct call and the `generate_image`/`text_to_speech`/`speech_to_text` agent tools. `WithSTTBaseURL`/`WithOpenAIBaseURL`/`WithTTSBaseURL` all support pointing at a self-hosted, OpenAI-API-compatible server (e.g. a local whisper.cpp instance) with no API key.
- `pkg/fim` + `pkg/fim/providers` (Mistral Codestral, DeepSeek): same standalone-client treatment for Fill-in-the-Middle code completion, previously reachable only through the `code_complete` agent tool. FIM is explicitly suited to IDE integrations/in-editor ghost text, where a tool-calling LLM turn per keystroke makes no sense.
- Session auto-title generation feature: AI auto-titles sessions after the first successful turn using the user message.
- `OnSessionTitled` callback and `DisableTitleGeneration` configuration options in `ClientConfig`.
- `CredentialResolver` interface in `pkg/sdk` — allows per-request API key injection without touching `ClientConfig.APIKey`
- `generate_image` tool backed by `image.Generation` interface (OpenAI DALL-E 3 and Google Gemini Imagen providers)
- `text_to_speech` and `speech_to_text` tools backed by pluggable audio providers (OpenAI TTS-1, Whisper)
- OpenTelemetry tracing (`internal/monitoring/tracer.go`) — OTLP gRPC export, no-op when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset
- `pkg/monitoring` public package exposing `InitTracer` and `Tracer`
- `.github/workflows/ci.yml` — build, test (race), lint on push/PR
- `.github/workflows/release.yml` — cross-platform binary release on `v*` tags
- `.githooks/pre-commit` — gofmt + go vet + golangci-lint (install with `make hooks`)
- `SetDefault` and `GetDefaultByUserID` on `ProviderSettingStore`
- Community files: `SECURITY.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `AGENTS.md`
- Architecture diagrams in `docs/vision/diagrams.md` (Mermaid)
- Project vision and 3-level roadmap in `docs/vision/`

### Changed
- Makefile: replaced `build-api` (removed) with `build-grpc`; added `fmt`, `vet`, `hooks` targets
- `docs/architecture.md`: removed `cmd/api` from entry points (it lives in nexus-product)
- `docs/transports.md`: translated to English, removed HTTP API section (nexus-product), fixed absolute paths

### Fixed
- `internal/tools/files/patch/patch.go`: replaced `if HasSuffix` with `strings.TrimSuffix` (golangci-lint S1017)
- `internal/providers/auth.go`: removed OAuth credential values from log output (security: S1)
- `pkg/rag/embedder`: `EmbedTexts` now splits large requests into batches (default 64 texts), since some hosted embedding APIs (e.g. Mistral) reject an oversized request outright instead of truncating it
- `readDoclingFile` (FileRead's docling path) now records read-state on all three success paths, fixing a false "file has not been read yet" block on binary formats read via docling.
- A re-ingested RAG file that shrank (fewer chunks than its previous version) no longer leaves the old version's stale trailing chunks behind — `Service.Ingest` now cleans them up via the existing (but previously unused) `DeleteFileChunks`.
- `ParagraphChunker.Split` now slices by rune instead of byte index — a byte-index cut could land inside a multi-byte UTF-8 character (e.g. accented French text) and corrupt it at the chunk boundary.
- Fixed a "typed nil" bug found while wiring up vectorless RAG: passing a nil `*embedder.Embedder` into an `Embedder` interface parameter produced a non-nil interface wrapping a nil pointer, so `embedder != nil` checks were incorrectly true and the first call panicked on a nil receiver.

### Removed
- `cmd/api` — moved to `nexus-product` (the open-source engine does not own the HTTP product layer)
- `docs/backend-boundary.md` and `docs/internal-boundary-audit.md` — monorepo-era documents, no longer relevant
- `docs/archive/` — speculative archived docs removed
- `internal/tools/special/docx` (docxgo-based): registered until 2026-06-03, deregistered but left orphaned since 2026-06-18. Its `content` param was inserted verbatim with zero markdown parsing — the new `docx_edit` converts `#`.."######" to real Word heading styles instead. Also drops the now-unused `mmonterroca/docxgo/v2` dependency.
- `internal/tools/special/brief`: a "send message to the user" tool from the pre-rename codebase, never registered at any point in its history.
- `internal/tools/special/config/configTool.go`: an arbitrary key/value settings store predating the current `contract.Tool` interface (incompatible signatures — could never have been registered as-is), replaced by the real `get_config` tool.

[Unreleased]: https://github.com/KPO-Tech/seshat/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/KPO-Tech/seshat/compare/v1.0.4...v1.1.0
