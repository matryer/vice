// Package rabbitmq provides a implementation for RabbitMQ transport.
package rabbitmq

import (
	"sync"

	"github.com/matryer/vice"
	"github.com/streadway/amqp"
)

//DefaultAddr is connection string used to connect to local RabbitMQ instance on default port.
const DefaultAddr = "amqp://guest:guest@localhost:5672/"

// Transport is a vice.Transport for RabbitMQ.
type Transport struct {
	sendChans    map[string]chan []byte
	receiveChans map[string]chan []byte

	sync.Mutex
	wg sync.WaitGroup

	errChan     chan error
	stopchan    chan struct{}
	stopPubChan chan struct{}
	stopSubChan chan struct{}

	connection *amqp.Connection

	// exported fields
	Addr string
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
		connection:   options.Connection,
	}
}

func (t *Transport) newConnection() (*amqp.Connection, error) {
	var err error
	if t.connection != nil {
		return t.connection, nil
	}

	conn, err := amqp.Dial(DefaultAddr)
	if err != nil {
		return nil, err
	}

	t.connection = conn
	return t.connection, err
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

	rmqch, err := c.Channel()
	if err != nil {
		return nil, err
	}

	q, err := rmqch.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when used
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	// fair load balancing messages on queue when there are multiple receivers
	err = rmqch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return nil, err
	}

	msgs, err := rmqch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return nil, err
	}

	ch := make(chan []byte, 1024)
	go func() {
		defer rmqch.Close()
		for {
			select {
			case d := <-msgs:
				ch <- d.Body
				d.Ack(false)
			case <-t.stopSubChan:
				return
			}
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

	rmqch, err := c.Channel()
	if err != nil {
		return nil, err
	}

	q, err := rmqch.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	ch := make(chan []byte, 1024)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer rmqch.Close()
		for {
			select {
			case <-t.stopPubChan:
				if len(ch) != 0 {
					continue
				}
				return
			case msg := <-ch:
				err = rmqch.Publish(
					"",     // exchange
					q.Name, // routing key
					false,  // mandatory
					false,  // immediate
					amqp.Publishing{
						DeliveryMode: amqp.Persistent,
						ContentType:  "text/plain",
						Body:         msg,
					})
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
	t.connection.Close()
	close(t.stopchan)
}

// Done gets a channel which is closed when the
// transport has successfully stopped.
func (t *Transport) Done() chan struct{} {
	return t.stopchan
}
