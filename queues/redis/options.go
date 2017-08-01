package redis

import "gopkg.in/redis.v3"

type Options struct {
	Client *redis.Client
}

type Option func(*Options)

func WithClient(c *redis.Client) Option {
	return func(o *Options) {
		o.Client = c
	}
}
