# Permissions And Sandbox Roadmap

This document tracks the permission-system fixes completed during the July 2026 audit and the remaining sandbox work. It is intentionally scoped to the backend permission pipeline and sandbox strategy; frontend rendering fixes are mentioned only where they affect backend behavior.

## Status

| Area | Status |
|---|---|
| Auto-mode classifier reliability | Improved |
| `request_permissions` escalation grants | Fixed |
| Tool progress failure reporting | Fixed |
| Codex classifier/provider compatibility | Fixed |
| Docker-backed sandbox strategy | Deferred |

## Context

Seshat can run tools in several permission modes. In auto mode, an LLM classifier reviews tool calls that are not covered by deterministic allow rules. This is useful for catching risky actions, but it must not become the only gate for ordinary development work, especially on platforms where no OS-level sandbox is active.

On Windows, Landlock is unavailable and Docker may not be configured. In that environment, bash commands run without OS confinement unless the user explicitly configures a Docker sandbox or sets stricter sandbox requirements. The permission system therefore needs two layers:

1. Deterministic rules and durable grants for actions the user has approved.
2. Sandboxed execution for commands whose main risk is host-system impact.

The classifier should remain a fallback for ambiguous or risky actions, not the sole mechanism for routine project work.

## Completed Fixes

### 1. Permission escalation now reaches the downstream operation

`request_permissions` is designed for the model to ask before performing an operation that needs additional access. Before the fix, approving a `request_permissions` call only returned a granted payload to the model. It did not register a grant in the permission integrator, so the immediately following `write_file`, `edit_file`, or similar operation could still be denied.

The backend now records approved filesystem grants from `request_permissions`:

- `scope="turn"` grants are stored in memory and keyed by session and turn.
- `scope="session"` grants are persisted using the existing session approval storage.
- Grants are keyed by access kind and path, such as `grant::write::/workspace/out.py`.
- Directory grants cover descendant paths by matching the requested path and its ancestors.

Regression coverage lives in `internal/permissions/integration_test.go`.

### 2. `request_permissions` bypasses the auto classifier

The auto-mode classifier must not re-classify `request_permissions` itself. That created a self-defeating path: the model asked for permission, then the classifier blocked the permission request before a human could approve it.

The permission mode now handles `request_permissions` before classifier routing:

- Interactive sessions return `Ask`.
- Headless or promptless contexts return `Deny`.

Regression coverage lives in `internal/permissions/auto/auto_test.go`.

### 3. Auto-mode prompts use current tool names

The classifier prompts and transcript tool registry now use current tool names such as `read_file`, `edit_file`, `write_file`, `task_list`, and `task_update`. Historical aliases such as `Read` and `TodoWrite` remain in the registry only for compatibility with older transcripts and surfaces.

### 4. Codex provider compatibility

Codex Responses API calls are SSE-first in this provider path. The non-streaming decode path now parses the same SSE format used by the streaming path, instead of trying to decode the response as a single OpenAI Chat Completions JSON object.

The Codex request builder also drops `temperature`, because the reasoning models served through this provider reject it.

Regression coverage lives in `internal/providers`.

### 5. Tool progress now reflects handled failures

Tool calls that return a handled error now emit failed terminal progress instead of always reporting completed progress. Panic recovery in concurrent tool batches also emits a failed terminal progress event, so the frontend does not remain stuck on an intermediate stage.

Regression coverage lives in `internal/execution`.

## Remaining Sandbox Work

The next architectural step is a Docker-backed sandbox profile for bash and other command-execution tools.

### Phase 1: persistent workspace sandbox

Recommended baseline:

- Provide one default image with common development tools: Python, Node, Go-adjacent utilities, ripgrep, git, curl, and shell basics.
- Build or pull the image lazily on first use.
- Keep one persistent sandbox per workspace instead of one disposable container per tool call.
- Mount only the active workspace by default.
- Keep installed dependencies and generated files inside that workspace sandbox across turns and sessions.

Once command execution is truly confined to the workspace sandbox, sandboxed bash can move closer to deterministic allow behavior. Boundary-crossing actions should remain gated:

- host mounts outside the workspace,
- injected secrets or environment variables,
- broad network egress,
- privileged containers,
- Docker socket access,
- writes to host paths outside the workspace.

### Phase 2: named sandbox profiles

Multiple profiles can be added later if a concrete use case appears:

- `default-dev`
- `python-data`
- `node-web`
- `network-restricted`
- `no-network`

The initial implementation should still model sandbox configuration as a profile internally, so phase 2 is additive rather than a rewrite.

## Operational Guidance

For now:

- Prefer deterministic grants for explicitly approved repeated actions.
- Keep `request_permissions` specific: include exact paths, access kinds, and scope.
- Do not rely on the auto classifier as the only guard for command execution.
- Treat Docker sandboxing as the planned path for reducing classifier involvement in routine shell work.

## Validation

The July 2026 audit changes were validated with:

```bash
go test ./... -timeout 600s
```
