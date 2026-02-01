package tokenizer

// TokenEstimator estimates the number of tokens in a text.
type TokenEstimator interface {
	Estimate(text string) (int, error)
}
