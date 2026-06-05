# RFP: ask-gemini-mcp

> Generated: 2026-06-06
> Status: Draft

## 1. Problem Statement

AI コーディングエージェント (Claude Code 等) が単一モデルの判断のみで作業を進めると、思考の偏り・盲点・誤った確信が発生する。複数モデルの意見を比較・参照する手段が必要だが、シェル実行権限のない MCP クライアント (claude.ai web、サンドボックス環境等) からは既存の gem-* CLI 群 (gem-search/gem-summary 等) も使えない。

本ツール `ask-gemini-mcp` は、MCP ツール経由で Vertex AI Gemini に質問・相談を投げ、回答を返す MCP サーバーを提供する。創発的議論 (「設計案 A と B どっちが良い?」のような視点取り込み) を主用途とし、シェルアクセスがない環境でも別モデルへの相談チャネルを確保する。

利用者は、設計判断や代替案検討の局面で異なる視点を取り入れたい AI エージェント (主に MCP クライアントとして動作する Claude Code/Desktop) およびその利用者を想定する。

## 2. Functional Specification

### Commands / API Surface

MCP ツールを 1 個のみ公開する:

```
ask_gemini(prompt: string) -> text | error
```

- 単一ツール・単一引数のシンプル設計
- 用途別ツール (review/discuss/factcheck 等) には分けない (YAGNI)
- 振る舞いの分岐は MCP クライアント側がプロンプトで表現する

### Input / Output

**Input** (MCP tool inputSchema):

```json
{
  "type": "object",
  "properties": {
    "prompt": {
      "type": "string",
      "description": "Geminiへの相談・質問内容。コンテキスト・背景・現状の自分の考えを含めて自由形式で書く。"
    }
  },
  "required": ["prompt"]
}
```

**Output**:
- 成功時: Gemini の回答テキストを `content: [{type: "text", text: "..."}]` でそのまま返却
- 失敗時: 構造化エラー `{code, message, details}` を content text に出力 (feedback_structured_mcp_tool_errors.md 準拠)

`system_prompt` 引数やメタデータ (トークン使用量等) は返さない。シンプル原則。必要なら呼び出し側 (Claude) が user prompt に組み込めば足りる。

### Configuration

`~/.config/ask-gemini-mcp/config.toml` を読み込む。schema は既存 gem-* と統一 (project_vertex_config_unified.md の 11 ツール統一): `[gcp] project / location` + `[model] name`。環境変数による上書きも従来パターンを踏襲する。

設定項目 (想定):
- `[gcp].project`: GCP プロジェクト ID (必須)
- `[gcp].location`: Vertex AI リージョン (デフォルト `us-central1`)
- `[model].name`: 使用する Gemini モデル名 (デフォルト `gemini-2.5-flash`)

env override: `ASK_GEMINI_PROJECT` / `ASK_GEMINI_LOCATION` / `ASK_GEMINI_MODEL`、`GOOGLE_CLOUD_PROJECT` / `GOOGLE_CLOUD_LOCATION` をフォールバックとして読む。

### External Dependencies

- **Vertex AI Gemini API** (Google Cloud)
- **Go SDK**: `google.golang.org/genai` (project_genai_go_sdk.md で確定済の現行 SDK)
- **MCP プロトコル**: stdio transport のみ
- **認証**: ADC (Application Default Credentials)

## 3. Design Decisions

### 実装言語: Go

- 単一バイナリ配布の容易性 (macOS notarize 含む既存パイプラインに乗る)
- Go 製 gem-* (gem-search/gem-image/gem-query/gem-summary) と運用統一
- data-toolbox-mcp の MCP サーバー骨格を流用可能
- `google.golang.org/genai` SDK 利用パターンが既に蓄積済

### 流用する既存資産

- **data-toolbox-mcp** (util-series): Go 製 MCP サーバーの実装骨格。stdio transport、構造化エラー、`//go:build e2e` での dummy MCP client harness。
- **gem-summary / gem-search** (util-series): `google.golang.org/genai` SDK 利用パターン、config.toml + env 設定パターン。
- **nlk** (Go): `guard` (将来必要時)、`backoff` (Gemini API レート制限対策)。

### Out of Scope (明示的にやらないこと)

scope creep を防ぐため以下を最初から除外する:

