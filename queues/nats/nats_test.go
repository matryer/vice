package nats

import (
	"sync"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/matryer/vice"
	"github.com/matryer/vice/vicetest"
	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
	uuid "github.com/satori/go.uuid"
)

func TestDefaultTransport(t *testing.T) {
	new := func() vice.Transport {
		return New()
	}

	vicetest.Transport(t, new)
}
func TestStreamingTransport(t *testing.T) {
	new := func() vice.Transport {
		return New(WithStreaming("test-cluster", uuid.NewV4().String()))
	}

	vicetest.Transport(t, new)
}

func TestReceive(t *testing.T) {
	is := is.New(t)

	transport := New()
	streamTransport := New(WithStreaming("test-cluster", uuid.NewV4().String()))

	var wg sync.WaitGroup

	go func() {
		for {
			select {
			case <-transport.Done():
				return
			case err := <-transport.ErrChan():
				is.NoErr(err)
			case msg := <-transport.Receive("test_receive"):
				is.Equal(msg, []byte("hello vice"))
				wg.Done()
			case msg := <-streamTransport.Receive("test_receive"):
				is.Equal(msg, []byte("hello vice"))
				wg.Done()
			case <-time.After(2 * time.Second):
				is.Fail() // time out: transport.Receive
			}
		}
	}()

	time.Sleep(time.Millisecond * 10)

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		is.Fail() // couldn't connect to nats server
	}
	defer nc.Close()

	ns, err := stan.Connect("test-cluster", "bla")
	if err != nil {
		is.Fail() // couldn't connect to nats server
	}
	defer ns.Close()

	wg.Add(2)
	err = nc.Publish("test_receive", []byte("hello vice"))
	is.NoErr(err)
	err = ns.Publish("test_receive", []byte("hello vice"))
	is.NoErr(err)
	wg.Wait()

	transport.Stop()
	<-transport.Done()

	streamTransport.Stop()
	<-transport.Done()
}

func TestSend(t *testing.T) {
	is := is.New(t)

	transport := New()
	streamTransport := New(WithStreaming("test-cluster", uuid.NewV4().String()))

	var wg sync.WaitGroup

	var msgs [][]byte

	go func() {
		for {
			select {
			case <-transport.Done():
				return
			case err := <-transport.ErrChan():
				is.NoErr(err)
			case msg := <-transport.Receive("test_send"):
				msgs = append(msgs, msg)
				is.Equal(msg, []byte("hello vice"))
				wg.Done()
			case msg := <-streamTransport.Receive("test_send"):
				msgs = append(msgs, msg)
				is.Equal(msg, []byte("hello vice"))
				wg.Done()
			case <-time.After(3 * time.Second):
				is.Fail() // time out: transport.Receive
			}
		}
	}()

	time.Sleep(time.Millisecond * 10)
	wg.Add(2)

	streamTransport.Send("test_send") <- []byte("hello vice")
	transport.Send("test_send") <- []byte("hello vice")

	wg.Wait()

	is.Equal(len(msgs), 2)
	is.Equal(transport.Send("test_send"), transport.Send("test_send"))
	is.True(transport.Send("test_send") != transport.Send("different"))
	is.Equal(streamTransport.Send("test_send"), streamTransport.Send("test_send"))
	is.True(streamTransport.Send("test_send") != streamTransport.Send("different"))

	transport.Stop()
	streamTransport.Stop()
}
