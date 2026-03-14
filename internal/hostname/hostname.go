package hostname

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// max hostname length per RFC 1123
const maxLen = 63

const hostnameFile = "/etc/hostname"

var validHostname = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// Get returns the current system hostname.
func Get() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

// Set sets the system hostname (requires root). Always writes /etc/hostname so it persists across reboot, then applies live.
func Set(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	if !Valid(name) {
		return fmt.Errorf("invalid hostname: %q", name)
	}
	// Always write /etc/hostname first so the hostname persists across reboot (systemd and others read this at boot).
	if err := os.WriteFile(hostnameFile, []byte(name+"\n"), 0644); err != nil {
		return fmt.Errorf("write %s: %w", hostnameFile, err)
	}
	// Apply live: prefer hostnamectl so systemd's static hostname is updated (avoids DHCP/NetworkManager overriding after reboot).
	var applied bool
	for _, hostnamectl := range []string{"/usr/bin/hostnamectl", "hostnamectl"} {
		cmd := exec.Command(hostnamectl, "set-hostname", name)
		if _, err := cmd.CombinedOutput(); err == nil {
			applied = true
			break
		}
	}
	if !applied {
		for _, hostnameBin := range []string{"/usr/bin/hostname", "/bin/hostname", "hostname"} {
			if filepath.IsAbs(hostnameBin) {
				if _, err := os.Stat(hostnameBin); err != nil {
					continue
				}
			} else if _, err := exec.LookPath(hostnameBin); err != nil {
				continue
			}
			cmd := exec.Command(hostnameBin, name)
			if cmd.Run() == nil {
				applied = true
				break
			}
		}
	}
	if !applied {
		return errors.New("could not set hostname for current session (hostnamectl and hostname command failed); /etc/hostname was written and will apply after reboot")
	}
	return nil
}

// Valid returns true if name is a valid hostname (alphanumeric and hyphen, 1–63 chars).
func Valid(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxLen {
		return false
	}
	return validHostname.MatchString(name)
}
