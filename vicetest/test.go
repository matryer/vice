package vicetest

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cheekybits/is"
	"github.com/matryer/vice"
)

// Transport runs standard transport tests. All Transport types pass
// this test.
//
// The transport argument should construct a clean Transport.
//
// For more information see https://github.com/matryer/vice/blob/master/docs/writing-transports.md
//
// Transports should be initialised with a clean state. Old persisted messages
// can interfere with the test.
// After the tests are run, the transport is closed (since that is part
// of the spec).
func Transport(t *testing.T, newTransport func(ctx context.Context) vice.Transport) {
	t.Run("testStandardTransportBehaviour", func(t *testing.T) {
		testStandardTransportBehaviour(t, newTransport)
	})
	// t.Run("testSendChannelsDontBlock", func(t *testing.T) {
	// 	testSendChannelsDontBlock(t, newTransport)
	// })
}

// testSendChannelsDontBlock ensures that send channels don't block, even
// if nothing (we know of) is receiving them.
func testSendChannelsDontBlock(t *testing.T, newTransport func(ctx context.Context) vice.Transport) {
	is := is.New(t)
	ctx, stopTransport := context.WithCancel(context.Background())
	transport := newTransport(ctx)
	defer func() {
		stopTransport()
		select {
		case <-transport.Done():
			return
		case <-time.After(1 * time.Second):
			is.Fail() // timed out waiting for transport to stop
		}
	}()
	select {
	case transport.Send("something") <- []byte("message"):
		return
	case <-time.After(1 * time.Second):
		is.Fail("send channels shouldn't block")
	}
}

// testStandardTransportBehaviour tests that transports load balance
// over many Receive channels.
func testStandardTransportBehaviour(t *testing.T, newTransport func(ctx context.Context) vice.Transport) {
	is := is.New(t)

	defer func() {
		if r := recover(); r != nil {
			is.Fail("old messages may have confused test:", r)
		}
	}()

	ctx, stopTransport := context.WithCancel(context.Background())
	transport := newTransport(ctx)
	defer func() {
		log.Println("STOPPING TRANSPORT")
		stopTransport()
		select {
		case <-transport.Done():
			log.Println("TRANSPORT STOPPED")
			return
		case <-time.After(1 * time.Second):
			is.Fail() // timed out waiting for transport to stop
		}
	}()

	// drain the queue
	for _, channel := range []string{"vicechannel1", "vicechannel2", "vicechannel3"} {
		log.Print("draining ", channel, ": ")
	loop:
		for {
			select {
			case <-transport.Receive(channel):
				fmt.Print(".")
			case <-time.After(100 * time.Millisecond):
				break loop
			}
		}
	}

	doneChan := make(chan struct{})
	messages := make(map[string][][]byte)
	var wg sync.WaitGroup

	go func() {
		defer close(doneChan)
		for {
			select {
			case <-transport.Done():
				log.Println("transport done")
				// finished
				return

			case err := <-transport.ErrChan():
				is.NoErr(err) // transport error

			case msg := <-transport.Receive("vicechannel1"):
				messages["vicechannel1"] = append(messages["vicechannel1"], msg)
				wg.Done()
			// case msg := <-transport.Receive("vicechannel1"):
			// 	messages["vicechannel1"] = append(messages["vicechannel1"], msg)
			// 	wg.Done()
			// case msg := <-transport.Receive("vicechannel1"):
			// 	messages["vicechannel1"] = append(messages["vicechannel1"], msg)
			// 	wg.Done()

			case msg := <-transport.Receive("vicechannel2"):
				messages["vicechannel2"] = append(messages["vicechannel2"], msg)
				log.Println("##### received:", string(msg))
				wg.Done()
			// case msg := <-transport.Receive("vicechannel2"):
			// 	messages["vicechannel2"] = append(messages["vicechannel2"], msg)
			// 	wg.Done()

			case msg := <-transport.Receive("vicechannel3"):
				messages["vicechannel3"] = append(messages["vicechannel3"], msg)
				wg.Done()
			}
		}
	}()

	// Let's give some time to initialize all receiving channels
	time.Sleep(time.Millisecond * 10)

	log.Println("sending messages...")

	// send 100 messages down each chan
	for i := 0; i < 100; i++ {
		msg := []byte(fmt.Sprintf("message %d", i))
		wg.Add(3)
		transport.Send("vicechannel1") <- msg
		transport.Send("vicechannel2") <- msg
		transport.Send("vicechannel3") <- msg
	}

	log.Println("waiting for them to be dealt with...")

	wg.Wait()
	stopTransport()
	<-doneChan

	is.Equal(len(messages), 3)
	is.Equal(len(messages["vicechannel1"]), 100)
	is.Equal(len(messages["vicechannel2"]), 100)
	is.Equal(len(messages["vicechannel3"]), 100)

}
