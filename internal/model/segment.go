package model

// Segment represents a contiguous group of blocks forming a semantic unit.
type Segment struct {
	Blocks      []Block
	HeadingPath []string
	Range       SourceRange
	TokenCount  int
	LineCount   int

	// Multi-agent support fields
	ID            string  // Content-addressable segment ID (SHA256)
	Coherence     float64 // Intra-segment semantic coherence (0.0-1.0)
	PrevSegmentID string  // Previous segment ID (empty if first)
	NextSegmentID string  // Next segment ID (empty if last)
}