- **マルチターン会話** — MCP クライアント側が会話履歴を持つので不要
- **複数 LLM プロバイダ対応** (OpenAI / Anthropic / ローカル LLM) — `ask-gemini-mcp` の命名で意図表明
- **RAG / 外部知識検索** — 単純な Q&A 中継に専念。gem-rag や gem-search に任せる
- **会話履歴・ログ永続化** — ステートレス徹底
- **HTTP/SSE トランスポート** — リモート公開は YAGNI。必要になったら HTTP/SSE transport を自前追加するか、stdio↔HTTP ブリッジを別途利用する (mcp-guardian は HTTP/SSE 上流 → stdio 下流の方向専用なのでこの用途には使えない)
- **OAuth / 認証管理** — ADC 完結
- **マルチユーザー対応 / workspace_id** — ローカル個人用途
- **プロンプトインジェクション対策** — 呼び出し側 (Claude) の責任。`ask-gemini-mcp` は透過パイプとして振る舞う

## 4. Development Plan

### Phase 1: Core (最小動作)

- リポジトリ scaffold (CONVENTIONS.md 準拠、`_wip/ask-gemini-mcp/` 配下)
- Go モジュール初期化、`Makefile` (build → `dist/`)
- 設定ローダー (`~/.config/nlink-jp/vertex-ai/config.toml` + env)
- MCP サーバー stdio 骨格 (data-toolbox-mcp から移植)
- `ask_gemini` ツール実装 (`google.golang.org/genai` 呼び出し)
- 構造化エラー (`{code, message, details}`)
- 単体テスト (config ローダー、エラー変換、モック genai クライアント)
- E2E テスト (`//go:build e2e` で dummy MCP client harness → 実 Gemini 呼び出し)
- README.md / README.ja.md / AGENTS.md / CHANGELOG.md 初版

この時点で機能完結し、独立にレビュー可能。

### Phase 2: Robustness (堅牢化)

- `nlk/backoff` 統合 (Gemini API レート制限対策、feedback_gemini_api_rate_limit.md)
- タイムアウト・コンテキストキャンセル (MCP クライアント切断時の中断)
- ロギング (stderr 経由、stdio MCP transport を汚さない)
- エッジケース処理 (空 prompt、超長 prompt、Gemini からの空応答等)
- E2E テスト追加 (エラーパス、タイムアウト)

この時点で堅牢化単独でレビュー可能。

### Phase 3: Release (リリース)

- Claude Code 等での実機ドッグフード (feedback_e2e_before_release.md)
- ドキュメント仕上げ (MCP クライアント設定例、トラブルシュート)
- `v0.1.0` リリース (リリースチェックリスト準拠、9-step プロセス)
- util-series サブモジュール追加、`check-org.sh` 確認
- org profile (`nlink-jp/.github/profile/README.md`) と nlink-web-site (EN/JA) 更新 (feedback_catalog_sync_two_surfaces.md)

### スケジュール

Phase 1 + Phase 2 を 1 セッションで完了させる。Phase 3 (リリース) は後日。

## 5. Required API Scopes / Permissions

### Google Cloud (Vertex AI)

- Google Cloud プロジェクト (config.toml で `project_id`, `location` 指定)
- Vertex AI API 有効化 (`aiplatform.googleapis.com`)
- 認証: ADC (`gcloud auth application-default login`)
- IAM ロール: `roles/aiplatform.user` (Vertex AI User)
- 必要スコープ: `https://www.googleapis.com/auth/cloud-platform`

### MCP クライアント側

特になし。stdio transport のため OS の標準入出力のみ。Claude Code / Claude Desktop の MCP server 設定 (例: `~/.config/claude/claude_desktop_config.json`) に登録するだけ。

### 追加権限

- ファイルシステムアクセス: 不要 (config.toml 読み込みのみ)
- ネットワーク: Vertex AI エンドポイントへの outbound のみ
- データ永続化: なし

## 6. Series Placement

**Series: util-series**

**Reason**:
- Vertex AI Gemini 使用ツールの集約先 (gem-search / gem-image / gem-query / gem-summary / gem-rag / gem-transcribe 等の gem-* 群が既存)
- data-toolbox-mcp が「util-series の MCP サーバー」という先例を確立済
- 配布形態 (Go 単一バイナリ + macOS notarize) が util-series 標準と一致
- lite-series は対象外 (ローカル LLM 中心)、cli-series は対象外 (サービス CLI クライアント枠)、lab-series は仕様が確立しているため不要

