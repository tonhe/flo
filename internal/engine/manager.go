package engine

import (
	"fmt"
	"sync"

	"github.com/tonhe/flo/internal/dashboard"
	"github.com/tonhe/flo/internal/identity"
)

// Manager coordinates multiple Pollers, one per dashboard.
type Manager struct {
	mu      sync.RWMutex
	engines map[string]*Poller
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{
		engines: make(map[string]*Poller),
	}
}

// Start creates and launches a Poller for the given dashboard.
func (m *Manager) Start(dash *dashboard.Dashboard, provider identity.Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.engines[dash.Name]; exists {
		return fmt.Errorf("engine %q already running", dash.Name)
	}

	p, err := NewPoller(dash, provider)
	if err != nil {
		return err
	}

	m.engines[dash.Name] = p
	go p.Run()
	return nil
}

// Stop halts the Poller for the named dashboard and removes it.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.engines[name]
	if !ok {
		return fmt.Errorf("engine %q not found", name)
	}

	p.Stop()
	delete(m.engines, name)
	return nil
}

// GetSnapshot returns a point-in-time snapshot for the named dashboard.
func (m *Manager) GetSnapshot(name string) (*DashboardSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.engines[name]
	if !ok {
		return nil, fmt.Errorf("engine %q not found", name)
	}
	return p.Snapshot(), nil
}

// Subscribe returns a channel that receives events for the named dashboard.
func (m *Manager) Subscribe(name string) (<-chan EngineEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.engines[name]
	if !ok {
		return nil, fmt.Errorf("engine %q not found", name)
	}
	return p.Subscribe(), nil
}

// ListEngines returns summary info for all running engines.
func (m *Manager) ListEngines() []EngineInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]EngineInfo, 0, len(m.engines))
	for _, p := range m.engines {
		infos = append(infos, p.Info())
	}
	return infos
}

// StopAll halts and removes all running engines.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, p := range m.engines {
		p.Stop()
		delete(m.engines, name)
	}
}
