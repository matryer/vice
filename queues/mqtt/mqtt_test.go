package mqtt

import (
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/matryer/is"
	"github.com/matryer/vice"
	"github.com/matryer/vice/vicetest"
)

func TestDefaultTransport(t *testing.T) {
	new := func() vice.Transport {
		return New()
	}

	vicetest.Transport(t, new)
}

func TestReceive(t *testing.T) {
	is := is.New(t)

	transport := New()

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
			case <-time.After(2 * time.Second):
				is.Fail() // time out: transport.Receive
			}
		}
	}()

	time.Sleep(time.Millisecond * 100)

	// create new client
	opts := mqtt.NewClientOptions().AddBroker("tcp://localhost:1883")
	cli := mqtt.NewClient(opts)
	if token := cli.Connect(); token.Wait() && token.Error() != nil {
		is.NoErr(token.Error())
	}

	wg.Add(1)

	// publish
	if token := cli.Publish("test_receive", 0, false, []byte("hello vice")); token.Wait() && token.Error() != nil {
		is.NoErr(token.Error())
	}

	wg.Wait()

	transport.Stop()
	<-transport.Done()
}

func TestSend(t *testing.T) {
	is := is.New(t)

	transport := New()

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
			case <-time.After(3 * time.Second):
				is.Fail() // time out: transport.Receive
			}
		}
	}()

	time.Sleep(time.Millisecond * 100)
	wg.Add(1)

	transport.Send("test_send") <- []byte("hello vice")

	wg.Wait()

	is.Equal(len(msgs), 1)
	is.Equal(transport.Send("test_send"), transport.Send("test_send"))
	is.True(transport.Send("test_send") != transport.Send("different"))

	transport.Stop()
}
