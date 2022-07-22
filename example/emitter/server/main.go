// server is a simple Greeter server that uses the emitter transport.

package main

import (
	"context"
	"log"
	"time"

	"github.com/matryer/vice/v2/queues/emitter"
)

// To run this, install emitter and start it with the "emitter" command.
// Run this program:
//   $ export EMITTER_SECRET_KEY=...
//   $ go run main.go
// Then use the "example/emitter/client" program to send names to the server.

const runDuration = 1 * time.Minute

// Greeter is a service that greets people.
func Greeter(ctx context.Context, names <-chan []byte, greetings chan<- []byte, errs <-chan error) {
	for {
		select {
		case <-ctx.Done():
			log.Println("finished")
			return
		case err := <-errs:
			log.Fatalf("an error occurred: %v", err)
		case name := <-names:
			greeting := "Hello " + string(name)
			greetings <- []byte(greeting)
			log.Printf("Sent greeting: %v", greeting)
		}
	}
}

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, runDuration)
	defer cancel()
	transport := emitter.New()
	defer func() {
		transport.Stop()
		<-transport.Done()
	}()
	names := transport.Receive("names")
	greetings := transport.Send("greetings")
	log.Printf("Greeter server listening on 'names' channel and responding on 'greetings' channel...")
	log.Printf("Note that this server will automatically shut down in %v", runDuration)
	Greeter(ctx, names, greetings, transport.ErrChan())
}
