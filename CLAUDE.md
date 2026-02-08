# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

<--- do not edit!! --->
# 開発スタイル

TDD で開発する（探索 → Red → Green → Refactoring）。
KPI やカバレッジ目標が与えられたら、達成するまで試行する。
不明瞭な指示は質問して明確にする。

# コード設計

- 関心の分離を保つ
- 状態とロジックを分離する
- 可読性と保守性を重視する
- コントラクト層（API/型）を厳密に定義し、実装層は再生成可能に保つ

# ツール

- VCS: jj を優先（未初期化なら `jj git init --colocate`）
- タスク: justfile
<--- do not edit!! --->

## 開発環境

devbox が導入済み。ツール実行は `devbox run` 経由で行う。

```bash
devbox run test       # go test ./...
devbox run lint       # golangci-lint run ./...
devbox run fmt        # gofmt -w .
devbox run fmt-check  # フォーマットチェック（CI 用）
```

`devbox shell` に入れば `go`, `just`, `golangci-lint` がそのまま使える。

## ビルド・テストコマンド

```bash
just build            # go build（-ldflags でバージョン注入）-o kire ./cmd/kire
just test             # go test ./...
just test-v           # go test -v ./...
just cover            # カバレッジレポート出力
just cover-html       # HTML カバレッジレポート（ブラウザ表示）
just lint             # go vet ./...
just run -- <ARGS>    # go run ./cmd/kire <ARGS>
just clean            # ビルド成果物を削除
just bench            # 品質評価 + パフォーマンスベンチマーク（全部）
just bench-quality    # Gold standard に対する品質評価のみ
just bench-perf       # Go testing.B パフォーマンスベンチマークのみ
just release-dry-run  # GoReleaser スナップショットビルド（publish なし）
```

単一パッケージのテスト実行:
```bash
go test ./internal/llmsplit/...
go test -run TestDetectBoundaries ./internal/llmsplit/...
```

## プロジェクト概要

kire (切れ) — 長文 Markdown を LLM でセマンティック境界を検出して自動分割する Go CLI ツール。Gemini の structured output で境界位置を決定する。

## アーキテクチャ

モジュール: `github.com/thirdlf03/kire`（Go 1.25.5）

パイプライン構成: `Parse → Tokenize → LineEstimate → [LLM-Refine? Embed+Sims : noop] → LLM Boundary → Optimize → IDs → Quality → Render → Output`

エントリポイントは `cmd/kire/main.go`、パイプライン統合は `internal/pipeline/pipeline.go`。

### パッケージ構成と処理フロー

- `model/` — 共有データ型（Block, Segment, Embedding）。全パッケージがこの型を使う
- `parser/` — goldmark で Markdown → `[]Block`。各ブロックに `SourceRange`（byte offset）と `HeadingPath` を付与
- `tokenizer/` — `TokenEstimator` インターフェース。local（ヒューリスティック）実装
- `llmsplit/` — LLM 境界検出。Gemini `GenerateContent` + `ResponseSchema` で structured output。`{"boundaries": [4, 10, 15]}` 形式の gap index 配列を返す
- `embedding/` — `Embedder` インターフェース。Gemini/OpenAI/Ollama/SentenceTransformer/TF-IDF/Mock の実装 + Cached/Concurrent デコレータ。`--llm-refine` 時のみ使用
- `boundary/` — `BoundaryResult` 構造体 + cosine similarity 計算ユーティリティ（MeanPool, BlockSimilarities, Smooth, DepthScore）
- `segment/` — Optimize（空コンフィグで LLM 境界をそのまま使用）、ID 生成、品質スコア計算
- `ctxheader/` — 親見出しをセグメント先頭に挿入（comment/front-matter/heading/none）
- `output/` — `SourceRange` ベースの原文復元 + overlap 付与 + セマンティックファイル名生成（slug.go）+ ファイル書き出し
- `vecmath/` — cosine similarity 計算
- `cache/` — SHA256 キーの JSON 永続キャッシュ
- `dag/` — セグメント間アンカー/リンク参照から依存グラフを構築、JSON/DOT/Markdown エクスポート
- `concurrency/` — セマフォ + `x/time/rate` によるワーカープール
- `pipeline/` — 上記すべてをオーケストレーション。`BoundaryDetector` インターフェースと `SimilaritySetter` オプショナルインターフェースで LLM 境界検出を抽象化
- `eval/` — ベンチマーク評価（Pk, WindowDiff, Precision/Recall/F1）

