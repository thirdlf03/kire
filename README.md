# kire

[![CI](https://github.com/thirdlf03/kire/actions/workflows/ci.yml/badge.svg)](https://github.com/thirdlf03/kire/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thirdlf03/kire.svg)](https://pkg.go.dev/github.com/thirdlf03/kire)
[![Go Report Card](https://goreportcard.com/badge/github.com/thirdlf03/kire)](https://goreportcard.com/report/github.com/thirdlf03/kire)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

[English](README.en.md)

Markdown を「話題の切れ目」で自動分割する Go CLI。LLM がドキュメント全体を読み、セマンティックな境界を直接検出する。

```bash
$ export GEMINI_API_KEY=your-key
$ kire spec.md
Wrote 3 segments to docs/spec/
Wrote docs/spec/index.md
```

## 特徴

- セグメントを結合すると原文とバイトレベルで一致する（lossless）
- LLM（Gemini）がテキストの意味を理解して分割位置を決定する
- 分割後も親見出しが各セグメントに自動付与される
- `--llm-refine` で embedding の cosine similarity を LLM の判断材料に追加できる
- SHA256 content-addressable ID、品質スコアでエージェント連携にも対応

## インストール

Homebrew (macOS / Linux):

```bash
brew install thirdlf03/tap/kire
```

Go:

```bash
go install github.com/thirdlf03/kire/cmd/kire@latest
```

ソースから:

```bash
git clone https://github.com/thirdlf03/kire.git
cd kire
just build
```

シェル補完を有効にするには、以下をシェルの設定ファイルに追加する:

```bash
# Bash (~/.bashrc)
source <(kire completion bash)

# Zsh (~/.zshrc)
source <(kire completion zsh)

# Fish (~/.config/fish/config.fish)
kire completion fish | source
```

## 例

[`example/`](example/) ディレクトリに入力と出力のサンプルがある。

Chat API の設計ドキュメント（1 ファイル・約 270 行）を入力すると:

```bash
$ kire example/input.md
```

話題の切れ目で 4 セグメントに分割される。

```
example/output/input/
├── 01-chat-api-設計ドキュメント.md   # 認証〜メッセージ送信
├── 02-チャンネル管理.md              # チャンネル管理〜プッシュ通知
├── 03-設定は-rest-api-で変更する.md  # 通知設定〜DB 設計
├── 04-ci-cd-パイプライン.md          # CI/CD〜監視
└── index.md                          # 目次 + Mermaid グラフ
```

各セグメントには親見出しがコンテキストとして自動付与される:

```markdown
<!-- context: Chat API 設計ドキュメント > メッセージング -->

### チャンネル管理
...
```

分割結果を結合すると原文とバイトレベルで一致する:

```bash
$ kire merge --strip-context example/output/input/ | diff example/input.md -
# 差分なし
```

## 使い方

```bash
# 基本的な使い方（GEMINI_API_KEY が必要）
export GEMINI_API_KEY=your-key
kire document.md

# 出力先とプレフィックスを指定
kire --out docs --prefix split document.md

# LLM-refine モード（embedding の cosine similarity を LLM に渡す）
kire --llm-refine document.md

# LLM-refine で embedder を指定
kire --llm-refine --embedder tfidf document.md

# 複数ファイルを一括処理
kire --in a.md --in b.md --out docs

# デバッグモードで詳細ログを出力
kire --debug document.md

# 強制的に上書き
kire --force document.md
```

`--dry-run` でファイル書き出しなしの確認ができる。

### オプション

```
--llm-model string        LLM モデル名 (デフォルト: gemini-2.5-flash-lite)
--llm-refine              embedding の cosine similarity を LLM の判断材料に追加
--overlap int             セグメント間の重複行数
--context-format          comment|front-matter|heading|none
--context-max-depth int   コンテキストの最大見出し深度 (0=無制限)
--embedder string         embedder (--llm-refine 用): auto|gemini|openai|ollama|tfidf|mock
--embed-model string      埋め込みモデル名 (--llm-refine 用)
--cache string            埋め込みキャッシュのファイルパス (--llm-refine 用)
--jsonl string            JSONL メタデータ出力 (--jsonl=- で stdout)
--agent-metadata          セグメント ID・品質スコアを含める
--state-file string       インクリメンタル処理の状態ファイル
--dag-json string         DAG を JSON で出力
--dag-dot string          DAG を DOT で出力
--prefix string           出力ファイル名のプレフィックス
--out string              出力ディレクトリ (デフォルト: docs)
--force                   既存出力ディレクトリを確認なしで上書き
--dry-run                 ファイル書き出しなしで実行
--quiet                   すべてのログ出力を抑制
--json                    JSON サマリを stdout に出力
```

### マージ

分割結果を元に戻す:

```bash
kire merge docs/my-spec/                          # stdout に出力
kire merge --strip-context -o merged.md docs/my-spec/  # コンテキストヘッダ除去
kire merge segment1.md segment2.md segment3.md     # ファイル個別指定
```

### DAG 出力

セグメント間のリンク依存グラフを出力:

```bash
kire --dag-json dag.json --dag-dot dag.dot document.md
```

### インクリメンタル処理

`--state-file` でソースハッシュを保存し、次回以降は変更なしならスキップ。変更があればセグメント単位で差分（Added/Removed/Modified/Unchanged）を検出する。

```bash
kire --state-file .kire-state.json document.md
```

## LLM-refine モード

`--llm-refine` を付けると、LLM に渡す前に embedding の cosine similarity を計算し、プロンプトに含める。類似度の低いギャップを LLM が境界として選びやすくなる。

```bash
# auto（GEMINI_API_KEY があれば Gemini、なければ TF-IDF）
kire --llm-refine document.md

# TF-IDF（API 不要、決定論的）
kire --llm-refine --embedder tfidf document.md

# OpenAI
export OPENAI_API_KEY=your-key
kire --llm-refine --embedder openai document.md

# Ollama（ローカル）
kire --llm-refine --embedder ollama document.md
```

利用可能な embedder:

| プロバイダ | API キー | デフォルトモデル | ローカル |
|-----------|----------|-----------------|---------|
| `gemini` | `GEMINI_API_KEY` | `gemini-embedding-001` | No |
| `openai` | `OPENAI_API_KEY` | `text-embedding-3-large` | No |
| `ollama` | 不要 (`OLLAMA_HOST` で接続先変更) | `nomic-embed-text` | Yes |
| `tfidf` | 不要 | — | Yes |
| `mock` | 不要 | — | Yes |

## マルチエージェント対応

`--agent-metadata` を付けると、セグメントごとのメタデータが JSONL / JSON サマリに含まれる。

メタデータの中身:

- `segment_id`: SHA256 の content-addressable ID。前後セグメントへの双方向リンク付き
- `coherence`: セグメント内ブロック間 cosine similarity の平均
- `confidence`: 境界の信頼度

Go API として使う場合、パイプラインの各ステージにフックを設定できる:

```go
cfg.Hooks = &pipeline.Hooks{
    OnParse:    func(blocks []model.Block) error { ... },
    OnEmbed:    func(embeddings []model.Embedding) error { ... },
    OnBoundary: func(result boundary.BoundaryResult) error { ... },
    OnSegment:  func(segments []model.Segment) error { ... },
    OnRender:   func(index int, content string) error { ... },
}
result, err := pipeline.Run(ctx, cfg)
```

## しくみ

```
Source → Parse → Tokenize → LineEstimate → [LLM-Refine? Embed+Sims : noop] → LLM Boundary → Optimize → IDs → Quality → Render → Output
```

goldmark で Markdown を AST 化してブロック単位に分解するところから始まる。各ブロックには SourceRange（byte offset）と HeadingPath が付く。

### パイプラインの詳細

1. Parse: Markdown を AST 化してブロック単位に分解
2. Tokenize: 各ブロックのトークン数を推定
3. LineEstimate: 各ブロックの行数を推定
4. Embed（`--llm-refine` 時のみ）: ブロックをベクトル化し、隣接ブロック間の cosine similarity を計算して LLM に渡す
5. LLM Boundary: LLM がドキュメント全体を読み、セマンティックな境界を検出。structured output で gap index の配列を返す
6. Optimize: LLM の境界をそのまま使用
7. IDs: SHA256 content-addressable ID を生成し、前後リンクを付与
8. Quality: セグメント品質メトリクスを計算
9. Render: SourceRange ベースで原文をバイトレベルで復元し、コンテキスト挿入と overlap 付与
10. Output: ファイルに書き出し、index.md と DAG を生成

<details>
<summary>パッケージ構成</summary>

```
cmd/kire/
├── main.go          CLI エントリポイント
├── root.go          フラグ定義
├── run.go           メイン実行ロジック
├── merge.go         merge サブコマンド
├── bench.go         ベンチマークサブコマンド
├── summary.go       JSON サマリ生成
├── config.go        設定構造体
├── logger.go        ログ設定
└── completion.go    シェル補完

internal/
├── model/           Block, Segment, Embedding, SourceRange
├── parser/          Markdown → []Block (goldmark)
├── tokenizer/       トークン推定
├── embedding/       Embedder interface + 各プロバイダ (--llm-refine 用)
│   ├── cached.go    キャッシュラッパー
│   └── concurrent.go 並列処理ラッパー
├── llmsplit/        LLM 境界検出 (Gemini structured output)
├── boundary/        BoundaryResult + 類似度計算ユーティリティ
├── segment/         セグメント最適化 + ID 生成 + 品質スコア
├── ctxheader/       親見出しコンテキスト挿入
├── output/          ファイル出力 + JSONL + マージ
├── cache/           埋め込みキャッシュ (JSON)
├── dag/             依存 DAG エクスポート (JSON/DOT)
├── concurrency/     Worker pool + rate limiter
├── vecmath/         ベクトル計算（cosine similarity）
├── pipeline/        パイプライン統合 + フック + インクリメンタル処理
└── eval/            ベンチマーク評価（Pk, WindowDiff, PRF）
```

</details>

## ベンチマーク

`kire bench` で分割品質を gold standard アノテーションに対して定量評価できる。評価指標は Pk（窓ベースの誤差、低いほど良い）、WindowDiff（Pk の改良版）、Precision/Recall/F1（境界の検出精度）。

テストデータは `testdata/bench_xl.md`（104 blocks, 18 gold boundaries）。語彙が意図的に重複するセクション構造で、heading-split では捉えられないトピック遷移を含む。

### LLM vs embedding ベース（final stage）

| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|-----:|-----:|------:|-----:|-----:|-----:|
| LLM | 18 | 0.20 | 0.20 | 0.47 | 0.44 | 0.46 |
| tfidf | 18 | 0.39 | 0.39 | 0.00 | 0.00 | 0.00 |
| gemini | 16 | 0.41 | 0.41 | 0.07 | 0.06 | 0.06 |

LLM モードが Pk で半分、F1 で圧倒的に優位。

```bash
just bench            # 品質評価 + パフォーマンスベンチマーク
just bench-quality    # 品質評価のみ
just bench-perf       # Go パフォーマンスベンチマークのみ
```

## 開発

開発ツール（Go, golangci-lint, just）は [devbox](https://github.com/jetify-com/devbox) で管理。

```bash
devbox shell       # 開発シェルに入る
```

```bash
just test          # テスト実行
just test-v        # 詳細出力
just cover         # カバレッジ確認
just lint          # go vet
just build         # ビルド
just run -- document.md
```

Lint と format:

```bash
devbox run fmt           # gofmt
devbox run fmt-check     # フォーマット差分チェック（CI 向け）
devbox run lint          # golangci-lint
```

## ライセンス

MIT
