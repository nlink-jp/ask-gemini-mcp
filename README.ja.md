# ask-gemini-mcp

[English version](README.md)

`ask_gemini(prompt)` という単一ツールを公開する Model Context Protocol
(MCP) サーバー。プロンプトを Vertex AI Gemini に転送し、応答を返します。

主用途は AI コーディングエージェント (Claude Code, Claude Desktop, …)
のセカンドオピニオン取得。シェル実行権限のない MCP クライアントから
既存の `gem-*` CLI を直接呼べない場面で有用です。

## ステータス

リリース前。`_wip/` 配下に scaffold 済み。MCP サーバー本体は後続コミット
で配線します。設計 RFP は [`docs/ja/ask-gemini-mcp-rfp.ja.md`](docs/ja/ask-gemini-mcp-rfp.ja.md)
を参照。

## クイックスタート

```sh
# 初回リリース後にインストール
go install github.com/nlink-jp/ask-gemini-mcp@latest

# Vertex AI 認証 (Application Default Credentials)
gcloud auth application-default login

# 設定
mkdir -p ~/.config/ask-gemini-mcp
cp config.example.toml ~/.config/ask-gemini-mcp/config.toml
$EDITOR ~/.config/ask-gemini-mcp/config.toml  # [gcp].project を設定
```

MCP クライアントに登録します。Claude Desktop の場合は
`~/.config/claude/claude_desktop_config.json` に以下を追記:

```json
{
  "mcpServers": {
    "ask-gemini": {
      "command": "/path/to/ask-gemini-mcp"
    }
  }
}
```

Claude Code の場合:

```sh
claude mcp add ask-gemini /path/to/ask-gemini-mcp
```

## 設定

優先順位: 組み込みデフォルト → TOML ファイル → 環境変数。

- **設定ファイル**: `~/.config/ask-gemini-mcp/config.toml` (`-c` フラグで上書き可)
- **環境変数**: `ASK_GEMINI_*` (ツール固有) > `GOOGLE_CLOUD_*` (汎用フォールバック)

| 変数                       | 必須 | デフォルト           |
|----------------------------|------|----------------------|
| `ASK_GEMINI_PROJECT`       | はい | —                    |
| `ASK_GEMINI_LOCATION`      | いいえ | `us-central1`      |
| `ASK_GEMINI_MODEL`         | いいえ | `gemini-2.5-flash` |

## ビルド

```sh
make build      # → dist/ask-gemini-mcp
make test       # go test ./...
make build-all  # 5 プラットフォーム向けクロスコンパイル
```

## ライセンス

[MIT](LICENSE)
