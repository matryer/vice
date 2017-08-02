package vicetest

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cheekybits/is"
	"github.com/matryer/vice"
)

// Transport runs standard transport tests. All Transport types pass
// this test.
//
// For more information see https://github.com/matryer/vice/blob/master/docs/writing-transports.md
//
// Transports should be initialised with a clean state. Old persisted messages
// can interfere with the test.
// After the tests are run, the transport is closed (since that is part
// of the spec).
func Transport(t *testing.T, transport func() vice.Transport) {
	t.Run("testStandardTransportBehaviour", func(t *testing.T) {
		testStandardTransportBehaviour(t, transport)
	})
	t.Run("testSendChannelsDontBlock", func(t *testing.T) {
		testSendChannelsDontBlock(t, transport)
	})
}

// testSendChannelsDontBlock ensures that send channels don't block, even
// if nothing (we know of) is receiving them.
func testSendChannelsDontBlock(t *testing.T, newTransport func() vice.Transport) {
	is := is.New(t)
	transport := newTransport()

	select {
	case transport.Send("something") <- []byte("message"):
		return
	case <-time.After(1 * time.Second):
		is.Fail("send channels shouldn't block")
	}
}

// testStandardTransportBehaviour tests that transports load balance
// over many Receive channels.
func testStandardTransportBehaviour(t *testing.T, newTransport func() vice.Transport) {
	is := is.New(t)
	defer func() {
		if r := recover(); r != nil {
			is.Fail("old messages may have confused test:", r)
		}
	}()

	transport := newTransport()
	transport1 := newTransport()
	transport2 := newTransport()

	doneChan := make(chan struct{})
	messages := make(map[string][][]byte)
	var wg sync.WaitGroup

	go func() {
		defer close(doneChan)
		for {
			select {
			case <-transport.Done():
				return

			case err := <-transport.ErrChan():
				is.NoErr(err)

			// test local load balancing with the same transport
			case msg := <-transport.Receive("vicechannel1"):
				messages["vicechannel1"] = append(messages["vicechannel1"], msg)
				wg.Done()
			case msg := <-transport.Receive("vicechannel1"):
				messages["vicechannel1"] = append(messages["vicechannel1"], msg)
				wg.Done()
			case msg := <-transport.Receive("vicechannel1"):
				messages["vicechannel1"] = append(messages["vicechannel1"], msg)
				wg.Done()

			case msg := <-transport.Receive("vicechannel2"):
				messages["vicechannel2"] = append(messages["vicechannel2"], msg)
				wg.Done()
			case msg := <-transport.Receive("vicechannel2"):
				messages["vicechannel2"] = append(messages["vicechannel2"], msg)
				wg.Done()

			case msg := <-transport.Receive("vicechannel3"):
				messages["vicechannel3"] = append(messages["vicechannel3"], msg)
				wg.Done()

			// test distibuted load balancing
			case msg := <-transport.Receive("vicechannel4"):
				messages["vicechannel4.1"] = append(messages["vicechannel4.1"], msg)
				wg.Done()
			case msg := <-transport1.Receive("vicechannel4"):
				messages["vicechannel4.2"] = append(messages["vicechannel4.2"], msg)
				wg.Done()
			case msg := <-transport2.Receive("vicechannel4"):
				messages["vicechannel4.3"] = append(messages["vicechannel4.3"], msg)
				wg.Done()
			}
		}
	}()

	// Let's give some time to initialize all receiving channels
	time.Sleep(time.Millisecond * 10)

	// send 100 messages down each chan
	for i := 0; i < 100; i++ {
		wg.Add(4)
		msg := []byte(fmt.Sprintf("message %d", i+1))
		transport.Send("vicechannel1") <- msg
		transport.Send("vicechannel2") <- msg
		transport.Send("vicechannel3") <- msg
		transport.Send("vicechannel4") <- msg
	}

	wg.Wait()
	transport.Stop()
	transport1.Stop()
	transport2.Stop()
	<-doneChan

	is.Equal(len(messages), 6)
	is.Equal(len(messages["vicechannel1"]), 100)
	is.Equal(len(messages["vicechannel2"]), 100)
	is.Equal(len(messages["vicechannel3"]), 100)

	is.NotEqual(len(messages["vicechannel4.1"]), 100)
	is.NotEqual(len(messages["vicechannel4.2"]), 100)
	is.NotEqual(len(messages["vicechannel4.3"]), 100)

	is.Equal(len(messages["vicechannel4.1"])+len(messages["vicechannel4.2"])+len(messages["vicechannel4.3"]), 100)
}
