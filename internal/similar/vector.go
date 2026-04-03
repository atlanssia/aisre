package similar

import (
	"encoding/binary"
	"math"
)

// EncodeVector encodes a float64 slice to a compact binary representation.
// Uses encoding/binary.LittleEndian — 8 bytes per float64.
func EncodeVector(v []float64) []byte {
	buf := make([]byte, len(v)*8)
	for i, f := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(f))
	}
	return buf
}

// DecodeVector decodes a binary representation back to a float64 slice.
func DecodeVector(buf []byte) []float64 {
	n := len(buf) / 8
	v := make([]float64, n)
	for i := range v {
		v[i] = math.Float64frombits(binary.LittleEndian.Uint64(buf[i*8:]))
	}
	return v
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 for zero vectors or length mismatch.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
