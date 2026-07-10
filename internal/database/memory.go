package database

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type MemoryDB struct {
	inbounds     sync.Map
	clients      sync.Map
	traffic      sync.Map
	createdAt    time.Time
	lastReset    time.Time
	inboundCount int64
	clientCount  int64
	trafficCount int64
}

func NewMemoryDB() *MemoryDB {
	return &MemoryDB{
		createdAt: time.Now(),
		lastReset: time.Now(),
	}
}

type InboundData struct {
	ID            int                   `json:"id"`
	Name          string                `json:"name"`
	Protocol      string                `json:"protocol"`
	Port          int                   `json:"port"`
	Settings      map[string]interface{} `json:"settings"`
	Clients       []string              `json:"clients"`
	Enable        bool                  `json:"enable"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

type ClientData struct {
	ID           string                `json:"id"`
	InboundID    int                   `json:"inbound_id"`
	Email        string                `json:"email"`
	Protocol     string                `json:"protocol"`
	LimitIP      int                   `json:"limit_ip"`
	LimitSpeed   int                   `json:"limit_speed"`
	TrafficUp    uint64                `json:"traffic_up"`
	TrafficDown  uint64                `json:"traffic_down"`
	ExpiryTime   int64                 `json:"expiry_time"`
	Enable       bool                  `json:"enable"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	ProtocolData map[string]interface{} `json:"protocol_data"`
}

