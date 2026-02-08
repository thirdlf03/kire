package llmsplit

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
	"google.golang.org/genai"
)

const defaultModel = "gemini-2.5-flash-lite"

// Config configures the LLM boundary detector.
type Config struct {
	Model       string  // LLM model name (default: "gemini-2.5-flash-lite")
	Temperature float32 // 0 = deterministic
}

func (c Config) model() string {
	if c.Model != "" {
		return c.Model
	}
	return defaultModel
}

// Splitter uses an LLM to detect semantic boundaries between blocks.
type Splitter struct {
	client       *genai.Client
	config       Config
	similarities []float64 // optional cosine similarities for --llm-refine mode
}

// SetSimilarities provides cosine similarity scores between adjacent blocks.
// When set, these are included in the LLM prompt as additional context.
func (s *Splitter) SetSimilarities(sims []float64) {
	s.similarities = sims
}

// New creates a new LLM splitter.
func New(client *genai.Client, cfg Config) *Splitter {
	return &Splitter{client: client, config: cfg}
}

// DetectBoundaries sends blocks to the LLM and returns a BoundaryResult.
// Similarities and DepthScores are empty (not applicable for LLM mode).
// Confidences are set to 1.0 for all boundaries.
func (s *Splitter) DetectBoundaries(ctx context.Context, blocks []model.Block) (boundary.BoundaryResult, error) {
	if len(blocks) <= 1 {
		return boundary.BoundaryResult{}, nil
	}

	prompt := buildPrompt(blocks, s.similarities)
	sysPrompt := systemPrompt
	if len(s.similarities) > 0 {
		sysPrompt = systemPromptRefine
	}
	resp, err := s.client.Models.GenerateContent(ctx, s.config.model(), []*genai.Content{
		genai.NewContentFromText(prompt, "user"),
	}, generateConfig(s.config.Temperature, sysPrompt))
	if err != nil {
		return boundary.BoundaryResult{}, fmt.Errorf("llmsplit: GenerateContent: %w", err)
	}

	text := resp.Text()
	var result llmResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return boundary.BoundaryResult{}, fmt.Errorf("llmsplit: unmarshal response: %w (raw: %s)", err, text)
	}

	boundaries := validateBoundaries(result.Boundaries, len(blocks))

	confidences := make([]float64, len(boundaries))
	for i := range confidences {
		confidences[i] = 1.0
	}

	return boundary.BoundaryResult{
		Boundaries:  boundaries,
		Confidences: confidences,
	}, nil
}

// llmResponse is the parsed structured output from the LLM.
type llmResponse struct {
	Boundaries []int `json:"boundaries"`
}

// generateConfig builds the GenerateContentConfig with structured output schema.
func generateConfig(temperature float32, sysPrompt string) *genai.GenerateContentConfig {
	t := temperature
	return &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(sysPrompt, "user"),
		Temperature:       &t,
		ResponseMIMEType:  "application/json",
		ResponseSchema:    boundarySchema(),
	}
}

// boundarySchema returns the genai.Schema for structured output.
func boundarySchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"boundaries": {
				Type:        genai.TypeArray,
				Description: "Sorted list of 0-indexed gap positions where semantic boundaries should be placed. Gap index i means a boundary between block i and block i+1.",
				Items: &genai.Schema{
					Type: genai.TypeInteger,
				},
			},
		},
		Required: []string{"boundaries"},
	}
}

const systemPrompt = `You split a Markdown document into semantically coherent segments.

The document is given as a numbered list of blocks. Gap index i is the position between block[i] and block[i+1]. Return gap indices where the topic changes.`

const systemPromptRefine = `You split a Markdown document into semantically coherent segments.

The document is given as a numbered list of blocks. Gap index i is the position between block[i] and block[i+1].
Each gap has a cosine similarity score (0.0–1.0) between adjacent block embeddings. Lower similarity suggests a topic change.
Use both the text content and the similarity scores to decide where to place boundaries.
Return gap indices where the topic changes.`

// buildPrompt constructs the user prompt from blocks.
// If similarities is non-empty, each gap's cosine similarity is included.
func buildPrompt(blocks []model.Block, similarities []float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The document has %d blocks (gap indices 0 to %d).\n\n", len(blocks), len(blocks)-2)

	hasSims := len(similarities) == len(blocks)-1

	for i, blk := range blocks {
		kindStr := blk.Kind.String()
		if blk.Kind == model.BlockHeading {
			kindStr = fmt.Sprintf("heading, level %d", blk.HeadingLevel)
		}
		fmt.Fprintf(&b, "[Block %d] (%s)\n", i, kindStr)
		b.WriteString(blk.Text)
		b.WriteByte('\n')
		if hasSims && i < len(similarities) {
			fmt.Fprintf(&b, "  --- gap %d similarity: %.4f ---\n", i, similarities[i])
		}
		b.WriteByte('\n')
	}

	return b.String()
}

// validateBoundaries filters, deduplicates, and sorts boundary indices.
// Valid range: 0 <= index <= numBlocks-2.
func validateBoundaries(raw []int, numBlocks int) []int {
	if numBlocks <= 1 {
		return nil
	}
	maxIdx := numBlocks - 2
	seen := make(map[int]bool, len(raw))
	var valid []int
	for _, idx := range raw {
		if idx < 0 || idx > maxIdx {
			continue
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		valid = append(valid, idx)
	}
	slices.Sort(valid)
	return valid
}
