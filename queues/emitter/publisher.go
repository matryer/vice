package emitter

import (
	"fmt"
	"strings"
	"time"

	eio "github.com/emitter-io/go/v2"
	"github.com/matryer/vice/v2"
)

func (t *Transport) makePublisher(name string) (chan []byte, error) {
	c, err := t.newClient()
	if err != nil {
		return nil, err
	}

	channelName := name
	if !strings.HasSuffix(channelName, "/") {
		channelName += "/" // emitter channel names end with a slash.
	}

	key, err := c.GenerateKey(t.secretKey, channelName, "w", t.ttl)
	if err != nil {
		return nil, fmt.Errorf("emitter.GenerateKey(%q,'w',%v): %w", channelName, t.ttl, err)
	}

	ch := make(chan []byte)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.stopPubChan:
				// uncomment the following code if using buffered channel
				/*
					if len(ch) != 0 {
						continue
					}
				*/
				c.Disconnect(100 * time.Millisecond)
				return
			case msg := <-ch:
				fmt.Printf("send: channel=%q, msg=%s\n", name, msg)
				if err := c.Publish(key, name, msg, eio.WithoutEcho()); err != nil {
					t.errChan <- &vice.Err{Message: msg, Name: name, Err: err}
				}
			}
		}
	}()
	return ch, nil
}
