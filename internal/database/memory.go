package database

import (
	"sync"
	"time"
)

// MemoryDB represents an in-memory database that resets on panel restart
type MemoryDB struct {
	inbounds   map[int]interface{}
	clients    map[int]interface{}
	traffic    map[string]interface{}
	mu         sync.RWMutex
	createdAt  time.Time
	lastReset  time.Time
}

// NewMemoryDB creates a new in-memory database instance
func NewMemoryDB() *MemoryDB {
	return &MemoryDB{
		inbounds:  make(map[int]interface{}),
		clients:   make(map[int]interface{}),
		traffic:   make(map[string]interface{}),
		createdAt: time.Now(),
		lastReset: time.Now(),
	}
}

// SaveInbound saves an inbound configuration to memory
func (m *MemoryDB) SaveInbound(id int, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inbounds[id] = data
	return nil
}

// GetInbound retrieves an inbound configuration from memory
func (m *MemoryDB) GetInbound(id int) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if data, exists := m.inbounds[id]; exists {
		return data, nil
	}
	return nil, ErrRecordNotFound
}

// DeleteInbound removes an inbound from memory
func (m *MemoryDB) DeleteInbound(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.inbounds, id)
	return nil
}

// SaveClient saves a client configuration to memory
func (m *MemoryDB) SaveClient(id int, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[id] = data
	return nil
}

// GetClient retrieves a client configuration from memory
func (m *MemoryDB) GetClient(id int) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if data, exists := m.clients[id]; exists {
		return data, nil
	}
	return nil, ErrRecordNotFound
}

// DeleteClient removes a client from memory
func (m *MemoryDB) DeleteClient(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, id)
	return nil
}

// AddTraffic records traffic data to memory
func (m *MemoryDB) AddTraffic(key string, upload, download uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.traffic[key]; !exists {
		m.traffic[key] = map[string]uint64{
			"upload":   0,
			"download": 0,
		}
	}
	
	traffic := m.traffic[key].(map[string]uint64)
	traffic["upload"] += upload
	traffic["download"] += download
	
	return nil
}

// GetTraffic retrieves traffic data from memory
func (m *MemoryDB) GetTraffic(key string) (map[string]uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if data, exists := m.traffic[key]; exists {
		traffic := data.(map[string]uint64)
		result := make(map[string]uint64)
		result["upload"] = traffic["upload"]
		result["download"] = traffic["download"]
		return result, nil
	}
	
	return map[string]uint64{"upload": 0, "download": 0}, nil
}

// ResetTraffic clears all traffic data
func (m *MemoryDB) ResetTraffic() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traffic = make(map[string]interface{})
	m.lastReset = time.Now()
	return nil
}

// Reset clears all in-memory data
func (m *MemoryDB) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inbounds = make(map[int]interface{})
	m.clients = make(map[int]interface{})
	m.traffic = make(map[string]interface{})
	m.lastReset = time.Now()
	return nil
}

// GetStats returns memory database statistics
func (m *MemoryDB) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"inbounds_count":  len(m.inbounds),
		"clients_count":   len(m.clients),
		"traffic_entries": len(m.traffic),
		"created_at":      m.createdAt,
		"last_reset":      m.lastReset,
		"uptime":          time.Since(m.createdAt).String(),
	}
}
