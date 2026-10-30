package enginepass

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
)

func TestWorkerLimitFollowsWorkersLive(t *testing.T) {
	if AllWorkers() < 3 {
		t.Skip("needs at least 3 cores")
	}
	lib := library.New(library.Options{})
	defer lib.Close()
	var limit atomic.Int64
	limit.Store(1)
	r := New(Options{Library: lib, Workers: func() int { return int(limit.Load()) }})
	defer r.Close()
	for i := range 6 {
		r.queue.push(&job{checksum: [32]byte{byte(i)}, id: int64(i), date: time.Now()})
	}
	r.updateLimit()

	handedOut := make(chan *job, 6)
	for range 4 {
		go func() {
			if j := r.next(); j != nil {
				handedOut <- j
			}
		}()
	}
	expect := func(n int) {
		t.Helper()
		for range n {
			select {
			case <-handedOut:
			case <-time.After(2 * time.Second):
				t.Fatalf("expected %d more jobs", n)
			}
		}
		select {
		case <-handedOut:
			t.Fatal("a job was handed out above the limit")
		case <-time.After(100 * time.Millisecond):
		}
	}

	expect(1)
	limit.Store(3)
	r.updateLimit()
	expect(2)

	limit.Store(1)
	r.updateLimit()
	r.done()
	r.done()
	expect(0)
	r.done()
	go func() {
		if j := r.next(); j != nil {
			handedOut <- j
		}
	}()
	expect(1)
}
