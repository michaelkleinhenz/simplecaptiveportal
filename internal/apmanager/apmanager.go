package apmanager

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/snapcast-client/captive-portal/internal/captivedns"
	"github.com/snapcast-client/captive-portal/internal/hostname"
	"github.com/snapcast-client/captive-portal/internal/networkmanager"
)

const (
	defaultWifiInterface = "wlan0"
	defaultPortalPort   = "80"
)

// APManager starts/stops the captive portal AP, captive DNS, and iptables HTTP redirect.
type APManager struct {
	log       *slog.Logger
	ifname    string
	port      string
	iptables  *iptablesHelper
	dnsServer *captivedns.Server
}

func New(log *slog.Logger, port string) *APManager {
	if port == "" {
		port = defaultPortalPort
	}
	ifname := defaultWifiInterface
	if s := discoverWifiInterface(); s != "" {
		ifname = s
	}
	return &APManager{
		log:      log,
		ifname:   ifname,
		port:     port,
		iptables: &iptablesHelper{log: log},
	}
}

func apSSIDFromHostname(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return "ConfigureMe"
	}
	return h + "Configure"
}

func discoverWifiInterface() string {
	out, err := exec.Command("nmcli", "-t", "-f", "DEVICE,TYPE", "device", "status").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[1]) == "wifi" {
			dev := strings.TrimSpace(parts[0])
			if dev != "" {
				return dev
			}
		}
	}
	return ""
}

func (a *APManager) StartAP() error {
	// Must activate AP first so NetworkManager can start DHCP; otherwise clients get "IP configuration error".
	// Then we add captive DNS so all lookups resolve to the gateway (config page loads automatically).
	_ = a.setupCaptiveDNSConfig()
	if err := a.iptables.setupRedirect(a.port); err != nil {
		a.log.Warn("iptables redirect setup failed (clients may need to open browser to gateway)", "err", err)
	}
	apSSID := apSSIDFromHostname(hostname.Get())
	if err := networkmanager.ActivateAP(a.ifname, apSSID, a.log); err != nil {
		return fmt.Errorf("activate AP: %w", err)
	}
	a.dnsServer = captivedns.New(gatewayIP, a.log)
	a.dnsServer.Start()
	return nil
}

func (a *APManager) StopAP() error {
	_ = networkmanager.DeactivateAP(a.log)
	if a.dnsServer != nil {
		_ = a.dnsServer.Shutdown()
		a.dnsServer = nil
	}
	a.iptables.teardownRedirect(a.port)
	a.teardownCaptiveDNSConfig()
	return nil
}

// iptablesHelper manages NAT redirect for captive portal (all HTTP to local server).
type iptablesHelper struct {
	log *slog.Logger
}

func (i *iptablesHelper) setupRedirect(port string) error {
	// Redirect HTTP from clients in typical NM shared subnet to ourselves
	// -s 10.42.0.0/24 -p tcp --dport 80 -j DNAT --to-destination 10.42.0.1:80
	args := []string{"-t", "nat", "-C", "PREROUTING", "-s", "10.42.0.0/24", "-p", "tcp", "--dport", "80", "-j", "DNAT", "--to-destination", "10.42.0.1:" + port}
	if err := exec.Command("iptables", args...).Run(); err == nil {
		return nil // already present
	}
	args = []string{"-t", "nat", "-A", "PREROUTING", "-s", "10.42.0.0/24", "-p", "tcp", "--dport", "80", "-j", "DNAT", "--to-destination", "10.42.0.1:" + port}
	cmd := exec.Command("iptables", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iptables: %w: %s", err, string(out))
	}
	return nil
}

func (i *iptablesHelper) teardownRedirect(port string) {
	args := []string{"-t", "nat", "-D", "PREROUTING", "-s", "10.42.0.0/24", "-p", "tcp", "--dport", "80", "-j", "DNAT", "--to-destination", "10.42.0.1:" + port}
	_ = exec.Command("iptables", args...).Run()
}
