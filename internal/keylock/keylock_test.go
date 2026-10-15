package keylock

import (
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestLockSerializesOneKey(t *testing.T) {
	var locks Striped
	unlock := locks.Lock("a")
	acquired := make(chan struct{})
	go func() {
		defer close(acquired)
		locks.Lock("a")()
	}()
	select {
	case <-acquired:
		t.Fatal("a second Lock of the same key did not wait for the first")
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	<-acquired
}

func TestLockLeavesOtherStripesFree(t *testing.T) {
	var locks Striped
	if stripe("a") == stripe("b") {
		t.Fatal("the test keys share a stripe; pick two that do not")
	}
	defer locks.Lock("a")()
	done := make(chan struct{})
	go func() {
		defer close(done)
		locks.Lock("b")()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a key on another stripe waited for a held lock")
	}
}
