package hostname

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// max hostname length per RFC 1123
const maxLen = 63

var validHostname = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// Get returns the current system hostname.
func Get() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

// Set sets the system hostname via hostnamectl (requires root).
func Set(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	if !Valid(name) {
		return fmt.Errorf("invalid hostname: %q", name)
	}
	cmd := exec.Command("hostnamectl", "set-hostname", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
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
