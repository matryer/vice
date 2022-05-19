package redis

import "github.com/go-redis/redis"

// Options is the configurations struct for the Redis transport
type Options struct {
	Client *redis.Client
}

// Option are functions used to setup the Options configuration struct
type Option func(*Options)

// WithClient configures Redis transport to use the given client
func WithClient(c *redis.Client) Option {
	return func(o *Options) {
		o.Client = c
	}
}
