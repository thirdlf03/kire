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
just benchmark        # 各 embedding プロバイダのベンチマーク比較
just bench            # 品質評価 + パフォーマンスベンチマーク（全部）
just bench-quality    # Gold standard に対する品質評価のみ
just bench-perf       # Go testing.B パフォーマンスベンチマークのみ
just release-dry-run  # GoReleaser スナップショットビルド（publish なし）
```

単一パッケージのテスト実行:
```bash
go test ./internal/boundary/...
go test -run TestDetectBoundaries ./internal/boundary/...
```

## プロジェクト概要

kire (切れ) — 長文 Markdown をセマンティック境界で自動分割する Go CLI ツール。Gemini Embedding + TextTiling 風境界検出を使用。

## アーキテクチャ

モジュール: `github.com/thirdlf03/kire`（Go 1.25.6）

パイプライン構成: `Parse → ParaSplit → PseudoHeading → Tokenize → Embed → Boundary → Lock → Segment → Render → Output`

エントリポイントは `cmd/kire/main.go`、パイプライン統合は `internal/pipeline/pipeline.go`。

### パッケージ構成と処理フロー

- `model/` — 共有データ型（Block, Segment, Embedding）。全パッケージがこの型を使う
- `parser/` — goldmark で Markdown → `[]Block`。各ブロックに `SourceRange`（byte offset）と `HeadingPath` を付与
- `tokenizer/` — `TokenEstimator` インターフェース。local（ヒューリスティック）実装
- `embedding/` — `Embedder` インターフェース。Gemini/Mock/TF-IDF の基本実装 + Cached/Concurrent のデコレータ
- `boundary/` — 隣接ブロック間 cosine similarity → smoothing → depth score → 閾値選定で境界検出
- `segment/` — Greedy 最適化: 見出し調整 → 境界分割 → undersized 結合 → oversized 再分割
- `ctxheader/` — 親見出しをセグメント先頭に挿入（comment/front-matter/heading/none）
- `output/` — `SourceRange` ベースの原文復元 + overlap 付与 + セマンティックファイル名生成（slug.go）+ ファイル書き出し
- `vecmath/` — cosine similarity 計算
- `cache/` — SHA256 キーの JSON 永続キャッシュ
- `dag/` — セグメント間アンカー/リンク参照から依存グラフを構築、JSON/DOT/Markdown エクスポート
- `concurrency/` — セマフォ + `x/time/rate` によるワーカープール
- `pipeline/` — 上記すべてをオーケストレーション

### 重要な設計パターン

- **デコレータパターン**: Embedder は `Base(Gemini/Mock/TFIDF) → CachedEmbedder → ConcurrentEmbedder` のように合成する
- **Lossless 復元**: `SourceRange`（byte offset）で原文をバイトレベルで復元。分割結果を結合すると原文と一致する
- **見出し追跡**: Parser が `HeadingPath` を維持し、Segment → Context → Output まで伝播する
- **フォールバック戦略**: Gemini API 不可時は TF-IDF へ自動切替（`cmd/kire/run.go` の `buildEmbedder()`）
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
negatable フラグ（`--no-section-lock` 等）は `negatable()` ヘルパーで定義。

## 依存パッケージ

- `github.com/yuin/goldmark` — Markdown パース
- `github.com/litao91/goldmark-mathjax` — 数式ブロックサポート
- `github.com/spf13/cobra` — CLI フレームワーク（サブコマンド + シェル補完）
- `google.golang.org/genai` — Gemini Embedding API
- `github.com/openai/openai-go/v3` — OpenAI Embedding API
- `golang.org/x/time/rate` — レート制限

テストフレームワークは標準の `testing` パッケージのみ使用。テーブル駆動テストが主パターン。

## テスト

- `embedding.NewMockEmbedder()` で API 不要のテストが可能（文字頻度ベースの決定論的ベクトル）
- Gemini API を使う統合テストは `GEMINI_API_KEY` 未設定時に `t.Skip()` でスキップされる
- Embedding プロバイダはレジストリパターン（`embedding/registry.go`）。新プロバイダ追加時は `init()` で `Register()` を呼ぶ

## 環境変数

- `GEMINI_API_KEY` — Gemini Embedding API キー。未設定でも TF-IDF/Mock embedder で動作する
- `OPENAI_API_KEY` — OpenAI Embedding API キー（`--embedder openai` 使用時）
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

- raw — 境界検出ステージの出力をそのまま評価する。embedder の境界検出力を比較するのに使う。optimizer の merge/pack/heading 調整が介入しないため、純粋な埋め込み品質の差が見える。
- final — optimizer 後の最終セグメント境界を評価する。実際の出力品質を見るのに使う。raw で弱い embedder でも optimizer の補正で結果が改善されることがあり、raw と順位が逆転するのは正常な挙動。

目的に応じてパラメータを変える必要がある。raw では制約（section lock, boundary hints 等）を外して embedder 差を見やすくし、final では実運用に近い制約を有効にする。

### プロファイル

`--profile` で目的別のプリセットを適用できる。明示フラグ > profile > gold params の優先順位で、明示指定されたフラグは上書きされない。

- `--profile embedder` — raw の境界検出力を比較する設定。eval-stage=raw, tolerance=0, min-gap=2, window=5, k=3, 各種制約オフ。
- `--profile output` — final の出力品質を比較する設定。eval-stage=final, max-tokens=800。

### 実行方法

```bash
# 基本（gold params が自動適用される）
kire bench testdata/gold/bench_xl.json testdata/bench_xl.md

