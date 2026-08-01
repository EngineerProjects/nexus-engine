# Changelog

All notable changes to seshat are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).  
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [1.2.3] — 2026-08-01

### Fixed
- `internal/permissions/engine.go`: `get_file_metadata` (a read-only `stat()` call, `RequiresPermission: false` in its own definition) was missing from `isAlwaysSafeTool`, so in Auto permission mode every call went through the two-stage LLM classifier instead of being heuristically allowed — extra API calls per check, and, observed live, an incorrect "blocking for safety" deny when the classifier's own response failed to parse, compounding with provider rate limits.
- Same file: audited every registered tool for the same gap and added 19 more with an identical, individually-verified safety profile (`IsReadOnly: true`, `RequiresPermission: false`, `IsDestructive: false`, and a trivial passthrough `CheckPermissions` with no conditional logic) — `notebook_read`; the read/search/fetch tools in `devto`, `hackernews`, `reddit`, `twitter` (their write/publish siblings are correctly left requiring classification); `get_config`, `get_goal`, `rag_search`, `repo_map`, `workflow_draft`; `seshat_list_skills`, `seshat_read_skill`, `seshat_validate_skill`; `list_agents`, `wait_agent`.

### Notes
- Deliberately not touched in this pass, pending a follow-up decision: the browser read tools (`browser_snapshot`/`browser_screenshot`/etc. — mechanically read-only, but a live browser session can be authenticated into real accounts, a different risk profile than a filesystem read) and `job_output`/`task_get`/`task_list`/`task_output`/`monitor` (heterogeneous `CheckPermissions` implementations — `task_get` has real conditional deny logic that `isAlwaysSafeTool`'s short-circuit would bypass entirely, unlike the trivial-passthrough tools added above).

## [1.2.2] — 2026-08-01

### Fixed
- `internal/agent/goal`: the durable goal store wrote through to its SQLite backend on every plain `Get()` (not just on mutation), and held its mutex during backend I/O in `Update`/`RecordTokenUsage` — under an active goal the runner reads goal state up to three times per agent turn, so this added needless write amplification and let one session's backend latency stall unrelated sessions through the shared lock. Reads are now lock-scoped snapshots that never write, and mutation methods release the lock before calling the backend. Returned `Goal` values are now always defensive clones, so callers can no longer mutate store-internal state without going through the mutex.
- `internal/seshattui/ui/dialog/doctor.go`, `internal/seshattui/ui/model/ui.go`: opening or refreshing the TUI's Doctor dialog ran `DoctorReport()` (SQLite ping, `git`/`uv` subprocess checks) synchronously inside Bubble Tea's event loop, freezing the whole UI for as long as those checks took. Both paths now dispatch through a `tea.Cmd`.
- `cmd/cli/background.go`: `seshat run --bg --name X` had a TOCTOU race — the name-availability check only scanned already-saved session files, but a new session isn't saved until after its process starts, so two concurrent launches with the same name could both pass the check and leave two live sessions sharing one name. Name reservation is now atomic (`O_EXCL`) and happens before the process starts.
- `pkg/companion`: `companion.Save` wrote profile data with a plain truncate-and-write, so a crash mid-write could corrupt `companion.json` and turn the next load into a hard error instead of falling back to defaults. It now writes to a temp file and renames it into place, matching the pattern already used for background session metadata.

### Notes
- The `v1.2.0` GitHub release was cut prematurely from a feature branch before it merged to `main` and has been marked as a superseded pre-release; `v1.2.1` was the first correct build of the feature set below.

## [1.2.1] — 2026-08-01

### Added
- Goals created through `create_goal` are now persisted in the SQLite session database when `SessionSQLitePath` is configured, and successful session turns record token usage against the active goal so `get_goal`/`update_goal` can survive client/session restart.
- TUI chat now replaces hidden `task_create`/`task_update` plan tool calls with a compact "Plan" task checklist block, keeping plan progress visible without exposing internal task tools.
- `seshat doctor` command backed by reusable `pkg/doctor` diagnostics for local config, runtime paths, SQLite session storage, provider credentials, and helper tools.
- TUI settings hub now exposes the same doctor diagnostics in a scrollable "Doctor" dialog with refresh support.
- `pkg/repomap`, `seshat repomap`, and the read-only `repo_map` tool provide Go-first repository structure summaries, with optional system-context injection via `SESHAT_REPO_MAP=1`.
- Local background session commands: `seshat run --bg [--name NAME] "PROMPT"` starts a detached child process, while `seshat ps`, `seshat logs <id-or-name> [-f]`, `seshat attach <id-or-name>`, and `seshat kill <id-or-name>` manage tracked background runs.
- `pkg/workflow`, `Client.RunWorkflow`, and `seshat workflow run <file>` provide a minimal static YAML/JSON DAG runner: nodes declare `needs`, independent nodes run in parallel, and dependency outputs are injected into downstream prompts.
- `workflow_draft` and `sdk.DraftWorkflow` let agents and headless hosts validate and render reusable static workflow DAG definitions before saving or running them.
- Workflow nodes now support `kind: agent|verifier|critic|router`; role-specific prompt scaffolding helps agents build review, critique, and routing stages while keeping the runner's execution model static.
- `pkg/companion`, `ClientConfig.Companion`, and `seshat companion` add a headless-first companion profile that can inject a lightweight collaboration presence into sessions without replacing the core Seshat system prompt.
- Kimi/Moonshot is now consistently available from CLI and TUI provider setup, using the international `https://api.moonshot.ai/v1` endpoint and OpenAI-compatible Bearer auth.

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
