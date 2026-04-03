package similar

import (
	"math"
	"testing"
)

func TestEncodeDecodeVector(t *testing.T) {
	original := []float64{0.1, -0.2, 0.3, 0.0, 1.5}

	encoded := EncodeVector(original)
	if len(encoded) != len(original)*8 {
		t.Fatalf("encoded length: got %d, want %d", len(encoded), len(original)*8)
	}

	decoded := DecodeVector(encoded)
	if len(decoded) != len(original) {
		t.Fatalf("decoded length: got %d, want %d", len(decoded), len(original))
	}
	for i := range original {
		diff := math.Abs(decoded[i] - original[i])
		if diff > 1e-10 {
			t.Errorf("decoded[%d]: got %f, want %f (diff %e)", i, decoded[i], original[i], diff)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical", []float64{1.0, 0.0, 0.0}, []float64{1.0, 0.0, 0.0}, 1.0},
		{"orthogonal", []float64{1.0, 0.0}, []float64{0.0, 1.0}, 0.0},
		{"opposite", []float64{1.0, 0.0}, []float64{-1.0, 0.0}, -1.0},
		{"partial", []float64{1.0, 1.0}, []float64{1.0, 0.0}, 1.0 / math.Sqrt2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-10 {
				t.Errorf("got %f, want %f", got, tt.want)
			}
		})
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	got := CosineSimilarity([]float64{0, 0, 0}, []float64{1, 2, 3})
	if got != 0.0 {
		t.Errorf("expected 0 for zero vector, got %f", got)
	}
}

func TestCosineSimilarity_LengthMismatch(t *testing.T) {
	got := CosineSimilarity([]float64{1.0}, []float64{1.0, 2.0})
	if got != 0.0 {
		t.Errorf("expected 0 for mismatched lengths, got %f", got)
	}
}

func TestEncodeVector_Empty(t *testing.T) {
	encoded := EncodeVector(nil)
	if len(encoded) != 0 {
		t.Errorf("expected 0 bytes for nil, got %d", len(encoded))
	}
}

func TestDecodeVector_Empty(t *testing.T) {
	decoded := DecodeVector(nil)
	if len(decoded) != 0 {
		t.Errorf("expected 0 floats for nil, got %d", len(decoded))
	}
}
