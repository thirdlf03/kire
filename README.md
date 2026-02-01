# kire

[![CI](https://github.com/thirdlf03/kire/actions/workflows/ci.yml/badge.svg)](https://github.com/thirdlf03/kire/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thirdlf03/kire.svg)](https://pkg.go.dev/github.com/thirdlf03/kire)
[![Go Report Card](https://goreportcard.com/badge/github.com/thirdlf03/kire)](https://goreportcard.com/report/github.com/thirdlf03/kire)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

[English](README.en.md)

Markdown を「話題の切れ目」で自動分割する Go CLI。見出し区切りではなく、embedding の類似度変化で境界を検出する。Markdown 版 [TextTiling](https://aclanthology.org/J97-1003/)。

```bash
$ kire spec.md
Wrote 3 segments to spec/
Wrote spec/index.md
```

## 特徴

- セグメントを結合すると原文とバイトレベルで一致する（lossless）
- `--min-tokens` / `--max-tokens` で LLM のコンテキストウィンドウに合わせた分割ができる
- 分割後も親見出しが各セグメントに自動付与される
- Gemini / OpenAI / Ollama / TF-IDF / Mock から embedder を選べる。API キーなしでも動く
- SHA256 content-addressable ID、品質スコアでエージェント連携にも対応

## インストール

```bash
go install github.com/thirdlf03/kire/cmd/kire@latest
```

ソースから:

```bash
git clone https://github.com/thirdlf03/kire.git
cd kire
just build
```

## 例

[`example/`](example/) ディレクトリに入力と出力のサンプルがある。

Chat API の設計ドキュメント（1 ファイル・約 270 行）を入力すると:

```bash
$ kire --embedder tfidf --min-tokens 100 --max-tokens 800 example/input.md
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
# とりあえず試す（API キー不要、Mock embedder で分割）
kire document.md

# 出力先とプレフィックスを指定
kire --out docs --prefix split document.md

# Gemini API を使う
export GEMINI_API_KEY=your-key
kire document.md

# API なしで使う（TF-IDF）
kire --embedder tfidf document.md

# 複数ファイルを一括処理
kire --in a.md --in b.md --out docs
```

`--dry-run` でファイル書き出しなしの確認、`--report` で HTML の境界レポートが出る。

全オプションは `kire --help` で。よく使うもの:

```
--min-tokens int     最小トークン数 (デフォルト: 300)
--max-tokens int     最大トークン数 (デフォルト: 3000)
--window int         類似度スムージング窓 (デフォルト: 3)
--threshold float    境界スコア閾値 (-1 = 自動)
--overlap int        セグメント間の重複行数
--embedder string    auto|gemini|openai|ollama|tfidf|mock
--cache string       埋め込みキャッシュのファイルパス
--jsonl              JSONL メタデータ出力 (--jsonl=- で stdout)
--agent-metadata     セグメント ID・品質スコアを含める
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

## 埋め込みプロバイダ

`--embedder` で選択。デフォルトは `auto`（`GEMINI_API_KEY` があれば Gemini、なければ TF-IDF）。

| プロバイダ | API キー | デフォルトモデル | ローカル |
|-----------|----------|-----------------|---------|
| `gemini` | `GEMINI_API_KEY` | `gemini-embedding-001` | No |
| `openai` | `OPENAI_API_KEY` | `text-embedding-3-large` | No |
| `ollama` | 不要 (`OLLAMA_HOST` で接続先変更) | `nomic-embed-text` | Yes |
| `tfidf` | 不要 | — | Yes |
| `mock` | 不要 | — | Yes |

```bash
# OpenAI
export OPENAI_API_KEY=your-key
kire --embedder openai document.md

# Ollama（ローカル）
kire --embedder ollama document.md
kire --embedder ollama --embed-model mxbai-embed-large document.md

# TF-IDF（API 不要、決定論的）
kire --embedder tfidf document.md
```

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
Source → Parse → ParaSplit → PseudoHeading → Tokenize → Embed → Boundary → Lock → Segment → Render → Output
```

goldmark で Markdown を AST 化してブロック単位に分解するところから始まる。各ブロックには SourceRange（byte offset）と HeadingPath が付く。長い段落は事前に分割し、見出しがないブロックには疑似見出し検出を適用する。

ブロックをベクトル化した後、隣接ブロック間の cosine similarity を計算し、smoothing → depth score → 閾値選定で境界を検出する。セクションロックにより見出し＋ボディが分断されないようにし、Greedy アルゴリズムでセグメントにまとめる。

最後に SourceRange ベースで原文をバイトレベルで復元し、コンテキスト挿入と overlap 付与を経てファイルに書き出す。

<details>
<summary>パッケージ構成</summary>

```
cmd/kire/
├── main.go          CLI エントリポイント
├── root.go          フラグ定義
├── run.go           メイン実行ロジック
├── merge.go         merge サブコマンド
└── summary.go       JSON サマリ生成

internal/
├── model/           Block, Segment, Embedding, SourceRange
├── parser/          Markdown → []Block (goldmark)
├── tokenizer/       トークン推定 (local/api/hybrid)
├── embedding/       Embedder interface + 各プロバイダ + Cached/Concurrent デコレータ
├── boundary/        TextTiling 境界スコアリング + confidence
├── segment/         Greedy 分割 + ID 生成 + 品質スコア
├── context/         親見出しコンテキスト挿入
├── output/          ファイル出力 + JSONL + マージ
├── cache/           埋め込みキャッシュ (JSON)
├── dag/             依存 DAG エクスポート (JSON/DOT)
├── concurrency/     Worker pool + rate limiter
└── pipeline/        パイプライン統合 + フック + インクリメンタル処理
```

</details>

## ベンチマーク

`kire bench` で分割品質を gold standard アノテーションに対して定量評価できる。評価指標は Pk（窓ベースの誤差、低いほど良い）、WindowDiff（Pk の改良版）、Precision/Recall/F1（境界の検出精度）。

テストデータは `testdata/bench_xl.md`（104 blocks, 18 gold boundaries）。語彙が意図的に重複するセクション構造で、heading-split では捉えられないトピック遷移を含む。

### 出力品質（最終セグメント境界の評価）

| Embedder | Segs | Pk | WDiff | P | R | F1 |
|----------|-----:|-----:|------:|-----:|-----:|-----:|
| tfidf | 17 | 0.10 | 0.10 | 0.88 | 0.78 | 0.82 |
| gemini | 17 | 0.10 | 0.10 | 0.88 | 0.78 | 0.82 |
| ollama | 14 | 0.35 | 0.35 | 0.23 | 0.17 | 0.19 |
| mock | 13 | 0.33 | 0.33 | 0.42 | 0.28 | 0.33 |
| openai | 17 | 0.41 | 0.41 | 0.00 | 0.00 | 0.00 |

`--profile output`（eval-stage=final, max-tokens=800）で測定。optimizer による merge/pack/heading 調整込みの結果。

### 境界検出力（embedder 比較）

| Embedder | Segs | Pk | WDiff | P | R | F1 |
|----------|-----:|-----:|------:|-----:|-----:|-----:|
| mock | 20 | 0.39 | 0.43 | 0.26 | 0.28 | 0.27 |
| ollama | 20 | 0.43 | 0.48 | 0.26 | 0.28 | 0.27 |
| gemini | 20 | 0.46 | 0.50 | 0.21 | 0.22 | 0.22 |
| openai | 20 | 0.49 | 0.54 | 0.21 | 0.22 | 0.22 |
| tfidf | 20 | 0.48 | 0.51 | 0.16 | 0.17 | 0.16 |

`--profile embedder --split-count 19`（eval-stage=raw, tolerance=0, 制約オフ）で測定。optimizer を介さない純粋な境界検出結果。

出力品質と境界検出力で順位が異なるのは正常な挙動で、optimizer が境界位置を補正するため。

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
