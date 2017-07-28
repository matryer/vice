package backoff

import (
	"errors"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	okAfter := 2
	calls := 0

	err := Do(100*time.Millisecond, 1*time.Second, 3, func() error {
		calls++
		if calls > okAfter {
			return nil
		}
		return errors.New("not ok")
	})
	if err != nil {
		t.Fatalf("Error should be nil but is: %v", err)
	}
	if calls > 3 {
		t.Fatalf("Calls should be < 10 but is: %v", calls)
	}
}

func TestBackoffMaxCalls(t *testing.T) {
	okAfter := 5
	calls := 0

	err := Do(100*time.Millisecond, 1*time.Second, 2, func() error {
		calls++
		if calls > okAfter {
			return nil
		}
		return errors.New("not ok")
	})
	if err == nil {
		t.Fatalf("Error should be not nil but is: %v", err)
	}
	if calls != 2 {
		t.Fatalf("Calls should be 2 but is: %v", calls)
	}
}