### 重要な設計パターン

- **BoundaryDetector インターフェース**: `pipeline.BoundaryDetector` で LLM 境界検出を抽象化。テストでは stubBoundaryDetector でモック
- **SimilaritySetter**: `--llm-refine` 時に cosine similarity データを渡すオプショナルインターフェース
- **デコレータパターン**: Embedder は `Base(Gemini/Mock/TFIDF) → CachedEmbedder → ConcurrentEmbedder` のように合成する
- **Lossless 復元**: `SourceRange`（byte offset）で原文をバイトレベルで復元。分割結果を結合すると原文と一致する
- **見出し追跡**: Parser が `HeadingPath` を維持し、Segment → Context → Output まで伝播する
- **レジストリパターン**: Embedding プロバイダは `embedding.Register()` で登録。`--embedder auto` は gemini → tfidf の順で試行

### マルチエージェント連携機能

パイプライン後半で自動付与されるメタデータ（`--agent-metadata` で JSONL に含まれる）:

- `segment/id.go` — SHA256 コンテンツアドレッサブル ID + prev/next リンクチェーン
- `segment/quality.go` — セグメント内 coherence（ブロック間 cosine similarity 平均）、境界 confidence

出力形式:
- `output/jsonl.go` — JSONL メタデータ。`--jsonl` でファイル出力、`--jsonl=-` で stdout 出力
- `output/merge.go` — 分割セグメントを再結合する `kire merge` サブコマンド

パイプライン拡張:
- `pipeline/hooks.go` — OnParse/OnEmbed/OnBoundary/OnSegment/OnRender の5フックポイント
- `pipeline/incremental.go` — `--state-file` でソースハッシュ + セグメント差分検出（Added/Removed/Modified/Unchanged）

### CLI フレームワーク

`spf13/cobra` を使用。フラグ定義は `cmd/kire/root.go` の `init()` にまとまっている。
`NoOptDefVal` を使ったオプショナル値フラグ（例: `--jsonl` は値なしで `"auto"`、`--jsonl=-` で stdout）がある。

### LLM 分割の仕組み

`internal/llmsplit/` パッケージが Gemini API を使って境界検出する:

- `Splitter` 構造体が `pipeline.BoundaryDetector` と `pipeline.SimilaritySetter` を実装
- `DetectBoundaries()` でブロック一覧をプロンプトとして送信し、structured output で gap index 配列を取得
- Temperature=0 で決定論的な出力
- `--llm-refine` 時は `SetSimilarities()` で cosine similarity データを受け取り、プロンプトに含める
- 出力された境界はバリデーション・重複排除・ソートされる
- Confidence は全境界で 1.0（LLM は確信度を返さないため）

## 依存パッケージ

- `google.golang.org/genai` — Gemini API クライアント（LLM 境界検出、必須）
- `github.com/yuin/goldmark` — Markdown パース
- `github.com/litao91/goldmark-mathjax` — 数式ブロックサポート
- `github.com/spf13/cobra` — CLI フレームワーク（サブコマンド + シェル補完）
- `github.com/openai/openai-go/v3` — OpenAI Embedding API（`--llm-refine --embedder openai` 時）
- `golang.org/x/time/rate` — レート制限

テストフレームワークは標準の `testing` パッケージのみ使用。テーブル駆動テストが主パターン。

## テスト

- `go test ./...` で全テスト実行
- LLM 統合テストは `GEMINI_API_KEY` 未設定時に `t.Skip()` でスキップされる
- pipeline テストは `stubBoundaryDetector` で LLM をモック
- `embedding.NewMockEmbedder()` で API 不要の embedding テストが可能（文字頻度ベースの決定論的ベクトル）
- Embedding プロバイダはレジストリパターン（`embedding/registry.go`）。新プロバイダ追加時は `init()` で `Register()` を呼ぶ

