package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hfawgen-glitch/3x-ui/internal/web/entity"
)

type ProtocolHandler struct {
}

func NewProtocolHandler() *ProtocolHandler {
	return &ProtocolHandler{}
}

func (h *ProtocolHandler) GetProtocols(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	protocols := map[string]interface{}{
		"all": entity.AllProtocols,
		"proxy": map[string]interface{}{
			"protocols": entity.GetProxyProtocols(),
			"count":     len(entity.GetProxyProtocols()),
		},
		"tunnel": map[string]interface{}{
			"protocols": entity.GetTunnelProtocols(),
			"count":     len(entity.GetTunnelProtocols()),
		},
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": protocols})
}

func (h *ProtocolHandler) ValidateProtocol(w http.ResponseWriter, r *http.Request) {
	protocol := r.URL.Query().Get("protocol")

	w.Header().Set("Content-Type", "application/json")

	if entity.IsValidProtocol(entity.Protocol(protocol)) {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": true})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false})
	}
}

func (h *ProtocolHandler) ValidatePort(w http.ResponseWriter, r *http.Request) {
	portStr := r.URL.Query().Get("port")
	port, err := strconv.Atoi(portStr)

	w.Header().Set("Content-Type", "application/json")

	if err != nil || port < 1 || port > 65535 {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false, "error": "Invalid port"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"valid": true})
}
