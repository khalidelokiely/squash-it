package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"squash-it/internal/hash"
	"testing"
)

var hasher = hash.NewMurmurHash(32)

func TestURLService_Invariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                                string
		action                              func(svc *URLService) error
		repoAction                          func(repo *MockRepository)
		bloomExists                         bool
		cacheHit                            bool
		dbHit                               bool
		wantErr                             error
		wantBloomAdded                      bool
		wantCreateCalls                     int
		wantFindByPathHashCalls             int
		wantFindByLongURLCalls              int
		wantUpdateClickCountByPathHashCalls int
		wantCacheSetCalled                  bool
		wantCacheGetCalled                  int
	}{
		{
			name: "[Exhaust Trial] Stop Generating Hash on Collision After Max Attempts",
			action: func(svc *URLService) error {
				_, err := svc.CreateURL(context.Background(), "https://google.com")
				return err
			},
			repoAction: func(repo *MockRepository) {
				repo.FindByPathHashFn = func(ctx context.Context, pathHash string) (*URL, error) {
					return &URL{
						PathHash: pathHash,
						LongURL:  "https://already-taken-by-someone-else.com",
					}, nil
				}
			},
			bloomExists:             true,
			cacheHit:                false,
			dbHit:                   true,
			wantErr:                 fmt.Errorf("failed to generate hash for url after %d attempts", collisionRetryAttempts),
			wantBloomAdded:          false,
			wantCreateCalls:         0,
			wantFindByPathHashCalls: int(collisionRetryAttempts),
			wantFindByLongURLCalls:  0,
			wantCacheGetCalled:      int(collisionRetryAttempts),
		},
		{
			name: "Add Key to Filter on Successful Creation",
			action: func(svc *URLService) error {
				_, err := svc.CreateURL(context.Background(), "https://www.google.com")
				return err
			},
			bloomExists:        false,
			cacheHit:           false,
			dbHit:              false,
			wantErr:            nil,
			wantBloomAdded:     true,
			wantCreateCalls:    1,
			wantCacheSetCalled: true,
		},
		{
			name: "Short Circuit Database Read When Bloom Filter Misses",
			action: func(svc *URLService) error {
				_, err := svc.GetURLFromHash(context.Background(), "squashio")
				return err
			},
			bloomExists:             false,
			wantErr:                 ErrUnknownHash,
			wantBloomAdded:          false,
			wantFindByPathHashCalls: 0,
			wantCacheSetCalled:      false,
		},
		{
			name: "Warm Cache On Cache Miss Upon Database Hit",
			action: func(svc *URLService) error {
				_, err := svc.GetURLFromHash(context.Background(), "squashio")
				return err
			},
			repoAction: func(repo *MockRepository) {
				repo.FindByPathHashFn = func(ctx context.Context, hashToken string) (*URL, error) {
					return &URL{
						PathHash: "squashio",
						LongURL:  "https://www.google.com",
					}, nil
				}
			},
			bloomExists:             true,
			cacheHit:                false,
			dbHit:                   true,
			wantErr:                 nil,
			wantBloomAdded:          false,
			wantFindByPathHashCalls: 1,
			wantCacheSetCalled:      true,
			wantCacheGetCalled:      1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.action == nil {
				t.Fatal("test case missing action function")
			}

			filter := NewMockFilter()
			filter.ExistsValue = tt.bloomExists

			repo := &MockRepository{}
			if tt.repoAction != nil {
				tt.repoAction(repo)
			}
			pipeline := NewMockPipeline()

			svc := NewURLService(repo, pipeline, filter, hasher)

			err := tt.action(svc)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if tt.wantFindByPathHashCalls != repo.FindByPathHashCalls {
				t.Errorf("wanted %v FindByPathHash calls, got %v", tt.wantFindByPathHashCalls, repo.FindByPathHashCalls)
			}

			if tt.wantFindByLongURLCalls != repo.FindByLongURLCalls {
				t.Errorf("wanted %v FindByLongURL calls, got %v", tt.wantFindByLongURLCalls, repo.FindByLongURLCalls)
			}

			if tt.wantCreateCalls != repo.CreateCalls {
				t.Errorf("wanted %v Create calls, got %v", tt.wantCreateCalls, repo.CreateCalls)
			}

			if tt.wantUpdateClickCountByPathHashCalls != repo.UpdateClickCountByPathHashCalls {
				t.Errorf("wanted %v UpdateClickCountByPathHash calls, got %v", tt.wantUpdateClickCountByPathHashCalls, repo.UpdateClickCountByPathHashCalls)
			}

			if tt.wantBloomAdded && len(filter.AddedKeys) == 0 {
				t.Errorf("expected key to be added to Bloom filter, but none was added")
			}

			if tt.wantCacheSetCalled && len(pipeline.SetCalls) == 0 {
				t.Errorf("expected key to be cached, but pipeline Set() was not called")
			}

			if pipeline.GetCalls != tt.wantCacheGetCalled {
				t.Errorf("wanted %v pipeline Get calls, got %v", tt.wantCacheGetCalled, pipeline.GetCalls)
			}
		})
	}
}

