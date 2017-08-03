package nats

import "github.com/nats-io/go-nats"

// Options can be used to create a customized transport.
type Options struct {
	Conn               *nats.Conn
	StreamingClusterID string
	StreamingClientID  string
	UseStreaming       bool
}

// Option is a function on the options for a nats transport.
type Option func(*Options)

// WithConnection is an Option to set underlying nats connection.
func WithConnection(c *nats.Conn) Option {
	return func(o *Options) {
		o.Conn = c
	}
}

// WithStreaming is an Option to activate nats streaming for the transport.
func WithStreaming(clusterID, clientID string) Option {
	return func(o *Options) {
		o.UseStreaming = true
		o.StreamingClientID = clientID
		o.StreamingClusterID = clusterID
	}
}
