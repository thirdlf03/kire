# kire

[![CI](https://github.com/thirdlf03/kire/actions/workflows/ci.yml/badge.svg)](https://github.com/thirdlf03/kire/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thirdlf03/kire.svg)](https://pkg.go.dev/github.com/thirdlf03/kire)
[![Go Report Card](https://goreportcard.com/badge/github.com/thirdlf03/kire)](https://goreportcard.com/report/github.com/thirdlf03/kire)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

[日本語](README.md)

A Go CLI that splits Markdown at topic boundaries. Instead of splitting at headings, it detects where the topic shifts using embedding similarity — [TextTiling](https://aclanthology.org/J97-1003/) for Markdown.

```bash
$ kire spec.md
Wrote 3 segments to spec/
Wrote spec/index.md
```

## Features

- Concatenating segments reproduces the original byte-for-byte (lossless)
- `--min-tokens` / `--max-tokens` to fit LLM context windows
- Parent headings are automatically prepended to each segment
- Gemini / OpenAI / Ollama / TF-IDF / Mock embedders. Works without an API key
- SHA256 content-addressable IDs and quality scores for agent workflows

## Installation

Homebrew (macOS / Linux):

```bash
brew install thirdlf03/tap/kire
```

Go:

```bash
go install github.com/thirdlf03/kire/cmd/kire@latest
```

From source:

```bash
git clone https://github.com/thirdlf03/kire.git
cd kire
just build
```

To enable shell completion, add the following to your shell config:

```bash
# Bash (~/.bashrc)
source <(kire completion bash)

# Zsh (~/.zshrc)
source <(kire completion zsh)

# Fish (~/.config/fish/config.fish)
kire completion fish | source
```

## Example

The [`example/`](example/) directory contains sample input and output.

Given a Chat API design document (1 file, ~270 lines):

```bash
$ kire --embedder tfidf --min-tokens 100 --max-tokens 800 example/input.md
```

It splits into 4 segments at topic boundaries.

```
example/output/input/
├── 01-chat-api-設計ドキュメント.md   # Auth & message sending
├── 02-チャンネル管理.md              # Channel mgmt & push notifications
├── 03-設定は-rest-api-で変更する.md  # Notification settings & DB design
├── 04-ci-cd-パイプライン.md          # CI/CD & monitoring
└── index.md                          # TOC + Mermaid graph
```

Parent headings are automatically prepended as context:

```markdown
<!-- context: Chat API 設計ドキュメント > メッセージング -->

### チャンネル管理
...
```

Merging the segments back reproduces the original byte-for-byte:

```bash
$ kire merge --strip-context example/output/input/ | diff example/input.md -
# no diff
```

## Usage

```bash
# Quick start (no API key needed, splits with Mock embedder)
kire document.md

# Specify output directory and prefix
kire --out docs --prefix split document.md

# Use the Gemini API
export GEMINI_API_KEY=your-key
kire document.md

# No API needed (TF-IDF)
kire --embedder tfidf document.md

# Batch process multiple files
kire --in a.md --in b.md --out docs
```

`--dry-run` previews without writing files. `--report` generates an HTML boundary report.

All options via `kire --help`. Common ones:

```
--min-tokens int     Minimum token count (default: 300)
--max-tokens int     Maximum token count (default: 3000)
--window int         Similarity smoothing window (default: 3)
--threshold float    Boundary score threshold (-1 = auto)
--overlap int        Overlapping lines between segments
--embedder string    auto|gemini|openai|ollama|tfidf|mock
--cache string       Embedding cache file path
--jsonl              JSONL metadata output (--jsonl=- for stdout)
--agent-metadata     Include segment IDs and quality scores
```

### Merge

Reassemble split segments:

```bash
kire merge docs/my-spec/                          # output to stdout
kire merge --strip-context -o merged.md docs/my-spec/  # strip context headers
kire merge segment1.md segment2.md segment3.md     # specify files individually
```

### DAG output

Export a link dependency graph between segments:

```bash
kire --dag-json dag.json --dag-dot dag.dot document.md
```

### Incremental processing

`--state-file` saves source hashes so unchanged files are skipped on subsequent runs. When changes are detected, diffs are reported per segment (Added/Removed/Modified/Unchanged).

```bash
kire --state-file .kire-state.json document.md
```

## Embedding providers

Selected via `--embedder`. Default is `auto` (Gemini if `GEMINI_API_KEY` is set, otherwise TF-IDF).

| Provider | API Key | Default Model | Local |
|----------|---------|---------------|-------|
| `gemini` | `GEMINI_API_KEY` | `gemini-embedding-001` | No |
| `openai` | `OPENAI_API_KEY` | `text-embedding-3-large` | No |
| `ollama` | Not required (`OLLAMA_HOST` to change endpoint) | `nomic-embed-text` | Yes |
| `tfidf` | Not required | — | Yes |
| `mock` | Not required | — | Yes |

```bash
# OpenAI
export OPENAI_API_KEY=your-key
kire --embedder openai document.md

# Ollama (local)
kire --embedder ollama --embed-model mxbai-embed-large document.md

# TF-IDF (no API, deterministic)
kire --embedder tfidf document.md
```

## Multi-agent support

`--agent-metadata` includes per-segment metadata in JSONL / JSON summary output.

What's in the metadata:

- `segment_id`: SHA256 content-addressable ID with bidirectional links to prev/next segments
- `coherence`: mean cosine similarity between blocks within a segment
- `confidence`: boundary reliability score

When using kire as a Go API, you can hook into each pipeline stage:

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

## How it works

```
Source → Parse → ParaSplit → PseudoHeading → Tokenize → Embed → Boundary → Lock → Segment → Render → Output
```

It starts by parsing Markdown into an AST with goldmark and breaking it into blocks. Each block gets a SourceRange (byte offset) and HeadingPath. Long paragraphs are pre-split, and blocks without explicit headings get pseudo-heading detection.

After vectorizing blocks, cosine similarity between adjacent blocks is computed, then smoothing → depth score → threshold selection detects boundaries. Section locking keeps headings together with their body, and a greedy algorithm groups blocks into segments.

Finally, the original text is restored byte-for-byte via SourceRange, context headers and overlap are applied, and files are written out.

<details>
<summary>Package layout</summary>

```
cmd/kire/
├── main.go          CLI entry point
├── root.go          Flag definitions
├── run.go           Main execution logic
├── merge.go         merge subcommand
└── summary.go       JSON summary generation

internal/
├── model/           Block, Segment, Embedding, SourceRange
├── parser/          Markdown → []Block (goldmark)
├── tokenizer/       Token estimation (local/api/hybrid)
├── embedding/       Embedder interface + providers + Cached/Concurrent decorators
├── boundary/        TextTiling boundary scoring + confidence
├── segment/         Greedy splitting + ID generation + quality scores
├── context/         Parent heading context insertion
├── output/          File output + JSONL + merge
├── cache/           Embedding cache (JSON)
├── dag/             Dependency DAG export (JSON/DOT)
├── concurrency/     Worker pool + rate limiter
└── pipeline/        Pipeline orchestration + hooks + incremental processing
```

</details>

## Benchmarks

`kire bench` evaluates segmentation quality against gold standard annotations. Metrics include Pk (window-based error, lower is better), WindowDiff (improved Pk), and boundary Precision/Recall/F1.

Test data is `testdata/bench_xl.md` (104 blocks, 18 gold boundaries). The document has intentionally overlapping vocabulary across sections, containing topic transitions that heading-based splitting cannot detect.

### Output quality (final segment boundaries)

| Embedder | Segs | Pk | WDiff | P | R | F1 |
|----------|-----:|-----:|------:|-----:|-----:|-----:|
| tfidf | 17 | 0.10 | 0.10 | 0.88 | 0.78 | 0.82 |
| gemini | 17 | 0.10 | 0.10 | 0.88 | 0.78 | 0.82 |
| ollama | 14 | 0.35 | 0.35 | 0.23 | 0.17 | 0.19 |
| mock | 13 | 0.33 | 0.33 | 0.42 | 0.28 | 0.33 |
| openai | 17 | 0.41 | 0.41 | 0.00 | 0.00 | 0.00 |

Measured with `--profile output` (eval-stage=final, max-tokens=800). Includes optimizer merge/pack/heading adjustments.

### Boundary detection (embedder comparison)

| Embedder | Segs | Pk | WDiff | P | R | F1 |
|----------|-----:|-----:|------:|-----:|-----:|-----:|
| mock | 20 | 0.39 | 0.43 | 0.26 | 0.28 | 0.27 |
| ollama | 20 | 0.43 | 0.48 | 0.26 | 0.28 | 0.27 |
| gemini | 20 | 0.46 | 0.50 | 0.21 | 0.22 | 0.22 |
| openai | 20 | 0.49 | 0.54 | 0.21 | 0.22 | 0.22 |
| tfidf | 20 | 0.48 | 0.51 | 0.16 | 0.17 | 0.16 |

Measured with `--profile embedder --split-count 19` (eval-stage=raw, tolerance=0, constraints off). Raw boundary detection without optimizer.

Rankings differ between output quality and detection because the optimizer adjusts boundary positions.

```bash
just bench            # Quality + performance benchmarks
just bench-quality    # Quality evaluation only
just bench-perf       # Go performance benchmarks only
```

## Development

Development tools (Go, golangci-lint, just) are managed with [devbox](https://github.com/jetify-com/devbox).

```bash
devbox shell       # Enter the dev shell
```

```bash
just test          # Run tests
just test-v        # Verbose output
just cover         # Coverage report
just lint          # go vet
just build         # Build
just run -- document.md
```

Lint and format:

```bash
devbox run fmt           # gofmt
devbox run fmt-check     # Format diff check (for CI)
devbox run lint          # golangci-lint
```

## License

MIT
