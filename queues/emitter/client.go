package emitter

import (
	"errors"
	"fmt"

	eio "github.com/emitter-io/go/v2"
)

func (t *Transport) newClient() error {
	if t.emitterAddress == "" {
		return errors.New("missing emitter address")
	}
	if t.secretKey == "" {
		return errors.New("missing emitter secret key")
	}

	t.c = eio.NewClient(eio.WithBrokers(t.emitterAddress), eio.WithAutoReconnect(true))
	if err := t.c.Connect(); err != nil {
		t.c = nil
		return fmt.Errorf("emitter.Connect(%q): %w - is the emitter service running?", t.emitterAddress, err)
	}
	return nil
}
