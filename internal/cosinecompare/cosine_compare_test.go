package cosinecompare

import (
	"math"
	"strings"
	"testing"
)

func TestNewCosineCompare_AcceptsZeroSixtyHundred(t *testing.T) {
	for _, pct := range []float64{0, 60, 100} {
		_, err := NewCosineCompare(pct)
		if err != nil {
			t.Errorf("NewCosineCompare(%v) returned unexpected error: %v", pct, err)
		}
	}
}

func TestNewCosineCompare_RejectsNegativeOne(t *testing.T) {
	_, err := NewCosineCompare(-1)
	if err == nil {
		t.Fatal("NewCosineCompare(-1) returned nil, expected error")
	}
	if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}

func TestNewCosineCompare_RejectsOneHundredOne(t *testing.T) {
	_, err := NewCosineCompare(101)
	if err == nil {
		t.Fatal("NewCosineCompare(101) returned nil, expected error")
	}
	if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}

func TestNewCosineCompare_RejectsNaN(t *testing.T) {
	_, err := NewCosineCompare(math.NaN())
	if err == nil {
		t.Fatal("NewCosineCompare(NaN) returned nil, expected error")
	}
	if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}

func TestNewCosineCompare_RejectsPositiveInfinity(t *testing.T) {
	_, err := NewCosineCompare(math.Inf(1))
	if err == nil {
		t.Fatal("NewCosineCompare(+Inf) returned nil, expected error")
	}
	if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}

func TestCosineSimilarity_IdenticalNonZeroIsOne(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	v := []float32{0.5, -0.25, 1.0, 2.0}
	got, err := c.CosineSimilarity(v, v)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(float64(got-1.0)) > 1e-5 {
		t.Errorf("expected ~1.0, got %v", got)
	}
}

func TestAreCosineSimilar_IdenticalAtSixtyIsTrue(t *testing.T) {
	c, err := NewCosineCompare(60)
	if err != nil {
		t.Fatalf("NewCosineCompare(60) returned error: %v", err)
	}
	v := []float32{0.5, -0.25, 1.0, 2.0}
	ok, err := c.AreCosineSimilar(v, v)
	if err != nil {
		t.Fatalf("AreCosineSimilar returned error: %v", err)
	}
	if !ok {
		t.Errorf("expected true at threshold 60 for identical vectors, got false")
	}
}

func TestAreCosineSimilar_IdenticalAtHundredIsTrue(t *testing.T) {
	c, err := NewCosineCompare(100)
	if err != nil {
		t.Fatalf("NewCosineCompare(100) returned error: %v", err)
	}
	v := []float32{0.5, -0.25, 1.0, 2.0}
	ok, err := c.AreCosineSimilar(v, v)
	if err != nil {
		t.Fatalf("AreCosineSimilar returned error: %v", err)
	}
	if !ok {
		t.Errorf("expected true at threshold 100 for identical vectors, got false")
	}
}

func TestCosineSimilarity_OrthogonalIsZero(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	left := []float32{1, 0}
	right := []float32{0, 1}
	got, err := c.CosineSimilarity(left, right)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(float64(got)) > 1e-5 {
		t.Errorf("expected ~0.0 for orthogonal vectors, got %v", got)
	}
}

func TestAreCosineSimilar_OrthogonalNotSimilarAtSixty(t *testing.T) {
	c, err := NewCosineCompare(60)
	if err != nil {
		t.Fatalf("NewCosineCompare(60) returned error: %v", err)
	}
	left := []float32{1, 0}
	right := []float32{0, 1}
	ok, err := c.AreCosineSimilar(left, right)
	if err != nil {
		t.Fatalf("AreCosineSimilar returned error: %v", err)
	}
	if ok {
		t.Errorf("expected false at threshold 60 for orthogonal vectors, got true")
	}
}

func TestAreCosineSimilar_OrthogonalSimilarAtZero(t *testing.T) {
	// At threshold 0, the cutoff score is 0. Orthogonal vectors score exactly 0,
	// so 0 >= 0 must be true. This exercises the >= boundary.
	c, err := NewCosineCompare(0)
	if err != nil {
		t.Fatalf("NewCosineCompare(0) returned error: %v", err)
	}
	left := []float32{1, 0}
	right := []float32{0, 1}
	ok, err := c.AreCosineSimilar(left, right)
	if err != nil {
		t.Fatalf("AreCosineSimilar returned error: %v", err)
	}
	if !ok {
		t.Errorf("expected true at threshold 0 for orthogonal vectors (>= boundary), got false")
	}
}

