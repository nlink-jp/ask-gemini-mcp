# RFP: ask-gemini-mcp

> Generated: 2026-06-06
> Status: Draft

## 1. Problem Statement

When an AI coding agent (Claude Code etc.) drives work using a single model's judgment alone, blind spots, biases, and false confidence emerge. A way to compare and reference multiple models' opinions is needed, but MCP clients without shell execution permission (claude.ai web, sandboxed environments, etc.) cannot use the existing `gem-*` CLI suite (gem-search, gem-summary, etc.) either.

`ask-gemini-mcp` provides an MCP server that forwards questions and consultations to Vertex AI Gemini via an MCP tool and returns the response. The primary use case is generative discussion (e.g., "Which of design A vs. B is better?" — bringing in a different perspective), ensuring a consultation channel to another model even in environments without shell access.

The intended users are AI agents (primarily MCP clients such as Claude Code/Desktop) that want to incorporate a different perspective when making design judgments or weighing alternatives, and the people using those agents.

## 2. Functional Specification

### Commands / API Surface

Expose exactly one MCP tool:

```
ask_gemini(prompt: string) -> text | error
```

- Single tool, single argument, deliberately simple
- No use-case-specific tools (review / discuss / factcheck etc.) — YAGNI
- Behavioral branching is expressed by the MCP client in the prompt itself

### Input / Output

**Input** (MCP tool inputSchema):

```json
{
  "type": "object",
  "properties": {
    "prompt": {
      "type": "string",
      "description": "Question or consultation to send to Gemini. Free-form; include context, background, and your current thinking."
    }
  },
  "required": ["prompt"]
}
```

**Output**:
- Success: return Gemini's answer text directly as `content: [{type: "text", text: "..."}]`
- Failure: structured error `{code, message, details}` emitted as content text (per feedback_structured_mcp_tool_errors.md)

No `system_prompt` argument, no metadata (token usage etc.). Simplicity over flexibility. If a system prompt is needed, the caller (Claude) can fold it into the user prompt.

### Configuration

Read from `~/.config/ask-gemini-mcp/config.toml`. Schema unified with the rest of `gem-*` (project_vertex_config_unified.md — 11-tool schema unification): `[gcp] project / location` + `[model] name`. Environment variable overrides follow the existing pattern.

Expected fields:
- `[gcp].project`: GCP project ID (required)
- `[gcp].location`: Vertex AI region (default `us-central1`)
- `[model].name`: Gemini model name (default `gemini-2.5-flash`)

Env overrides: `ASK_GEMINI_PROJECT` / `ASK_GEMINI_LOCATION` / `ASK_GEMINI_MODEL`; `GOOGLE_CLOUD_PROJECT` / `GOOGLE_CLOUD_LOCATION` are honored as fallbacks.

### External Dependencies

- **Vertex AI Gemini API** (Google Cloud)
- **Go SDK**: `google.golang.org/genai` (the current SDK established by project_genai_go_sdk.md)
- **MCP protocol**: stdio transport only
- **Authentication**: ADC (Application Default Credentials)

## 3. Design Decisions

### Implementation language: Go

- Easy single-binary distribution (rides the existing macOS notarize pipeline)
- Operational parity with Go-based `gem-*` (gem-search / gem-image / gem-query / gem-summary)
- Can borrow the MCP server skeleton from data-toolbox-mcp
- Established usage patterns for `google.golang.org/genai`

### Reused existing assets

- **data-toolbox-mcp** (util-series): Go MCP server skeleton — stdio transport, structured errors, `//go:build e2e` dummy MCP client harness
- **gem-summary / gem-search** (util-series): `google.golang.org/genai` SDK usage, config.toml + env settings pattern
- **nlk** (Go): `guard` (when needed in the future), `backoff` (for Gemini API rate limits)

### Out of Scope (explicit non-goals)

To prevent scope creep, the following are excluded from the start:

