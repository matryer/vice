package emitter

import (
	"errors"
	"fmt"

	eio "github.com/emitter-io/go/v2"
)

func (t *Transport) newClient() (*eio.Client, error) {
	if t.emitterAddress == "" {
		return nil, errors.New("missing emitter address")
	}
	if t.secretKey == "" {
		return nil, errors.New("missing emitter secret key")
	}

	c := eio.NewClient(eio.WithBrokers(t.emitterAddress), eio.WithAutoReconnect(true))
	if err := c.Connect(); err != nil {
		return nil, fmt.Errorf("emitter.Connect(%q): %w - is the emitter service running?", t.emitterAddress, err)
	}
	return c, nil
}
