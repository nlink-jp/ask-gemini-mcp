# ADR-0001: MCP のプロトコル層を nlink-jp/mcp にする — 新旧両方の時代を話す

| 項目 | 値 |
|-------|-----|
| Status | **Accepted**（2026-11-28） |
| Date | 2026-11-28 |
| Binds | ask-gemini-mcp |
| Decision makers | nlink-jp maintainers |
| Triggered by | 組織 ADR-030（MCP の実装は新旧両方の時代を話し、プロトコル層は nlink-jp/mcp だけから取る）。issue nlink-jp/ask-gemini-mcp#1 |

このリポで最初の ADR である。これまでの決定は RFP（`docs/ja/ask-gemini-mcp-rfp.ja.md`）にある。双子の ask-llm-mcp も、同じ決定を
自分の ADR-0001 に置く。

## Context

`internal/mcpserver`・`internal/transport`・`internal/jsonrpc` は、手書きの stdio MCP サーバだった。版は固定の
2024-11-05 で、旧方式（`initialize` の握手）だけを話す。そのため、2026-07-28 版の新方式専用のクライアントからは
つながらない。ask-gemini-mcp は家系 C（mcpserver。nlink-jp/mcp の文書 server-migration §1）に属し、その参照実装は
data-toolbox-mcp（同リポの ADR-0013）である。実際の相手の測り直し（2026-11-27、ADR-030 §7）は asn-lookup の
ADR-0001 にある。Claude Code 2.1.311・gem-agent・lagent は、どれも両対応だった。

## Decision

共通の方針（nlink-jp/mcp の `docs/ja/server-migration.ja.md`、v0.3.0）に従う。このリポに当てはまる項目は次のとおり。

1. **外から見える形は変えない（方針 §2）。** `mcpserver.New(name, version, transport, logger)`・`RegisterTool`・
   `Tool`・`ToolHandler`（`(string, error)` を返す）・`ContentBlock` を残し、足すものは無い。`Serve(ctx)` は、
   登録順に `AddTool` してから `ToolServer.Serve` を呼ぶ。ハンドラの文は text block 1 つに、`*toolerr.Error` は
   その `{code, message, details}` の JSON の isError に、ほかの error は文の isError に写す。呼び出しごとのログも
   残す（`tool call begin`、続けて所要時間とバイト数つきの `tool call ok`、または所要時間と誤りつきの `tool call
   failed`）。instructions は、これまでどおり出さない。
2. **transport（方針 §2）**: `transport.NewStdioTransport(in, out)` は、入出力の入れ物として残す（`In`・`Out`）。
   1 MiB の `bufio.Scanner` と、読み書きの処理と、その試験は消す。`internal/jsonrpc` は、プロトコル層だけが
   使っていたので消す。
3. **選択肢（方針 §4）**: `Workers` は 1（呼び出しは 1 つずつ処理していた）、`QueueDepth` は 256。ログは、`cmd` が
   stderr に作る slog をそのまま渡す。ロガーがなければ、`New` は Warn 以上を stderr に出す。
4. **変わる振る舞い（方針 §3）**:
   - 未知のツールは -32601 から -32602 になる。`ping` には -32601 ではなく `{}` を返す。
   - initialize の前の `tools/*` は -32602 になる。オブジェクトでない `arguments` も -32602 になる。
   - `"id":null` の要求は -32600 になる（これまでは通知として黙っていた）。
   - 1 行の上限は 1 MiB から 16 MiB になる。上限を超えた行は -32600 を返して読み続ける（これまでは止まった）。
     空行には答えなくなる（これまでは -32700）。
   - stdout への書き込みの失敗で止まる（これまではログに出して読み続けた）。
   - ツールの panic には -32603 を返し、サーバは動き続ける（これまではプロセスが終わった）。
   - 版は 2024-11-05 固定から、旧方式の交渉と新方式に変わる。
   - SIGINT・SIGTERM（`signal.NotifyContext`）で、読み取りの途中でもサーバの ctx が取り消される。動いている呼び出しは
     取り消され、`Serve` はモジュールのシャットダウンの猶予（5 秒）のうちに戻る。`Serve` は nil を返すので、呼び出し側の
     `context.Canceled` の見分けを消す。
5. **取り消し（方針 §5）**: `notifications/cancelled` で呼び出しの ctx が取り消される。送っている途中の Vertex AI Gemini への
   要求は止まり、再試行もしない（取り消しは、もとから再試行の対象ではなかった）。モジュールは、取り消された呼び出しには
   答えない。上流がすでに受け取った分は、使われたものとして残る。
6. **依存**: `github.com/nlink-jp/mcp` を足す。
7. **版**: minor を上げる（v0.3.0）。

## 見つけた不具合（別の fix コミットで直した）

- `ask_gemini` は、宣言していない引数を拒んでいたが、大文字小文字の違う宣言済みの名前（`prompt` に対する `Prompt`）は
  受け入れ、そのままモデルに問うていた。encoding/json が名前を大文字小文字を区別せずに突き合わせるためである
  （組織 ADR-021 §4）。これも名前を挙げて拒むようにした。直す前に落ちる試験つき。
- AGENTS.md と CLAUDE.md は、stdin を閉じると処理中の呼び出しが取り消される、と書いていた。実際にはそうならなかった。
  サーバは、動いている呼び出しが戻ってから次の行を読むので、EOF に気づくのはその後だった。いまの振る舞いを書いた。
  stdin を閉じると、届いた呼び出しにすべて答えてから終わる。取り消しは `notifications/cancelled` である。

## Consequences

- 新方式専用のクライアントからもつながる。旧方式のクライアントには、これまでどおり答える。
- 試験はモジュールの `client` で動かす（`internal/mcpserver`・`cmd`）。探りのクライアント 3 種の組み合わせを足した。
  Go の e2e（`e2e/`・`make test-e2e`）は、握手を自分で書く代わりに、モジュールのクライアントでバイナリを開く。

## Alternatives considered

| 案 | 採らない理由 |
|---|---|
| `mcpserver` を消し、ツールを直接モジュールに登録する | ツールと試験の登録の形が変わる。薄い層を残せば、ツールは変わらない |
| `ToolHandler` を data-toolbox-mcp と同じく `any` を返す形にする | 得るものが無いのにツールが変わる。文を block にするのは層が受け持つ |

## References

- 組織 ADR-030、ADR-025、ADR-021 §4
- nlink-jp/mcp v0.3.0: `docs/ja/server-migration.ja.md`
- data-toolbox-mcp ADR-0013（家系 C の参照実装）
- asn-lookup ADR-0001（実際の相手の測り直し）
- issue nlink-jp/ask-gemini-mcp#1
