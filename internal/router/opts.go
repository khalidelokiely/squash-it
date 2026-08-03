package router

import (
	"strings"
	"time"
)

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
func WithHostPorts(port string) Option {
	return Option{func(o *Options) {
		if port == "" {
			return
		}
		o.Port = strings.TrimPrefix(port, ":") // Cleanly set the port
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
