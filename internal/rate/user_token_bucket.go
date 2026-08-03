package rate

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type tokenBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type UserTokenBucket struct {
	mu           sync.Mutex
	userTokens   map[string]*tokenBucket
	rate         rate.Limit
	burstLimit   int
	lastCleanUp  atomic.Int64
	isCleaningUp atomic.Int32
	cleanUpAfter time.Duration
}

func NewUserTokenBucket(ratePerMinute, burst int, cleanUpAfter time.Duration) *UserTokenBucket {
	limitPerSecond := rate.Limit(float64(ratePerMinute) / 60.0)

	utb := &UserTokenBucket{
		userTokens:   make(map[string]*tokenBucket),
		rate:         limitPerSecond,
		burstLimit:   burst,
		cleanUpAfter: cleanUpAfter,
	}

	utb.lastCleanUp.Store(time.Now().Unix())

	return utb
}

func (u *UserTokenBucket) Allow(userID string) bool {
	u.mu.Lock()

	bucket, ok := u.userTokens[userID]

	if !ok {
		bucket = &tokenBucket{
			limiter: rate.NewLimiter(u.rate, u.burstLimit),
		}

		u.userTokens[userID] = bucket
	}

	bucket.lastSeen = time.Now()
	allowed := bucket.limiter.Allow()

	u.mu.Unlock()

	lastCleanupUnix := u.lastCleanUp.Load()

	if time.Since(time.Unix(lastCleanupUnix, 0)) > u.cleanUpAfter {
		if u.isCleaningUp.CompareAndSwap(0, 1) {
			go u.cleanUp()
		}
	}

	return allowed
}

// cleanUp Lazy Async cleanup loop the decision came from complexity of most of the use cases.
// instead of spinning up a goroutine with a ticker we perform the check lazily so we don't exhaust
// CPU when idle.
func (u *UserTokenBucket) cleanUp() {
	defer u.isCleaningUp.Store(0)
	u.mu.Lock()
	defer u.mu.Unlock()

	lastCleanUpUnix := u.lastCleanUp.Load()
	// Cleaned up less than n duration ago
	if time.Since(time.Unix(lastCleanUpUnix, 0)) < u.cleanUpAfter {
		return
	}

	for userID, bucket := range u.userTokens {
		if time.Since(bucket.lastSeen) > u.cleanUpAfter {
			delete(u.userTokens, userID)
		}
	}

	u.lastCleanUp.Store(time.Now().Unix())
}
