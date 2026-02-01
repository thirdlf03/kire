package tokenizer

import "math"

// LocalEstimator estimates token count using a byte-based heuristic:
// ceil(ascii_bytes/4 + non_ascii_bytes/3)
type LocalEstimator struct{}

func NewLocalEstimator() *LocalEstimator {
	return &LocalEstimator{}
}

func (e *LocalEstimator) Estimate(text string) (int, error) {
	if len(text) == 0 {
		return 0, nil
	}
	var asciiBytes, nonASCIIBytes int
	for i := 0; i < len(text); i++ {
		if text[i] < 0x80 {
			asciiBytes++
		} else {
			nonASCIIBytes++
		}
	}
	tokens := float64(asciiBytes)/4.0 + float64(nonASCIIBytes)/3.0
	return int(math.Ceil(tokens)), nil
}
