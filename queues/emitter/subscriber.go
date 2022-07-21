package emitter

import (
	"fmt"
	"strings"

	eio "github.com/emitter-io/go/v2"
)

func (t *Transport) makeSubscriber(name string) (chan []byte, error) {
	if t.c == nil {
		if err := t.newClient(); err != nil {
			return nil, err
		}
	}

	channelName := name
	if !strings.HasSuffix(channelName, "/") {
		channelName += "/" // emitter channel names end with a slash.
	}

	key, err := t.c.GenerateKey(t.secretKey, channelName, "r", t.ttl)
	if err != nil {
		return nil, fmt.Errorf("emitter.GenerateKey(%q,'r',%v): %w", channelName, t.ttl, err)
	}

	msgs := make(chan []byte, 1024)
	ch := make(chan []byte)
	go func() {
		// defer func() {
		// if err := t.c.Unsubscribe(key, name); err != nil {
		// 	t.errChan <- &vice.Err{Message: []byte("Unsubscribe failed"), Name: name, Err: err}
		// }
		// close(msgs)
		// }()
		for {
			select {
			case d := <-msgs:
				fmt.Printf("recv: channel=%q, msg=%s\n", name, d)
				ch <- d
			case <-t.stopSubChan:
				return
			}
		}
	}()
	f := func(_ *eio.Client, msg eio.Message) {
		fmt.Printf("RECV: channel=%q, topic=%q, msg=%s\n", name, msg.Topic(), msg.Payload())
		msgs <- msg.Payload()
	}

	if err := t.c.Subscribe(key, name, f, eio.WithAtMostOnce()); err != nil {
		return nil, fmt.Errorf("emitter.Subscribe(%q): %w", name, err)
	}

	return ch, nil
}
