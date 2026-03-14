package networkmanager

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	nmcliBin     = "nmcli"
	apConName    = "CaptivePortalAP"
	defaultAPSSID = "ConfigureMe" // used when hostname is empty
	maxRetries   = 5
	retryDelay   = 2 * time.Second
	portalSubnet = "10.42.0.0/24"
	portalGateIP = "10.42.0.1"
)

// AddWifiConnection adds a WiFi connection with the given SSID and password via nmcli,
// then activates it. Caller may then reboot.
func AddWifiConnection(ifname, ssid, password string, log *slog.Logger) error {
	conName := sanitizeConName(ssid)
	for i := 0; i < maxRetries; i++ {
		// Remove existing connection with same name to avoid conflicts
		_ = runNmcli("connection", "delete", conName)

		if password == "" {
			// Open network
			err := runNmcli("connection", "add",
				"type", "wifi",
				"con-name", conName,
				"ifname", ifname,
				"ssid", ssid,
				"wifi-sec.key-mgmt", "none",
			)
			if err != nil {
				if log != nil {
					log.Warn("nmcli add wifi failed (open)", "err", err, "attempt", i+1)
				}
				time.Sleep(retryDelay)
				continue
			}
		} else {
			err := runNmcli("connection", "add",
				"type", "wifi",
				"con-name", conName,
				"ifname", ifname,
				"ssid", ssid,
				"--", "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", password,
			)
			if err != nil {
				if log != nil {
					log.Warn("nmcli add wifi failed", "err", err, "attempt", i+1)
				}
				time.Sleep(retryDelay)
				continue
			}
		}

		// Set as autoconnect and higher priority so it connects on boot
		_ = runNmcli("connection", "modify", conName, "connection.autoconnect-priority", "100")
		_ = runNmcli("connection", "modify", conName, "connection.autoconnect", "yes")

		// Activate; if we reboot immediately, NM will still connect on next boot
		if err := runNmcli("connection", "up", conName); err != nil && log != nil {
			log.Warn("nmcli up failed (will try on reboot)", "err", err)
		}
		return nil
	}
	return fmt.Errorf("failed to add WiFi connection after %d attempts", maxRetries)
}

func sanitizeConName(ssid string) string {
	s := strings.TrimSpace(ssid)
	if s == "" {
		return "WiFi"
	}
	// nmcli connection names should be filesystem-safe
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "WiFi"
	}
	return b.String()
}

// EnsureAPConnection creates or updates the captive portal AP connection profile (open AP). apSSID is the broadcast SSID (e.g. "<hostname>Configure").
func EnsureAPConnection(ifname, apSSID string, log *slog.Logger) error {
	if apSSID == "" {
		apSSID = defaultAPSSID
	}
	out, err := exec.Command(nmcliBin, "-t", "-f", "NAME", "connection", "show").Output()
	exists := err == nil && strings.Contains(string(out), apConName)
	if exists {
		_ = runNmcli("connection", "modify", apConName, "wifi.ssid", apSSID)
		return nil
	}
	_ = runNmcli("connection", "delete", apConName)
	err = runNmcli("connection", "add",
		"type", "wifi",
		"con-name", apConName,
		"autoconnect", "no",
		"ifname", ifname,
		"wifi.mode", "ap",
		"wifi.ssid", apSSID,
		"ipv4.method", "shared",
		"ipv6.method", "shared",
	)
	if err != nil {
		return fmt.Errorf("create AP profile: %w", err)
	}
	return nil
}

// ActivateAP brings up the captive portal AP. apSSID is the broadcast SSID (e.g. "<hostname>Configure").
func ActivateAP(ifname, apSSID string, log *slog.Logger) error {
	if err := EnsureAPConnection(ifname, apSSID, log); err != nil {
		return err
	}
	for i := 0; i < maxRetries; i++ {
		err := runNmcli("connection", "up", apConName)
		if err == nil {
			return nil
		}
		if log != nil {
			log.Warn("nmcli up AP failed", "err", err, "attempt", i+1)
		}
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("failed to activate AP after %d attempts", maxRetries)
}

// DeactivateAP brings down the captive portal AP.
func DeactivateAP(log *slog.Logger) error {
	err := runNmcli("connection", "down", apConName)
	if err != nil && log != nil {
		log.Debug("nmcli down AP", "err", err)
	}
	return err
}

func runNmcli(args ...string) error {
	cmd := exec.Command(nmcliBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// WifiNetwork is a scanned network for the UI.
type WifiNetwork struct {
	SSID     string `json:"ssid"`
	Signal   int    `json:"signal"`   // 0–100
	Security string `json:"security"` // e.g. "WPA2", "WPA", "" for open
}

// ListWifiNetworks runs a scan and returns visible networks (SSID, signal, security).
// ifname is the WiFi interface (e.g. wlan0). If the command fails with ifname (e.g. AP mode, wrong device),
// it is retried without ifname so the list can still be returned.
func ListWifiNetworks(ifname string, log *slog.Logger) ([]WifiNetwork, error) {
	// Rescan; ignore errors (e.g. when in AP mode)
	if ifname != "" {
		_ = exec.Command(nmcliBin, "device", "wifi", "rescan", "ifname", ifname).Run()
	} else {
		_ = exec.Command(nmcliBin, "device", "wifi", "rescan").Run()
	}
	time.Sleep(1 * time.Second)

	out, err := runWifiList(ifname)
	if err != nil && ifname != "" {
		if log != nil {
			log.Debug("nmcli wifi list with ifname failed, retrying without", "err", err)
		}
		out, err = runWifiList("")
	}
	if err != nil {
		if log != nil {
			log.Debug("nmcli device wifi list failed", "err", err, "out", string(out))
		}
		return []WifiNetwork{}, nil
	}
	return parseWifiList(out), nil
}

func runWifiList(ifname string) ([]byte, error) {
	args := []string{"-t", "-f", "SSID,SIGNAL,SECURITY", "device", "wifi", "list"}
	if ifname != "" {
		args = append(args, "ifname", ifname)
	}
	cmd := exec.Command(nmcliBin, args...)
	return cmd.CombinedOutput()
}

func parseWifiList(out []byte) []WifiNetwork {
	var list []WifiNetwork
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		// SSID can contain colons; last two fields are SIGNAL and SECURITY
		ssid := strings.TrimSpace(strings.Join(parts[:len(parts)-2], ":"))
		signalStr := strings.TrimSpace(parts[len(parts)-2])
		security := strings.TrimSpace(parts[len(parts)-1])
		if ssid == "" || ssid == "--" {
			continue
		}
		if seen[ssid] {
			continue
		}
		seen[ssid] = true
		signal := 0
		if n, err := strconv.Atoi(signalStr); err == nil && n >= 0 && n <= 100 {
			signal = n
		}
		list = append(list, WifiNetwork{SSID: ssid, Signal: signal, Security: security})
	}
	return list
}

// Reboot triggers a system reboot (requires root).
func Reboot(log *slog.Logger) error {
	cmd := exec.Command("systemctl", "reboot")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl reboot: %w", err)
	}
	if log != nil {
		log.Info("reboot triggered")
	}
	return nil
}
