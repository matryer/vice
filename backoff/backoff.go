package backoff

import (
	"log"
	"time"
)

// Do retries the function f backing off the double of time each retry until a successfully call is made.
// initialBackoff is minimal time to wait for the next call
// maxBackoff is the maximum time between calls, if is 0 there is no maximum
// maxCalls is the maximum number of call to the function, if is 0 there is no maximum
func Do(initialBackoff, maxBackoff time.Duration, maxCalls int, f func() error) error {
	backoff := time.Duration(0)
	calls := 0
	for {
		err := f()
		if err == nil {
			return nil
		}
		calls++
		if calls >= maxCalls && maxCalls != 0 {
			return err
		}
		switch {
		case backoff == 0:
			backoff = initialBackoff
		case backoff > maxBackoff && maxBackoff != 0:
			backoff = maxBackoff
		default:
			backoff *= 2
		}
		time.Sleep(backoff)
		log.Printf("[backoff %v] Retry after %v due to the Error: %v\n", calls, backoff, err)
	}
}
