package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/hfawgen-glitch/3x-ui/internal/config"
	"github.com/hfawgen-glitch/3x-ui/internal/service"
)

type AmnesiaHandler struct {
	amnesiaService *service.AmnesiaService
}

func NewAmnesiaHandler(amnesiaService *service.AmnesiaService) *AmnesiaHandler {
	return &AmnesiaHandler{
		amnesiaService: amnesiaService,
	}
}

func (h *AmnesiaHandler) CreateInbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name     string                 `json:"name"`
		Protocol string                 `json:"protocol"`
		Port     int                    `json:"port"`
		Settings map[string]interface{} `json:"settings"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request"})
		return
	}

	inbound, err := h.amnesiaService.CreateInbound(req.Name, req.Protocol, req.Port, req.Settings)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": inbound})
}

func (h *AmnesiaHandler) AddClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	inboundID, err := strconv.Atoi(r.URL.Query().Get("inbound_id"))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid inbound ID"})
		return
	}

	var req struct {
		Email      string `json:"email"`
		Protocol   string `json:"protocol"`
		LimitIP    int    `json:"limit_ip"`
		LimitSpeed int    `json:"limit_speed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request"})
		return
	}

	client, err := h.amnesiaService.AddClient(inboundID, req.Email, req.Protocol, req.LimitIP, req.LimitSpeed)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": client})
}

func (h *AmnesiaHandler) GetTraffic(w http.ResponseWriter, r *http.Request) {
	inboundID, err := strconv.Atoi(r.URL.Query().Get("inbound_id"))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid inbound ID"})
		return
	}

	traffic, err := h.amnesiaService.GetInboundTraffic(inboundID)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": traffic})
}

func (h *AmnesiaHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    h.amnesiaService.db.GetStats(),
	})
}

func (h *AmnesiaHandler) Reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.amnesiaService.Reset(); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Reset successful"})
}

func (h *AmnesiaHandler) GetProtocols(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	protocols := map[string]interface{}{
		"enabled":              config.Amnesia.Enabled,
		"supported_protocols":  config.Amnesia.SupportedProtocols,
		"default_protocol":     config.Amnesia.DefaultProtocol,
		"warning_message":      config.Amnesia.WarningMessage,
		"proxy_protocols":      []string{"vless", "vmess", "trojan", "shadowsocks", "socks5", "http"},
		"tunnel_protocols":     []string{"wireguard", "awg2", "hysteria", "hysteria2", "tuic"},
		"multi_protocols":      []string{"mixed"},
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": protocols})
}
