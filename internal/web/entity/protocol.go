package entity

// Protocol represents supported protocols in the panel
type Protocol string

const (
	// Proxy Protocols
	ProtocolVLESS       Protocol = "vless"
	ProtocolVMess       Protocol = "vmess"
	ProtocolTrojan      Protocol = "trojan"
	ProtocolShadowsocks Protocol = "shadowsocks"
	ProtocolSocks5      Protocol = "socks5"
	ProtocolHTTP        Protocol = "http"
	
	// WireGuard and tunneling protocols
	ProtocolWireGuard   Protocol = "wireguard"
	ProtocolAWG2        Protocol = "awg2"
	ProtocolHysteria    Protocol = "hysteria"
	ProtocolHysteria2   Protocol = "hysteria2"
	ProtocolTUIC        Protocol = "tuic"
	
	// Mixed/Multi protocol
	ProtocolMixed       Protocol = "mixed"
)

// ProtocolConfig represents the configuration for a specific protocol
type ProtocolConfig struct {
	Protocol   Protocol `json:"protocol"`
	Port       int      `json:"port"`
	Settings   map[string]interface{} `json:"settings"`
	StreamSettings StreamSettings `json:"stream_settings,omitempty"`
}

// StreamSettings represents stream/transport settings
type StreamSettings struct {
	Network    string                 `json:"network"`
	Security   string                 `json:"security"`
	TLSSettings map[string]interface{} `json:"tls_settings,omitempty"`
	WSSettings map[string]interface{} `json:"ws_settings,omitempty"`
	GRPCSettings map[string]interface{} `json:"grpc_settings,omitempty"`
	HTTPSettings map[string]interface{} `json:"http_settings,omitempty"`
}

// InboundConfig represents an inbound configuration with protocol support
type InboundConfig struct {
	ID            int              `json:"id"`
	Name          string           `json:"name"`
	Protocol      Protocol         `json:"protocol"`
	Port          int              `json:"port"`
	Protocol_      ProtocolConfig   `json:"protocol_config"`
	Settings      map[string]interface{} `json:"settings"`
	Clients       []ClientConfig   `json:"clients,omitempty"`
	Traffic       TrafficData      `json:"traffic,omitempty"`
	Remark        string           `json:"remark"`
	Enable        bool             `json:"enable"`
	CreatedAt     int64            `json:"created_at"`
	UpdatedAt     int64            `json:"updated_at"`
}

// ClientConfig represents a client connected to an inbound
type ClientConfig struct {
	ID           int               `json:"id"`
	InboundID    int               `json:"inbound_id"`
	Email        string            `json:"email"`
	Protocol     Protocol          `json:"protocol"`
	LimitIP      int               `json:"limit_ip"`
	LimitSpeed   int               `json:"limit_speed"`
	Traffic      TrafficData       `json:"traffic"`
	ExpiryTime   int64             `json:"expiry_time"`
	Enable       bool              `json:"enable"`
	CreatedAt    int64             `json:"created_at"`
	UpdatedAt    int64             `json:"updated_at"`
	ProtocolData map[string]interface{} `json:"protocol_data"` // Protocol-specific configuration
}

// TrafficData represents traffic statistics
type TrafficData struct {
	Upload      uint64 `json:"upload"`
	Download    uint64 `json:"download"`
	Total       uint64 `json:"total"`
	LastUpdated int64  `json:"last_updated"`
}

// IsValidProtocol checks if a protocol is valid
func IsValidProtocol(p Protocol) bool {
	validProtocols := []Protocol{
		ProtocolVLESS,
		ProtocolVMess,
		ProtocolTrojan,
		ProtocolShadowsocks,
		ProtocolSocks5,
		ProtocolHTTP,
		ProtocolWireGuard,
		ProtocolAWG2,
		ProtocolHysteria,
		ProtocolHysteria2,
		ProtocolTUIC,
		ProtocolMixed,
	}
	
	for _, valid := range validProtocols {
		if p == valid {
			return true
		}
	}
	return false
}
