package connmon

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// HasConnectivity returns true if the system has working internet (not just link).
// It tries a short HTTP request to a well-known URL used for captive detection.
func HasConnectivity(ctx context.Context) (bool, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, network, addr)
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://clients3.google.com/generate_204", nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == 204, nil
}

// ConnMon provides connectivity checking with optional logging.
type ConnMon struct {
	log *slog.Logger
}

func New(log *slog.Logger) *ConnMon {
	return &ConnMon{log: log}
}

func (c *ConnMon) HasConnectivity(ctx context.Context) (bool, error) {
	ok, err := HasConnectivity(ctx)
	if err != nil && c.log != nil {
		c.log.Debug("connectivity check", "ok", ok, "err", err)
	}
	return ok, err
}
