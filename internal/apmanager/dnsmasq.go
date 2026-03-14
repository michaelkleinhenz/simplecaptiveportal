package apmanager

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	dnsmasqSharedDir  = "/etc/NetworkManager/dnsmasq-shared.d"
	captivePortalConf = "99-captive-portal.conf"
)

// gatewayIP is the IP we use for the AP (NM shared mode default).
const gatewayIP = "10.42.0.1"

// setupCaptiveDNSConfig writes a dnsmasq snippet so all DNS resolve to the gateway.
// Used when NetworkManager uses dnsmasq-shared.d for shared connections.
func (a *APManager) setupCaptiveDNSConfig() error {
	if err := os.MkdirAll(dnsmasqSharedDir, 0755); err != nil {
		return fmt.Errorf("mkdir dnsmasq-shared.d: %w", err)
	}
	path := filepath.Join(dnsmasqSharedDir, captivePortalConf)
	// address=/#/10.42.0.1 makes dnsmasq resolve any domain to the gateway
	body := "address=/#/" + gatewayIP + "\n"
	return os.WriteFile(path, []byte(body), 0644)
}

func (a *APManager) teardownCaptiveDNSConfig() {
	path := filepath.Join(dnsmasqSharedDir, captivePortalConf)
	_ = os.Remove(path)
}
