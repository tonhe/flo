package engine

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tonhe/flo/internal/dashboard"
	"github.com/tonhe/flo/internal/identity"
)

// Poller runs a polling loop for a single dashboard, collecting SNMP metrics
// from all configured targets at the dashboard's configured interval.
type Poller struct {
	mu           sync.RWMutex
	dash         *dashboard.Dashboard
	provider     identity.Provider
	clients      map[string]*gosnmp.GoSNMP
	data         map[string]*TargetStats
	prevCounters map[string]map[int]CounterSample
	subscribers  []chan EngineEvent
	stopCh       chan struct{}
	pollCount    int
	errorCount   int
	lastPoll     time.Time
}

// NewPoller creates a Poller for the given dashboard and identity provider.
func NewPoller(dash *dashboard.Dashboard, provider identity.Provider) (*Poller, error) {
	p := &Poller{
		dash:         dash,
		provider:     provider,
		clients:      make(map[string]*gosnmp.GoSNMP),
		data:         make(map[string]*TargetStats),
		prevCounters: make(map[string]map[int]CounterSample),
		stopCh:       make(chan struct{}),
	}
	return p, nil
}

// initTargetStats pre-populates empty stats for all configured targets and
// interfaces without performing any SNMP calls. This allows the UI to render
// immediately while the first poll runs asynchronously.
func (p *Poller) initTargetStats() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, group := range p.dash.Groups {
		for _, target := range group.Targets {
			ts := &TargetStats{
				Host:  target.Host,
				Label: target.Label,
			}
			for _, ifName := range target.Interfaces {
				ts.Interfaces = append(ts.Interfaces, InterfaceStats{
					Name:    ifName,
					Status:  "",
					History: NewRingBuffer[RateSample](p.dash.MaxHistory),
				})
			}
			p.data[target.Host] = ts
		}
	}
	p.notify()
}

// Run starts the polling loop. It blocks until Stop is called.
// It pre-populates empty stats so the UI can render immediately,
// then kicks off the first poll asynchronously.
func (p *Poller) Run() {
	p.initTargetStats()

	ticker := time.NewTicker(p.dash.Interval)
	defer ticker.Stop()

	go p.poll()

	for {
		select {
		case <-ticker.C:
			p.poll()
		case <-p.stopCh:
			p.cleanup()
			return
		}
	}
}

// poll executes a single poll cycle across all targets.
func (p *Poller) poll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, group := range p.dash.Groups {
		for _, target := range group.Targets {
			p.pollTarget(group.Name, target)
		}
	}

	p.pollCount++
	p.lastPoll = time.Now()
	p.notify()
}

// pollTarget collects SNMP counters for a single target and updates stats.
func (p *Poller) pollTarget(groupName string, target dashboard.Target) {
	client, err := p.getOrCreateClient(target)
	if err != nil {
		p.setTargetError(target, err)
		return
	}

	ts := p.getOrCreateTargetStats(target)
	now := time.Now()

	for i, iface := range ts.Interfaces {
		counters, err := p.getInterfaceCounters(client, iface.IfIndex)
		if err != nil {
			ts.Interfaces[i].PollError = err
			p.errorCount++
			continue
		}

		status, _ := p.getInterfaceStatus(client, iface.IfIndex)
		ts.Interfaces[i].Status = status

		prevKey := target.Host
		if p.prevCounters[prevKey] != nil {
			if prev, ok := p.prevCounters[prevKey][iface.IfIndex]; ok {
				rate, err := CalculateRate(prev, counters)
				if err == nil {
					ts.Interfaces[i].InRate = rate.InRate
					ts.Interfaces[i].OutRate = rate.OutRate
					ts.Interfaces[i].Utilization = CalculateUtilization(rate.InRate, rate.OutRate, iface.Speed)
					ts.Interfaces[i].History.Add(rate)
				}
			}
		}

		if p.prevCounters[prevKey] == nil {
			p.prevCounters[prevKey] = make(map[int]CounterSample)
		}
		p.prevCounters[prevKey][iface.IfIndex] = counters

		ts.Interfaces[i].LastPoll = now
		ts.Interfaces[i].PollError = nil
	}

	ts.LastPoll = now
	ts.PollError = nil
}

