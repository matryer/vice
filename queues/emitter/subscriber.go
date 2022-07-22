package emitter

import (
	"fmt"
	"strings"
	"time"

	eio "github.com/emitter-io/go/v2"
)

func (t *Transport) makeSubscriber(name string) (chan []byte, error) {
	c, err := t.newClient()
	if err != nil {
		return nil, err
	}

	channelName := name
	if t.sharedGroup != "" {
		channelName = fmt.Sprintf("$share/%v/%v", t.sharedGroup, name)
	}
	if !strings.HasSuffix(channelName, "/") {
		channelName += "/" // emitter channel names end with a slash.
	}

	key, err := c.GenerateKey(t.secretKey, channelName, "r", t.ttl)
	if err != nil {
		return nil, fmt.Errorf("emitter.GenerateKey(%q,'r',%v): %w", channelName, t.ttl, err)
	}

	msgs := make(chan []byte, 1024)
	ch := make(chan []byte)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case d := <-msgs:
				ch <- d
			case <-t.stopSubChan:
				c.Disconnect(100 * time.Millisecond)
				return
			}
		}
	}()
	f := func(_ *eio.Client, msg eio.Message) {
		msgs <- msg.Payload()
	}

	if t.sharedGroup != "" {
		if err := c.SubscribeWithGroup(key, name, t.sharedGroup, f, eio.WithoutEcho()); err != nil {
			return nil, fmt.Errorf("emitter.SubscribeWithGroup(%q): %w", name, err)
		}
	} else {
		if err := c.Subscribe(key, name, f, eio.WithoutEcho()); err != nil {
			return nil, fmt.Errorf("emitter.Subscribe(%q): %w", name, err)
		}
	}

	return ch, nil
}
