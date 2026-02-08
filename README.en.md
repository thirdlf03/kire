# kire

[![CI](https://github.com/thirdlf03/kire/actions/workflows/ci.yml/badge.svg)](https://github.com/thirdlf03/kire/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thirdlf03/kire.svg)](https://pkg.go.dev/github.com/thirdlf03/kire)
[![Go Report Card](https://goreportcard.com/badge/github.com/thirdlf03/kire)](https://goreportcard.com/report/github.com/thirdlf03/kire)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

[日本語](README.md)

A Go CLI that splits Markdown at topic boundaries. An LLM reads the entire document and directly detects semantic boundaries.

```bash
$ export GEMINI_API_KEY=your-key
$ kire spec.md
Wrote 3 segments to docs/spec/
Wrote docs/spec/index.md
```

## Features

- Concatenating segments reproduces the original byte-for-byte (lossless)
- LLM (Gemini) understands the text and decides where to split
- Parent headings are automatically prepended to each segment
- `--llm-refine` adds embedding cosine similarity as additional context for the LLM
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
$ kire example/input.md
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
# Basic usage (requires GEMINI_API_KEY)
export GEMINI_API_KEY=your-key
kire document.md

# Specify output directory and prefix
kire --out docs --prefix split document.md

# LLM-refine mode (pass embedding cosine similarity to LLM)
kire --llm-refine document.md

# LLM-refine with a specific embedder
kire --llm-refine --embedder tfidf document.md

# Batch process multiple files
kire --in a.md --in b.md --out docs
```

`--dry-run` previews without writing files.

### Options

```
--llm-model string        LLM model name (default: gemini-2.5-flash-lite)
--llm-refine              Add embedding cosine similarity as LLM context
--overlap int             Overlapping lines between segments
--context-format          comment|front-matter|heading|none
--context-max-depth int   Max heading depth for context (0 = unlimited)
--embedder string         Embedder (for --llm-refine): auto|gemini|openai|ollama|tfidf|mock
--embed-model string      Embedding model name (for --llm-refine)
--cache string            Embedding cache file path (for --llm-refine)
--jsonl                   JSONL metadata output (--jsonl=- for stdout)
--agent-metadata          Include segment IDs and quality scores
--state-file string       Incremental processing state file
--dag-json string         Export DAG as JSON
--dag-dot string          Export DAG as DOT
--prefix string           Output file prefix (empty = semantic naming)
--out string              Output directory (default: docs)
--force                   Overwrite without confirmation
--dry-run                 Run without writing files
--quiet                   Suppress all log output
--json                    Output JSON summary to stdout
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

## LLM-refine mode

With `--llm-refine`, embedding cosine similarities are computed before calling the LLM and included in the prompt. This helps the LLM identify gaps with low similarity as potential boundaries.

```bash
# auto (Gemini if GEMINI_API_KEY is set, otherwise TF-IDF)
kire --llm-refine document.md

# TF-IDF (no API, deterministic)
kire --llm-refine --embedder tfidf document.md

# OpenAI
export OPENAI_API_KEY=your-key
kire --llm-refine --embedder openai document.md

# Ollama (local)
kire --llm-refine --embedder ollama document.md
```

Available embedders:

| Provider | API Key | Default Model | Local |
|----------|---------|---------------|-------|
| `gemini` | `GEMINI_API_KEY` | `gemini-embedding-001` | No |
| `openai` | `OPENAI_API_KEY` | `text-embedding-3-large` | No |
| `ollama` | Not required (`OLLAMA_HOST` to change endpoint) | `nomic-embed-text` | Yes |
| `tfidf` | Not required | — | Yes |
| `mock` | Not required | — | Yes |

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
Source → Parse → Tokenize → LineEstimate → [LLM-Refine? Embed+Sims : noop] → LLM Boundary → Optimize → IDs → Quality → Render → Output
```

It starts by parsing Markdown into an AST with goldmark and breaking it into blocks. Each block gets a SourceRange (byte offset) and HeadingPath.

The LLM reads all blocks and returns a list of gap indices where semantic boundaries should be placed, using structured output to ensure valid JSON. In LLM-refine mode, embedding cosine similarities between adjacent blocks are computed first and included in the prompt.

Finally, the original text is restored byte-for-byte via SourceRange, context headers and overlap are applied, and files are written out.

<details>
<summary>Package layout</summary>

```
cmd/kire/
├── main.go          CLI entry point
├── root.go          Flag definitions
├── run.go           Main execution logic
├── merge.go         merge subcommand
├── bench.go         Benchmark subcommand
├── summary.go       JSON summary generation
├── config.go        Config structs
├── logger.go        Log setup
└── completion.go    Shell completion

internal/
├── model/           Block, Segment, Embedding, SourceRange
├── parser/          Markdown → []Block (goldmark)
├── tokenizer/       Token estimation
├── embedding/       Embedder interface + providers (for --llm-refine)
│   ├── cached.go    Cache decorator
│   └── concurrent.go Concurrent decorator
├── llmsplit/        LLM boundary detection (Gemini structured output)
├── boundary/        BoundaryResult + similarity utilities
├── segment/         Segment optimization + ID generation + quality scores
├── ctxheader/       Parent heading context insertion
├── output/          File output + JSONL + merge
├── cache/           Embedding cache (JSON)
├── dag/             Dependency DAG export (JSON/DOT)
├── concurrency/     Worker pool + rate limiter
├── vecmath/         Vector math (cosine similarity)
├── pipeline/        Pipeline orchestration + hooks + incremental processing
└── eval/            Benchmark evaluation (Pk, WindowDiff, PRF)
```

</details>

## Benchmarks

`kire bench` evaluates segmentation quality against gold standard annotations. Metrics include Pk (window-based error, lower is better), WindowDiff (improved Pk), and boundary Precision/Recall/F1.

Test data is `testdata/bench_xl.md` (104 blocks, 18 gold boundaries). The document has intentionally overlapping vocabulary across sections, containing topic transitions that heading-based splitting cannot detect.

### LLM vs embedding-based (final stage)

| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|-----:|-----:|------:|-----:|-----:|-----:|
| LLM | 18 | 0.20 | 0.20 | 0.47 | 0.44 | 0.46 |
| tfidf | 18 | 0.39 | 0.39 | 0.00 | 0.00 | 0.00 |
| gemini | 16 | 0.41 | 0.41 | 0.07 | 0.06 | 0.06 |

LLM mode halves the Pk and significantly outperforms embedding-based methods on F1.

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
