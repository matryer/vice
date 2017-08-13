// Package nats provides a Vice implementation for NATS.
package nats

import (
	"sync"
	"time"

	"github.com/matryer/vice"
	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
)

// DefaultAddr is the NATS default TCP address.
const DefaultAddr = nats.DefaultURL

// make sure Transport satisfies vice.Transport interface.
var _ vice.Transport = (*Transport)(nil)

type unsubscriber interface {
	Unsubscribe() error
}

type publisher interface {
	Publish(subject string, data []byte) error
}

// Transport is a vice.Transport for NATS queue.
type Transport struct {
	sync.Mutex
	wg sync.WaitGroup

	receiveChans map[string]chan []byte
	sendChans    map[string]chan []byte

	errChan chan error
	// stopchan is closed when everything has stopped.
	stopchan    chan struct{}
	stopPubChan chan struct{}

	subscriptions      []unsubscriber
	natsConn           *nats.Conn
	natsStreamingConn  stan.Conn
	natsStreaming      bool
	streamingClusterID string
	streamingClientID  string

	// exported fields
	NatsAddr string
}

// New returns a new Transport
func New(opts ...Option) *Transport {
	var options Options
	for _, o := range opts {
		o(&options)
	}

	return &Transport{
		NatsAddr: DefaultAddr,

		receiveChans: make(map[string]chan []byte),
		sendChans:    make(map[string]chan []byte),

		errChan:     make(chan error, 10),
		stopchan:    make(chan struct{}),
		stopPubChan: make(chan struct{}),

		subscriptions: []unsubscriber{},

		natsConn:           options.Conn,
		natsStreaming:      options.UseStreaming,
		streamingClusterID: options.StreamingClusterID,
		streamingClientID:  options.StreamingClientID,
	}
}

func (t *Transport) newStreamingConnection() (stan.Conn, error) {
	var err error
	if t.natsStreamingConn != nil {
		return t.natsStreamingConn, nil
	}

	t.natsStreamingConn, err = stan.Connect(t.streamingClusterID, t.streamingClientID, stan.NatsConn(t.natsConn))
	return t.natsStreamingConn, err
}

func (t *Transport) newConnection() (*nats.Conn, error) {
	var err error
	if t.natsConn != nil {
		return t.natsConn, err
	}

	t.natsConn, err = nats.Connect(t.NatsAddr)
	return t.natsConn, err
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

	var sub unsubscriber
	if t.natsStreaming {
		var s stan.Conn
		s, err = t.newStreamingConnection()
		if err != nil {
			return nil, err
		}

		sub, err = s.QueueSubscribe(name, "vice-"+name, func(m *stan.Msg) {
			ch <- m.Data
		}, stan.DurableName("vice-"+name))
	} else {
		sub, err = c.QueueSubscribe(name, "vice-"+name, func(m *nats.Msg) {
			ch <- m.Data
		})
	}
	if err != nil {
		return nil, err
	}
	t.subscriptions = append(t.subscriptions, sub)
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
	var (
		c   publisher
		err error
	)

	c, err = t.newConnection()
	if err != nil {
		return nil, err
	}

	if t.natsStreaming {
		c, err = t.newStreamingConnection()
		if err != nil {
			return nil, err
		}
	}

	ch := make(chan []byte, 1024)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.stopPubChan:
				if len(ch) != 0 && t.natsConn.IsConnected() {
					continue
				}
				return
			case msg := <-ch:
				if err := c.Publish(name, msg); err != nil {
					t.errChan <- vice.Err{Message: msg, Name: name, Err: err}
					time.Sleep(1 * time.Second)
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
	t.Lock()
	defer t.Unlock()

	for _, s := range t.subscriptions {
		s.Unsubscribe()
	}

	close(t.stopPubChan)
	t.wg.Wait()

	if t.natsStreamingConn != nil {
		t.natsStreamingConn.Close()
	}

	t.natsConn.Flush()
	t.natsConn.Close()
	t.natsConn = nil

	close(t.stopchan)
}

// Done gets a channel which is closed when the
// transport has successfully stopped.
func (t *Transport) Done() chan struct{} {
	return t.stopchan
}
