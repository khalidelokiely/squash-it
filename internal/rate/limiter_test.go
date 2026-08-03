package rate

import (
	"testing"
	"time"
)

type InspectableUserTokenBucket struct {
	*UserTokenBucket
}

func NewInspectableUserTokenBucket(rpm, burst int, ttl time.Duration) *InspectableUserTokenBucket {
	return &InspectableUserTokenBucket{
		UserTokenBucket: NewUserTokenBucket(rpm, burst, ttl),
	}
}

func (i *InspectableUserTokenBucket) CheckUserExists(user string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, ok := i.userTokens[user]
	return ok
}

func newTestLimiter(t testing.TB, rpm, burst int, ttl time.Duration) *InspectableUserTokenBucket {
	t.Helper()
	return NewInspectableUserTokenBucket(rpm, burst, ttl)
}

func TestLimiter_LazyCleanupHappens(t *testing.T) {
	t.Parallel()

	t.Run("Lazy Clean Up Should Expire User Tokens After TTL", func(t *testing.T) {
		t.Parallel()

		ttl := 100 * time.Millisecond
		limiter := newTestLimiter(t, 5, 5, ttl)

		limiter.Allow("test_user_1")

		if !limiter.CheckUserExists("test_user_1") {
			t.Fatalf("expected test_user_1 to exist after Allow()")
		}

		time.Sleep(ttl + 1*time.Millisecond)

		limiter.Allow("test_user_2")

		time.Sleep(ttl + 1*time.Millisecond)

		if limiter.CheckUserExists("test_user_1") {
			t.Fatalf("expected test_user_1 to be evicted after TTL expiration")
		}
	})
}