// getOrCreateClient returns an existing SNMP client or creates a new one.
func (p *Poller) getOrCreateClient(target dashboard.Target) (*gosnmp.GoSNMP, error) {
	if client, ok := p.clients[target.Host]; ok {
		return client, nil
	}

	id, err := p.provider.Get(target.Identity)
	if err != nil {
		return nil, err
	}

	client, err := NewSNMPClient(target.Host, target.Port, id, 5*time.Second)
	if err != nil {
		return nil, err
	}

	if err := client.Connect(); err != nil {
		return nil, err
	}

	p.clients[target.Host] = client
	return client, nil
}

// getOrCreateTargetStats returns existing stats or initializes new ones.
// On first call for a target, it performs SNMP walks to resolve interface
// names to ifIndex values and discover interface speeds.
// If stats were pre-populated by initTargetStats (IfIndex == 0), the
// interface resolution is performed on this call.
func (p *Poller) getOrCreateTargetStats(target dashboard.Target) *TargetStats {
	if ts, ok := p.data[target.Host]; ok {
		// If interfaces haven't been resolved yet (pre-populated), resolve now
		if len(ts.Interfaces) > 0 && ts.Interfaces[0].IfIndex == 0 {
			resolved := p.resolveInterfaces(target)
			for i, iface := range ts.Interfaces {
				if info, found := resolved[iface.Name]; found {
					ts.Interfaces[i].IfIndex = info.IfIndex
					ts.Interfaces[i].Speed = info.Speed
					ts.Interfaces[i].Description = info.Description
				}
			}
		}
		return ts
	}

	// Fallback: create new (should not happen after initTargetStats)
	ts := &TargetStats{
		Host:  target.Host,
		Label: target.Label,
	}

	resolved := p.resolveInterfaces(target)

	for _, ifName := range target.Interfaces {
		stats := InterfaceStats{
			Name:    ifName,
			History: NewRingBuffer[RateSample](p.dash.MaxHistory),
		}
		if info, ok := resolved[ifName]; ok {
			stats.IfIndex = info.IfIndex
			stats.Speed = info.Speed
			stats.Description = info.Description
		}
		ts.Interfaces = append(ts.Interfaces, stats)
	}
	p.data[target.Host] = ts
	return ts
}

// resolveInterfaces walks ifName, ifDescr, and ifHighSpeed on the target
// to build a mapping from interface name -> (ifIndex, speed, description).
// This is needed because dashboard configs reference interfaces by name,
// but SNMP counters are indexed by ifIndex.
func (p *Poller) resolveInterfaces(target dashboard.Target) map[string]DiscoveredInterface {
	result := make(map[string]DiscoveredInterface)

	client, err := p.getOrCreateClient(target)
	if err != nil {
		return result
	}

	// Build ifIndex -> DiscoveredInterface from walks
	byIndex := make(map[int]*DiscoveredInterface)

	walkOID(client, OIDifName, func(idx int, val string) {
		if _, ok := byIndex[idx]; !ok {
			byIndex[idx] = &DiscoveredInterface{IfIndex: idx}
		}
		byIndex[idx].Name = val
	})

	walkOID(client, OIDifDescr, func(idx int, val string) {
		if _, ok := byIndex[idx]; !ok {
			byIndex[idx] = &DiscoveredInterface{IfIndex: idx}
		}
		if byIndex[idx].Name == "" {
			byIndex[idx].Name = val
		}
		byIndex[idx].Description = val
	})

	walkOID(client, OIDifHighSpeed, func(idx int, val string) {
		if iface, ok := byIndex[idx]; ok {
			speed, _ := strconv.ParseUint(val, 10, 64)
			iface.Speed = speed
		}
	})

	// Map by name (exact match) and also by description (fallback)
	for _, iface := range byIndex {
		result[iface.Name] = *iface
		if iface.Description != "" && iface.Description != iface.Name {
			result[iface.Description] = *iface
		}
	}

	return result
}

