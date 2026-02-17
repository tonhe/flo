package engine

import (
	"errors"
	"time"
)

// ErrCounterWrap indicates that an SNMP counter has wrapped around.
var ErrCounterWrap = errors.New("counter wrap detected")

// CounterSample holds raw SNMP counter values at a point in time.
type CounterSample struct {
	InOctets  uint64
	OutOctets uint64
	Timestamp time.Time
}

// RateSample holds calculated bit rates at a point in time.
type RateSample struct {
	Timestamp time.Time
	InRate    float64
	OutRate   float64
}

// CalculateRate computes the bit rate between two counter samples.
// Returns ErrCounterWrap if either counter has decreased.
func CalculateRate(prev, curr CounterSample) (RateSample, error) {
	elapsed := curr.Timestamp.Sub(prev.Timestamp).Seconds()
	if elapsed <= 0 {
		return RateSample{}, errors.New("zero or negative elapsed time")
	}

	if curr.InOctets < prev.InOctets || curr.OutOctets < prev.OutOctets {
		return RateSample{}, ErrCounterWrap
	}

	deltaIn := curr.InOctets - prev.InOctets
	deltaOut := curr.OutOctets - prev.OutOctets

	return RateSample{
		Timestamp: curr.Timestamp,
		InRate:    float64(deltaIn) * 8 / elapsed,
		OutRate:   float64(deltaOut) * 8 / elapsed,
	}, nil
}

// CalculateUtilization returns the utilization percentage given rates and
// interface speed in Mbps. It uses whichever direction (in/out) is higher.
func CalculateUtilization(inRate, outRate float64, speedMbps uint64) float64 {
	if speedMbps == 0 {
		return 0
	}
	maxRate := inRate
	if outRate > maxRate {
		maxRate = outRate
	}
	return maxRate / (float64(speedMbps) * 1_000_000) * 100
}
