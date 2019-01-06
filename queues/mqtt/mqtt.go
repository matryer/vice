package mqtt

import (
	"sync"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/matryer/vice"
)

const (
	DefaultMQTTAddress = "tcp://127.0.0.1:1883"

	SharedQueuePrefix = "$share/vice/"
)

// make sure Transport satisfies vice.Transport interface.
var _ vice.Transport = (*Transport)(nil)

type Transport struct {
	sync.Mutex
	wg sync.WaitGroup

	clientOptions *mqtt.ClientOptions
	subQoS        byte
	pubQoS        byte
	pubRetained   bool
	subTimeout    time.Duration
	pubTimeout    time.Duration

	subClients map[string]mqtt.Client

	pubChans map[string]chan []byte
	subChans map[string]chan []byte

	errChan     chan error
	stopChan    chan struct{}
	stopPubChan chan struct{}
}

func New(opts ...Option) *Transport {
	var options Options
	for _, o := range opts {
		o(&options)
	}

	if options.ClientOptions == nil {
		options.ClientOptions = mqtt.NewClientOptions()
		options.ClientOptions.AddBroker(DefaultMQTTAddress)
	}

	if options.PubTimeout == 0 {
		options.PubTimeout = time.Second
	}
	if options.SubTimeout == 0 {
		options.SubTimeout = time.Second
	}

	return &Transport{
		clientOptions: options.ClientOptions,

		subQoS:     options.SubQoS,
		subTimeout: options.SubTimeout,

		pubQoS:      options.PubQoS,
		pubRetained: options.PubRetained,
		pubTimeout:  options.PubTimeout,

		subClients: make(map[string]mqtt.Client),

		pubChans: make(map[string]chan []byte),
		subChans: make(map[string]chan []byte),

		errChan:     make(chan error, 10),
		stopChan:    make(chan struct{}),
		stopPubChan: make(chan struct{}),
	}
}

func (t *Transport) Receive(name string) <-chan []byte {
	t.Lock()
	defer t.Unlock()

	subCh, ok := t.subChans[name]
	if ok {
		return subCh
	}

	subCh, err := t.makeSubscriber(name)
	if err != nil {
		t.errChan <- &vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	t.subChans[name] = subCh
	return subCh
}

func (t *Transport) makeSubscriber(topic string) (chan []byte, error) {
	ch := make(chan []byte, 1024)

	cli := mqtt.NewClient(t.clientOptions)
	if token := cli.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	if token := cli.Subscribe(SharedQueuePrefix+topic, t.subQoS, func(c mqtt.Client, msg mqtt.Message) {
		ch <- msg.Payload()
	}); token.WaitTimeout(t.subTimeout) && token.Error() != nil {
		return nil, token.Error()
	}

	t.subClients[topic] = cli

	return ch, nil
}

func (t *Transport) Send(name string) chan<- []byte {
	t.Lock()
	defer t.Unlock()

	pubCh, ok := t.pubChans[name]
	if ok {
		return pubCh
	}

	pubCh, err := t.makePublisher(name)
	if err != nil {
		t.errChan <- &vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	t.pubChans[name] = pubCh
	return pubCh
}

func (t *Transport) makePublisher(topic string) (chan []byte, error) {

	ch := make(chan []byte, 1024)

	cli := mqtt.NewClient(t.clientOptions)
	if token := cli.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.stopPubChan:
				cli.Disconnect(100)
				return
			case msg := <-ch:
				if token := cli.Publish(topic, t.pubQoS, t.pubRetained, msg); token.WaitTimeout(t.pubTimeout) && token.Error() != nil {
					t.errChan <- &vice.Err{Name: topic, Err: token.Error()}
				}
			}
		}
	}()

	return ch, nil
}

func (t *Transport) ErrChan() <-chan error {
	return t.errChan
}

func (t *Transport) Stop() {
	t.Lock()
	defer t.Unlock()

	for topic, cli := range t.subClients {
		if token := cli.Unsubscribe(SharedQueuePrefix + topic); token.Wait() && token.Error() != nil {
			t.errChan <- &vice.Err{Name: topic, Err: token.Error()}
		}
		cli.Disconnect(100)
	}

	close(t.stopPubChan)
	t.wg.Wait()

	close(t.stopChan)
}

func (t *Transport) Done() chan struct{} {
	return t.stopChan
}
