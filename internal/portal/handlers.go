package portal

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"

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
	action := r.FormValue("action")

	switch action {
	case "wifi":
		s.handleConfigureWifi(w, r)
		return
	case "settings":
		s.handleConfigureSettings(w, r)
		return
	default:
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
}

func (s *Server) handleConfigureWifi(w http.ResponseWriter, r *http.Request) {
	ssid := strings.TrimSpace(r.FormValue("ssid"))
	password := r.FormValue("password")
	s.cfg.SetWiFi(ssid, password)
	if err := s.cfg.Save(s.cfgPath); err != nil {
		s.log.Warn("save config", "err", err)
	}
	if ssid == "" {
		http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
		return
	}
	ifname := s.wifiIface
	if ifname == "" {
		ifname = discoverWifiInterface()
	}
	if err := networkmanager.AddWifiConnection(ifname, ssid, password, s.log); err != nil {
		s.log.Error("add wifi connection", "err", err)
		http.Error(w, "Failed to configure WiFi. Check logs.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
}

func (s *Server) handleConfigureSettings(w http.ResponseWriter, r *http.Request) {
	snapcastServer := strings.TrimSpace(r.FormValue("snapcast_server"))
	clientName := strings.TrimSpace(r.FormValue("hostname"))
	for key, values := range r.Form {
		if key == "action" || key == "snapcast_server" || key == "hostname" {
			continue
		}
		if len(values) > 0 {
			s.cfg.SetExtra(key, strings.TrimSpace(values[0]))
		}
	}
	if err := s.cfg.Save(s.cfgPath); err != nil {
		s.log.Warn("save config", "err", err)
	}
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
		} else if err := snapclient.RestartService(); err != nil {
			s.log.Warn("restart snapclient after hostname change", "err", err)
		}
	}
	http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
}

func (s *Server) handleReboot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Rebooting</title></head><body><p>Rebooting…</p></body></html>`))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		_ = networkmanager.Reboot(s.log)
	}()
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Shutting down</title></head><body><p>Shutting down…</p></body></html>`))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		_ = networkmanager.Shutdown(s.log)
	}()
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
