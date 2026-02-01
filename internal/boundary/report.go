package boundary

import (
	"fmt"
	"html/template"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/thirdlf03/kire/internal/model"
)

// ReportConfig holds the data needed to render an HTML boundary report.
type ReportConfig struct {
	Blocks    []model.Block
	Result    BoundaryResult
	Segments  []model.Segment
	InputName string
	Source    []byte
}

// WriteHTMLReport renders an interactive HTML report with similarity/depth charts
// and a block detail table.
func WriteHTMLReport(w io.Writer, cfg ReportConfig) error {
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("parse report template: %w", err)
	}

	boundarySet := make(map[int]bool, len(cfg.Result.Boundaries))
	for _, b := range cfg.Result.Boundaries {
		boundarySet[b] = true
	}

	type blockRow struct {
		Index      int
		Kind       string
		Preview    string
		Similarity string
		DepthScore string
		IsBoundary bool
	}

	rows := make([]blockRow, len(cfg.Blocks))
	for i, b := range cfg.Blocks {
		preview := b.Text
		if utf8.RuneCountInString(preview) > 80 {
			runes := []rune(preview)
			preview = string(runes[:80]) + "…"
		}
		row := blockRow{
			Index:      i,
			Kind:       b.Kind.String(),
			Preview:    preview,
			IsBoundary: boundarySet[i],
		}
		if i < len(cfg.Result.Similarities) {
			row.Similarity = fmt.Sprintf("%.4f", cfg.Result.Similarities[i])
		} else {
			row.Similarity = "—"
		}
		if i < len(cfg.Result.DepthScores) {
			row.DepthScore = fmt.Sprintf("%.4f", cfg.Result.DepthScores[i])
		} else {
			row.DepthScore = "—"
		}
		rows[i] = row
	}

	thresholdStr := fmt.Sprintf("%.6f", cfg.Result.Threshold)

	type segmentView struct {
		Index      int
		TokenCount int
		BlockCount int
		Content    string
	}
	segViews := make([]segmentView, len(cfg.Segments))
	for i, seg := range cfg.Segments {
		content := ""
		if cfg.Source != nil && seg.Range.End <= len(cfg.Source) {
			content = string(cfg.Source[seg.Range.Start:seg.Range.End])
		}
		// Escape </script> to prevent XSS when embedding in <script type="text/plain"> tags
		content = strings.ReplaceAll(content, "</script>", `<\/script>`)
		segViews[i] = segmentView{
			Index:      i + 1,
			TokenCount: seg.TokenCount,
			BlockCount: len(seg.Blocks),
			Content:    content,
		}
	}

	data := struct {
		InputName      string
		SimsJSON       template.JS
		DepthsJSON     template.JS
		BoundariesJSON template.JS
		ThresholdJS    template.JS
		ThresholdFmt   string
		Blocks         []blockRow
		NumSegments    int
		NumBoundaries  int
		Segments       []segmentView
	}{
		InputName:      cfg.InputName,
		SimsJSON:       template.JS(jsFloatSlice(cfg.Result.Similarities)),
		DepthsJSON:     template.JS(jsFloatSlice(cfg.Result.DepthScores)),
		BoundariesJSON: template.JS(jsIntSlice(cfg.Result.Boundaries)),
		ThresholdJS:    template.JS(thresholdStr),
		ThresholdFmt:   fmt.Sprintf("%.4f", cfg.Result.Threshold),
		Blocks:         rows,
		NumSegments:    len(cfg.Segments),
		NumBoundaries:  len(cfg.Result.Boundaries),
		Segments:       segViews,
	}

	return tmpl.Execute(w, data)
}

