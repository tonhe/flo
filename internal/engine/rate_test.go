package engine

import (
	"testing"
	"time"
)

func TestCalculateRate(t *testing.T) {
	prev := CounterSample{
		InOctets:  1000,
		OutOctets: 500,
		Timestamp: time.Now().Add(-10 * time.Second),
	}
	curr := CounterSample{
		InOctets:  2000,
		OutOctets: 1500,
		Timestamp: time.Now(),
	}
	rate, err := CalculateRate(prev, curr)
	if err != nil {
		t.Fatalf("CalculateRate() error: %v", err)
	}
	if rate.InRate < 790 || rate.InRate > 810 {
		t.Errorf("expected InRate ~800, got %f", rate.InRate)
	}
	if rate.OutRate < 790 || rate.OutRate > 810 {
		t.Errorf("expected OutRate ~800, got %f", rate.OutRate)
	}
}

func TestCalculateRateCounterWrap(t *testing.T) {
	prev := CounterSample{
		InOctets:  100,
		OutOctets: 50,
		Timestamp: time.Now().Add(-10 * time.Second),
	}
	curr := CounterSample{
		InOctets:  50,
		OutOctets: 50,
		Timestamp: time.Now(),
	}
	_, err := CalculateRate(prev, curr)
	if err != ErrCounterWrap {
		t.Errorf("expected ErrCounterWrap, got %v", err)
	}
}

func TestCalculateUtilization(t *testing.T) {
	util := CalculateUtilization(500_000_000, 300_000_000, 1000)
	if util < 49 || util > 51 {
		t.Errorf("expected ~50%%, got %f", util)
	}
}
