package nats

import "github.com/nats-io/go-nats"

type Options struct {
	Conn *nats.Conn
}

type Option func(*Options)

func WithConnection(c *nats.Conn) Option {
	return func(o *Options) {
		o.Conn = c
	}
}