// getInterfaceCounters fetches HC in/out octet counters for a single interface.
func (p *Poller) getInterfaceCounters(client *gosnmp.GoSNMP, ifIndex int) (CounterSample, error) {
	oids := []string{
		fmt.Sprintf("%s.%d", OIDifHCInOctets, ifIndex),
		fmt.Sprintf("%s.%d", OIDifHCOutOctets, ifIndex),
	}

	result, err := client.Get(oids)
	if err != nil {
		return CounterSample{}, err
	}

	cs := CounterSample{Timestamp: time.Now()}
	for _, v := range result.Variables {
		val := gosnmp.ToBigInt(v.Value).Uint64()
		if v.Name == oids[0] || v.Name == "."+oids[0] {
			cs.InOctets = val
		} else {
			cs.OutOctets = val
		}
	}
	return cs, nil
}

// getInterfaceStatus fetches the operational status of an interface.
func (p *Poller) getInterfaceStatus(client *gosnmp.GoSNMP, ifIndex int) (string, error) {
	oid := fmt.Sprintf("%s.%d", OIDifOperStatus, ifIndex)

	result, err := client.Get([]string{oid})
	if err != nil {
		return "unknown", err
	}

	if len(result.Variables) > 0 {
		val := gosnmp.ToBigInt(result.Variables[0].Value).Int64()
		switch val {
		case 1:
			return "up", nil
		case 2:
			return "down", nil
		case 3:
			return "testing", nil
		default:
			return "unknown", nil
		}
	}
	return "unknown", nil
}

// setTargetError records a poll error for a target.
func (p *Poller) setTargetError(target dashboard.Target, err error) {
	ts := p.getOrCreateTargetStats(target)
	ts.PollError = err
	p.errorCount++
}

// Snapshot returns a point-in-time copy of all dashboard data.
// This method acquires a read lock and is safe to call from any goroutine.
func (p *Poller) Snapshot() *DashboardSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshotLocked()
}

// snapshotLocked builds a DashboardSnapshot without acquiring any lock.
// The caller must hold at least a read lock on p.mu.
func (p *Poller) snapshotLocked() *DashboardSnapshot {
	snap := &DashboardSnapshot{
		Name:      p.dash.Name,
		LastPoll:  p.lastPoll,
		PollCount: p.pollCount,
	}

	for _, group := range p.dash.Groups {
		gs := GroupSnapshot{Name: group.Name}
		for _, target := range group.Targets {
			if ts, ok := p.data[target.Host]; ok {
				gs.Targets = append(gs.Targets, *ts)
			}
		}
		snap.Groups = append(snap.Groups, gs)
	}
	return snap
}

// Subscribe returns a channel that receives an event after each poll cycle.
func (p *Poller) Subscribe() <-chan EngineEvent {
	ch := make(chan EngineEvent, 1)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subscribers = append(p.subscribers, ch)
	return ch
}

// notify sends the current snapshot to all subscribers (non-blocking).
// Must be called while holding the write lock on p.mu.
func (p *Poller) notify() {
	snap := p.snapshotLocked()
	event := EngineEvent{DashboardName: p.dash.Name, Snapshot: snap}
	for _, ch := range p.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// Info returns summary information about this engine.
func (p *Poller) Info() EngineInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return EngineInfo{
		Name:       p.dash.Name,
		State:      EngineRunning,
		LastPoll:   p.lastPoll,
		PollCount:  p.pollCount,
		ErrorCount: p.errorCount,
	}
}

// Stop signals the polling loop to exit.
func (p *Poller) Stop() {
	close(p.stopCh)
}

// cleanup closes all SNMP connections.
func (p *Poller) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, client := range p.clients {
		if client.Conn != nil {
			client.Conn.Close()
		}
	}
}
