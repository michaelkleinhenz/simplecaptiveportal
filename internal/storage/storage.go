package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds persisted settings (WiFi + extensible extra fields).
type Config struct {
	SSID     string            `json:"ssid,omitempty"`
	Password string            `json:"password,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
	mu       sync.RWMutex      `json:"-"`
}

// Default path for config file (writable when running as root or with caps).
var DefaultPath = "/etc/captive-portal/config.json"

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Extra: make(map[string]string)}, nil
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Extra == nil {
		c.Extra = make(map[string]string)
	}
	return &c, nil
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultPath
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c *Config) GetSSID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.SSID
}

func (c *Config) GetPassword() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Password
}

func (c *Config) SetWiFi(ssid, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SSID = ssid
	c.Password = password
}

func (c *Config) GetExtra(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Extra[key]
}

func (c *Config) SetExtra(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Extra == nil {
		c.Extra = make(map[string]string)
	}
	c.Extra[key] = value
}

// GetExtraMap returns a copy of the Extra map for safe use in templates.
func (c *Config) GetExtraMap() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := make(map[string]string, len(c.Extra))
	for k, v := range c.Extra {
		m[k] = v
	}
	return m
}
