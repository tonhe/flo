package engine

import (
	"testing"
	"time"
)

func TestRingBufferAdd(t *testing.T) {
	rb := NewRingBuffer[RateSample](5)
	for i := 0; i < 3; i++ {
		rb.Add(RateSample{Timestamp: time.Now(), InRate: float64(i)})
	}
	if rb.Len() != 3 {
		t.Errorf("expected len 3, got %d", rb.Len())
	}
}

func TestRingBufferWrap(t *testing.T) {
	rb := NewRingBuffer[RateSample](3)
	for i := 0; i < 5; i++ {
		rb.Add(RateSample{InRate: float64(i)})
	}
	if rb.Len() != 3 {
		t.Errorf("expected len 3, got %d", rb.Len())
	}
	items := rb.All()
	if items[0].InRate != 2 {
		t.Errorf("expected oldest item InRate=2, got %f", items[0].InRate)
	}
	if items[2].InRate != 4 {
		t.Errorf("expected newest item InRate=4, got %f", items[2].InRate)
	}
}

func TestRingBufferEmpty(t *testing.T) {
	rb := NewRingBuffer[RateSample](10)
	if rb.Len() != 0 {
		t.Error("new ring buffer should be empty")
	}
	items := rb.All()
	if len(items) != 0 {
		t.Error("All() on empty buffer should return empty slice")
	}
}

func TestRingBufferLast(t *testing.T) {
	rb := NewRingBuffer[RateSample](5)
	rb.Add(RateSample{InRate: 1})
	rb.Add(RateSample{InRate: 2})
	rb.Add(RateSample{InRate: 3})
	last, ok := rb.Last()
	if !ok {
		t.Fatal("Last() should return true for non-empty buffer")
	}
	if last.InRate != 3 {
		t.Errorf("expected InRate=3, got %f", last.InRate)
	}
}
