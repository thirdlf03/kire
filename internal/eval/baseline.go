package eval

import (
	"math/rand/v2"

	"github.com/thirdlf03/kire/internal/model"
)

// HeadingSplit places a boundary before every heading block.
// Returns sorted gap indices (the gap before the heading).
func HeadingSplit(blocks []model.Block) []int {
	var boundaries []int
	for i, b := range blocks {
		if b.Kind == model.BlockHeading && i > 0 {
			boundaries = append(boundaries, i-1)
		}
	}
	return boundaries
}

// FixedSplit places boundaries every n blocks, producing roughly equal-sized segments.
func FixedSplit(numBlocks, n int) []int {
	if n <= 0 || numBlocks <= n {
		return nil
	}
	var boundaries []int
	for i := n; i < numBlocks; i += n {
		boundaries = append(boundaries, i-1)
	}
	return boundaries
}

// RandomSplit places numBoundaries boundaries at random gap positions.
// The seed parameter ensures reproducibility.
func RandomSplit(numBlocks, numBoundaries int, seed int64) []int {
	if numBlocks < 2 || numBoundaries <= 0 {
		return nil
	}
	maxGaps := numBlocks - 1
	if numBoundaries > maxGaps {
		numBoundaries = maxGaps
	}

	src := rand.NewPCG(uint64(seed), 0)
	rng := rand.New(src)

	// Fisher-Yates shuffle on gap indices, pick first numBoundaries
	gaps := make([]int, maxGaps)
	for i := range gaps {
		gaps[i] = i
	}
	for i := maxGaps - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		gaps[i], gaps[j] = gaps[j], gaps[i]
	}

	selected := gaps[:numBoundaries]
	// Sort the selected boundaries
	sortInts(selected)
	return selected
}

func sortInts(a []int) {
	// Simple insertion sort for small slices
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}
