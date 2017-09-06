package redis

import redis "gopkg.in/redis.v5"

type Options struct {
	Client *redis.Client
}

type Option func(*Options)

func WithClient(c *redis.Client) Option {
	return func(o *Options) {
		o.Client = c
	}
}
