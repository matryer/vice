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
	transport := New()
	vicetest.Transport(t, transport)
}
```

The `vicetest.Transport` function takes a `testing.T`, and a _fresh_ instance of your transport. Transports should be cleaned before running this test, as messages left-over from previous tests may interfere.

If `vicetest.Transport` passes, your transport performs as expected and can be contributed to the project.

## Defaults

Queue technologies often have sensible defaults. These should be understood by the Transport. If possible, users should be able to call `transport.New()` with no additional arguments.

To give users more control, consider function fields in the Transport that perform tasks like connecting, registering and deregistering interest etc. This gives users fine grained control of the underlying drivers and means the transport implementation doesn't get bloated with configuration or strange abstractions.
