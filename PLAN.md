# ベンチマーク修正計画

## 背景

Copilot CLI との討論で以下の問題を特定した。

### 致命的な問題
1. **fixed-N ベースラインが F1=0.93 で全 embedder を上回る** — gold boundary が quasi-periodic（gap: 3,6,4,2,5,2）のため等間隔分割が偶然当たる
2. **n=1 文書** — 統計的主張が不可能
3. **mock(26dim 文字頻度) = tfidf** — セマンティクスではなく文体の統計的規則性をテストしている

### 構造的な問題
4. **embedder vs algorithm が分離不能** — 全 embedder が gap 14/23 を見逃す原因不明
5. **見出し不一致率が過剰** — 実文書では見出し ≈ トピック遷移が多い。gemini/openai を不公平に罰する
6. **Pk/WindowDiff が 25 blocks では粗い** — k=3 で窓位置 22 個

## 修正方針

### やること
- AUROC メトリクスの追加（embedder-only 品質指標）
- テスト文書の追加（非周期的 boundary、見出し 50% 一致）
- 層別レポート（heading-aligned vs mid-section）
- tolerance=0 での F1 も報告

### やらないこと
- TextTiling アルゴリズムの変更（それは別タスク）
- RAG 下流評価（スコープ外）
- supervised segmentation の導入

---

## Phase 1: AUROC メトリクス追加

raw cosine similarity から gold boundary の検出可能性を測る。パイプラインのアルゴリズムを介さず、embedding の信号品質だけを評価する。

### 1-1. `internal/eval/metrics.go` に `BoundaryAUROC` 追加

```go
func BoundaryAUROC(goldBoundaries []int, similarities []float64) float64
```

- `similarities[i]` = cosine_sim(block[i], block[i+1])、長さ numBlocks-1
- gold boundary gap を positive（label=1）、その他を negative（label=0）
- score = `1 - similarities[i]`（類似度が低いほど境界らしい）
- Mann-Whitney U 統計量として計算（ソート + ランク和）
- 0.5 = ランダム、1.0 = 完全分離

### 1-2. `BoundaryScores` に `AUROC` フィールド追加

```go
type BoundaryScores struct {
    Precision float64 `json:"precision"`
    Recall    float64 `json:"recall"`
    F1        float64 `json:"f1"`
}
```

AUROC は BoundaryScores とは別の概念（embedding-only vs pipeline）なので、`MethodResult` に直接フィールドを追加する。

```go
type MethodResult struct {
    ...
    AUROC *float64 `json:"auroc,omitempty"` // nil for baselines (no similarities)
}
```

### 1-3. `Evaluate` に similarities を渡すオプション追加

baselines は similarities を持たないので optional。

```go
func EvaluateWithSimilarities(gold *GoldAnnotation, numBlocks int, hyp []int,
    name string, k, tolerance int, similarities []float64) MethodResult
```

既存の `Evaluate` はそのまま（後方互換）。

### 1-4. `cmd/kire/bench.go` で similarities を `EvaluateWithSimilarities` に渡す

`pipeline.Run` の結果 `result.Boundary.Similarities` を渡す。

### 1-5. `FormatTable` に AUROC 列追加

AUROC が nil のベースラインは `-` 表示。

### 1-6. テスト

- `TestBoundaryAUROC_Perfect` — gold boundary 位置の sim が全て non-boundary より低い → 1.0
- `TestBoundaryAUROC_Random` — sim が一様 → ≈0.5
- `TestBoundaryAUROC_Inverted` — gold 位置が最も高い sim → 0.0

---

## Phase 2: テスト文書追加

### 設計原則
- **非周期的 boundary**: segment 長を意図的にばらつかせる（1-8 blocks）
- **見出し 50% 一致**: gold boundary の半分は見出し位置、半分は mid-section
- **語彙重複の制御**: 文体を均一にしつつトピックだけ変える箇所を含める
- **50+ blocks**: メトリクスの分解能を確保

### 2-1. `testdata/bench_api_design.md` + `testdata/gold/bench_api_design.json`

REST API 設計ガイド。50-60 blocks。トピック: 認証、レート制限、エラーハンドリング、バージョニング、ページネーション、キャッシュ。語彙重複が大きい（endpoint, request, response, header, status code が全トピックに登場）。

gold boundary のうち半分は見出し位置に一致させる。segment 長は 2-8 blocks でばらつかせる。

### 2-2. gold annotation 作成手順

1. dumpblocks で block 構造を確認
2. 各 block のトピックラベルを付与
3. トピック遷移点を gold boundary として設定
4. heading-split との overlap 率が 40-60% になるよう調整
5. segment 長の分散が大きい（CV > 0.5）ことを確認

---

## Phase 3: 層別レポート

### 3-1. `GoldAnnotation` に boundary 分類を追加

```go
type GoldAnnotation struct {
    DocID      string            `json:"doc_id"`
    Unit       string            `json:"unit"`
    Boundaries []int             `json:"boundaries"`
    Notes      map[string]string `json:"notes,omitempty"`
    HeadingAligned []int         `json:"heading_aligned,omitempty"` // 見出し一致の boundary subset
}
```

### 3-2. `eval.StratifiedScores` 構造体

```go
type StratifiedScores struct {
    HeadingAligned BoundaryScores `json:"heading_aligned"`
    MidSection     BoundaryScores `json:"mid_section"`
}
```

### 3-3. `MethodResult` に層別スコア追加

```go
type MethodResult struct {
    ...
    Stratified *StratifiedScores `json:"stratified,omitempty"`
}
```

### 3-4. 既存の `BoundaryPRF` を使って各層を個別評価

heading_aligned な gold boundaries だけを ref にして BoundaryPRF を計算、mid-section も同様。

### 3-5. `FormatTable` に層別行を追加

```
kire (tfidf)            6   0.25   0.25   1.00   0.71   0.83   0.75
  heading-aligned                                 1.00   0.67   0.80
  mid-section                                     1.00   0.75   0.86
```

---

## Phase 4: tolerance=0 レポート

### 4-1. `MethodResult` に tolerance=0 の F1 追加

```go
type MethodResult struct {
    ...
    BoundaryStrict BoundaryScores `json:"boundary_scores_strict"` // tolerance=0
}
```

### 4-2. `Evaluate` / `EvaluateWithSimilarities` で両方計算

tolerance=0 と tolerance=N を両方計算してセット。

### 4-3. `FormatTable` に strict F1 列追加

---

## Phase 5: bench_long.md の gold 修正

現在の quasi-periodic boundary を非周期的に修正する。文書を書き換えて segment 長のばらつきを大きくする。

現状 gap: 3, 6, 4, 2, 5, 2 → 目標 gap: 1, 5, 2, 8, 3, 1, 4（CV > 0.6）

文書の段落を追加・統合して block 数を 35-40 に増やし、boundary 配置を非周期的にする。

---

## 実装順序

1. Phase 1（AUROC）→ テストを先に書く
2. Phase 5（bench_long.md 修正）→ fixed-N 問題を即座に潰す
3. Phase 2（新テスト文書）→ n=1 問題を解消
4. Phase 3（層別レポート）→ heading_aligned 情報が必要
5. Phase 4（tolerance=0）→ 表示の追加のみ

各 Phase で TDD（テスト → 実装 → リファクタ）を回す。
