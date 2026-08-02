package router

import "time"

type Options struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// Add more
}

type OptionMutatingFunction func(o *Options)

type Option struct {
	F OptionMutatingFunction
}

// WithHostPorts option allows changing default listening port
// TODO: Add checks to the hostPort string
func WithHostPorts(hostPort string) Option {
	return Option{func(o *Options) {
		if hostPort == "" {
			return
		}
		o.Port = ":" + o.Port
	}}
}

// WithReadTimeout option allows changing duration of read timeout
func WithReadTimeout(duration time.Duration) Option {
	return Option{func(o *Options) {
		o.ReadTimeout = duration
	}}
}

// WithWriteTimeout option allows changing duration of write timeout
func WithWriteTimeout(duration time.Duration) Option {
	return Option{func(o *Options) {
		o.WriteTimeout = duration
	}}
}
