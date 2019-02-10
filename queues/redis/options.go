package redis

import "github.com/go-redis/redis"

type Options struct {
	Client *redis.Client
}

type Option func(*Options)

func WithClient(c *redis.Client) Option {
	return func(o *Options) {
		o.Client = c
	}
}
