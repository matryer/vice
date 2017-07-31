package vice

import "fmt"

// Transport provides message sending and receiving
// capabilities over a messaging queue technology.
// Clients should always check for errors coming through ErrChan.
type Transport interface {
	// Receive gets a channel on which to receive messages
	// with the specified name.
	Receive(name string) <-chan []byte
	// Send gets a channel on which messages with the
	// specified name may be sent.
	Send(name string) chan<- []byte
	// ErrChan gets a channel through which errors
	// are sent.
	ErrChan() <-chan error

	// Stop stops the transport. The channel returned from Done() will be closed
	// when the transport has stopped.
	Stop()
	// Done gets a channel which is closed when the
	// transport has successfully stopped.
	Done() chan struct{}
}

// Err represents a vice error.
type Err struct {
	Message []byte
	Name    string
	Err     error
}

func (e Err) Error() string {
	if len(e.Message) > 0 {
		return fmt.Sprintf("%s: |%s| <- `%s`", e.Err, e.Name, string(e.Message))
	}
	return fmt.Sprintf("%s: |%s|", e.Err, e.Name)
}
