package nsq

import (
	"io/ioutil"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/matryer/vice"
	"github.com/matryer/vice/vicetest"
	"github.com/nsqio/go-nsq"
)

func newTestTransport() vice.Transport {
	transport := New()
	transport.NewProducer = func() (*nsq.Producer, error) {
		producer, err := nsq.NewProducer(DefaultTCPAddr, nsq.NewConfig())
		if err != nil {
			return nil, err
		}
		logger := log.New(ioutil.Discard, "logger: ", log.Lshortfile)
		producer.SetLogger(logger, nsq.LogLevelError)
		return producer, nil
	}
	transport.NewConsumer = func(name string) (*nsq.Consumer, error) {
		consumer, err := nsq.NewConsumer(name, "vice", nsq.NewConfig())
		if err != nil {
			return nil, err
		}
		logger := log.New(ioutil.Discard, "logger: ", log.Lshortfile)
		consumer.SetLogger(logger, nsq.LogLevelError)
		return consumer, nil
	}

	return transport
}

func TestTransport(t *testing.T) {
	vicetest.Transport(t, newTestTransport)
}

func TestSend(t *testing.T) {
	is := is.New(t)

	transport := newTestTransport()
	defer func() {
		transport.Stop()
		<-transport.Done()
	}()

	testConsumer := NewNSQTestConsumer(t, "test_send", "test")
	defer testConsumer.Stop()

	select {
	case transport.Send("test_send") <- []byte("hello vice"):
	case <-time.After(1 * time.Second):
		is.Fail() // timeout: transport.Send
	}

	select {
	case msg := <-testConsumer.messages:
		is.Equal(string(msg.Body), "hello vice")
	case <-time.After(2 * time.Second):
		is.Fail() // timeout: testConsumer <- messages
	}

	is.Equal(testConsumer.Count(), 1)

	// make sure Send with same name returns the same
	// channel
	is.Equal(transport.Send("test_send"), transport.Send("test_send"))
	is.True(transport.Send("test_send") != transport.Send("different"))

}

// func TestSendErrs(t *testing.T) {
// 	is := is.New(t)

// 	transport, err := New("no-such-host:4150")
// 	is.NoErr(err)
// 	defer func() {
// 		transport.Stop()
// 		<-transport.Done()
// 	}()

// 	select {
// 	case transport.Send("test_send_err") <- []byte("hello vice"):
// 	case <-time.After(1 * time.Second):
// 		is.Fail("timeout: transport.Send")
// 	}

// 	// expect an error...
// 	select {
// 	case err := <-transport.ErrChan():
// 		is.OK(err)
// 		is.Equal(err.Name, "test_send_err")
// 		is.Equal(err.Message, []byte(`hello vice`))
// 		is.Equal(err.Err.Error(), "dial tcp: lookup no-such-host: no such host")
// 		is.Equal(err.Error(), "dial tcp: lookup no-such-host: no such host: |test_send_err| <- `hello vice`")
// 	case <-time.After(2 * time.Second):
// 		is.Fail("timeout: transport.SendErrs()")
// 	}

// 	// SendErrs should always be the same channel
// 	is.Equal(transport.ErrChan(), transport.ErrChan())

// }

func TestReceive(t *testing.T) {
	is := is.New(t)

	transport := newTestTransport()
	defer func() {
		transport.Stop()
		<-transport.Done()
	}()

	testProducer := NewNSQTestProducer(t, "test_receive")
	testProducer.Send([]byte("hello vice"))

	ch := transport.Receive("test_receive")
	select {
	case msg := <-ch:
		is.Equal(msg, []byte("hello vice"))
	case <-time.After(2 * time.Second):
		is.Fail() // timeout: transport.Receive
	}

}

// func TestReceiveErrs(t *testing.T) {
// 	is := is.New(t)

// 	transport, err := New("no-such-host:4150")
// 	is.NoErr(err)
// 	defer func() {
// 		transport.Stop()
// 		<-transport.Done()
// 	}()

// 	_ = transport.Receive("test_receive_err")
// 	select {
// 	case err := <-transport.ErrChan():
// 		is.OK(err)
// 		is.Equal(err.Name, "test_receive_err")
// 		is.Nil(err.Message)
// 		is.Equal(err.Err.Error(), "dial tcp: lookup no-such-host: no such host")
// 		is.Equal(err.Error(), "dial tcp: lookup no-such-host: no such host: |test_receive_err|")
// 	case <-time.After(2 * time.Second):
// 		is.Fail("timeout: transport.Receive")
// 	}

// }

type NSQTestConsumer struct {
	t *testing.T

	// buffer channel to analize the messages
	messages chan *nsq.Message
	counter  int32
	consumer *nsq.Consumer
}

func NewNSQTestConsumer(t *testing.T, topic, channel string) *NSQTestConsumer {
	is := is.New(t)

	logger := log.New(ioutil.Discard, "logger: ", log.Lshortfile)
	consumer, err := nsq.NewConsumer(topic, channel, nsq.NewConfig())
	consumer.SetLogger(logger, nsq.LogLevelError)
	is.NoErr(err)

	testConsumer := &NSQTestConsumer{
		t:        t,
		messages: make(chan *nsq.Message, 30),
		consumer: consumer,
	}

	consumer.AddHandler(nsq.HandlerFunc(func(msg *nsq.Message) error {
		log.Println("@@ NSQTestConsumer Consuming ", string(msg.Body))
		atomic.AddInt32(&testConsumer.counter, 1)
		testConsumer.messages <- msg
		return nil
	}))
	is.NoErr(consumer.ConnectToNSQD(DefaultTCPAddr))

	return testConsumer
}

func (tc *NSQTestConsumer) Count() int {
	return int(atomic.LoadInt32(&tc.counter))
}

func (tc *NSQTestConsumer) Stop() {
	log.Println("stopping NSQTestConsumer")
	tc.consumer.Stop()
	select {
	case <-tc.consumer.StopChan:
		log.Println("NSQTestConsumer stopped")
	case <-time.After(2 * time.Second):
		tc.t.Fatal("Could not stop the NSQTestConsumer in time")
	}
}

type NSQTestProducer struct {
	t *testing.T

	producer *nsq.Producer
	topic    string
}

func NewNSQTestProducer(t *testing.T, topic string) *NSQTestProducer {
	producer, err := nsq.NewProducer(DefaultTCPAddr, nsq.NewConfig())
	if err != nil {
		t.Fatal("failed to create producer:", err) // TODO: handle errors
	}

	testProducer := &NSQTestProducer{
		t:        t,
		topic:    topic,
		producer: producer,
	}
	return testProducer
}

func (tp *NSQTestProducer) Send(msg []byte) {
	log.Println("NSQTestProducer: send", string(msg))
	if err := tp.producer.Publish(tp.topic, msg); err != nil {
		tp.t.Fatal("NSQTestProducer: failed to Publish:", err)
	}
	log.Println("NSQTestProducer:  ^ published")
}

func (tp *NSQTestProducer) Stop() {
	log.Println("stopping NSQTestProducer")
	tp.producer.Stop()
	log.Println("NSQTestProducer stopped")
}