# embedder 比較（raw, 制約なし）
kire bench --profile embedder --embedder gemini testdata/gold/bench_xl.json testdata/bench_xl.md

# bench_xl で embedder 比較するときは --split-count 19 を推奨（過分割防止）
kire bench --profile embedder --split-count 19 --embedder gemini \
  testdata/gold/bench_xl.json testdata/bench_xl.md

# 出力品質比較（final, max-tokens=800）
kire bench --profile output --embedder gemini testdata/gold/bench_xl.json testdata/bench_xl.md

# raw と final を同時に表示
kire bench --eval-stage both testdata/gold/bench_xl.json testdata/bench_xl.md

# JSON 出力（スクリプトでの後処理向き）
kire bench --json testdata/gold/bench_xl.json testdata/bench_xl.md

# ベースライン比較をスキップ
kire bench --no-baselines testdata/gold/bench_xl.json testdata/bench_xl.md

# tolerance / k を変更
kire bench --tolerance 0 --k 3 testdata/gold/bench_long.json testdata/bench_long.md
```

### 全 embedder 一括比較

```bash
# embedder 比較（raw）
for e in tfidf mock ollama gemini openai; do
  echo "=== $e ==="
  kire bench --profile embedder --split-count 19 --no-baselines --embedder "$e" \
    testdata/gold/bench_xl.json testdata/bench_xl.md 2>/dev/null
  echo ""
done

# 出力品質（final）
for e in tfidf mock ollama gemini openai; do
  echo "=== $e ==="
  kire bench --profile output --no-baselines --embedder "$e" \
    testdata/gold/bench_xl.json testdata/bench_xl.md 2>/dev/null
  echo ""
done
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
    "tolerance": 0,
    "min_tokens": 200,
    "max_tokens": 500,
    "max_lines": 0
  },
  "notes": { "boundary_4": "説明..." }
}
```

params は「どの評価モードでも安全に効く設定」を入れる。raw 専用の設定（split-count 等）は params に入れず、コマンド実行時に手動指定する。これは split-count が section lock と干渉して final 評価を劣化させるため。

新しい gold annotation を作成するときは、parser でブロック構造を確認する。

```bash
# 一時的なブロックダンプ（プロジェクト内に置いて go run で実行、完了後に削除）
go run ./cmd/dumpblocks testdata/your_file.md
```

### テストデータの設計方針

bench_long.md（25 blocks）は語彙が重複する近接トピック（collaborative filtering, content-based, hybrid, online learning, evaluation, A/B testing, monitoring）が同一セクション内で遷移する構造になっている。短文書のためブロック数が少なく、min-gap やその他の制約が結果を支配しやすい。embedder 比較には使えるが、結果はノイジーになりうる。

bench_xl.md（104 blocks）は9セクション × 約10パラグラフの長文で、セクション内の中間地点にトピック遷移を配置した構造。語彙が意図的に重複している（pipeline, latency, throughput, backpressure 等）ため、heading-split では捉えられない境界をセマンティック分割が検出できるかを測定できる。embedder の出力品質評価に適している。

gold boundary はトピック遷移の実際の位置に設定し、見出し位置とは意図的にずらしてある。

### ベースライン戦略

`kire bench` はデフォルトで 3 つのベースラインを自動実行する。

- heading-split — 全見出しブロックの直前に境界を置く
- fixed-N — N ブロックごとに均等分割（N は gold の平均セグメント長）
- random — gold と同数の境界をランダム配置（seed=42 で再現可能）

### パフォーマンスベンチマーク

```bash
# 全パッケージの Go ベンチマーク
go test -bench=. -benchmem ./internal/...

# 特定パッケージのみ
go test -bench=. -benchmem ./internal/eval/...
go test -bench=. -benchmem ./internal/boundary/...
go test -bench=. -benchmem ./internal/parser/...
```

対象: Pk/WindowDiff 計算性能、DetectBoundaries（20/100/500 blocks）、Parse（simple/nested/bench_long）

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
