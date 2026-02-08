# LLM Prompt History

## v1 (initial - before 2026-02-08 tuning)

### systemPrompt

```
You split a Markdown document into semantically coherent segments.

The document is given as a numbered list of blocks. Gap index i is the position between block[i] and block[i+1]. Return gap indices where the topic changes.
```

### systemPromptRefine

```
You split a Markdown document into semantically coherent segments.

The document is given as a numbered list of blocks. Gap index i is the position between block[i] and block[i+1].
Each gap has a cosine similarity score (0.0–1.0) between adjacent block embeddings. Lower similarity suggests a topic change.
Use both the text content and the similarity scores to decide where to place boundaries.
Return gap indices where the topic changes.
```

### Results (evil_tasks_spec, 198 blocks, gold 7 boundaries)

Not measured with v1 prompt directly, but v2 below produced 71 boundaries (massive over-split).

## v2 (2026-02-08 - added boundary placement rule)

### systemPrompt

```
You split a Markdown document into semantically coherent segments.

The document is given as a numbered list of blocks. Gap index i is the position between block[i] and block[i+1]. Return gap indices where the topic changes.

A boundary separates the end of one topic from the start of the next. When a block introduces a new subject—such as a new feature, entity, or subsystem, possibly with a background or motivation paragraph—place the boundary just before that block so the introductory context stays with the new topic, not the previous one.
```

### systemPromptRefine

```
You split a Markdown document into semantically coherent segments.

The document is given as a numbered list of blocks. Gap index i is the position between block[i] and block[i+1].
Each gap has a cosine similarity score (0.0–1.0) between adjacent block embeddings. Lower similarity suggests a topic change.
Use both the text content and the similarity scores to decide where to place boundaries.
Return gap indices where the topic changes.

A boundary separates the end of one topic from the start of the next. When a block introduces a new subject—such as a new feature, entity, or subsystem, possibly with a background or motivation paragraph—place the boundary just before that block so the introductory context stays with the new topic, not the previous one.
```

### Results

evil_tasks_spec (198 blocks, gold 7 boundaries, tolerance 1):
- kire (llm): 72 segs, Pk=0.54, P=0.10, R=1.00, F1=0.18
- random: 8 segs, Pk=0.39, F1=0.29

bench_long (25 blocks, gold 7 boundaries):
- kire (llm): 4 segs, Pk=0.86, F1=0.00

Problem: massive over-segmentation on flat documents (71 boundaries vs 7 gold).
The LLM treats nearly every block transition as a topic change.

## v3 (2026-02-08 - granularity guidelines)

### systemPrompt

```
You split a Markdown document into semantically coherent segments.

The document is given as a numbered list of blocks. Gap index i is the position between block[i] and block[i+1]. Return gap indices where the topic changes.

Guidelines:
- Only place boundaries at major topic shifts — where the document transitions from one subject, feature, or entity to a fundamentally different one.
- Do NOT place boundaries between sub-sections of the same topic. For example, multiple endpoints of the same API resource, validation rules for the same entity, or detailed specifications within a feature all belong in one segment.
- When a block introduces a new subject — such as a new feature, entity, or subsystem, possibly with a background or motivation paragraph — place the boundary just before that introductory block so it stays with the new topic.
- Prefer fewer, larger segments over many small ones. A 200-block document typically needs only 5–15 boundaries, not 50+.
```

### systemPromptRefine

```
You split a Markdown document into semantically coherent segments.

The document is given as a numbered list of blocks. Gap index i is the position between block[i] and block[i+1].
Each gap has a cosine similarity score (0.0–1.0) between adjacent block embeddings. Lower similarity suggests a topic change.
Use both the text content and the similarity scores to decide where to place boundaries.
Return gap indices where the topic changes.

Guidelines:
- Only place boundaries at major topic shifts — where the document transitions from one subject, feature, or entity to a fundamentally different one.
- Do NOT place boundaries between sub-sections of the same topic. For example, multiple endpoints of the same API resource, validation rules for the same entity, or detailed specifications within a feature all belong in one segment.
- When a block introduces a new subject — such as a new feature, entity, or subsystem, possibly with a background or motivation paragraph — place the boundary just before that introductory block so it stays with the new topic.
- Prefer fewer, larger segments over many small ones. A 200-block document typically needs only 5–15 boundaries, not 50+.
```

### Results

evil_tasks_spec (198 blocks, gold 7 boundaries, tolerance 1):
- kire (llm): 9 segs, Pk=0.14, P=0.88, R=1.00, F1=0.93
- random: 8 segs, Pk=0.39, F1=0.29