type MockFilter struct {
	AddedKeys   []string
	ExistsValue bool
	ExistsMap   map[string]bool
}

func NewMockFilter() *MockFilter {
	return &MockFilter{
		AddedKeys: make([]string, 0),
		ExistsMap: make(map[string]bool),
	}
}

func (f *MockFilter) Serialize() ([]byte, error) {
	panic("implement me")
}

func (f *MockFilter) WriteTo(writer io.Writer) (int64, error) {
	panic("implement me")
}

func (f *MockFilter) Exists(key string) bool {
	if val, ok := f.ExistsMap[key]; ok {
		return val
	}
	return f.ExistsValue
}

func (f *MockFilter) Add(key string) {
	f.AddedKeys = append(f.AddedKeys, key)
}

func (f *MockFilter) WasAdded(key string) bool {
	for _, k := range f.AddedKeys {
		if k == key {
			return true
		}
	}
	return false
}

type MockRepository struct {
	FindByPathHashCalls             int
	FindByLongURLCalls              int
	CreateCalls                     int
	UpdateClickCountByPathHashCalls int

	FindByPathHashFn func(ctx context.Context, hashToken string) (*URL, error)
	CreateFn         func(ctx context.Context, url *URL) error
}

func (m *MockRepository) FindByLongURL(ctx context.Context, longURL string) (*URL, error) {
	m.FindByLongURLCalls++
	return nil, ErrNotFound
}

func (m *MockRepository) UpdateClickCountByPathHash(ctx context.Context, pathHash string) error {
	m.UpdateClickCountByPathHashCalls++
	return nil
}

func (m *MockRepository) FindByPathHash(ctx context.Context, pathHash string) (*URL, error) {
	m.FindByPathHashCalls++
	if m.FindByPathHashFn != nil {
		return m.FindByPathHashFn(ctx, pathHash)
	}
	return nil, ErrNotFound
}

func (m *MockRepository) Create(ctx context.Context, url *URL) error {
	m.CreateCalls++
	if m.CreateFn != nil {
		return m.CreateFn(ctx, url)
	}
	return nil
}

type MockPipeline struct {
	GetCalls int
	SetCalls map[string]string

	GetFn func(ctx context.Context, key string) (string, bool, error)
}

func NewMockPipeline() *MockPipeline {
	return &MockPipeline{
		SetCalls: make(map[string]string),
	}
}

func (p *MockPipeline) Get(ctx context.Context, key string) (string, bool, error) {
	p.GetCalls++
	if p.GetFn != nil {
		return p.GetFn(ctx, key)
	}
	return "", false, nil
}

func (p *MockPipeline) Set(ctx context.Context, key, value string) error {
	p.SetCalls[key] = value
	return nil
}
