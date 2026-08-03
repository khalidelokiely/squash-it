package hash

import (
	"strings"
	"testing"
)

func TestMurmurHash_Generate8CharHashExactly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		input        string
		expectedSize int
	}{
		{name: "Small String", input: "https://www.google.com", expectedSize: 8},
		{name: "Very Large String", input: func() string {
			var builder strings.Builder
			size := 100 * 1024 * 1024
			builder.Grow(size)
			chunk := "Some very large string chunk\n"
			for builder.Len() < size-len(chunk) {
				builder.WriteString(chunk)
			}

			// 3. Generate the final string safely with zero-copy
			return builder.String()

		}(), expectedSize: 8},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			murmur := NewMurmurHash(10)
			hashToken, _ := murmur.Generate8CharHash(tc.input, 0)

			if len(hashToken) != tc.expectedSize {
				t.Errorf("expected hash token length %d, got %d", tc.expectedSize, len(hashToken))
			}
		})
	}
}

func TestMurmurHash_GenerateDifferentTokenOnCollision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		input1            string
		input2            string
		seed1             uint32
		seed2             uint32
		expectedDifferent bool
	}{
		{
			name:   "Should Collide",
			input1: "https://www.google.com",
			input2: "https://www.google.com",
			seed1:  0, seed2: 0,
			expectedDifferent: false,
		},
		{
			name:   "Should Not Collide",
			input1: "https://www.google.com",
			input2: "https://www.google.com",
			seed1:  0, seed2: 1,
			expectedDifferent: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			murmur := NewMurmurHash(10)
			hashToken1, _ := murmur.Generate8CharHash(tc.input1, tc.seed1)
			hashToken2, _ := murmur.Generate8CharHash(tc.input2, tc.seed2)

			if tc.expectedDifferent && hashToken1 == hashToken2 {
				t.Errorf("expected different hash tokens, got %v and %v", hashToken1, hashToken2)
			}
		})
	}
}
