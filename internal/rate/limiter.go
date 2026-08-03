package rate

type Limiter interface {
	Allow(user string) bool
}
