package backoff

import (
	"log"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// Do retries the function f backing off the double of time each retry until a successfully call is made.
// initialBackoff is minimal time to wait for the next call
// maxBackoff is the maximum time between calls, if is 0 there is no maximum
// maxCalls is the maximum number of call to the function, if is 0 there is no maximum
func Do(min, max time.Duration, maxTries int, f func() error) error {
	var err error
	for rc := 0; rc < maxTries; rc++ {
		err = f()
		if err == nil {
			return nil
		}

		delay := (1 << uint(rc)) * (rand.Intn(int(min/time.Millisecond)) + int(min/time.Millisecond))
		st := time.Duration(delay) * time.Millisecond
		time.Sleep(st)
		log.Printf("[backoff %v] Retry after %v due to the Error: %v\n", rc, st, err)
	}
	return err
}