func TestAreCosineSimilar_HundredNonIdenticalIsFalse(t *testing.T) {
	// At threshold 100, the cutoff score is 1.0. Two non-identical vectors
	// score strictly below 1.0, so this exercises the "score under cutoff is
	// not similar" branch.
	c, err := NewCosineCompare(100)
	if err != nil {
		t.Fatalf("NewCosineCompare(100) returned error: %v", err)
	}
	left := []float32{1, 0}
	right := []float32{0, 1}
	ok, err := c.AreCosineSimilar(left, right)
	if err != nil {
		t.Fatalf("AreCosineSimilar returned error: %v", err)
	}
	if ok {
		t.Errorf("expected false at threshold 100 for non-identical vectors, got true")
	}
}

func TestCosineSimilarity_OppositeIsNegativeOne(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	left := []float32{1, 0}
	right := []float32{-1, 0}
	got, err := c.CosineSimilarity(left, right)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(float64(got-(-1.0))) > 1e-5 {
		t.Errorf("expected ~-1.0 for opposite vectors, got %v", got)
	}
}

func TestAreCosineSimilar_OppositeNotSimilarAtSixtyOrZero(t *testing.T) {
	for _, pct := range []float64{0, 60} {
		c, err := NewCosineCompare(pct)
		if err != nil {
			t.Fatalf("NewCosineCompare(%v) returned error: %v", pct, err)
		}
		left := []float32{1, 0}
		right := []float32{-1, 0}
		ok, err := c.AreCosineSimilar(left, right)
		if err != nil {
			t.Fatalf("AreCosineSimilar at threshold %v returned error: %v", pct, err)
		}
		if ok {
			t.Errorf("expected false at threshold %v for opposite vectors, got true", pct)
		}
	}
}

func TestCosineSimilarity_NilVectorsError(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	if _, err := c.CosineSimilarity(nil, []float32{1, 0}); err == nil {
		t.Error("expected error for nil left, got nil")
	} else if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
	if _, err := c.CosineSimilarity([]float32{1, 0}, nil); err == nil {
		t.Error("expected error for nil right, got nil")
	} else if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}

func TestCosineSimilarity_EmptyVectorsError(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	if _, err := c.CosineSimilarity([]float32{}, []float32{1, 0}); err == nil {
		t.Error("expected error for empty left, got nil")
	} else if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
	if _, err := c.CosineSimilarity([]float32{1, 0}, []float32{}); err == nil {
		t.Error("expected error for empty right, got nil")
	} else if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}

func TestCosineSimilarity_LengthMismatchError(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	left := []float32{1, 0, 0}
	right := []float32{1, 0}
	_, err = c.CosineSimilarity(left, right)
	if err == nil {
		t.Fatal("expected error for length mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
	if !strings.Contains(err.Error(), "length mismatch") {
		t.Errorf("expected error to mention %q, got %q", "length mismatch", err.Error())
	}
}

func TestAreCosineSimilar_LengthMismatchReturnsFalseAndError(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	left := []float32{1, 0, 0}
	right := []float32{1, 0}
	ok, err := c.AreCosineSimilar(left, right)
	if ok {
		t.Error("expected false on length mismatch, got true")
	}
	if err == nil {
		t.Error("expected error on length mismatch, got nil")
	} else if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}

// TestAreCosineSimilar_EmptyVectorsDoNotPanic makes sure empty slices are
// caught before vek is called and turned into a returned error instead of a
// runtime panic.
func TestAreCosineSimilar_EmptyVectorsDoNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AreCosineSimilar panicked on empty vectors: %v", r)
		}
	}()
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	ok, err := c.AreCosineSimilar([]float32{}, []float32{1, 0})
	if ok {
		t.Error("expected false for empty vector, got true")
	}
	if err == nil {
		t.Error("expected error for empty vector, got nil")
	}
}

func TestCosineSimilarity_ZeroVectorIsUndefinedError(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	left := []float32{0, 0}
	right := []float32{1, 0}
	_, err = c.CosineSimilarity(left, right)
	if err == nil {
		t.Fatal("expected undefined-similarity error, got nil")
	}
	if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}

func TestAreCosineSimilar_ZeroVectorReturnsFalseAndError(t *testing.T) {
	c, err := NewCosineCompare(DefaultSimilarityPercent)
	if err != nil {
		t.Fatalf("NewCosineCompare returned error: %v", err)
	}
	left := []float32{0, 0}
	right := []float32{1, 0}
	ok, err := c.AreCosineSimilar(left, right)
	if ok {
		t.Error("expected false for zero vector, got true")
	}
	if err == nil {
		t.Fatal("expected undefined-similarity error, got nil")
	}
	if !strings.Contains(err.Error(), "cosine compare:") {
		t.Errorf("expected error to start with %q, got %q", "cosine compare:", err.Error())
	}
}
