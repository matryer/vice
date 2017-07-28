package nats

import (
	"testing"
	"time"

	"sync"

	"github.com/matryer/is"
	"github.com/matryer/vice/vicetest"
	"github.com/nats-io/nats"
)

func TestTransport(t *testing.T) {
	transport := New()
	vicetest.Transport(t, transport)
}

func TestReceive(t *testing.T) {
	is := is.New(t)

	transport := New()

	var wg sync.WaitGroup

	go func() {

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

	}()

	time.Sleep(time.Millisecond * 10)

	nc, err := nats.Connect(nats.DefaultURL)
	defer nc.Close()
	if err != nil {
		is.Fail() // couldn't connect to nats server
	}
	wg.Add(1)
	err = nc.Publish("test_receive", []byte("hello vice"))
	is.NoErr(err)
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

	}()

	time.Sleep(time.Millisecond * 10)
	wg.Add(1)

	select {
	case transport.Send("test_send") <- []byte("hello vice"):
	case <-time.After(2 * time.Second):
		is.Fail() // time out: transport.Send
	}
	wg.Wait()
	transport.Stop()

	is.Equal(len(msgs), 1)
	is.Equal(transport.Send("test_send"), transport.Send("test_send"))
	is.True(transport.Send("test_send") != transport.Send("different"))
}
