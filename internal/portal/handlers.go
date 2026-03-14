package portal

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/snapcast-client/captive-portal/internal/hostname"
	"github.com/snapcast-client/captive-portal/internal/networkmanager"
	"github.com/snapcast-client/captive-portal/internal/snapclient"
)

type indexData struct {
	CaptiveMode    bool
	SSID           string
	Saved          bool
	SnapcastServer string
	Hostname       string
	Extra          map[string]string
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	saved := r.URL.Query().Get("saved") == "1"
	data := indexData{
		CaptiveMode:    s.captiveMode.Load(),
		SSID:           s.cfg.GetSSID(),
		Saved:          saved,
		SnapcastServer: snapclient.ReadHost(snapclient.DefaultPath),
		Hostname:       hostname.Get(),
		Extra:          s.cfg.GetExtraMap(),
	}
	if err := indexTmpl.Execute(w, data); err != nil {
		s.log.Error("template execute", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	ssid := strings.TrimSpace(r.FormValue("ssid"))
	password := r.FormValue("password")

	// Save to config for display / backup
	s.cfg.SetWiFi(ssid, password)
	snapcastServer := strings.TrimSpace(r.FormValue("snapcast_server"))
	clientName := strings.TrimSpace(r.FormValue("hostname"))
	for key, values := range r.Form {
		if key == "ssid" || key == "password" || key == "snapcast_server" || key == "hostname" {
			continue
		}
		if len(values) > 0 {
			s.cfg.SetExtra(key, strings.TrimSpace(values[0]))
		}
	}
	if err := s.cfg.Save(s.cfgPath); err != nil {
		s.log.Warn("save config", "err", err)
	}
	// Write Snapcast server to /etc/default/snapclient and restart snapclient so it picks up the new host
	if snapclient.ValidHost(snapcastServer) {
		if err := snapclient.WriteHost(snapclient.DefaultPath, snapcastServer); err != nil {
			s.log.Warn("write snapclient defaults", "err", err)
		} else if err := snapclient.RestartService(); err != nil {
			s.log.Warn("restart snapclient service", "err", err)
		}
	}
	if clientName != "" && hostname.Valid(clientName) {
		if err := hostname.Set(clientName); err != nil {
			s.log.Warn("set hostname", "err", err)
		}
	}

	if ssid != "" {
		ifname := s.wifiIface
		if ifname == "" {
			ifname = discoverWifiInterface()
		}
		if err := networkmanager.AddWifiConnection(ifname, ssid, password, s.log); err != nil {
			s.log.Error("add wifi connection", "err", err)
			http.Error(w, "Failed to configure WiFi. Check logs.", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Rebooting</title></head><body><p>WiFi configured. Rebooting in a few seconds…</p></body></html>`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		go func() {
			time.Sleep(2 * time.Second)
			_ = networkmanager.Reboot(s.log)
		}()
		return
	}

	// If no SSID (e.g. only extra options), just show success and redirect
	http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
}

func (s *Server) handleNetworks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ifname := s.wifiIface
	if ifname == "" {
		ifname = discoverWifiInterface()
	}
	list, err := networkmanager.ListWifiNetworks(ifname, s.log)
	if err != nil {
		s.log.Warn("wifi list failed", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func discoverWifiInterface() string {
	out, err := exec.Command("nmcli", "-t", "-f", "DEVICE,TYPE", "device", "status").Output()
	if err != nil {
		return "wlan0"
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
	return "wlan0"
}
