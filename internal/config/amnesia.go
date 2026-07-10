package config

import (
	"os"
	"strconv"
	"strings"
)

type AmnesiaConfig struct {
	Enabled                 bool
	AutoResetInterval       int
	EnableTraffic           bool
	AutoCleanup             bool
	CleanupInterval         int
	MaxInbounds             int
	MaxClientsPerInbound    int
	EnableLogging           bool
	LogFile                 string
	DefaultProtocol         string
	WarningMessage          string
	EnableBackup            bool
	BackupPath              string
	SupportedProtocols      []string
}

var Amnesia = &AmnesiaConfig{
	Enabled:              getEnvBool("XUI_AMNESIA_ENABLED", false),
	AutoResetInterval:    getEnvInt("XUI_AMNESIA_AUTO_RESET", 0),
	EnableTraffic:        getEnvBool("XUI_AMNESIA_TRAFFIC", true),
	AutoCleanup:          getEnvBool("XUI_AMNESIA_AUTO_CLEANUP", true),
	CleanupInterval:      getEnvInt("XUI_AMNESIA_CLEANUP_INTERVAL", 300),
	MaxInbounds:          getEnvInt("XUI_AMNESIA_MAX_INBOUNDS", 100),
	MaxClientsPerInbound: getEnvInt("XUI_AMNESIA_MAX_CLIENTS", 1000),
	EnableLogging:        getEnvBool("XUI_AMNESIA_LOGGING", true),
	LogFile:              getEnv("XUI_AMNESIA_LOG_FILE", "/var/log/x-ui/amnesia.log"),
	DefaultProtocol:      getEnv("XUI_AMNESIA_DEFAULT_PROTO", "vless"),
	WarningMessage:       "Amnesia mode active: All data will be lost on restart",
	EnableBackup:         getEnvBool("XUI_AMNESIA_BACKUP", true),
	BackupPath:           getEnv("XUI_AMNESIA_BACKUP_PATH", "/tmp/x-ui-amnesia-backup"),
	SupportedProtocols: []string{
		"vless", "vmess", "trojan", "shadowsocks", "socks5", "http",
		"wireguard", "awg2", "hysteria", "hysteria2", "tuic", "mixed",
	},
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, exists := os.LookupEnv(key); exists {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, exists := os.LookupEnv(key); exists {
		i, err := strconv.Atoi(val)
		if err == nil {
			return i
		}
	}
	return defaultVal
}

func (a *AmnesiaConfig) IsProtocolSupported(proto string) bool {
	for _, p := range a.SupportedProtocols {
		if strings.EqualFold(p, proto) {
			return true
		}
	}
	return false
}
