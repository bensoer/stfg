package provider

import (
	"errors"
	"math"
	"testing"
)

func TestToFloat32Vector_RejectsEmpty(t *testing.T) {
	_, err := toFloat32Vector("openai", []float64{})
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
	var mre *ModelResponseError
	if !errors.As(err, &mre) {
		t.Errorf("expected *ModelResponseError, got %T: %v", err, err)
	}
}

func TestToFloat32Vector_ConvertsFiniteValues(t *testing.T) {
	in := []float64{0, 0.5, -0.25, 1.5}
	out, err := toFloat32Vector("openai", in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("expected length %d, got %d", len(in), len(out))
	}
	for i, v := range in {
		if out[i] != float32(v) {
			t.Errorf("expected out[%d] = %v, got %v", i, float32(v), out[i])
		}
	}
}

func TestToFloat32Vector_RejectsNaN(t *testing.T) {
	_, err := toFloat32Vector("openai", []float64{0.1, math.NaN(), 0.2})
	if err == nil {
		t.Fatal("expected error for NaN input, got nil")
	}
	var mre *ModelResponseError
	if !errors.As(err, &mre) {
		t.Errorf("expected *ModelResponseError, got %T: %v", err, err)
	}
}

func TestToFloat32Vector_RejectsInf(t *testing.T) {
	_, err := toFloat32Vector("openai", []float64{0.1, math.Inf(1), 0.2})
	if err == nil {
		t.Fatal("expected error for +Inf input, got nil")
	}
	var mre *ModelResponseError
	if !errors.As(err, &mre) {
		t.Errorf("expected *ModelResponseError, got %T: %v", err, err)
	}

	_, err = toFloat32Vector("openai", []float64{0.1, math.Inf(-1), 0.2})
	if err == nil {
		t.Fatal("expected error for -Inf input, got nil")
	}
	if !errors.As(err, &mre) {
		t.Errorf("expected *ModelResponseError, got %T: %v", err, err)
	}
}
