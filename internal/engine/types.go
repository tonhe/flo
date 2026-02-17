package engine

import "time"

// InterfaceStats holds the current state and metrics for a single interface.
type InterfaceStats struct {
	IfIndex     int
	Name        string
	Description string
	Speed       uint64 // Mbps
	Status      string // "up", "down", "testing"
	InRate      float64
	OutRate     float64
	Utilization float64
	History     *RingBuffer[RateSample]
	PollError   error
	LastPoll    time.Time
}

// TargetStats holds the current state and metrics for a single SNMP target.
type TargetStats struct {
	Host       string
	Label      string
	Interfaces []InterfaceStats
	PollError  error
	LastPoll   time.Time
}

// DashboardSnapshot is a point-in-time view of all targets in a dashboard.
type DashboardSnapshot struct {
	Name      string
	Groups    []GroupSnapshot
	LastPoll  time.Time
	PollCount int
}

// GroupSnapshot is a point-in-time view of a target group.
type GroupSnapshot struct {
	Name    string
	Targets []TargetStats
}

// EngineState represents the lifecycle state of a polling engine.
type EngineState int

const (
	EngineStopped EngineState = iota
	EngineRunning
	EngineError
)

// EngineInfo provides summary information about a running engine.
type EngineInfo struct {
	Name       string
	State      EngineState
	LastPoll   time.Time
	PollCount  int
	ErrorCount int
}

// EngineEvent is emitted to subscribers after each poll cycle.
type EngineEvent struct {
	DashboardName string
	Snapshot      *DashboardSnapshot
}