type TrafficEntry struct {
	Up        uint64    `json:"up"`
	Down      uint64    `json:"down"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *MemoryDB) SaveInbound(id int, data interface{}) error {
	inbound, ok := data.(*InboundData)
	if !ok {
		return ErrInvalidData
	}

	inbound.UpdatedAt = time.Now()

	key := fmt.Sprintf("inbound_%d", id)
	_, loaded := m.inbounds.LoadOrStore(key, inbound)
	if loaded {
		m.inbounds.Store(key, inbound)
	} else {
		atomic.AddInt64(&m.inboundCount, 1)
	}

	return nil
}

func (m *MemoryDB) GetInbound(id int) (interface{}, error) {
	key := fmt.Sprintf("inbound_%d", id)
	data, exists := m.inbounds.Load(key)
	if !exists {
		return nil, ErrRecordNotFound
	}


inbound := data.(*InboundData)
	copy := *inbound
	return &copy, nil
}

func (m *MemoryDB) GetAllInbounds() ([]InboundData, error) {
	var inbounds []InboundData
	m.inbounds.Range(func(key, value interface{}) bool {
		if inbound, ok := value.(*InboundData); ok {
			copy := *inbound
			inbounds = append(inbounds, copy)
		}
		return true
	})
	return inbounds, nil
}

func (m *MemoryDB) DeleteInbound(id int) error {
	key := fmt.Sprintf("inbound_%d", id)
	if _, exists := m.inbounds.LoadAndDelete(key); exists {
		atomic.AddInt64(&m.inboundCount, -1)
		return nil
	}
	return ErrRecordNotFound
}

func (m *MemoryDB) SaveClient(id string, inboundID int, data interface{}) error {
	client, ok := data.(*ClientData)
	if !ok {
		return ErrInvalidData
	}

	client.ID = id
	client.InboundID = inboundID
	client.UpdatedAt = time.Now()

	key := fmt.Sprintf("client_%d_%s", inboundID, id)
	_, loaded := m.clients.LoadOrStore(key, client)
	if loaded {
		m.clients.Store(key, client)
	} else {
		atomic.AddInt64(&m.clientCount, 1)
	}

	return nil
}

func (m *MemoryDB) GetClient(id string, inboundID int) (interface{}, error) {
	key := fmt.Sprintf("client_%d_%s", inboundID, id)
	data, exists := m.clients.Load(key)
	if !exists {
		return nil, ErrRecordNotFound
	}

	client := data.(*ClientData)
	copy := *client
	return &copy, nil
}

func (m *MemoryDB) GetClientsByInbound(inboundID int) ([]ClientData, error) {
	var clients []ClientData
	m.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*ClientData); ok && client.InboundID == inboundID {
			copy := *client
			clients = append(clients, copy)
		}
		return true
	})
	return clients, nil
}

func (m *MemoryDB) DeleteClient(id string, inboundID int) error {
	key := fmt.Sprintf("client_%d_%s", inboundID, id)
	if _, exists := m.clients.LoadAndDelete(key); exists {
		atomic.AddInt64(&m.clientCount, -1)
		return nil
	}
	return ErrRecordNotFound
}

func (m *MemoryDB) AddTraffic(key string, upload, download uint64) error {
	traffic, ok := m.traffic.Load(key)
	if !ok {
		traffic = &TrafficEntry{
			Up:        upload,
			Down:      download,
			UpdatedAt: time.Now(),
		}
		m.traffic.Store(key, traffic)
		atomic.AddInt64(&m.trafficCount, 1)
	} else {
		te := traffic.(*TrafficEntry)
		te.Up += upload
		te.Down += download
		te.UpdatedAt = time.Now()
	}

	return nil
}

func (m *MemoryDB) GetTraffic(key string) (map[string]uint64, error) {
	data, exists := m.traffic.Load(key)
	if !exists {
		return map[string]uint64{"up": 0, "down": 0}, nil
	}

	te := data.(*TrafficEntry)
	return map[string]uint64{"up": te.Up, "down": te.Down}, nil
}

func (m *MemoryDB) GetAllTraffic() map[string]map[string]uint64 {
	result := make(map[string]map[string]uint64)
	m.traffic.Range(func(key, value interface{}) bool {
		if te, ok := value.(*TrafficEntry); ok {
			result[key.(string)] = map[string]uint64{
				"up":   te.Up,
				"down": te.Down,
			}
		}
		return true
	})
	return result
}

func (m *MemoryDB) ResetTraffic() error {
	m.traffic.Range(func(key, value interface{}) bool {
		m.traffic.Delete(key)
		atomic.AddInt64(&m.trafficCount, -1)
		return true
	})
	m.lastReset = time.Now()
	return nil
}

func (m *MemoryDB) Reset() error {
	m.inbounds.Range(func(key, value interface{}) bool {
		m.inbounds.Delete(key)
		return true
	})
	m.clients.Range(func(key, value interface{}) bool {
		m.clients.Delete(key)
		return true
	})
	m.traffic.Range(func(key, value interface{}) bool {
		m.traffic.Delete(key)
		return true
	})

	atomic.StoreInt64(&m.inboundCount, 0)
	atomic.StoreInt64(&m.clientCount, 0)
	atomic.StoreInt64(&m.trafficCount, 0)
	m.lastReset = time.Now()

	return nil
}

func (m *MemoryDB) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"inbounds_count":  atomic.LoadInt64(&m.inboundCount),
		"clients_count":   atomic.LoadInt64(&m.clientCount),
		"traffic_entries": atomic.LoadInt64(&m.trafficCount),
		"created_at":      m.createdAt,
		"last_reset":      m.lastReset,
		"uptime":          time.Since(m.createdAt).String(),
	}
}

func (m *MemoryDB) Export() (map[string]interface{}, error) {
	var inbounds []InboundData
	m.inbounds.Range(func(key, value interface{}) bool {
		if inbound, ok := value.(*InboundData); ok {
			copy := *inbound
			inbounds = append(inbounds, copy)
		}
		return true
	})

	var clients []ClientData
	m.clients.Range(func(key, value interface{}) bool {
		if client, ok := value.(*ClientData); ok {
			copy := *client
			clients = append(clients, copy)
		}
		return true
	})

	traffic := m.GetAllTraffic()

	return map[string]interface{}{
		"inbounds": inbounds,
		"clients":  clients,
		"traffic":  traffic,
		"stats":    m.GetStats(),
	}, nil
}

func (m *MemoryDB) Backup() (string, error) {
	export, err := m.Export()
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
