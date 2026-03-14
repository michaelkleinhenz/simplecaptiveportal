package snapclient

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const DefaultPath = "/etc/default/snapclient"

// ReadHost returns the snapcast server host from SNAPCLIENT_OPTS (-h <host>), or empty if unset/unreadable.
func ReadHost(path string) string {
	if path == "" {
		path = DefaultPath
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "SNAPCLIENT_OPTS=") {
			continue
		}
		rest := strings.TrimPrefix(line, "SNAPCLIENT_OPTS=")
		rest = strings.TrimSpace(rest)
		rest = strings.Trim(rest, `"'`)
		rest = strings.TrimSpace(rest)
		fields := strings.Fields(rest)
		for i, f := range fields {
			if f == "-h" && i+1 < len(fields) {
				return strings.Trim(fields[i+1], `"'`)
			}
			if strings.HasPrefix(f, "-h=") {
				return strings.Trim(f[3:], `"'`)
			}
		}
		return ""
	}
	return ""
}

// WriteHost writes /etc/default/snapclient with a single line: SNAPCLIENT_OPTS="-h <host>".
// host can be an IP or hostname; empty host writes the line with empty -h (snapclient will use default).
func WriteHost(path, host string) error {
	if path == "" {
		path = DefaultPath
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	line := `SNAPCLIENT_OPTS="-h ` + host + `"` + "\n"
	return os.WriteFile(path, []byte(line), 0644)
}

// RestartService runs systemctl restart snapclient so the new host is picked up.
func RestartService() error {
	cmd := exec.Command("systemctl", "restart", "snapclient")
	return cmd.Run()
}

// ValidHost returns true if s is a valid IP or hostname for snapclient.
func ValidHost(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if net.ParseIP(s) != nil {
		return true
	}
	if len(s) > 0 && len(s) < 254 {
		return true
	}
	return false
}
