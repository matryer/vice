// server is a simple Greeter server that uses the emitter transport.

package main

import (
	"context"
	"log"
	"time"

	"github.com/matryer/vice/queues/emitter"
)

// To run this, install emitter and start it with emitter command.
// Run this program:
//   $ export EMITTER_SECRET_KEY=...
//   $ go run main.go
// Send a name: curl -d 'Mat' 'http://127.0.0.1:4151/pub?topic=names'

// Greeter is a service that greets people.
func Greeter(ctx context.Context, names <-chan []byte, greetings chan<- []byte, errs <-chan error) {
	for {
		select {
		case <-ctx.Done():
			log.Println("finished")
			return
		case err := <-errs:
			log.Println("an error occurred:", err)
		case name := <-names:
			greeting := "Hello " + string(name)
			greetings <- []byte(greeting)
		}
	}
}

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	transport := emitter.New()
	defer func() {
		transport.Stop()
		<-transport.Done()
	}()
	names := transport.Receive("names")
	greetings := transport.Send("greetings")
	Greeter(ctx, names, greetings, transport.ErrChan())
}
