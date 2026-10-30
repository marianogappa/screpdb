package enginepass

import (
	"container/heap"
	"time"
)

type job struct {
	checksum [32]byte
	id       int64
	date     time.Time
	path     string
	// upgrade re-runs a replay whose digest is from an older, still readable engine version: it waits
	// until every replay without engine data is done.
	upgrade bool
	index   int
}

// queue orders pending engine passes: the replay the user has open first, then newest first. New replays
// from the file watcher are the newest, so they come next without a tier of their own.
type queue struct {
	byChecksum map[[32]byte]*job
	byDate     jobHeap
	focus      [32]byte
	hasFocus   bool
}

func newQueue() *queue { return &queue{byChecksum: map[[32]byte]*job{}} }

func (q *queue) push(j *job) {
	if old, ok := q.byChecksum[j.checksum]; ok {
		heap.Remove(&q.byDate, old.index)
	}
	q.byChecksum[j.checksum] = j
	heap.Push(&q.byDate, j)
}

func (q *queue) remove(checksum [32]byte) {
	if j, ok := q.byChecksum[checksum]; ok {
		heap.Remove(&q.byDate, j.index)
		delete(q.byChecksum, checksum)
	}
}

func (q *queue) setFocus(checksum [32]byte) {
	q.focus, q.hasFocus = checksum, true
}

func (q *queue) pop() *job {
	if q.hasFocus {
		q.hasFocus = false
		if j, ok := q.byChecksum[q.focus]; ok {
			q.remove(j.checksum)
			return j
		}
	}
	if len(q.byDate) == 0 {
		return nil
	}
	j := heap.Pop(&q.byDate).(*job)
	delete(q.byChecksum, j.checksum)
	return j
}

func (q *queue) len() int { return len(q.byChecksum) }

type jobHeap []*job

func (h jobHeap) Len() int { return len(h) }

func (h jobHeap) Less(i, k int) bool {
	if h[i].upgrade != h[k].upgrade {
		return !h[i].upgrade
	}
	if !h[i].date.Equal(h[k].date) {
		return h[i].date.After(h[k].date)
	}
	return h[i].id > h[k].id
}

func (h jobHeap) Swap(i, k int) {
	h[i], h[k] = h[k], h[i]
	h[i].index, h[k].index = i, k
}

func (h *jobHeap) Push(x any) {
	j := x.(*job)
	j.index = len(*h)
	*h = append(*h, j)
}

func (h *jobHeap) Pop() any {
	old := *h
	j := old[len(old)-1]
	old[len(old)-1] = nil
	*h = old[:len(old)-1]
	return j
}
