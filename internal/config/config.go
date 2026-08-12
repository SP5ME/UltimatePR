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
type Beacon struct {
	Enabled         bool   `yaml:"enabled"`
	Port            string `yaml:"port"`
	Destination     string `yaml:"destination"`
	Text            string `yaml:"text"`
	IntervalMinutes int    `yaml:"interval_minutes"`
}
type History struct {
	Enabled               bool   `yaml:"enabled"`
	Database              string `yaml:"database"`
	MaxStations           int    `yaml:"max_stations"`
	MaxSessionsPerStation int    `yaml:"max_sessions_per_station"`
	MaxLinesPerStation    int    `yaml:"max_lines_per_station"`
	MaxBytes              int    `yaml:"max_bytes"`
	RetentionDays         int    `yaml:"retention_days"`
}
type BBS struct {
	Enabled       bool          `yaml:"enabled"`
	Listen        string        `yaml:"listen"`
	ForwardListen string        `yaml:"forward_listen"`
	Database      string        `yaml:"database"`
	Title         string        `yaml:"title"`
	Callsign      string        `yaml:"callsign"`
	SSID          uint8         `yaml:"ssid"`
	Address       string        `yaml:"hierarchical_address"`
	Language      string        `yaml:"language"`
	Forwarding    BBSForwarding `yaml:"forwarding"`
}
type BBSForwarding struct {
	Enabled               bool      `yaml:"enabled"`
	IntervalMinutes       int       `yaml:"interval_minutes"`
	ConnectTimeoutSeconds int       `yaml:"connect_timeout_seconds"`
	SessionTimeoutSeconds int       `yaml:"session_timeout_seconds"`
	MaxMessages           int       `yaml:"max_messages_per_session"`
	MaxBodyBytes          int       `yaml:"max_body_bytes"`
	Peers                 []BBSPeer `yaml:"peers"`
}
type BBSPeer struct {
	ID             string   `yaml:"id"`
	Callsign       string   `yaml:"callsign"`
	Enabled        bool     `yaml:"enabled"`
	Transport      string   `yaml:"transport"`
	Host           string   `yaml:"host"`
	Port           uint16   `yaml:"port"`
	ViaNode        string   `yaml:"via_node"`
	Schedule       []string `yaml:"schedule"`
	PrivateRoutes  []string `yaml:"private_routes"`
	BulletinScopes []string `yaml:"bulletin_scopes"`
}
type Node struct {
	Enabled   bool           `yaml:"enabled"`
	Alias     string         `yaml:"alias"`
	Listen    string         `yaml:"listen"`
	Language  string         `yaml:"language"`
	Neighbors []NodeNeighbor `yaml:"neighbors"`
	Routes    []NodeRoute    `yaml:"routes"`
	Services  []NodeService  `yaml:"services"`
}
type NodeNeighbor struct {
	ID       string `yaml:"id"`
	Enabled  bool   `yaml:"enabled"`
	Callsign string `yaml:"callsign"`
	Port     string `yaml:"port"`
	Quality  int    `yaml:"quality"`
	Locked   bool   `yaml:"locked"`
}
type NodeRoute struct {
	Destination string `yaml:"destination"`
	Via         string `yaml:"via"`
	Quality     int    `yaml:"quality"`
	Enabled     bool   `yaml:"enabled"`
}
type NodeService struct {
	Name     string `yaml:"name"`
	Callsign string `yaml:"callsign"`
	Command  string `yaml:"command"`
	Enabled  bool   `yaml:"enabled"`
}
type Port struct {
	ID               string   `yaml:"id"`
	Type             string   `yaml:"type"`
	Host             string   `yaml:"host"`
	Port             uint16   `yaml:"port"`
	Channel          uint8    `yaml:"channel"`
	MaxFrameBytes    int      `yaml:"max_frame_bytes"`
	ReconnectSeconds int      `yaml:"reconnect_seconds"`
	Listen           string   `yaml:"listen"`
	RemoteHost       string   `yaml:"remote_host"`
	RemotePort       uint16   `yaml:"remote_port"`
	FCS              bool     `yaml:"fcs"`
	AllowFrom        []string `yaml:"allow_from"`
}
type Config struct {
	Server   Station `yaml:"server"`
	Web      Web     `yaml:"web"`
	Ports    []Port  `yaml:"ports"`
	Terminal Station `yaml:"terminal"`
	Beacon   Beacon  `yaml:"beacon"`
	History  History `yaml:"history"`
	BBS      BBS     `yaml:"bbs"`
	Node     Node    `yaml:"node"`
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(b)
}

