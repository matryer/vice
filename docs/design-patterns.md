# Design patterns

## Service function

Wrapping your service in a single function is a good way to express the intent about what it does, and what dependencies it has. Take in the channels as arguments. A service that greets people (by reading |names| and sending messages down |greetings|) might be written like this:

```go
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
```

Just by inspecting the signature, it's clear that our `Greeter` services takes in `names`, and outputs `greetings`. It is good practice to also take in the `transport.ErrChan()` to make sure the underlying transport is healthy. In the code above, the context can be used to stop the Greeter from running.

This approach allows you to unit test your service using idiomatic Go code, without the need for any transports at all:

```go
func TestGreeter(t *testing.T) {

	// setup a context that will stop the Greeter once this
	// test finishes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create test channels (no need for a real queue)
	names := make(chan []byte)
	greetings := make(chan []byte)
	errs := make(chan error)

	// run the greeter
	go Greeter(ctx, names, greetings, errs)

	// send the name and get the greeting
	names <- []byte("Vice")
	greeting := <-greetings

	if string(greeting) != "Hello Vice" {
		t.Error("unexpected message")
	}
}
```

You may also simulate transport errors, and ensure your service responds in a timely fashion by introducing a timeout when waiting on the response channel:

```go
select {
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for greeting")
	case greeting := <-greetings:
		if string(greeting) != "Hello Vice" {
			t.Error("unexpected message")
		}
}
```

To wire the service up for test and production environments, you just need to pass in channels obtained from the `vice.Transport`:

```go
Greeting(ctx, transport.Receive("names"), transport.Send("greetings"), transport.ErrChan())
```
