package nats

import (
	"sync"

	"github.com/matryer/vice"
	"github.com/nats-io/nats"
)

// DefaultAddr is the NATS default TCP address.
const DefaultAddr = nats.DefaultURL

// make sure Transport satisfies vice.Transport interface.
var _ vice.Transport = (*Transport)(nil)

// Transport is a vice.Transport for NATS queue.
type Transport struct {
	rm           sync.Mutex
	receiveChans map[string]chan []byte

	sm        sync.Mutex
	sendChans map[string]chan []byte

	errChan chan error
	// stopchan is closed when everything has stopped.
	stopchan chan struct{}
	//stopPubChan chan struct{}

	subscriptions []*nats.Subscription
	connections   []*nats.Conn

	// exported fields
	NatsAddr string
}

// New returns a new Transport
func New() *Transport {
	return &Transport{
		NatsAddr: DefaultAddr,

		receiveChans: make(map[string]chan []byte),
		sendChans:    make(map[string]chan []byte),

		errChan:  make(chan error, 10),
		stopchan: make(chan struct{}),

		subscriptions: []*nats.Subscription{},
		connections:   []*nats.Conn{},
	}
}

func (t *Transport) newConnection() (*nats.Conn, error) {
	return nats.Connect(t.NatsAddr)
}

// Receive gets a channel on which to receive messages
// with the specified name.
func (t *Transport) Receive(name string) <-chan []byte {
	t.rm.Lock()
	defer t.rm.Unlock()

	ch := make(chan []byte)
	ch, ok := t.receiveChans[name]
	if ok {
		return ch
	}

	ch, err := t.makeSubscriber(name)
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
	}

	t.receiveChans[name] = ch
	return ch
}

func (t *Transport) makeSubscriber(name string) (chan []byte, error) {
	ch := make(chan []byte)

	c, err := t.newConnection()
	if err != nil {
		return nil, err
	}

	sub, err := c.Subscribe(name, func(m *nats.Msg) {
		data := m.Data
		ch <- data
		return
	})
	if err != nil {
		return nil, err
	}
	t.connections = append(t.connections, c)
	t.subscriptions = append(t.subscriptions, sub)
	return ch, nil
}

// Send gets a channel on which messages with the
// specified name may be sent.
func (t *Transport) Send(name string) chan<- []byte {
	t.sm.Lock()
	defer t.sm.Unlock()

	ch := make(chan []byte)
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
	ch := make(chan []byte)
	c, err := t.newConnection()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case msg := <-ch:
				if err := c.Publish(name, msg); err != nil {
					t.errChan <- vice.Err{Message: msg, Name: name, Err: err}
					continue
				}
			default:
			}
		}
	}()
	t.connections = append(t.connections, c)
	return ch, nil
}

// ErrChan gets the channel on which errors are sent.
func (t *Transport) ErrChan() <-chan error {
	return t.errChan
}

// Stop stops the transport. StopChan will be closed
// when the transport has stopped.
func (t *Transport) Stop() {
	for _, s := range t.subscriptions {
		s.Unsubscribe()
	}
	for _, conn := range t.connections {
		conn.Close()
	}
	close(t.stopchan)
}

// StopChan gets a channel which is closed when the
// transport has successfully stopped.
func (t *Transport) StopChan() chan struct{} {
	return t.stopchan
}
