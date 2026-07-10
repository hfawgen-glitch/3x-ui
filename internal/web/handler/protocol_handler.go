package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hfawgen-glitch/3x-ui/internal/web/entity"
)

// ProtocolHandler handles protocol-related operations
type ProtocolHandler struct {
	memoryDB interface{}
}

// NewProtocolHandler creates a new protocol handler
func NewProtocolHandler(db interface{}) *ProtocolHandler {
	return &ProtocolHandler{
		memoryDB: db,
	}
}

// CreateInboundWithProtocol creates a new inbound with protocol selection
func (h *ProtocolHandler) CreateInboundWithProtocol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config entity.InboundConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !entity.IsValidProtocol(config.Protocol) {
		http.Error(w, "Invalid protocol", http.StatusBadRequest)
		return
	}

	if err := validateProtocolConfig(config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.saveInbound(config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Inbound created successfully",
		"data":    config,
	})
}

// AddClientToInbound adds a client to an inbound
func (h *ProtocolHandler) AddClientToInbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	inboundID, err := strconv.Atoi(r.URL.Query().Get("inbound_id"))
	if err != nil {
		http.Error(w, "Invalid inbound ID", http.StatusBadRequest)
		return
	}

	var client entity.ClientConfig
	if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	client.InboundID = inboundID

	if client.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	if err := h.saveClient(client); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Client added successfully",
		"data":    client,
	})
}

// GetTraffic retrieves traffic statistics
func (h *ProtocolHandler) GetTraffic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Traffic key is required", http.StatusBadRequest)
		return
	}

	traffic := h.getTraffic(key)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    traffic,
	})
}

// GetProtocolList returns list of supported protocols
func (h *ProtocolHandler) GetProtocolList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	protocols := map[string]interface{}{
		"proxy_protocols": []map[string]string{
			{"id": "vless", "name": "VLESS"},
			{"id": "vmess", "name": "VMess"},
			{"id": "trojan", "name": "Trojan"},
			{"id": "shadowsocks", "name": "Shadowsocks"},
			{"id": "socks5", "name": "Socks5"},
			{"id": "http", "name": "HTTP"},
		},
		"tunnel_protocols": []map[string]string{
			{"id": "wireguard", "name": "WireGuard"},
			{"id": "awg2", "name": "AWG2"},
			{"id": "hysteria", "name": "Hysteria"},
			{"id": "hysteria2", "name": "Hysteria2"},
			{"id": "tuic", "name": "TUIC"},
		},
		"multi_protocol": []map[string]string{
			{"id": "mixed", "name": "Mixed"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    protocols,
	})
}

// Helper functions
func (h *ProtocolHandler) saveInbound(config entity.InboundConfig) error {
	return nil
}

func (h *ProtocolHandler) saveClient(client entity.ClientConfig) error {
	return nil
}

func (h *ProtocolHandler) getTraffic(key string) map[string]interface{} {
	return map[string]interface{}{
		"upload":   0,
		"download": 0,
	}
}

func validateProtocolConfig(config entity.InboundConfig) error {
	switch config.Protocol {
	case entity.ProtocolAWG2:
		if config.Port < 1 || config.Port > 65535 {
			return ErrInvalidPort
		}
	case entity.ProtocolWireGuard:
		if config.Port < 1 || config.Port > 65535 {
			return ErrInvalidPort
		}
	case entity.ProtocolVLESS, entity.ProtocolVMess, entity.ProtocolTrojan:
		if config.Port < 1 || config.Port > 65535 {
			return ErrInvalidPort
		}
	}
	return nil
}

var ErrInvalidPort = &Error{"invalid port"}

type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}
