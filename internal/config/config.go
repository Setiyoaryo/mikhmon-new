package config

import (
	"encoding/json"
	"os"
	"sync"
)

type RouterSession struct {
	Name        string `json:"name"`
	IP          string `json:"ip"`
	User        string `json:"user"`
	Password    string `json:"password"` // stored encrypted
	Hotspot     string `json:"hotspot"`
	DNS         string `json:"dns"`
	Currency    string `json:"currency"`
	Reload      int    `json:"reload"` // auto-reload seconds
	Interface   string `json:"interface"`
	InfoLP      string `json:"infolp"`
	IdleTimeout int    `json:"idle_timeout"`
	LiveReport  string `json:"live_report"`
}

type AdminCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	Admin    AdminCredentials          `json:"admin"`
	Sessions map[string]*RouterSession `json:"sessions"`
	Theme    string                    `json:"theme"`
	Lang     string                    `json:"lang"`
	QRPrint  bool                      `json:"qr_print"`
	mu       sync.RWMutex
	path     string
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		path:     path,
		Sessions: make(map[string]*RouterSession),
		Admin:    AdminCredentials{Username: "mikhmon", Password: "mikhmon"},
		Theme:    "dark",
		Lang:     "en",
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, cfg.Save()
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.path = path
	return cfg, nil
}

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

func (c *Config) GetSession(name string) *RouterSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Sessions[name]
}

func (c *Config) SetSession(name string, s *RouterSession) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Sessions[name] = s
}

func (c *Config) DeleteSession(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Sessions, name)
}
