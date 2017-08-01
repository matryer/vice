// Package redis provides a Vice implementation for REDIS.
package redis

import (
	"sync"

	"github.com/matryer/vice"
	"gopkg.in/redis.v3"
)

// Transport is a vice.Transport for redis.
type Transport struct {
	sm        sync.Mutex
	sendChans map[string]chan []byte

	rm           sync.Mutex
	receiveChans map[string]chan []byte

	errChan     chan error
	stopchan    chan struct{}
	stopPubChan chan struct{}
	stopSubChan chan struct{}

	clients []*redis.Client

	NewClient func() *redis.Client
}

// New returns a new transport
func New() *Transport {
	return &Transport{
		sendChans:    make(map[string]chan []byte),
		receiveChans: make(map[string]chan []byte),
		errChan:      make(chan error, 10),
		stopchan:     make(chan struct{}),
		stopPubChan:  make(chan struct{}),
		stopSubChan:  make(chan struct{}),
		clients:      []*redis.Client{},

		NewClient: func() *redis.Client {
			return redis.NewClient(&redis.Options{
				Network:    "tcp",
				Addr:       "127.0.0.1:6379",
				Password:   "",
				DB:         0,
				MaxRetries: 0,
			})
		},
	}
}

func (t *Transport) newConnection() (*redis.Client, error) {
	c := t.NewClient()
	// test connection
	_, err := c.Ping().Result()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Receive gets a channel on which to receive messages
// with the specified name.
func (t *Transport) Receive(name string) <-chan []byte {
	t.rm.Lock()
	defer t.rm.Unlock()

	ch, ok := t.receiveChans[name]
	if ok {
		return ch
	}

	c, err := t.newConnection()
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	ch = t.makeSubscriber(c, name)

	t.clients = append(t.clients, c)
	t.receiveChans[name] = ch
	return ch
}

func (t *Transport) makeSubscriber(c *redis.Client, name string) chan []byte {
	ch := make(chan []byte, 1024)
	ps, err := c.Subscribe(name)
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	go func() {
		for {
			select {
			case <-t.stopSubChan:
				return
			default:
				m, err := ps.ReceiveMessage()
				if err != nil {
					t.errChan <- vice.Err{Name: name, Err: err}
					continue
				}
				data := []byte(m.Payload)
				ch <- data
			}
		}
	}()
	return ch
}

// Send gets a channel on which messages with the
// specified name may be sent.
func (t *Transport) Send(name string) chan<- []byte {
	t.sm.Lock()
	defer t.sm.Unlock()

	ch, ok := t.sendChans[name]
	if ok {
		return ch
	}

	c, err := t.newConnection()
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	ch, err = t.makePublisher(c, name)
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}
	t.sendChans[name] = ch
	return ch
}

func (t *Transport) makePublisher(c *redis.Client, name string) (chan []byte, error) {
	ch := make(chan []byte, 1024)

	go func() {
		for {
			select {
			case <-t.stopPubChan:
				return

			case msg := <-ch:
				err := c.Publish(name, string(msg)).Err()
				if err != nil {
					t.errChan <- vice.Err{Message: msg, Name: name, Err: err}
					continue
				}
			}
		}
	}()
	return ch, nil
}

// ErrChan gets the channel on which errors are sent.
func (t *Transport) ErrChan() <-chan error {
	return t.errChan
}

// Stop stops the transport.
// The channel returned from Done() will be closed
// when the transport has stopped.
func (t *Transport) Stop() {
	for _, c := range t.clients {
		c.Close()
	}
	close(t.stopSubChan)
	close(t.stopPubChan)
	close(t.stopchan)
}

// Done gets a channel which is closed when the
// transport has successfully stopped.
func (t *Transport) Done() chan struct{} {
	return t.stopchan
}
