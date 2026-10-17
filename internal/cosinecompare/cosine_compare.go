// Package cosinecompare computes cosine similarity between two float32
// embeddings and gates the result against a configured similarity percentage.
// The package wraps github.com/viterin/vek/vek32 and turns its panic-prone
// preconditions into returned errors, so callers do not need recover.
package cosinecompare

import (
	"fmt"
	"math"

	"github.com/viterin/vek/vek32"
)

// DefaultSimilarityPercent is the percentage used when the caller does not
// have a project-specific value. 60 means a score in [-1, 1] is considered
// similar when it is at least 0.60.
const DefaultSimilarityPercent = 60

// CosineCompare owns the similarity percentage used by AreCosineSimilar. The
// percentage is the floor that a cosine score must meet or exceed to count
// as similar. 60 means score >= 0.60.
type CosineCompare struct {
	similarityPercent float64
}

// NewCosineCompare returns a CosineCompare configured to treat two embeddings
// as similar when their cosine score meets or exceeds similarityPercent on a
// 0-100 scale. It rejects NaN, Inf, and values outside [0, 100]; 0 and 100
// are valid. The constructor performs no I/O.
func NewCosineCompare(similarityPercent float64) (*CosineCompare, error) {
	if math.IsNaN(similarityPercent) {
		return nil, fmt.Errorf("cosine compare: similarity percent is NaN")
	}
	if math.IsInf(similarityPercent, 0) {
		return nil, fmt.Errorf("cosine compare: similarity percent is infinite")
	}
	if similarityPercent < 0 {
		return nil, fmt.Errorf("cosine compare: similarity percent %v is below 0", similarityPercent)
	}
	if similarityPercent > 100 {
		return nil, fmt.Errorf("cosine compare: similarity percent %v is above 100", similarityPercent)
	}
	return &CosineCompare{similarityPercent: similarityPercent}, nil
}

// CosineSimilarity returns the cosine similarity of the two embeddings, in
// [-1, 1] for ordinary non-zero vectors. nil and empty are treated as the
// same empty-vector failure; vek's implementation panics on those inputs, so
// this method rejects them before calling it. A zero vector produces an
// undefined result from the pure-Go fallback, so NaN and Inf are reported as
// errors rather than as a similarity score.
func (c *CosineCompare) CosineSimilarity(left, right []float32) (float32, error) {
	if len(left) == 0 || len(right) == 0 {
		return 0, fmt.Errorf("cosine compare: embedding is empty")
	}
	if len(left) != len(right) {
		return 0, fmt.Errorf("cosine compare: embedding length mismatch")
	}
	score := vek32.CosineSimilarity(left, right)
	if math.IsNaN(float64(score)) || math.IsInf(float64(score), 0) {
		return 0, fmt.Errorf("cosine compare: undefined similarity")
	}
	return score, nil
}

// AreCosineSimilar reports whether the two embeddings are at least as similar
// as the configured percentage. The score is compared with a strict
// greater-than-or-equal against similarityPercent / 100, so exactly 60% passes
// when the percentage is 60. Negative scores are not similar at any positive
// percentage. A comparison error is returned unchanged and false is reported;
// the caller decides whether the error is fatal for its use case.
func (c *CosineCompare) AreCosineSimilar(left, right []float32) (bool, error) {
	score, err := c.CosineSimilarity(left, right)
	if err != nil {
		return false, err
	}
	threshold := float32(c.similarityPercent / 100)
	return score >= threshold, nil
}
