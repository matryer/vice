package rabbitmq

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func BenchmarkTransport(b *testing.B) {
	defer func() {
		if r := recover(); r != nil {
			b.Fatal("old messages may have confused test")
		}
	}()

	transport := New()
	transport1 := New()
	transport2 := New()

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
				b.Fatal("failed with ", err)

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
	for i := 0; i < b.N; i++ {
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

}

func BenchmarkTransport2(b *testing.B) {
	defer func() {
		if r := recover(); r != nil {
			b.Fatal("old messages may have confused test")
		}
	}()

	transport := New()
	transport1 := New()

	doneChan := make(chan struct{})
	var wg sync.WaitGroup

	go func() {
		defer close(doneChan)
		for {
			select {
			case <-transport.Done():
				return

			case err := <-transport.ErrChan():
				b.Fatal("failed with ", err)

				// test local load balancing with the same transport
			case <-transport1.Receive("vicechannelbench"):
				wg.Done()
			}
		}
	}()
	msg := []byte("message")
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		transport.Send("vicechannelbench") <- msg
	}

	wg.Wait()
	transport.Stop()
	transport1.Stop()
	<-doneChan

}

func BenchmarkTransport3(b *testing.B) {
	defer func() {
		if r := recover(); r != nil {
			b.Fatal("old messages may have confused test")
		}
	}()

	transport := New()

	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)
		for {
			select {
			case <-transport.Done():
				return

			case err := <-transport.ErrChan():
				b.Fatal("failed with ", err)
			}
		}
	}()

	msg := []byte("message")
	for i := 0; i < b.N; i++ {
		transport.Send("vicechannelbench2") <- msg
	}

	transport.Stop()
	<-doneChan

}
