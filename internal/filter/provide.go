package filter

import "squash-it/internal/config"

func New(config config.Config) (Filter, error) {
	return nil, nil
}

func NewPersistentFromConfig(cfg config.Config) (*PersistentBloomFilter, error) {
	return NewPersistentBloomFilter(
		cfg.BloomFilePath,
		cfg.BloomCapacity,
		cfg.BloomSaveInterval)
}
