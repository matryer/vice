package rabbitmq

import "github.com/streadway/amqp"

// Options can be used to create a customized transport.
type Options struct {
	Connection *amqp.Connection
}

// Option is a function on the options for a RabbitMQ transport.
type Option func(*Options)

// WithConnection is an Option to set underlying RabbitMQ connection.
func WithConnection(c *amqp.Connection) Option {
	return func(o *Options) {
		o.Connection = c
	}
}
