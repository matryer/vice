package emitter

import (
	"fmt"
	"strings"
	"time"

	eio "github.com/emitter-io/go/v2"
)

const groupName = "vice"

func (t *Transport) makeSubscriber(name string) (chan []byte, error) {
	c, err := t.newClient()
	if err != nil {
		return nil, err
	}

	channelName := fmt.Sprintf("$share/%v/%v", groupName, name)
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

	if err := c.SubscribeWithGroup(key, name, groupName, f, eio.WithoutEcho()); err != nil {
		return nil, fmt.Errorf("emitter.Subscribe(%q): %w", name, err)
	}

	return ch, nil
}
