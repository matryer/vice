package redis

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/matryer/vice"
	"github.com/matryer/vice/vicetest"
)

func TestTransport(t *testing.T) {
	new := func() vice.Transport {
		return New()
	}
	vicetest.Transport(t, new)
}

func TestConnection(t *testing.T) {
	is := is.New(t)

	tr := New()

	c, err := tr.newConnection()
	is.True(c != nil)
	is.NoErr(err)

	err = c.Close()
	is.NoErr(err)
}

func TestSubscriber(t *testing.T) {
	is := is.New(t)
	msgToReceive := []byte("hello vice")

	transport := New()

	client2, err := transport.newConnection()
	is.NoErr(err)

	var wg sync.WaitGroup
	doneChan := make(chan struct{})

	waitChan := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(doneChan)
		for {
			select {
			case <-transport.Done():
				return
			case err := <-transport.ErrChan():
				fmt.Println(err)
				is.NoErr(err)
				wg.Done()
			case msg := <-transport.Receive("test_receive"):
				is.Equal(msg, msgToReceive)
				wg.Done()
			case <-time.After(2 * time.Second):
				is.Fail() // time out: transport.Receive
				wg.Done()
			default:
				once.Do(func() {
					close(waitChan)
				})
			}
		}
	}()

	<-waitChan

	wg.Add(1)
	cmd := client2.RPush("test_receive", string(msgToReceive))
	is.NoErr(cmd.Err())
	wg.Wait()
	transport.Stop()
	client2.Close()
	<-doneChan
}

func TestPublisher(t *testing.T) {
	is := is.New(t)
	msgToSend := []byte("hello vice")

	transport := New()
	var wg sync.WaitGroup
	doneChan := make(chan struct{})

	waitChan := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(doneChan)
		for {
			select {
			case <-transport.Done():
				return
			case err := <-transport.ErrChan():
				is.NoErr(err)
			case msg := <-transport.Receive("test_send"):
				is.Equal(msg, msgToSend)
				wg.Done()
			case <-time.After(2 * time.Second):
				is.Fail() // time out: transport.Receive
			default:
				once.Do(func() {
					close(waitChan)
				})
			}
		}
	}()

	<-waitChan

	wg.Add(1)
	transport.Send("test_send") <- msgToSend

	wg.Wait()
	transport.Stop()
	<-doneChan
}
