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
- `--min-tokens` / `--max-tokens` / `--max-lines` で LLM のコンテキストウィンドウに合わせた分割ができる
- 分割後も親見出しが各セグメントに自動付与される
- Gemini / OpenAI / Ollama / TF-IDF / Mock から embedder を選べる。API キーなしでも動く
- SHA256 content-addressable ID、品質スコアでエージェント連携にも対応
- セクションロックで見出しと本文が分断されないように保護
- 長い段落の自動分割と疑似見出し検出

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

# 行数ベースでの制御（トークン制限を無効化）
kire --max-lines 500 --max-tokens 0 document.md

# デバッグモードで詳細ログを出力
kire --debug document.md

# 強制的に上書き
kire --force document.md
```

`--dry-run` でファイル書き出しなしの確認、`--report` で HTML の境界レポートが出る。

### よく使うオプション

```
--min-tokens int          最小トークン数 (デフォルト: 300)
--max-tokens int          最大トークン数 (デフォルト: 3000, 0=無制限)
--max-lines int           最大行数 (-1=自動, 0=無制限)
--window int              類似度スムージング窓 (デフォルト: 3)
--block-k int             ブロック比較窓 (デフォルト: 3)
--block-k-auto            ドキュメント長に応じて block-k を自動選択
--threshold float         境界スコア閾値 (-1 = 自動)
--overlap int             セグメント間の重複行数
--min-gap int             境界間の最小間隔 (デフォルト: 3)
--split-count int         目標セグメント数 (0=自動)
--boundary-method         texttiling|kcpd|hybrid
--beta-strategy           auto|bic|crossval|theory
--embedder string         auto|gemini|openai|ollama|tfidf|mock
--embed-model string      埋め込みモデル名
--cache string            埋め込みキャッシュのファイルパス
--jsonl string            JSONL メタデータ出力 (--jsonl=- で stdout)
--agent-metadata          セグメント ID・品質スコアを含める
--state-file string       インクリメンタル処理の状態ファイル
--context-format          comment|front-matter|heading|none
--context-max-depth int   コンテキストの最大見出し深度 (0=無制限)
--prefix string           出力ファイル名のプレフィックス
--out string              出力ディレクトリ (デフォルト: docs)
--force                   既存出力ディレクトリを確認なしで上書き
--dry-run                 ファイル書き出しなしで実行
--quiet                   すべてのログ出力を抑制
```

### 高度な境界制御

```bash
# 境界検出の詳細制御
kire --boundary-method kcpd --beta-strategy theory document.md

# 強制境界パターン（正規表現）
kire --force-boundary "^## API|```go" document.md

# 段落分割パターン
kire --para-split-pattern "^\d+\.|^[-*] " document.md

# セクションロックを無効化（見出しと本文が分断される可能性あり）
kire --no-section-lock document.md

# リスト境界の抑制を無効化
kire --no-suppress-list-boundary document.md

# アトミックブロック保護を無効化
kire --no-atomic-boundary document.md

# 疑似見出し検出を無効化
kire --no-pseudo-heading document.md

# 見出しバリアを設定（結合時にこのレベル以上の見出しで分割）
kire --pack-heading-barrier 2 document.md
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

### 埋め込みキャッシュと並列処理

```bash
# キャッシュを使用（同じテキストの埋め込みを再利用）
kire --cache embedding-cache.json document.md

# 並列数と QPS 制限を設定
kire --embed-concurrency 8 --embed-qps 10 document.md
```

## マルチエージェント対応

`--agent-metadata` を付けると、セグメントごとのメタデータが JSONL / JSON サマリに含まれる。

メタデータの中身:

- `segment_id`: SHA256 の content-addressable ID。前後セグメントへの双方向リンク付き
- `coherence`: セグメント内ブロック間 cosine similarity の平均
- `confidence`: 境界の信頼度
- `prev_segment_id` / `next_segment_id`: 前後セグメントへのリンク

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

goldmark で Markdown を AST 化してブロック単位に分解するところから始まる。各ブロックには SourceRange（byte offset）と HeadingPath が付く。

### パイプラインの詳細

1. Parse: Markdown を AST 化してブロック単位に分解
2. ParaSplit: 長い段落をパターンに基づいて事前分割
3. PseudoHeading: 見出しがないブロックに対して疑似見出し検出
4. Tokenize: 各ブロックのトークン数を推定
5. Embed: ブロックをベクトル化（Gemini/OpenAI/Ollama/TF-IDF/Mock）
6. Boundary: 隣接ブロック間の cosine similarity を計算し、smoothing → depth score → 閾値選定で境界を検出
   - TextTiling: 伝統的な depth score ベース
   - KCPD: Kernel Change Point Detection
   - Hybrid: 複数手法の組み合わせ
7. Lock: セクションロックで見出し＋ボディが分断されないように保護
8. Segment: Greedy アルゴリズムでセグメントにまとめる（サイズ制約に応じて merge/split/pack）
9. Render: SourceRange ベースで原文をバイトレベルで復元し、コンテキスト挿入と overlap 付与
10. Output: ファイルに書き出し、index.md と DAG を生成

### 境界検出のヒント

以下のルールで境界検出を制御可能:

- 強制境界 (--force-boundary): 指定パターンにマッチするブロックの前に必ず境界を挿入
- リスト境界抑制 (--suppress-list-boundary): 連続するリスト間や見出し→リストの間に境界を入れない
- アトミックブロック保護 (--atomic-boundary): code/table/math ブロックの前に境界を入れない

<detail>
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
├── negatable.go     Negatable フラグユーティリティ
└── completion.go    シェル補完

internal/
├── model/           Block, Segment, Embedding, SourceRange
├── parser/          Markdown → []Block (goldmark)
│   ├── paragraph_split.go    長い段落の分割
│   └── pseudo_heading.go     疑似見出し検出
├── tokenizer/       トークン推定
├── embedding/       Embedder interface + 各プロバイダ
│   ├── cached.go    キャッシュラッパー
│   └── concurrent.go 並列処理ラッパー
├── boundary/        境界検出アルゴリズム
│   ├── boundary.go  TextTiling
│   ├── kcpd.go      Kernel Change Point Detection
│   ├── hints.go     ヒントルール
│   └── report.go    HTML レポート生成
├── segment/         セグメント処理
│   ├── optimizer.go  Greedy 最適化
│   ├── lock.go      セクションロック
│   ├── id.go        ID 生成
│   └── quality.go   品質スコア計算
├── ctxheader/       親見出しコンテキスト挿入
├── output/          ファイル出力 + JSONL + マージ
├── cache/           埋め込みキャッシュ (JSON)
├── dag/             依存 DAG エクスポート (JSON/DOT)
├── concurrency/     Worker pool + rate limiter
├── pipeline/        パイプライン統合 + フック + インクリメンタル処理
├── vecmath/         ベクトル計算（cosine similarity）
└── eval/            ベンチマーク評価（Pk, WindowDiff, PRF）
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
just bench-quality    # 品質評価のみ（texttiling/kcpd/hybrid）
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
