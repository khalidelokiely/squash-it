package hash

import (
	"strings"
	"testing"
)

// TestMurmurHash_Generate8CharHashExactly verifies that hash output length matches expected character bounds regardless of input size.
func TestMurmurHash_Generate8CharHashExactly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantLen int
	}{
		{
			name:    "Standard URL String",
			input:   "https://www.google.com",
			wantLen: 8,
		},
		{
			name: "Large Multi-Megabyte Payload",
			input: func() string {
				var builder strings.Builder
				size := 10 * 1024 * 1024
				builder.Grow(size)
				chunk := "Some very large string chunk\n"
				for builder.Len() < size-len(chunk) {
					builder.WriteString(chunk)
				}
				return builder.String()
			}(),
			wantLen: 8,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			murmur := NewMurmurHash(10)
			hashToken, err := murmur.Generate8CharHash(tt.input, 0)
			if err != nil {
				t.Fatalf("Generate8CharHash(%q) unexpected error: %v", tt.input, err)
			}

			if got := len(hashToken); got != tt.wantLen {
				t.Errorf("len(Generate8CharHash(%q)) = %d, want %d", tt.input, got, tt.wantLen)
			}
		})
	}
}

// TestMurmurHash_GenerateDifferentTokenOnCollision verifies hash determinism and seed-based collision resolution.
func TestMurmurHash_GenerateDifferentTokenOnCollision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input1        string
		input2        string
		seed1         uint32
		seed2         uint32
		wantDifferent bool
	}{
		{
			name:          "Deterministic Output Same Seed",
			input1:        "https://www.google.com",
			input2:        "https://www.google.com",
			seed1:         0,
			seed2:         0,
			wantDifferent: false,
		},
		{
			name:          "Distinct Output Different Seed",
			input1:        "https://www.google.com",
			input2:        "https://www.google.com",
			seed1:         0,
			seed2:         1,
			wantDifferent: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			murmur := NewMurmurHash(10)

			hashToken1, err := murmur.Generate8CharHash(tt.input1, tt.seed1)
			if err != nil {
				t.Fatalf("Generate8CharHash(%q, %d) unexpected error: %v", tt.input1, tt.seed1, err)
			}

			hashToken2, err := murmur.Generate8CharHash(tt.input2, tt.seed2)
			if err != nil {
				t.Fatalf("Generate8CharHash(%q, %d) unexpected error: %v", tt.input2, tt.seed2, err)
			}

			gotDifferent := hashToken1 != hashToken2
			if gotDifferent != tt.wantDifferent {
				t.Errorf("hash comparison: got different=%v, want different=%v (token1=%q, token2=%q)",
					gotDifferent, tt.wantDifferent, hashToken1, hashToken2)
			}
		})
	}
}
