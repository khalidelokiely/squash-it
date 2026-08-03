package hash

import (
	"fmt"

	"github.com/twmb/murmur3"
)

type Hasher interface {
	// Generate8CharHash generates a hash that is exactly of length chars
	Generate8CharHash(input string, seed uint32) (result string, err error)
}

// MurmurHash why? https://mojoauth.com/compare-hashing-algorithms/fnv-1a-vs-murmurhash#popular-use-cases-of-murmurhash
type MurmurHash struct {
	seed uint32
}

func NewMurmurHash(seed uint32) *MurmurHash {
	return &MurmurHash{
		seed: seed,
	}
}

func (h *MurmurHash) Generate8CharHash(input string, seed uint32) (result string, err error) {
	if seed != 0 {
		seed += h.seed
	} else {
		seed = h.seed
	}

	hasher := murmur3.SeedNew32(seed)
	_, err = hasher.Write([]byte(input))

	if err != nil {
		return "", err
	}

	hashInt := hasher.Sum32()

	hashToken := fmt.Sprintf("%08x", hashInt)

	if len(hashToken) > 8 {
		hashToken = hashToken[len(hashToken)-8:]
	}

	return hashToken, nil
}