- **Multi-turn conversation** — the MCP client maintains conversation history; not needed here
- **Multiple LLM providers** (OpenAI / Anthropic / local LLMs) — the name `ask-gemini-mcp` signals the intent
- **RAG / external knowledge search** — focus on simple Q&A relay; leave that to gem-rag or gem-search
- **Persistent conversation history / logs** — strictly stateless
- **HTTP/SSE transport** — remote exposure is YAGNI. If ever needed, either add a native HTTP/SSE transport here or use a separate stdio↔HTTP bridge (mcp-guardian is the wrong direction — it converts an HTTP/SSE upstream into a stdio downstream, not vice versa)
- **OAuth / auth management** — ADC is sufficient
- **Multi-user / `workspace_id`** — local personal use
- **Prompt-injection mitigation** — caller (Claude) is responsible; `ask-gemini-mcp` behaves as a transparent pipe

## 4. Development Plan

### Phase 1: Core (minimum viable)

- Repo scaffold (CONVENTIONS.md compliant, under `_wip/ask-gemini-mcp/`)
- Go module init, `Makefile` (build → `dist/`)
- Config loader (`~/.config/nlink-jp/vertex-ai/config.toml` + env)
- MCP server stdio skeleton (ported from data-toolbox-mcp)
- `ask_gemini` tool implementation (calls `google.golang.org/genai`)
- Structured errors (`{code, message, details}`)
- Unit tests (config loader, error mapping, mocked genai client)
- E2E tests (`//go:build e2e` dummy MCP client harness → real Gemini calls)
- Initial README.md / README.ja.md / AGENTS.md / CHANGELOG.md

Functionally complete at this point; reviewable independently.

### Phase 2: Robustness

- Integrate `nlk/backoff` (Gemini API rate limits, feedback_gemini_api_rate_limit.md)
- Timeout / context cancellation (interrupt on MCP client disconnect)
- Logging (via stderr; do not pollute stdio MCP transport)
- Edge cases (empty prompt, oversized prompt, empty Gemini response, etc.)
- Additional E2E tests (error paths, timeouts)

Reviewable as a standalone hardening pass.

### Phase 3: Release

- Real-world dogfooding via Claude Code etc. (feedback_e2e_before_release.md)
- Documentation polish (MCP client config example, troubleshooting)
- `v0.1.0` release (per the 9-step release checklist)
- Add submodule to util-series, run `check-org.sh`
- Update org profile (`nlink-jp/.github/profile/README.md`) and nlink-web-site (EN/JA) (feedback_catalog_sync_two_surfaces.md)

### Schedule

Complete Phase 1 + Phase 2 in a single session. Phase 3 (release) happens on a later day.

## 5. Required API Scopes / Permissions

### Google Cloud (Vertex AI)

- Google Cloud project (`project_id`, `location` specified in config.toml)
- Vertex AI API enabled (`aiplatform.googleapis.com`)
- Authentication: ADC (`gcloud auth application-default login`)
- IAM role: `roles/aiplatform.user` (Vertex AI User)
- Scope: `https://www.googleapis.com/auth/cloud-platform`

### MCP client side

None. stdio transport uses only OS stdin/stdout. Just register the server in Claude Code / Claude Desktop's MCP server config (e.g., `~/.config/claude/claude_desktop_config.json`).

### Additional permissions

- Filesystem access: not required (only config.toml read)
- Network: outbound to Vertex AI endpoints only
- Data persistence: none

## 6. Series Placement

**Series: util-series**

**Reason**:
- Natural home for Vertex AI Gemini tools (the existing `gem-*` family: gem-search / gem-image / gem-query / gem-summary / gem-rag / gem-transcribe etc.)
- data-toolbox-mcp already established the precedent of "an MCP server living in util-series"
- Distribution form factor (Go single binary + macOS notarize) matches util-series standards
- lite-series excluded (local-LLM focused), cli-series excluded (service CLI clients), lab-series excluded (specification and patterns are already established)

