// Package redis provides a Vice implementation for REDIS.
package redis

import (
	"sync"
	"time"

	"github.com/matryer/vice"
	"gopkg.in/redis.v3"
)

// Transport is a vice.Transport for redis.
type Transport struct {
	sendChans    map[string]chan []byte
	receiveChans map[string]chan []byte

	sync.Mutex
	wg sync.WaitGroup

	errChan     chan error
	stopchan    chan struct{}
	stopPubChan chan struct{}
	stopSubChan chan struct{}

	client *redis.Client
}

// New returns a new transport
func New(opts ...Option) *Transport {
	var options Options
	for _, o := range opts {
		o(&options)
	}

	return &Transport{
		sendChans:    make(map[string]chan []byte),
		receiveChans: make(map[string]chan []byte),
		errChan:      make(chan error, 10),
		stopchan:     make(chan struct{}),
		stopPubChan:  make(chan struct{}),
		stopSubChan:  make(chan struct{}),
		client:       options.Client,
	}
}

func (t *Transport) newConnection() (*redis.Client, error) {
	var err error
	if t.client != nil {
		return t.client, nil
	}

	t.client = redis.NewClient(&redis.Options{
		Network:    "tcp",
		Addr:       "127.0.0.1:6379",
		Password:   "",
		DB:         0,
		MaxRetries: 0,
	})

	// test connection
	_, err = t.client.Ping().Result()
	return t.client, err
}

// Receive gets a channel on which to receive messages
// with the specified name.
func (t *Transport) Receive(name string) <-chan []byte {
	t.Lock()
	defer t.Unlock()

	ch, ok := t.receiveChans[name]
	if ok {
		return ch
	}

	ch, err := t.makeSubscriber(name)
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	t.receiveChans[name] = ch
	return ch
}

func (t *Transport) makeSubscriber(name string) (chan []byte, error) {
	c, err := t.newConnection()
	if err != nil {
		return nil, err
	}

	ch := make(chan []byte, 1024)
	go func() {
		for {
			data, err := c.BRPop(0*time.Second, name).Result()
			if err != nil {
				select {
				case <-t.stopSubChan:
					return
				default:
					t.errChan <- vice.Err{Name: name, Err: err}
					continue
				}
			}

			ch <- []byte(data[len(data)-1])
		}
	}()
	return ch, nil
}

// Send gets a channel on which messages with the
// specified name may be sent.
func (t *Transport) Send(name string) chan<- []byte {
	t.Lock()
	defer t.Unlock()

	ch, ok := t.sendChans[name]
	if ok {
		return ch
	}

	ch, err := t.makePublisher(name)
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}
	t.sendChans[name] = ch
	return ch
}

func (t *Transport) makePublisher(name string) (chan []byte, error) {
	c, err := t.newConnection()
	if err != nil {
		return nil, err
	}

	ch := make(chan []byte, 1024)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.stopPubChan:
				if len(ch) != 0 {
					_, err := t.client.Ping().Result()
					if err == nil {
						continue
					}
				}
				return
			case msg := <-ch:
				err := c.RPush(name, string(msg)).Err()
				if err != nil {
					t.errChan <- vice.Err{Message: msg, Name: name, Err: err}
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
	close(t.stopSubChan)
	close(t.stopPubChan)
	t.wg.Wait()
	t.client.Close()
	close(t.stopchan)
}

// Done gets a channel which is closed when the
// transport has successfully stopped.
func (t *Transport) Done() chan struct{} {
	return t.stopchan
}
