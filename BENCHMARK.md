# Benchmark Log

## Template

```
### YYYY-MM-DD HH:MM

**Command:**
```bash
<command>
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| ... | ... | ... | ... | ... | ... | ... |

**Notes:** <optional notes>
```

---

## Log

### 2026-02-03 23:10

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta-strategy theory --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 21 | 0.03 | 0.04 | 0.85 | 0.94 | 0.89 |

**Notes:** C=0.088 → 0.022 に修正後。auto (F1=0.97) より低いが機能するようになった

---

### 2026-02-03 23:10

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta-strategy theory --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 13 | 0.27 | 0.68 | 0.33 | 0.57 | 0.42 |

**Notes:** C=0.022 で auto と同等の結果

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 51 | 0.47 | 0.71 | 0.18 | 0.50 | 0.26 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta-strategy auto --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 20 | 0.01 | 0.01 | 0.95 | 1.00 | 0.97 |

**Notes:** KCPD + auto shows dramatic improvement over TextTiling

---

### 2026-02-03 22:42

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta-strategy theory --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 1 | 0.53 | 0.53 | 0.00 | 0.00 | 0.00 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta-strategy bic --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 21 | 0.02 | 0.03 | 0.90 | 1.00 | 0.95 |

**Notes:** BIC strategy performs slightly below auto but still excellent

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta-strategy crossval --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 53 | 0.47 | 0.74 | 0.19 | 0.56 | 0.29 |

**Notes:** CrossVal strategy did not improve over baseline

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method hybrid --beta-strategy auto --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 34 | 0.57 | 0.61 | 0.03 | 0.06 | 0.04 |

**Notes:** Hybrid method performed poorly on this dataset

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --embedder mock testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (mock) | 37 | 0.50 | 0.64 | 0.19 | 0.39 | 0.26 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --embedder mock testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (mock) | 20 | 0.01 | 0.01 | 0.95 | 1.00 | 0.97 |

**Notes:** Even mock embedder achieves excellent results with KCPD

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile output --no-baselines --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf,final) | 18 | 0.39 | 0.39 | 0.00 | 0.00 | 0.00 |

**Notes:** Final stage with optimizer (output profile)

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile output --no-baselines --boundary-method kcpd --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf,final) | 19 | 0.26 | 0.34 | 0.50 | 0.50 | 0.50 |

**Notes:** KCPD improves final stage results as well

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --embedder gemini testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (gemini) | 37 | 0.43 | 0.54 | 0.22 | 0.44 | 0.30 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --embedder gemini testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (gemini) | 19 | 0.00 | 0.00 | 1.00 | 1.00 | 1.00 |

**Notes:** Perfect score! Gemini + KCPD achieves F1=1.00 on bench_xl

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 7 | 0.45 | 0.64 | 0.33 | 0.29 | 0.31 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 13 | 0.27 | 0.68 | 0.33 | 0.57 | 0.42 |

**Notes:** Modest improvement on smaller document (25 blocks)

---

### 2026-02-03 22:42

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta-strategy theory --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 4 | 0.77 | 0.77 | 0.00 | 0.00 | 0.00 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --embedder tfidf testdata/gold/nested_headings.json testdata/nested_headings.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 5 | 0.44 | 0.67 | 0.25 | 0.33 | 0.29 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --embedder tfidf testdata/gold/nested_headings.json testdata/nested_headings.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 6 | 0.33 | 0.67 | 0.20 | 0.33 | 0.25 |

**Notes:** Small document (12 blocks), KCPD slightly worse

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --embedder tfidf testdata/gold/simple.json testdata/simple.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 4 | 0.17 | 0.33 | 0.33 | 0.50 | 0.40 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --embedder tfidf testdata/gold/simple.json testdata/simple.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 5 | 0.17 | 0.67 | 0.25 | 0.50 | 0.33 |

**Notes:** Very small document (9 blocks), TextTiling slightly better

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --embedder gemini testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (gemini) | 10 | 0.36 | 0.59 | 0.11 | 0.14 | 0.12 |

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --embedder gemini testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (gemini) | 10 | 0.27 | 0.41 | 0.44 | 0.57 | 0.50 |

**Notes:** KCPD improves F1 from 0.12 to 0.50 on bench_long with Gemini

---

### 2026-02-03 22:31

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --tolerance 1 --embedder gemini testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (gemini) | 10 | 0.27 | 0.41 | 0.78 | 1.00 | 0.88 |

**Notes:** With tolerance=1, F1 jumps from 0.50 to 0.88. Analysis shows boundaries are detected 1 block off from gold in some cases.

**Boundary comparison:**
```
Gold:     [1, 4, 10, 14, 16, 21, 23]
Detected: [1, 4, 7, 11, 14, 18, 21]
```
- Exact match: 1, 4, 14, 21 (4/7)
- Off by 1: 10→11
- Off by 2: 16→18
- Missed: 23
- False positive: 7

bench_long の gold boundaries はセクション内の微妙なトピック遷移を指定しており、正確な位置検出が難しい

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.01 --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 53 | 0.47 | 0.74 | 0.19 | 0.56 | 0.29 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.05 --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 47 | 0.48 | 0.69 | 0.28 | 0.72 | 0.41 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.1 --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 21 | 0.02 | 0.03 | 0.90 | 1.00 | 0.95 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.2 --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 21 | 0.02 | 0.03 | 0.90 | 1.00 | 0.95 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.5 --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 20 | 0.01 | 0.01 | 0.95 | 1.00 | 0.97 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 1.0 --embedder tfidf testdata/gold/bench_xl.json testdata/bench_xl.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 19 | 0.02 | 0.02 | 0.94 | 0.94 | 0.94 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.01 --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 13 | 0.27 | 0.68 | 0.33 | 0.57 | 0.42 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.05 --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 13 | 0.27 | 0.68 | 0.33 | 0.57 | 0.42 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.1 --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 13 | 0.27 | 0.68 | 0.33 | 0.57 | 0.42 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.2 --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 13 | 0.27 | 0.68 | 0.33 | 0.57 | 0.42 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 0.5 --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 7 | 0.55 | 0.64 | 0.17 | 0.14 | 0.15 |

---

### 2026-02-03 22:55

**Command:**
```bash
./kire bench --profile embedder --no-baselines --boundary-method kcpd --beta 1.0 --embedder tfidf testdata/gold/bench_long.json testdata/bench_long.md
```

**Result:**
| Method | Segs | Pk | WDiff | P | R | F1 |
|--------|------|-----|-------|-----|-----|-----|
| kire (tfidf) | 2 | 0.68 | 0.68 | 0.00 | 0.00 | 0.00 |
