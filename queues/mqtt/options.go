package mqtt

import (
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

type Options struct {
	ClientOptions *mqtt.ClientOptions
	SubQoS        byte
	PubQoS        byte
	PubRetained   bool

	SubTimeout time.Duration
	PubTimeout time.Duration
}

type Option func(*Options)

func WithClientOptions(c *mqtt.ClientOptions) Option {
	return func(o *Options) {
		o.ClientOptions = c
	}
}

func WithPub(qos byte, retained bool, timeout time.Duration) Option {
	return func(o *Options) {
		o.PubQoS = qos
		o.PubRetained = retained
		o.PubTimeout = timeout
	}
}

func WithSub(qos byte, timeout time.Duration) Option {
	return func(o *Options) {
		o.SubQoS = qos
		o.SubTimeout = timeout
	}
}
