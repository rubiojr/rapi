package progress_test

import (
	"testing"
	"time"

	"github.com/rubiojr/rapi/internal/test"
	"github.com/rubiojr/rapi/internal/ui/progress"
)

func TestCounter(t *testing.T) {
	const N = 100
	const startTotal = uint64(12345)

	var (
		finalSeen  = false
		increasing = true
		last       uint64
		lastTotal  = startTotal
		ncalls     int
		nmaxChange int
	)

	report := func(value uint64, total uint64, d time.Duration, final bool) {
		if final {
			finalSeen = true
		}
		if value < last {
			increasing = false
		}
		last = value
		if total != lastTotal {
			nmaxChange++
		}
		lastTotal = total
		ncalls++
	}
	c := progress.New(10*time.Millisecond, startTotal, report)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < N; i++ {
			time.Sleep(time.Millisecond)
			c.Add(1)
		}
		c.SetMax(42)
	}()

	<-done
	c.Done()

	test.Assert(t, finalSeen, "final call did not happen")
	test.Assert(t, increasing, "values not increasing")
	test.Equals(t, uint64(N), last)
	test.Equals(t, uint64(42), lastTotal)
	test.Equals(t, int(1), nmaxChange)

	t.Log("number of calls:", ncalls)
}

func TestCounterNil(t *testing.T) {
	// Shouldn't panic.
	var c *progress.Counter = nil
	c.Add(1)
	c.Done()
}

func TestCounterNoTick(t *testing.T) {
	finalSeen := false
	otherSeen := false

	report := func(value, total uint64, d time.Duration, final bool) {
		if final {
			finalSeen = true
		} else {
			otherSeen = true
		}
	}
	c := progress.New(0, 1, report)
	time.Sleep(time.Millisecond)
	c.Done()

	test.Assert(t, finalSeen, "final call did not happen")
	test.Assert(t, !otherSeen, "unexpected status update")
}