## 7. External Platform Constraints

### Vertex AI Gemini side

- **Rate limits**: prior incidents with 429 floods on large bursts (feedback_gemini_api_rate_limit.md) → addressed in Phase 2 via `nlk/backoff`
- **Gemini 3 migration**: 2.5 → 3 confirmed after 2026-10-16 (post-GA) (project_gemini3_migration.md) → impact is a config change only, since the model name lives in config
- **Gemini 3 thought signature**: each Part's opaque continuation token must be echoed back on the next request (feedback_gemini3_thought_signature.md) → stateless design does not continue, so no impact
- **`files.upload()` unavailable**: that API is Vertex AI Developer API only (feedback_vertex_files_upload.md) → not used here, no impact

### MCP protocol side

- **MCP 2024-11-05 has no cancellation notification** (feedback_mcp_no_protocol_cancel.md) → as the server, detect MCP client disconnects during long Gemini calls via `context.Context` cancellation propagation and abort
- **MCP client validates inputSchema upfront** (feedback_mcp_client_validates_input_schema.md) → still validate on the server side as defense-in-depth
- **Structured errors** (feedback_structured_mcp_tool_errors.md) → covered in Phase 1

---

## Implementation Notes

### Config path finalization (2026-06-06)

The first draft listed `~/.config/nlink-jp/vertex-ai/config.toml` as a "shared path unified with existing gem-*". On entering implementation we surveyed the actual on-disk state and found that **all 14 existing gem-* tools use per-tool paths `~/.config/<tool-name>/config.toml`** (the unification covers only the TOML schema, not the path). This tool follows the same convention and uses `~/.config/ask-gemini-mcp/config.toml`.

---

## Discussion Log

### Naming (2026-06-06)

- Initial candidates: `gem-consult` / `llm-consult-mcp` / `second-opinion` / `ask-gemini`
- First proposal: `ask-llm-mcp` (a generic name leaving room for multi-provider support)
- Final decision: **`ask-gemini-mcp`** — narrow to Gemini only, signal that intent in the name

### Use case definition

- Among (a) code review, (b) fact-check, (c) generative discussion, (d) all-of-the-above, **(c) generative discussion** was chosen as the primary use case. Focus is on incorporating a different perspective during design comparisons and similar choices.

### Differentiation

The key differentiator vs. existing `gem-*` CLIs converged on "usable from MCP clients without shell execution permission." This is the most essential reason to build the tool. In shell-capable environments, `gem-*` already covers the same need.

### Tool design

- Single tool `ask_gemini(prompt)` adopted (over use-case-specific tools / mode parameters). Reason: with generative discussion as the focus, branching is unnecessary; just put it all in the prompt; YAGNI.
- Stateless adopted (over carrying `session_id`). The MCP client already holds the conversation history.
- Model selection fixed in config (over per-call argument or per-model tool exposure). Config unification is operationally cleaner given the impending Gemini 3 migration.
- stdio-only transport (over Streamable HTTP). Remote exposure is YAGNI. If ever needed, add a native HTTP/SSE transport or place a separate stdio↔HTTP bridge in front (mcp-guardian goes the other way and cannot be used here).
- Minimal I/O (no `system_prompt`, no metadata). Simplicity principle.

### Prompt-injection policy

Wrapping with `nlk/guard` was rejected in favor of a transparent-pipe policy. `ask-gemini-mcp` behaves as a simple consultation channel; the prompt contents are the caller's (Claude's) responsibility. Layered defense judged unnecessary here.

### Phase plan

Three-phase structure adopted (Core / Robustness / Release). Schedule: Phase 1+2 in one session → Phase 3 release on a later day. With data-toolbox-mcp and gem-* as reference implementations, the expected scope is small.

### Series placement

Locked to util-series. Natural fit given the gem-* family and data-toolbox-mcp precedent. The option of parking it in lab-series as an experiment was rejected (the spec and implementation patterns are already established).