func Parse(b []byte) (Config, error) {
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func Save(path string, raw []byte) error {
	if _, err := Parse(raw); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0640); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func SaveModel(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return Save(path, b)
}

func (c Config) Validate() error {
	if err := validStation(c.Server); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := validStation(c.Terminal); err != nil {
		return fmt.Errorf("terminal: %w", err)
	}
	if c.Beacon.Destination != "" {
		if _, err := parseStationText(c.Beacon.Destination); err != nil {
			return fmt.Errorf("beacon.destination: %w", err)
		}
	}
	if c.Beacon.Enabled && (c.Beacon.Port == "" || strings.TrimSpace(c.Beacon.Text) == "" || c.Beacon.IntervalMinutes < 1) {
		return fmt.Errorf("beacon: port, text and interval >= 10 seconds are required")
	}
	if c.History.Enabled {
		if strings.TrimSpace(c.History.Database) == "" || c.History.MaxStations < 1 || c.History.MaxSessionsPerStation < 1 || c.History.MaxLinesPerStation < 1 || c.History.MaxBytes < 1024 || c.History.RetentionDays < 1 {
			return fmt.Errorf("history: database and positive limits are required")
		}
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
		if p.Type != "kiss-tcp" && p.Type != "axudp" {
			return fmt.Errorf("ports[%d]: unsupported type %q", i, p.Type)
		}
		if p.Type == "kiss-tcp" && (p.Host == "" || p.Port == 0 || p.Channel > 15) {
			return fmt.Errorf("ports[%d]: invalid KISS host, port or channel", i)
		}
		if p.Type == "axudp" {
			if _, _, err := net.SplitHostPort(p.Listen); err != nil {
				return fmt.Errorf("ports[%d].listen: %w", i, err)
			}
			if p.RemoteHost == "" || p.RemotePort == 0 {
				return fmt.Errorf("ports[%d]: AXUDP remote host and port required", i)
			}
		}
		if p.MaxFrameBytes < 256 || p.MaxFrameBytes > 65535 {
			return fmt.Errorf("ports[%d]: max_frame_bytes out of range", i)
		}
		if p.Type == "kiss-tcp" && (p.ReconnectSeconds < 1 || p.ReconnectSeconds > 3600) {
			return fmt.Errorf("ports[%d]: reconnect_seconds out of range", i)
		}
	}
	if c.Node.Enabled {
		if !validLanguage(c.Node.Language) {
			return fmt.Errorf("node.language must be pl or en")
		}
		if len(strings.TrimSpace(c.Node.Alias)) < 1 || len(c.Node.Alias) > 6 {
			return fmt.Errorf("node.alias length must be 1..6")
		}
		if _, _, err := net.SplitHostPort(c.Node.Listen); err != nil {
			return fmt.Errorf("node.listen: %w", err)
		}
		for i, n := range c.Node.Neighbors {
			if !n.Enabled {
				continue
			}
			if n.ID == "" || n.Port == "" || n.Quality < 0 || n.Quality > 255 {
				return fmt.Errorf("node.neighbors[%d]: invalid id, port or quality", i)
			}
			if _, err := parseStationText(n.Callsign); err != nil {
				return fmt.Errorf("node.neighbors[%d]: %w", i, err)
			}
		}
		active := map[string]bool{}
		for _, n := range c.Node.Neighbors {
			if n.Enabled {
				active[n.ID] = true
			}
		}
		for i, r := range c.Node.Routes {
			if r.Enabled && !active[r.Via] {
				return fmt.Errorf("node.routes[%d]: unknown or disabled neighbor %q", i, r.Via)
			}
		}
	}
	if c.BBS.Enabled {
		if !validLanguage(c.BBS.Language) {
			return fmt.Errorf("bbs.language must be pl or en")
		}
		if c.BBS.Forwarding.Enabled {
			if _, _, err := net.SplitHostPort(c.BBS.ForwardListen); err != nil {
				return fmt.Errorf("bbs.forward_listen: %w", err)
			}
		}
		if c.BBS.Callsign != "" {
			if err := validStation(Station{Callsign: c.BBS.Callsign, SSID: c.BBS.SSID}); err != nil {
				return fmt.Errorf("bbs: %w", err)
			}
		}
		for i, p := range c.BBS.Forwarding.Peers {
			if p.ID == "" || p.Callsign == "" {
				return fmt.Errorf("bbs.forwarding.peers[%d]: id and callsign required", i)
			}
			if p.Transport != "telnet" && p.Transport != "node" {
				return fmt.Errorf("bbs.forwarding.peers[%d]: transport must be telnet or node", i)
			}
			if p.Transport == "telnet" && (p.Host == "" || p.Port == 0) {
				return fmt.Errorf("bbs.forwarding.peers[%d]: host and port required", i)
			}
		}
	}
	return nil
}

func validLanguage(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "pl" || v == "en"
}

func parseStationText(v string) (Station, error) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(v)), "-")
	s := Station{Callsign: parts[0]}
	if len(parts) > 2 {
		return s, fmt.Errorf("invalid callsign")
	}
	if len(parts) == 2 {
		var n int
		if _, err := fmt.Sscanf(parts[1], "%d", &n); err != nil || n < 0 || n > 15 {
			return s, fmt.Errorf("invalid SSID")
		}
		s.SSID = uint8(n)
	}
	return s, validStation(s)
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
