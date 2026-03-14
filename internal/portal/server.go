package portal

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/snapcast-client/captive-portal/internal/storage"
)

//go:embed static
var staticFS embed.FS

//go:embed templates
var templateFS embed.FS

var indexTmpl *template.Template

func init() {
	var err error
	indexTmpl, err = template.ParseFS(templateFS, "templates/index.html")
	if err != nil {
		panic("parse template: " + err.Error())
	}
}

type Server struct {
	port        string
	cfg         *storage.Config
	cfgPath     string
	captiveMode atomic.Bool
	srv         *http.Server
	mu          sync.Mutex
	log         *slog.Logger
	wifiIface   string
}

func NewServer(port string) (*Server, error) {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = storage.DefaultPath
	}
	cfg, err := storage.Load(cfgPath)
	if err != nil {
		cfg = &storage.Config{}
		cfg.Extra = make(map[string]string)
	}
	s := &Server{
		port:      port,
		cfg:       cfg,
		cfgPath:   cfgPath,
		log:       slog.Default(),
		wifiIface: "wlan0",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/configure", s.handleConfigure)
	mux.HandleFunc("/api/networks", s.handleNetworks)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	s.srv = &http.Server{Addr: ":" + port, Handler: s.captiveWrap(mux)}
	return s, nil
}

func (s *Server) captiveWrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.captiveMode.Load() && r.URL.Path != "/" && r.URL.Path != "/configure" && r.URL.Path != "/api/networks" && !strings.HasPrefix(r.URL.Path, "/static/") {
			// Captive portal: redirect any other path to config page
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Server) Start() {
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("portal server error", "err", err)
		}
	}()
	s.log.Info("portal server listening", "port", s.port)
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		_ = s.srv.Close()
	}
}

func (s *Server) SetCaptiveMode(captive bool) {
	s.captiveMode.Store(captive)
}
