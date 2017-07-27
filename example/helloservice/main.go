package main

import (
	"log"
	"time"

	"github.com/matryer/vice"
	"github.com/matryer/vice/queues/nsq"
)

// Listens on the |names| channel for names,
// and sends "Hello {name}" on the |greeting| channel.

func main() {
	transport, err := nsq.New()
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		transport.Stop()
		<-transport.StopChan()
	}()
	if err := run(transport); err != nil {
		log.Println(err)
	}
}

func run(transport vice.Transport) error {
	for {
		select {
		case err := <-transport.ErrChan():
			return err
		case name := <-transport.Receive("names"):
			log.Println("Saying hello to", string(name))
			transport.Send("greeting") <- append([]byte("Hello "), name...)
		case <-time.After(15 * time.Second):
			log.Println("Nothing happend :(")
		}
	}
}
