package boundary

import (
	"math"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestNewGramMatrix(t *testing.T) {
	embeddings := []model.Embedding{
		{Vector: []float64{1, 0, 0}},
		{Vector: []float64{0, 1, 0}},
		{Vector: []float64{0, 0, 1}},
	}
	g := NewGramMatrix(embeddings)
	if g.n != 3 {
		t.Fatalf("expected n=3, got %d", g.n)
	}
	// Diagonal should be 1.0 (cosine of unit vector with itself)
	for i := 0; i < 3; i++ {
		if math.Abs(g.K[i][i]-1.0) > 1e-9 {
			t.Errorf("K[%d][%d] = %f, expected 1.0", i, i, g.K[i][i])
		}
	}
	// Orthogonal vectors should have 0 similarity
	if math.Abs(g.K[0][1]) > 1e-9 {
		t.Errorf("K[0][1] = %f, expected 0.0", g.K[0][1])
	}
}

func TestNewGramMatrix_Empty(t *testing.T) {
	g := NewGramMatrix(nil)
	if g.n != 0 {
		t.Fatalf("expected n=0, got %d", g.n)
	}
}

func TestGramMatrix_SegmentCost_SingleElement(t *testing.T) {
	embeddings := []model.Embedding{
		{Vector: []float64{1, 0, 0}},
		{Vector: []float64{0, 1, 0}},
	}
	g := NewGramMatrix(embeddings)
	// Single element segment: cost should be 0
	// Cost(a,a+1) = K[a][a] - (1/1)*K[a][a] = 0
	cost := g.SegmentCost(0, 1)
	if math.Abs(cost) > 1e-9 {
		t.Errorf("single element cost = %f, expected 0", cost)
	}
}

func TestGramMatrix_SegmentCost_IdenticalVectors(t *testing.T) {
	v := []float64{1, 0, 0}
	embeddings := []model.Embedding{
		{Vector: v},
		{Vector: v},
		{Vector: v},
	}
	g := NewGramMatrix(embeddings)
	// All identical unit vectors: all K[i][j] = 1
	// Cost(0,3) = 3*1 - (1/3)*(3*3*1) = 3 - 3 = 0
	cost := g.SegmentCost(0, 3)
	if math.Abs(cost) > 1e-9 {
		t.Errorf("identical vectors cost = %f, expected 0", cost)
	}
}

func TestGramMatrix_SegmentCost_OrthogonalVectors(t *testing.T) {
	embeddings := []model.Embedding{
		{Vector: []float64{1, 0, 0}},
		{Vector: []float64{0, 1, 0}},
		{Vector: []float64{0, 0, 1}},
	}
	g := NewGramMatrix(embeddings)
	// Orthogonal vectors: K[i][j] = 0 for i!=j, K[i][i] = 1
	// Cost(0,3) = 3*1 - (1/3)*(3*1) = 3 - 1 = 2
	cost := g.SegmentCost(0, 3)
	if math.Abs(cost-2.0) > 1e-9 {
		t.Errorf("orthogonal vectors cost = %f, expected 2.0", cost)
	}
}

func TestGramMatrix_SegmentCost_SplitIsCheaper(t *testing.T) {
	// Two clear topics: splitting at boundary should reduce total cost
	topicA := []float64{1, 0, 0}
	topicB := []float64{0, 1, 0}
	embeddings := []model.Embedding{
		{Vector: topicA},
		{Vector: topicA},
		{Vector: topicA},
		{Vector: topicB},
		{Vector: topicB},
		{Vector: topicB},
	}
	g := NewGramMatrix(embeddings)

	costWhole := g.SegmentCost(0, 6)
	costSplit := g.SegmentCost(0, 3) + g.SegmentCost(3, 6)

	if costSplit >= costWhole {
		t.Errorf("split cost (%f) should be less than whole cost (%f)", costSplit, costWhole)
	}
}

func TestGramMatrix_SegmentCost_Bounds(t *testing.T) {
	embeddings := []model.Embedding{
		{Vector: []float64{1, 0}},
		{Vector: []float64{0, 1}},
		{Vector: []float64{1, 0}},
	}
	g := NewGramMatrix(embeddings)

	// Subsegments
	cost01 := g.SegmentCost(0, 1)
	cost12 := g.SegmentCost(1, 2)
	cost23 := g.SegmentCost(2, 3)
	// Each single element cost should be 0
	if math.Abs(cost01) > 1e-9 || math.Abs(cost12) > 1e-9 || math.Abs(cost23) > 1e-9 {
		t.Errorf("single element costs should be 0: got %f, %f, %f", cost01, cost12, cost23)
	}

	// Two-element segment of different topics
	cost02 := g.SegmentCost(0, 2)
	if cost02 <= 0 {
		t.Errorf("cost of mixed segment should be > 0, got %f", cost02)
	}
}

func TestCVSScore_ClearTopics(t *testing.T) {
	topicA := []float64{1, 0, 0}
	topicB := []float64{0, 1, 0}
	embeddings := []model.Embedding{
		{Vector: topicA},
		{Vector: topicA},
		{Vector: topicA},
		{Vector: topicB},
		{Vector: topicB},
		{Vector: topicB},
	}

	// Correct boundary: higher CVS score
	correctBoundaries := []int{2}
	wrongBoundaries := []int{1}

	scoreCorrect := CVSScore(embeddings, correctBoundaries)
	scoreWrong := CVSScore(embeddings, wrongBoundaries)

	if scoreCorrect <= scoreWrong {
		t.Errorf("correct boundary CVS (%f) should exceed wrong boundary CVS (%f)",
			scoreCorrect, scoreWrong)
	}
}

func TestCVSScore_NoBoundaries(t *testing.T) {
	embeddings := []model.Embedding{
		{Vector: []float64{1, 0, 0}},
		{Vector: []float64{1, 0, 0}},
	}
	score := CVSScore(embeddings, nil)
	// Single segment: CVS = norm of mean vector = 1.0
	if math.Abs(score-1.0) > 1e-9 {
		t.Errorf("expected CVS=1.0 for identical vectors, got %f", score)
	}
}

func TestCVSScore_Empty(t *testing.T) {
	score := CVSScore(nil, nil)
	if score != 0 {
		t.Errorf("expected 0 for empty, got %f", score)
	}
}

func TestCVSScore_AllIdentical(t *testing.T) {
	v := []float64{1, 0, 0}
	embeddings := []model.Embedding{
		{Vector: v}, {Vector: v}, {Vector: v}, {Vector: v},
	}
	// For identical unit vectors, each segment mean has norm 1.0.
	// CVS = number_of_segments * 1.0 (no penalty applied here).
	// This is correct: the penalty in KCPD/CVS optimization prevents over-segmentation.
	score0 := CVSScore(embeddings, nil)         // 1 segment → 1.0
	score1 := CVSScore(embeddings, []int{1})    // 2 segments → 2.0
	score2 := CVSScore(embeddings, []int{0, 2}) // 3 segments → 3.0

	if math.Abs(score0-1.0) > 1e-9 {
		t.Errorf("expected 1.0, got %f", score0)
	}
	if math.Abs(score1-2.0) > 1e-9 {
		t.Errorf("expected 2.0, got %f", score1)
	}
	if math.Abs(score2-3.0) > 1e-9 {
		t.Errorf("expected 3.0, got %f", score2)
	}
}