## 7. External Platform Constraints

### Vertex AI Gemini 側

- **レート制限**: 大量逐次リクエストで 429 連発の既往 (feedback_gemini_api_rate_limit.md) → Phase 2 で `nlk/backoff` により対応
- **Gemini 3 移行**: 2.5 → 3 への移行が 2026-10-16 以降 GA で確定 (project_gemini3_migration.md) → config でモデル指定可能なので影響は config 変更のみ
- **Gemini 3 thought signature**: 各 Part の opaque continuation token を次リクエストに echo back 必須 (feedback_gemini3_thought_signature.md) → ステートレス設計のため continuation せず影響なし
- **`files.upload()` 不可**: Vertex AI Developer API 専用 (feedback_vertex_files_upload.md) → 本ツールでは使用しないので影響なし

### MCP プロトコル側

- **MCP 2024-11-05 にキャンセル通知なし** (feedback_mcp_no_protocol_cancel.md) → サーバー側として、長時間 Gemini 呼び出し中の MCP クライアント切断は `context.Context` での cancel 伝播で検知し打ち切る
- **MCP クライアント inputSchema 事前検証** (feedback_mcp_client_validates_input_schema.md) → サーバー側でも defense-in-depth として検証
- **構造化エラー** (feedback_structured_mcp_tool_errors.md) → Phase 1 で対応

---

## Implementation Notes

### config path の確定 (2026-06-06)

初稿では `~/.config/nlink-jp/vertex-ai/config.toml` という共通 path を「既存 gem-* と統一」と記載していたが、実装着手時の調査で **既存 gem-* 全 14 ツールはすべて個別 path `~/.config/<tool-name>/config.toml` を使用** していることが判明 (統一されているのは TOML schema のみ)。本ツールも同じ流儀に従い `~/.config/ask-gemini-mcp/config.toml` を採用した。

---

## Discussion Log

### 命名 (2026-06-06)

- 初期候補: `gem-consult` / `llm-consult-mcp` / `second-opinion` / `ask-gemini`
- 第 1 案: `ask-llm-mcp` (汎用名で複数プロバイダ対応の余地を残す)
- 最終決定: **`ask-gemini-mcp`** (Gemini 1 本に絞る意図を命名で明示)

### 用途定義

- (a) コードレビュー (b) 事実確認 (c) 創発的議論 (d) 全部 の選択肢から **(c) 創発的議論** を主用途と確定。設計案比較等の異なる視点取り込みに焦点。

### 差別化点

既存 gem-* CLI 群との差別化は「MCP プロトコル経由でシェル実行権限がないクライアントから利用できる」点に集約された。これが採用理由として最も本質的。シェルが使える環境なら gem-* で代替可能。

### ツール設計

- 単一ツール `ask_gemini(prompt)` を採用 (vs 用途別複数ツール / mode パラメータ)。理由: 創発的議論主用途なら分岐不要、プロンプトに全部書けば良い、YAGNI。
- ステートレス採用 (vs `session_id` 持ち回り)。MCP クライアント側が会話履歴を持つため十分。
- モデル選択は config 固定 (vs 引数指定 / モデル別ツール公開)。Gemini 3 移行を見据えた config 統一が運用しやすい。
- トランスポートは stdio のみ (vs Streamable HTTP)。リモート展開は YAGNI。必要になったら自前で HTTP/SSE transport を追加するか stdio↔HTTP ブリッジを別途利用する (mcp-guardian は逆方向専用なので不可)。
- 入出力は最小限 (system_prompt なし、メタデータなし)。シンプル原則。

### プロンプトインジェクション方針

`nlk/guard` でのラップは見送り、透過パイプ方針を採用。`ask-gemini-mcp` は相談チャネルとして単純に振る舞い、prompt の中身は呼び出し側 (Claude) の責任とする。多重防衛は過剰と判断。

### Phase 計画

3 段階構成 (Core / Robustness / Release) を採用。Phase 1+2 を 1 セッションで完了 → Phase 3 を後日リリース、というスケジュール。data-toolbox-mcp + gem-* の参考実装があるため実装規模は小さい想定。

### 配置先

util-series で確定。gem-* 群と data-toolbox-mcp の先例があるため自然な選択。lab-series で実験扱いとする選択肢は却下 (仕様・実装パターンが既に確立)。