func jsFloatSlice(vals []float64) string {
	if len(vals) == 0 {
		return "[]"
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			parts[i] = "0"
		} else {
			parts[i] = fmt.Sprintf("%.6f", v)
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func jsIntSlice(vals []int) string {
	if len(vals) == 0 {
		return "[]"
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

const segmentColors = "var SEG_COLORS=['#1976d2','#43a047','#e65100','#8e24aa','#00838f','#c62828','#558b2f','#4527a0','#ef6c00','#00695c'];"

const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Boundary Report: {{.InputName}}</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.7/dist/chart.umd.min.js" integrity="sha384-vsrfeLOOY6KuIYKDlmVH5UiBmgIdB1oEf7p01YgWHuqmOHfZr374+odEv96n9tNC" crossorigin="anonymous"></script>
<script src="https://cdn.jsdelivr.net/npm/chartjs-plugin-annotation@3.1.0/dist/chartjs-plugin-annotation.min.js" integrity="sha384-3N9GHhCtN3CQef6tNfqgZlv7sQLYIkcChN+uaTZ7xVdzKYp/SjBNPxa92+hM7EAY" crossorigin="anonymous"></script>
<script src="https://cdn.jsdelivr.net/npm/marked@15.0.6/marked.min.js" integrity="sha384-b5hg04N6F0rvyz1a/GVoPPY0JcqGTARCmEuFCqwQKX3zq7LsxhV+n+6Ykh2pQOCH" crossorigin="anonymous"></script>
<style>
  *, *::before, *::after { box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 20px; background: #fafafa; color: #333; }
  h1 { color: #333; margin-bottom: 8px; }
  .stats { display: flex; flex-wrap: wrap; gap: 16px; margin: 10px 0 20px; }
  .stat { background: #fff; padding: 10px 16px; border-radius: 6px; box-shadow: 0 1px 2px rgba(0,0,0,0.08); }
  .stat-value { font-size: 24px; font-weight: bold; color: #1976d2; }
  .stat-label { font-size: 12px; color: #666; }

  /* Tabs */
  .tab-bar { display: flex; gap: 0; border-bottom: 2px solid #ddd; margin-bottom: 0; }
  .tab-btn { padding: 10px 24px; border: none; background: none; font-size: 15px; cursor: pointer; color: #666;
             border-bottom: 2px solid transparent; margin-bottom: -2px; transition: all 0.15s; }
  .tab-btn:hover { color: #333; background: #f5f5f5; }
  .tab-btn.active { color: #1976d2; border-bottom-color: #1976d2; font-weight: 600; }
  .tab-panel { display: none; }
  .tab-panel.active { display: block; }

  /* Chart guide */
  .chart-guide { background: #f8f9fa; border: 1px solid #e0e0e0; border-radius: 6px; padding: 14px 18px;
                 margin: 16px auto; max-width: 1200px; font-size: 13px; line-height: 1.7; color: #555; }
  .chart-guide summary { cursor: pointer; font-weight: 600; color: #333; font-size: 14px; }
  .chart-guide ul { margin: 8px 0 0; padding-left: 20px; }
  .chart-guide li { margin-bottom: 4px; }
  .legend-line { display: inline-block; width: 28px; height: 3px; vertical-align: middle; margin-right: 4px; border-radius: 2px; }

  /* Analysis tab */
  .chart-container { max-width: 1200px; margin: 20px auto; background: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  canvas { width: 100% !important; }
  table { width: 100%; border-collapse: collapse; margin: 20px 0; font-size: 14px; }
  th, td { padding: 6px 10px; border: 1px solid #ddd; text-align: left; }
  th { background: #f5f5f5; position: sticky; top: 0; }
  .boundary-row { background: #fff3e0; font-weight: bold; }
  .preview { max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  /* Split View tab */
  .split-container { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; margin-top: 20px; align-items: start; }
  @media (max-width: 900px) { .split-container { grid-template-columns: 1fr; } }
  .split-pane { background: #fff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); overflow: hidden; }
  .split-pane-header { padding: 12px 16px; background: #f5f5f5; font-weight: 600; font-size: 14px; border-bottom: 1px solid #ddd;
                       position: sticky; top: 0; z-index: 1; }
  .split-pane-body { padding: 0; max-height: 80vh; overflow-y: auto; }

  /* Original pane */
  .seg-region { padding: 16px 20px; }
  .seg-region .md-rendered { font-size: 14px; line-height: 1.7; }
  .seg-region .md-rendered h1, .seg-region .md-rendered h2, .seg-region .md-rendered h3 { margin-top: 0.5em; }
  .seg-region .md-rendered pre { background: #f5f5f5; padding: 10px; border-radius: 4px; overflow-x: auto; }
  .seg-region .md-rendered code { background: #f0f0f0; padding: 1px 4px; border-radius: 3px; font-size: 0.9em; }
  .seg-region .md-rendered pre code { background: none; padding: 0; }

  /* Segment card (right pane) */
  .segment-card { margin: 16px; border: 1px solid #e0e0e0; border-radius: 6px; overflow: hidden; border-left: 4px solid transparent; }
  .segment-card-header { padding: 8px 12px; background: #fafafa; font-size: 13px; font-weight: 600;
                         display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #eee; }
  .segment-card-header .badge { font-size: 11px; font-weight: normal; color: #666; }
  .segment-card-body { padding: 12px 16px; font-size: 14px; line-height: 1.7; }
  .segment-card-body pre { background: #f5f5f5; padding: 10px; border-radius: 4px; overflow-x: auto; }
  .segment-card-body code { background: #f0f0f0; padding: 1px 4px; border-radius: 3px; font-size: 0.9em; }
  .segment-card-body pre code { background: none; padding: 0; }
  .segment-card-body h1, .segment-card-body h2, .segment-card-body h3 { margin-top: 0.5em; }
  .seg-dot { display: inline-block; width: 10px; height: 10px; border-radius: 2px; margin-right: 6px; vertical-align: middle; }
</style>
</head>
<body>
<h1>Boundary Report: {{.InputName}}</h1>
<div class="stats">
  <div class="stat"><div class="stat-value">{{len .Blocks}}</div><div class="stat-label">Blocks</div></div>
  <div class="stat"><div class="stat-value">{{.NumSegments}}</div><div class="stat-label">Segments</div></div>
  <div class="stat"><div class="stat-value">{{.NumBoundaries}}</div><div class="stat-label">Boundaries</div></div>
  <div class="stat"><div class="stat-value">{{.ThresholdFmt}}</div><div class="stat-label">Threshold</div></div>
</div>

<div class="tab-bar">
  <button class="tab-btn active" data-tab="analysis">Analysis</button>
  <button class="tab-btn" data-tab="split">Split View</button>
</div>

<!-- Analysis Tab -->
<div id="tab-analysis" class="tab-panel active">

<details class="chart-guide" open>
<summary>How to Read / グラフの読み方</summary>
<ul>
  <li><span class="legend-line" style="background:#1976d2"></span><strong>Similarity Curve</strong> &#8212;
      Cosine similarity between adjacent blocks. High values (close to 1.0) mean similar topics; <em>dips</em> indicate potential topic changes.<br>
      <small>隣接ブロック間のコサイン類似度。1.0 に近いほど話題が似ています。谷（落ち込み）がトピックの変わり目候補です。</small></li>
  <li><span class="legend-line" style="background:#43a047"></span><strong>Depth Score Curve</strong> &#8212;
      How deep each dip is relative to surrounding peaks. Higher = stronger boundary evidence.<br>
      <small>周囲のピークに対する谷の深さ。値が大きいほどトピック境界の根拠が強く、閾値を超えた箇所が分割点になります。</small></li>
  <li><span class="legend-line" style="background:#e53935"></span><strong>Threshold</strong> (red line) &#8212;
      Depth score cutoff. Gaps above this line become boundaries.<br>
      <small>閾値（赤い水平線）。この線より上の深度スコアを持つギャップが境界として選択されます。</small></li>
  <li><span class="legend-line" style="background:#ff9800; border-top:2px dashed #ff9800; height:0"></span><strong>Boundary</strong> (orange dashed lines) &#8212;
      Detected split points.<br>
      <small>検出された分割点（オレンジの破線）。この位置で文書がセグメントに分割されます。</small></li>
</ul>
</details>

<div class="chart-container">
  <h2>Similarity Curve</h2>
  <canvas id="simChart"></canvas>
</div>
<div class="chart-container">
  <h2>Depth Score Curve</h2>
  <canvas id="depthChart"></canvas>
</div>
<div class="chart-container">
  <h2>Block Details</h2>
  <table>
    <thead><tr><th>#</th><th>Kind</th><th>Preview</th><th>Sim&#8594;next</th><th>Depth</th><th>Boundary</th></tr></thead>
    <tbody>
    {{range .Blocks}}
    <tr{{if .IsBoundary}} class="boundary-row"{{end}}>
      <td>{{.Index}}</td>
      <td>{{.Kind}}</td>
      <td class="preview">{{.Preview}}</td>
      <td>{{.Similarity}}</td>
      <td>{{.DepthScore}}</td>
      <td>{{if .IsBoundary}}&#9986;{{end}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
</div>
</div>

<!-- Split View Tab -->
<div id="tab-split" class="tab-panel">
<div class="split-container">
  <div class="split-pane">
    <div class="split-pane-header">Original</div>
    <div class="split-pane-body" id="original-pane">
      <div class="seg-region">
        <div class="md-rendered" data-md-src="orig-full">Loading...</div>
        <script type="text/plain" id="orig-full">{{range .Segments}}{{.Content}}{{end}}</script>
      </div>
    </div>
  </div>
  <div class="split-pane">
    <div class="split-pane-header">Segments ({{.NumSegments}})</div>
    <div class="split-pane-body" id="segments-pane">
      {{range .Segments}}
      <div class="segment-card" data-seg="{{.Index}}">
        <div class="segment-card-header">
          <span><span class="seg-dot"></span>Segment {{.Index}}</span>
          <span class="badge">{{.BlockCount}} blocks &#183; {{.TokenCount}} tokens</span>
        </div>
        <div class="segment-card-body" data-md-src="seg-card-{{.Index}}">Loading...</div>
        <script type="text/plain" id="seg-card-{{.Index}}">{{.Content}}</script>
      </div>
      {{end}}
    </div>
  </div>
</div>
</div>

<script>
` + segmentColors + `
function segColor(i) { return SEG_COLORS[(i - 1) % SEG_COLORS.length]; }

/* Tab switching */
document.querySelectorAll('.tab-btn').forEach(function(btn) {
  btn.addEventListener('click', function() {
    var name = btn.getAttribute('data-tab');
    document.querySelectorAll('.tab-panel').forEach(function(p) { p.classList.remove('active'); });
    document.querySelectorAll('.tab-btn').forEach(function(b) { b.classList.remove('active'); });
    document.getElementById('tab-' + name).classList.add('active');
    btn.classList.add('active');
  });
});

/* Render markdown + apply segment colors */
document.addEventListener('DOMContentLoaded', function() {
  if (typeof marked !== 'undefined') {
    marked.use({ renderer: { html: function() { return ''; } } });
    marked.setOptions({ breaks: true, gfm: true });
    document.querySelectorAll('[data-md-src]').forEach(function(el) {
      var srcEl = document.getElementById(el.getAttribute('data-md-src'));
      if (srcEl) { el.innerHTML = marked.parse(srcEl.textContent); }
    });
  }
  /* Apply segment colors */
  document.querySelectorAll('.segment-card').forEach(function(el) {
    var idx = parseInt(el.getAttribute('data-seg'), 10);
    el.style.borderLeftColor = segColor(idx);
    var dot = el.querySelector('.seg-dot');
    if (dot) dot.style.background = segColor(idx);
  });
});

/* Charts */
(function() {
  var simData = {{.SimsJSON}};
  var depthData = {{.DepthsJSON}};
  var boundaryData = {{.BoundariesJSON}};
  var threshold = {{.ThresholdJS}};

  var labels = simData.map(function(_, i) { return i; });

  function makeBoundaryAnnotations(color) {
    var acc = {};
    boundaryData.forEach(function(b) {
      acc['boundary' + b] = {
        type: 'line', xMin: b, xMax: b,
        borderColor: color, borderWidth: 2, borderDash: [6, 3],
        label: { display: true, content: 'B' + b, position: 'start', font: { size: 10 } }
      };
    });
    return acc;
  }

  new Chart(document.getElementById('simChart'), {
    type: 'line',
    data: {
      labels: labels,
      datasets: [{
        label: 'Cosine Similarity', data: simData,
        borderColor: '#1976d2', backgroundColor: 'rgba(25,118,210,0.1)',
        fill: true, tension: 0.2, pointRadius: 3
      }]
    },
    options: {
      responsive: true,
      plugins: {
        annotation: { annotations: makeBoundaryAnnotations('#ff9800') },
        tooltip: { callbacks: { title: function(items) { return 'Gap ' + items[0].dataIndex; } } }
      },
      scales: { y: { beginAtZero: true, max: 1 } }
    }
  });

  var depthAnnotations = makeBoundaryAnnotations('#ff9800');
  depthAnnotations['threshold'] = {
    type: 'line', yMin: threshold, yMax: threshold,
    borderColor: '#e53935', borderWidth: 2,
    label: { display: true, content: 'Threshold: ' + threshold.toFixed(4), position: 'end', font: { size: 10 } }
  };

  new Chart(document.getElementById('depthChart'), {
    type: 'line',
    data: {
      labels: labels,
      datasets: [{
        label: 'Depth Score', data: depthData,
        borderColor: '#43a047', backgroundColor: 'rgba(67,160,71,0.1)',
        fill: true, tension: 0.2, pointRadius: 3
      }]
    },
    options: {
      responsive: true,
      plugins: { annotation: { annotations: depthAnnotations } },
      scales: { y: { beginAtZero: true } }
    }
  });
})();
</script>
</body>
</html>`
