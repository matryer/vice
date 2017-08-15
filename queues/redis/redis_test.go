package redis

import (
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	redis "gopkg.in/redis.v3"

	"github.com/matryer/is"
	"github.com/matryer/vice"
	"github.com/matryer/vice/vicetest"
)

var options = func() []Option { return nil }

func TestMain(m *testing.M) {
	// if in CI derive the connection params
	if os.Getenv("GO_ENV") == "ci" {
		url, err := url.Parse(os.Getenv("REDIS_PORT"))
		if err != nil {
			panic(err)
		}

		options = func() []Option {
			return []Option{WithClient(redis.NewClient(&redis.Options{
				Network: url.Scheme,
				Addr:    fmt.Sprintf("%s:%s", url.Hostname(), url.Port()),
			}))}
		}
	}

	// run the test suite
	os.Exit(m.Run())
}

func TestTransport(t *testing.T) {
	new := func() vice.Transport {
		return New(options()...)
	}
	vicetest.Transport(t, new)
}

func TestConnection(t *testing.T) {
	is := is.New(t)

	tr := New(options()...)

	c, err := tr.newConnection()
	is.True(c != nil)
	is.NoErr(err)

	err = c.Close()
	is.NoErr(err)
}

func TestSubscriber(t *testing.T) {
	is := is.New(t)
	msgToReceive := []byte("hello vice")

	transport := New(options()...)

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

	transport := New(options()...)
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
