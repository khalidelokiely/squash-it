package rate

type Limiter interface {
	// Allow Checks if user has tokens in their bucket
	Allow(user string) bool
}
