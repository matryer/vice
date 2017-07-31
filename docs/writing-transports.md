# Writing `vice.Transport` implementations

Queuing transports implement the [vice.Transport](https://github.com/matryer/vice/blob/master/transport.go) interface.

## Before you start

Ensure the technology is not already implemented by checking the [queues](https://github.com/matryer/vice/tree/master/queues) folder.

## Specification test

To ensure a transport works as expected, call `vicetest.Transport` from the `test` package along with your other transport specific unit tests:

```
package my_transport

import (
	"testing"
	"github.com/matryer/vice/vicetest"
)

func TestTransport(t *testing.T) {
	vicetest.Transport(t, func() vice.Transport {
		return yourtransport.New()
	})
}
```

The `vicetest.Transport` function takes a `testing.T`, and a function that creates a _fresh_ instance of your transport. Transports should be cleaned before running this test, as messages left-over from previous tests may interfere.

If `vicetest.Transport` passes, your transport performs as expected and can be contributed to the project.

## Stopping

Transports should accept a context, which when cancelled should shut them down. Once a Transport is shut down, it should
close the channel returned by the `Done()` method.

## Defaults

Queue technologies often have sensible defaults. These should be understood by the Transport. If possible, users should be able to call `transport.New()` with no additional arguments.

To give users more control, consider function fields in the Transport that perform tasks like connecting, registering and deregistering interest etc. This gives users fine grained control of the underlying drivers and means the transport implementation doesn't get bloated with configuration or strange abstractions.
