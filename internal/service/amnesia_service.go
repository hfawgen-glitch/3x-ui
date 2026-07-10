package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/hfawgen-glitch/3x-ui/internal/config"
	"github.com/hfawgen-glitch/3x-ui/internal/database"
)

type AmnesiaService struct {
	db              *database.MemoryDB
	mu              sync.RWMutex
	configured      bool
	autoResetTicker *time.Ticker
	cleanupTicker   *time.Ticker
}

func NewAmnesiaService(db *database.MemoryDB) *AmnesiaService {
	return &AmnesiaService{
		db:         db,
		configured: false,
	}
}

func (as *AmnesiaService) Initialize() error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if !config.Amnesia.Enabled {
		return nil
	}

	if config.Amnesia.AutoResetInterval > 0 {
		as.autoResetTicker = time.NewTicker(time.Duration(config.Amnesia.AutoResetInterval) * time.Second)
		go as.autoResetLoop()
	}

	if config.Amnesia.AutoCleanup {
		as.cleanupTicker = time.NewTicker(time.Duration(config.Amnesia.CleanupInterval) * time.Second)
		go as.cleanupLoop()
	}

	as.configured = true
	return nil
}

func (as *AmnesiaService) Shutdown() {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.autoResetTicker != nil {
		as.autoResetTicker.Stop()
	}

	if as.cleanupTicker != nil {
		as.cleanupTicker.Stop()
	}
}

func (as *AmnesiaService) IsEnabled() bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return config.Amnesia.Enabled && as.configured
}

func (as *AmnesiaService) CreateInbound(name, protocol string, port int, settings map[string]interface{}) (*database.InboundData, error) {
	if !config.Amnesia.IsProtocolSupported(protocol) {
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	inbound := &database.InboundData{
		ID:        int(time.Now().Unix()%1000000) + port,
		Name:      name,
		Protocol:  protocol,
		Port:      port,
		Settings:  settings,
		Clients:   []string{},
		Enable:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := as.db.SaveInbound(inbound.ID, inbound); err != nil {
		return nil, err
	}

	return inbound, nil
}

func (as *AmnesiaService) UpdateInbound(id int, name string, settings map[string]interface{}) error {
	inbound, err := as.db.GetInbound(id)
	if err != nil {
		return err
	}

	inboundData := inbound.(*database.InboundData)
	inboundData.Name = name
	inboundData.Settings = settings
	inboundData.UpdatedAt = time.Now()

	return as.db.SaveInbound(id, inboundData)
}

func (as *AmnesiaService) AddClient(inboundID int, email, protocol string, limitIP, limitSpeed int) (*database.ClientData, error) {
	inbound, err := as.db.GetInbound(inboundID)
	if err != nil {
		return nil, err
	}

	inboundData := inbound.(*database.InboundData)

	client := &database.ClientData{
		ID:           fmt.Sprintf("%s_%d", email, time.Now().UnixNano()),
		InboundID:    inboundID,
		Email:        email,
		Protocol:     protocol,
		LimitIP:      limitIP,
		LimitSpeed:   limitSpeed,
		TrafficUp:    0,
		TrafficDown:  0,
		ExpiryTime:   0,
		Enable:       true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		ProtocolData: make(map[string]interface{}),
	}

	if err := as.db.SaveClient(client.ID, inboundID, client); err != nil {
		return nil, err
	}

	inboundData.Clients = append(inboundData.Clients, client.ID)
	if err := as.db.SaveInbound(inboundID, inboundData); err != nil {
		as.db.DeleteClient(client.ID, inboundID)
		return nil, err
	}

	return client, nil
}

func (as *AmnesiaService) RecordTraffic(key string, upload, download uint64) error {
	return as.db.AddTraffic(key, upload, download)
}

func (as *AmnesiaService) GetInboundTraffic(inboundID int) (map[string]uint64, error) {
	key := fmt.Sprintf("inbound_%d", inboundID)
	return as.db.GetTraffic(key)
}

func (as *AmnesiaService) GetClientTraffic(clientID string, inboundID int) (map[string]uint64, error) {
	key := fmt.Sprintf("client_%d_%s", inboundID, clientID)
	return as.db.GetTraffic(key)
}

func (as *AmnesiaService) Reset() error {
	if config.Amnesia.EnableBackup {
		if _, err := as.db.Backup(); err != nil {
			return err
		}
	}

	return as.db.Reset()
}

func (as *AmnesiaService) autoResetLoop() {
	if as.autoResetTicker == nil {
		return
	}

	for range as.autoResetTicker.C {
		as.Reset()
	}
}

func (as *AmnesiaService) cleanupLoop() {
	if as.cleanupTicker == nil {
		return
	}

	for range as.cleanupTicker.C {
		clients, _ := as.db.GetAllInbounds()
		for _, inbound := range clients {
			for _, clientID := range inbound.Clients {
				client, err := as.db.GetClient(clientID, inbound.ID)
				if err != nil {
					continue
				}

				clientData := client.(*database.ClientData)
				if clientData.ExpiryTime > 0 && clientData.ExpiryTime < time.Now().Unix() {
					as.db.DeleteClient(clientID, inbound.ID)
				}
			}
		}
	}
}
