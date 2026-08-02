package main

import (
	"fmt"

	"github.com/twmb/murmur3"
)

type Hasher interface {
	// GenerateNCharHash generates a hash that is exactly of length chars
	// TODO: Change function signature once the hash length is settled
	GenerateNCharHash(chars int, input string, seed uint32) (result string, lastSeed uint32)
}

type MurmurHash struct {
	seed uint32
}

func NewMurmurHash(seed uint32) *MurmurHash {
	return &MurmurHash{
		seed: seed,
	}
}

func (h *MurmurHash) GenerateNCharHash(chars int, input string, seed uint32) (result string, lastSeed uint32) {
	if seed != 0 {
		seed += h.seed
	} else {
		seed = h.seed
	}

	hasher := murmur3.SeedNew32(seed)
	_, err := hasher.Write([]byte(input))

	if err != nil {
		return "", seed
	}

	hashInt := hasher.Sum32()

	hashToken := fmt.Sprintf("%0*x", chars, hashInt)

	if len(hashToken) > chars {
		hashToken = hashToken[len(hashToken)-chars:]
	}

	return hashToken, seed
}
