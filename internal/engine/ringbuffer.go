package engine

import "sync"

// RingBuffer is a generic, thread-safe, fixed-capacity circular buffer.
type RingBuffer[T any] struct {
	mu    sync.RWMutex
	items []T
	head  int
	count int
	cap   int
}

// NewRingBuffer creates a new RingBuffer with the given capacity.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	return &RingBuffer[T]{
		items: make([]T, capacity),
		cap:   capacity,
	}
}

// Add inserts an item into the ring buffer, overwriting the oldest if full.
func (r *RingBuffer[T]) Add(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[r.head] = item
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
}

// Len returns the number of items currently in the buffer.
func (r *RingBuffer[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

// All returns all items in order from oldest to newest.
func (r *RingBuffer[T]) All() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]T, r.count)
	start := 0
	if r.count == r.cap {
		start = r.head
	}
	for i := 0; i < r.count; i++ {
		result[i] = r.items[(start+i)%r.cap]
	}
	return result
}

// Last returns the most recently added item.
func (r *RingBuffer[T]) Last() (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var zero T
	if r.count == 0 {
		return zero, false
	}
	idx := (r.head - 1 + r.cap) % r.cap
	return r.items[idx], true
}
