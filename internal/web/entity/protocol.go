package entity

type Protocol string

const (
	ProtocolVLESS       Protocol = "vless"
	ProtocolVMess       Protocol = "vmess"
	ProtocolTrojan      Protocol = "trojan"
	ProtocolShadowsocks Protocol = "shadowsocks"
	ProtocolSocks5      Protocol = "socks5"
	ProtocolHTTP        Protocol = "http"
	ProtocolWireGuard   Protocol = "wireguard"
	ProtocolAWG2        Protocol = "awg2"
	ProtocolHysteria    Protocol = "hysteria"
	ProtocolHysteria2   Protocol = "hysteria2"
	ProtocolTUIC        Protocol = "tuic"
	ProtocolMixed       Protocol = "mixed"
)

var AllProtocols = []Protocol{
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

func IsValidProtocol(p Protocol) bool {
	for _, valid := range AllProtocols {
		if p == valid {
			return true
		}
	}
	return false
}

func GetProxyProtocols() []Protocol {
	return []Protocol{
		ProtocolVLESS,
		ProtocolVMess,
		ProtocolTrojan,
		ProtocolShadowsocks,
		ProtocolSocks5,
		ProtocolHTTP,
	}
}

func GetTunnelProtocols() []Protocol {
	return []Protocol{
		ProtocolWireGuard,
		ProtocolAWG2,
		ProtocolHysteria,
		ProtocolHysteria2,
		ProtocolTUIC,
	}
}
