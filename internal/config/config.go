package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Station struct {
	Callsign string `yaml:"callsign"`
	SSID     uint8  `yaml:"ssid"`
}
type Web struct {
	Listen string `yaml:"listen"`
}
type BBS struct {
	Enabled  bool   `yaml:"enabled"`
	Listen   string `yaml:"listen"`
	Database string `yaml:"database"`
	Title    string `yaml:"title"`
}
type Port struct {
	ID               string `yaml:"id"`
	Type             string `yaml:"type"`
	Host             string `yaml:"host"`
	Port             uint16 `yaml:"port"`
	Channel          uint8  `yaml:"channel"`
	MaxFrameBytes    int    `yaml:"max_frame_bytes"`
	ReconnectSeconds int    `yaml:"reconnect_seconds"`
}
type Config struct {
	Server   Station `yaml:"server"`
	Web      Web     `yaml:"web"`
	Ports    []Port  `yaml:"ports"`
	Terminal Station `yaml:"terminal"`
	BBS      BBS     `yaml:"bbs"`
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	if err := validStation(c.Server); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := validStation(c.Terminal); err != nil {
		return fmt.Errorf("terminal: %w", err)
	}
	host, _, err := net.SplitHostPort(c.Web.Listen)
	if err != nil {
		return fmt.Errorf("web.listen: %w", err)
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("web.listen must use loopback in milestone one")
	}
	if c.BBS.Enabled {
		if _, _, err := net.SplitHostPort(c.BBS.Listen); err != nil {
			return fmt.Errorf("bbs.listen: %w", err)
		}
		if strings.TrimSpace(c.BBS.Database) == "" {
			return fmt.Errorf("bbs.database is required")
		}
	}
	seen := map[string]bool{}
	for i, p := range c.Ports {
		if p.ID == "" || seen[p.ID] {
			return fmt.Errorf("ports[%d]: id is empty or duplicated", i)
		}
		seen[p.ID] = true
		if p.Type != "kiss-tcp" {
			return fmt.Errorf("ports[%d]: unsupported type %q", i, p.Type)
		}
		if p.Host == "" || p.Port == 0 || p.Channel > 15 {
			return fmt.Errorf("ports[%d]: invalid host, port or channel", i)
		}
		if p.MaxFrameBytes < 256 || p.MaxFrameBytes > 65535 {
			return fmt.Errorf("ports[%d]: max_frame_bytes out of range", i)
		}
		if p.ReconnectSeconds < 1 || p.ReconnectSeconds > 3600 {
			return fmt.Errorf("ports[%d]: reconnect_seconds out of range", i)
		}
	}
	return nil
}

func validStation(s Station) error {
	cs := strings.ToUpper(strings.TrimSpace(s.Callsign))
	if len(cs) < 1 || len(cs) > 6 {
		return fmt.Errorf("callsign length must be 1..6")
	}
	for _, r := range cs {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return fmt.Errorf("invalid callsign")
		}
	}
	if s.SSID > 15 {
		return fmt.Errorf("SSID must be 0..15")
	}
	return nil
}
