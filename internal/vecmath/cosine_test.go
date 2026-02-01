package vecmath_test

import (
	"math"
	"testing"

	"github.com/thirdlf03/kire/internal/vecmath"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical vectors", []float64{1, 2, 3}, []float64{1, 2, 3}, 1.0},
		{"opposite vectors", []float64{1, 0, 0}, []float64{-1, 0, 0}, -1.0},
		{"orthogonal vectors", []float64{1, 0}, []float64{0, 1}, 0.0},
		{"length mismatch", []float64{1, 2}, []float64{1, 2, 3}, 0.0},
		{"empty slices", []float64{}, []float64{}, 0.0},
		{"nil slices", nil, nil, 0.0},
		{"zero vector a", []float64{0, 0, 0}, []float64{1, 2, 3}, 0.0},
		{"zero vector b", []float64{1, 2, 3}, []float64{0, 0, 0}, 0.0},
		{"both zero vectors", []float64{0, 0}, []float64{0, 0}, 0.0},
		{"scaled vectors", []float64{1, 2, 3}, []float64{2, 4, 6}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vecmath.CosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("CosineSimilarity(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
