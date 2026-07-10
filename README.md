# 3X-UI - Enhanced Version with AWG2 Protocol and Amnesia Mode

A web-based control panel for Xray-core with advanced features including multi-protocol support, in-memory amnesia mode, and AWG2 (Adguard WireGuard 2) integration.

## 🚀 New Features

### AWG2 Protocol Support
- Full support for AWG2 (Adguard WireGuard 2) protocol
- Protocol selector in UI for easy switching between:
  - **Proxy Protocols**: VLESS, VMess, Trojan, Shadowsocks, Socks5, HTTP
  - **Tunnel Protocols**: WireGuard, AWG2, Hysteria, Hysteria2, TUIC
  - **Mixed Mode**: Support for multiple protocols on single port

### Amnesia Mode (In-Memory Database)
When enabled, the panel operates with an in-memory database that provides:
- **Zero Persistence**: All configurations are lost on panel restart
- **Privacy-Focused**: Perfect for temporary VPN setups or testing
- **Real-Time Management**: 
  - Create and edit inbound configurations
  - Add/manage clients on the fly
  - Monitor real-time traffic statistics
- **Automatic Cleanup**: Optional automatic connection cleanup
- **Backup Before Reset**: Optional automatic backup before data loss

## ⚡ Installation

### Quick Install
```bash
bash <(curl -Ls https://raw.githubusercontent.com/hfawgen-glitch/3x-ui/main/install.sh)
```

### Enable Amnesia Mode
```bash
export XUI_DB_TYPE=memory
export XUI_AMNESIA_ENABLED=1
x-ui start
```

## 📋 Usage

### Protocol Selection
1. Navigate to Inbounds section
2. Create new inbound or edit existing
3. Select desired protocol from dropdown

### Amnesia Mode Configuration
Edit `/etc/x-ui/amnesia.yaml`

## 📊 API Endpoints

- `GET /api/protocols` - List all supported protocols
- `POST /api/inbounds` - Create inbound with protocol
- `POST /api/inbounds/:id/clients` - Add client
- `GET /api/traffic?key=inbound_1` - Get traffic statistics

## 🛡️ Security Notes

### Amnesia Mode
- All data stored in RAM only
- No encryption needed (no persistence)
- Restart = complete data loss
- Ideal for disposable VPN services

### Protocol Security
- **VLESS/VMess**: TLS recommended
- **Trojan**: TLS required
- **AWG2**: Built-in encryption
- **WireGuard**: Built-in encryption

## 📝 Logs
- Panel logs: `/var/log/x-ui/x-ui.log`
- Amnesia logs: `/var/log/x-ui/amnesia.log`
- Xray logs: `/var/log/x-ui/xray.log`

## 📄 License
Provided as-is for educational and testing purposes.