## 環境変数

- `GEMINI_API_KEY` — Gemini API キー。LLM 境界検出に必須
- `OPENAI_API_KEY` — OpenAI Embedding API キー（`--llm-refine --embedder openai` 使用時）
- `OLLAMA_HOST` — Ollama サーバーアドレス（デフォルト: `http://localhost:11434`）
- `SENTENCETRANSFORMER_HOST` — SentenceTransformer HTTP サーバーアドレス（デフォルト: `http://localhost:8080`）

## 品質評価ベンチマーク

`internal/eval/` パッケージと `kire bench` サブコマンドで、分割品質を定量評価できる。

### 評価指標

- Pk — 窓ベースのセグメンテーション誤差。低いほど良い（0.0 が完全一致）
- WindowDiff — Pk の改良版。false positive/negative を対称に扱う
- Precision / Recall / F1 — 境界の検出精度。tolerance パラメータで許容誤差を設定可能

### raw と final の使い分け

`kire bench` は `--eval-stage` で評価対象を切り替える。

- raw — LLM 境界検出の出力をそのまま評価する
- final — optimizer 後の最終セグメント境界を評価する（現在 optimizer は空コンフィグなので raw と同じ境界になることが多い）

### 実行方法

```bash
# 基本（gold params が自動適用される）
kire bench testdata/gold/bench_xl.json testdata/bench_xl.md

# LLM-refine モードで評価
kire bench --llm-refine --embedder gemini testdata/gold/bench_xl.json testdata/bench_xl.md

# raw と final を同時に表示
kire bench --eval-stage both testdata/gold/bench_xl.json testdata/bench_xl.md

# JSON 出力（スクリプトでの後処理向き）
kire bench --json testdata/gold/bench_xl.json testdata/bench_xl.md

# ベースライン比較をスキップ
kire bench --no-baselines testdata/gold/bench_xl.json testdata/bench_xl.md

# tolerance / k を変更
kire bench --tolerance 0 --k 3 testdata/gold/bench_long.json testdata/bench_long.md
```

### Gold アノテーション

`testdata/gold/` に JSON 形式で配置。`boundaries` はソート済みのブロック間 gap index（0-indexed）。`params` でベンチの推奨パラメータを指定できる（全フィールド省略可）。

```json
{
  "doc_id": "bench_xl",
  "unit": "block",
  "boundaries": [4, 10, 15, ...],
  "params": {
    "eval_stage": "final",
    "tolerance": 0
  },
  "notes": { "boundary_4": "説明..." }
}
```

新しい gold annotation を作成するときは、parser でブロック構造を確認する。

```bash
# 一時的なブロックダンプ（プロジェクト内に置いて go run で実行、完了後に削除）
go run ./cmd/dumpblocks testdata/your_file.md
```

### ベースライン戦略

`kire bench` はデフォルトで 3 つのベースラインを自動実行する。

- heading-split — 全見出しブロックの直前に境界を置く
- fixed-N — N ブロックごとに均等分割（N は gold の平均セグメント長）
- random — gold と同数の境界をランダム配置（seed=42 で再現可能）

### パッケージ構成

```
internal/eval/
├── metrics.go           # Pk, WindowDiff, BoundaryPRF
├── metrics_test.go
├── metrics_bench_test.go
├── gold.go              # GoldAnnotation の JSON 読み書き
├── gold_test.go
├── baseline.go          # HeadingSplit, FixedSplit, RandomSplit
├── baseline_test.go
├── report.go            # Evaluate, FormatTable
└── report_test.go

cmd/kire/bench.go        # kire bench サブコマンド

testdata/gold/
├── simple.json
├── nested_headings.json
├── bench_long.json
└── bench_xl.json
```

依存方向は `eval` → `model` のみ。`pipeline` には依存しない。`cmd/kire/bench.go` が `eval` と `pipeline` を結合する。

## 文体ルール

- README や docs で `**ラベル**:` 構文（太字コロン）を使わない。文章として自然に書く