bench_long (25 blocks, gold 7 boundaries):
- kire (llm): 4 segs, Pk=0.86, F1=0.00 (unchanged from v2)

All documents:

| Document         | Blocks | Gold | Segs | Pk   | P    | R    | F1   |
|------------------|--------|------|------|------|------|------|------|
| evil_tasks_spec  | 198    | 7    | 9    | 0.14 | 0.88 | 1.00 | 0.93 |
| bench_xl         | 104    | 18   | 10   | 0.18 | 1.00 | 0.50 | 0.67 |
| nested_headings  | 12     | 3    | 3    | 0.09 | 1.00 | 0.67 | 0.80 |
| simple           | 9      | 2    | 4    | 0.12 | 0.67 | 1.00 | 0.80 |
| bench_long       | 25     | 7    | 4    | 0.86 | 0.00 | 0.00 | 0.00 |

Massive improvement on flat documents. Over-segmentation solved (71→8 boundaries).
F1 jumped from 0.18 to 0.93. Key was adding granularity guidelines.
Problem: "Prefer fewer, larger segments" + hard number (5-15) causes under-segmentation on headed docs (bench_xl R=0.50).

## v4 (2026-02-08 - heading signals, removed hard numbers)

Removed "Prefer fewer" + hard number. Added "Headings are strong boundary signals". Softened "Do NOT" to allow more splitting.

| Document         | Blocks | Gold | Segs | Pk   | P    | R    | F1   |
|------------------|--------|------|------|------|------|------|------|
| evil_tasks_spec  | 198    | 7    | 12   | 0.21 | 0.64 | 1.00 | 0.78 |
| bench_xl         | 104    | 18   | 10   | 0.18 | 1.00 | 0.50 | 0.67 |
| nested_headings  | 12     | 3    | 3    | 0.09 | 1.00 | 0.67 | 0.80 |
| simple           | 9      | 2    | 5    | 0.25 | 0.50 | 1.00 | 0.67 |
| bench_long       | 25     | 7    | 5    | 0.77 | 0.25 | 0.14 | 0.18 |

Verdict: bench_xl unchanged, evil_tasks_spec regressed (0.93→0.78), simple regressed. Minor bench_long improvement.

## v5 (2026-02-08 - softer grouping, heading signals)

Replaced "Do NOT split sub-sections" with "Group closely related content together". Kept heading signals.

| Document         | Blocks | Gold | Segs | Pk   | P    | R    | F1   |
|------------------|--------|------|------|------|------|------|------|
| evil_tasks_spec  | 198    | 7    | 66   | 0.55 | 0.11 | 1.00 | 0.19 |
| bench_xl         | 104    | 18   | 10   | 0.18 | 1.00 | 0.50 | 0.67 |
| nested_headings  | 12     | 3    | 7    | 0.27 | 0.50 | 1.00 | 0.67 |
| simple           | 9      | 2    | 5    | 0.25 | 0.50 | 1.00 | 0.67 |
| bench_long       | 25     | 7    | 5    | 0.77 | 0.25 | 0.14 | 0.18 |

Verdict: Terrible. evil_tasks_spec back to v2 levels (65 boundaries). Softening "Do NOT" was catastrophic for flat docs.

## v6 (2026-02-09 - adaptive prompting: flat vs headed)

Architecture change: detect `hasHeadings(blocks)` and select different system prompt.
- Flat docs: v3 guidelines (prefer fewer, DO NOT split sub-sections)
- Headed docs: heading-aware guidelines (headings = strong boundary signals)
- 4 variants: flat/headed × default/refine

| Document         | Blocks | Gold | Segs | Pk   | P    | R    | F1   |
|------------------|--------|------|------|------|------|------|------|
| evil_tasks_spec  | 198    | 7    | 8    | 0.08 | 1.00 | 1.00 | 1.00 |
| bench_xl         | 104    | 18   | 10   | 0.18 | 1.00 | 0.50 | 0.67 |
| nested_headings  | 12     | 3    | 3    | 0.09 | 1.00 | 0.67 | 0.80 |
| simple           | 9      | 2    | 4    | 0.12 | 0.67 | 1.00 | 0.80 |
| bench_long       | 25     | 7    | 4    | 0.86 | 0.00 | 0.00 | 0.00 |

evil_tasks_spec: PERFECT (F1=1.00). Flat prompt preserved from v3 works even better.
Headed docs: identical to v3. bench_xl R=0.50 unchanged — gold has 18 boundaries but only 9 headings.

## Conclusion

v6 (adaptive) is the best overall. Flat docs get perfect scores, headed docs match v3.
Remaining issue: bench_xl gold has more boundaries than headings — needs sub-heading boundary detection or gold revision.
