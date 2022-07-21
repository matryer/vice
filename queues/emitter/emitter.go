// Package emitter provides a Vice implementation for Emitter.io.
package emitter

import (
	"log"
	"os"
	"sync"
	"time"

	eio "github.com/emitter-io/go/v2"
	"github.com/matryer/vice/v2"
)

const (
	defaultAddress = "tcp://127.0.0.1:8080"
	emitterEnvVar  = "EMITTER_SECRET_KEY"
)

// Transport is a vice.Transport for Emitter.io.
type Transport struct {
	secretKey    string
	sendChans    map[string]chan []byte
	receiveChans map[string]chan []byte

	sync.Mutex
	wg sync.WaitGroup

	errChan     chan error
	stopchan    chan struct{}
	stopPubChan chan struct{}
	stopSubChan chan struct{}

	c   *eio.Client
	ttl int

	emitterAddress string
}

// make sure Transport satisfies vice.Transport interface.
var _ vice.Transport = (*Transport)(nil)

// Option is a function that modifies a Transport.
type Option func(*Transport)

// New returns a new Transport.
// The environment variable "EMITTER_SECRET_KEY" is used for the secret key.
// The default emitter address is "tcp://127.0.0.1:8080".
func New(opts ...Option) *Transport {
	t := &Transport{
		secretKey:      os.Getenv(emitterEnvVar),
		sendChans:      make(map[string]chan []byte),
		receiveChans:   make(map[string]chan []byte),
		errChan:        make(chan error, 10),
		stopchan:       make(chan struct{}),
		stopPubChan:    make(chan struct{}),
		stopSubChan:    make(chan struct{}),
		emitterAddress: defaultAddress,
	}

	for _, o := range opts {
		o(t)
	}

	if t.secretKey == "" {
		log.Printf("WARNING: env var %q missing, calls will fail.", emitterEnvVar)
	}

	return t
}

// WithAddress overrides the default emitter.io address.
func WithAddress(address string) Option {
	return func(t *Transport) { t.emitterAddress = address }
}

// WithSecretKey overrides the emitter secret key.
// The default is to use the environment variable "EMITTER_SECRET_KEY".
func WithSecretKey(secretKey string) Option {
	return func(t *Transport) { t.secretKey = secretKey }
}

// WithTTL sets the ttl (time to live) value (in seconds) for each message.
// The default value is 0 which means it does not expire (although the
// default is that no messages are saved).
func WithTTL(ttl int) Option {
	return func(t *Transport) { t.ttl = ttl }
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
		t.errChan <- &vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	t.receiveChans[name] = ch
	return ch
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
		t.errChan <- &vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}
	t.sendChans[name] = ch
	return ch
}

// ErrChan gets a channel through which errors
// are sent.
func (t *Transport) ErrChan() <-chan error { return t.errChan }

// Stop stops the transport. The channel returned from Done() will be closed
// when the transport has stopped.
func (t *Transport) Stop() {
	close(t.stopSubChan)
	close(t.stopPubChan)
	t.wg.Wait()
	t.c.Disconnect(100 * time.Millisecond)
	close(t.stopchan)
}

// Done gets a channel which is closed when the
// transport has successfully stopped.
func (t *Transport) Done() chan struct{} { return t.stopchan }
