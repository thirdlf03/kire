default:
    @just --list

# Build the kire binary
build:
    go build -ldflags "-X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o kire ./cmd/kire

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test -v ./...

# Run tests with coverage
cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Show coverage in browser
cover-html: cover
    go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
    go vet ./...

# Run kire
run *ARGS:
    go run ./cmd/kire {{ARGS}}

# Run embedding provider benchmark
benchmark: build
    #!/usr/bin/env bash
    set -euo pipefail
    providers=("gemini" "openai" "ollama:nomic-embed-text" "ollama:mxbai-embed-large" "tfidf")
    input="testdata/bench_design_doc.md"
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT
    cat > "$tmpdir/analyze.py" << 'PYEOF'
    import json, sys, math
    d = json.load(open(sys.argv[1]))
    segs = d["segments"]
    toks = [s["token_count"] for s in segs]
    n = len(toks)
    avg = sum(toks) / n
    std = math.sqrt(sum((t - avg) ** 2 for t in toks) / n)
    boundaries = d.get("boundaries", [])
    print(f"segs={n}  tok={sum(toks)}  avg={avg:.0f}  std={std:.1f}  min={min(toks)}  max={max(toks)}  boundaries={len(boundaries)}")
    for i, s in enumerate(segs):
        name = s["filename"].split("-", 1)[-1].replace(".md", "")
        print(f"    seg{i+1}: {s['token_count']}tok {s['block_count']}blk  {name}")
    PYEOF
    echo "=== Embedding Provider Benchmark ==="
    echo "Input: $input"
    echo ""
    for p in "${providers[@]}"; do
        embedder="${p%%:*}"
        model="${p#*:}"
        label="$p"
        out="$tmpdir/${p//[:\/]/_}.json"
        args=(--embedder "$embedder" --dry-run --json --quiet)
        if [[ "$embedder" != "$model" ]]; then
            args+=(--embed-model "$model")
        fi
        if ./kire "${args[@]}" "$input" > "$out" 2>/dev/null; then
            echo "✓ $label"
            python3 "$tmpdir/analyze.py" "$out"
            echo ""
        else
            echo "✗ $label  FAILED"
            echo ""
        fi
    done

# Evaluate segmentation quality against gold standard
bench-quality: build
    #!/usr/bin/env bash
    set -euo pipefail
    methods=("texttiling" "kcpd" "hybrid")
    docs=(
        "testdata/gold/simple.json testdata/simple.md"
        "testdata/gold/nested_headings.json testdata/nested_headings.md"
        "testdata/gold/bench_long.json testdata/bench_long.md"
        "testdata/gold/bench_xl.json testdata/bench_xl.md"
    )
    for method in "${methods[@]}"; do
        echo "=== boundary-method: $method ==="
        for doc in "${docs[@]}"; do
            ./kire bench --boundary-method "$method" $doc
            echo ""
        done
    done

# Run Go performance benchmarks
bench-perf:
    go test -bench=. -benchmem ./internal/...

# Run all benchmarks (quality + performance)
bench: bench-quality bench-perf

# Test GoReleaser config locally (snapshot, no publish)
release-dry-run:
    goreleaser release --snapshot --clean

# Clean build artifacts
clean:
    rm -f kire coverage.out coverage.html
    rm -rf dist/
