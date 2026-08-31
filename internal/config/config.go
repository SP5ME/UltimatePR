package config

import (
	"fmt"
	"net"
	neturl "net/url"
	"os"
	"strconv"
	"strings"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	bbscore "github.com/packet-radio/ultimatepr/internal/bbs"
	"github.com/packet-radio/ultimatepr/internal/netallow"
	"gopkg.in/yaml.v3"
)

type Station struct {
	Callsign string `yaml:"callsign"`
	SSID     uint8  `yaml:"ssid"`
}
type Web struct {
	Listen           string   `yaml:"listen"`
	Username         string   `yaml:"username"`
	PasswordHash     string   `yaml:"password_hash,omitempty" json:"-"`
	AllowedAddresses []string `yaml:"allowed_addresses"`
}
type API struct {
	Enabled bool       `yaml:"enabled"`
	Tokens  []APIToken `yaml:"tokens,omitempty"`
}
type APIToken struct {
	Name   string   `yaml:"name"`
	Hash   string   `yaml:"hash" json:"-"`
	Scopes []string `yaml:"scopes"`
}
type Application struct {
	Mode           string `yaml:"mode"`
	OperatorName   string `yaml:"operator_name"`
	Locator        string `yaml:"locator"`
	QTH            string `yaml:"qth"`
	Language       string `yaml:"language"`
	UpdateChannel  string `yaml:"update_channel"`
	WelcomeMessage string `yaml:"welcome_message"`
	AwayMessage    string `yaml:"away_message"`
	GoodbyeMessage string `yaml:"goodbye_message"`
	InfoMessage    string `yaml:"info_message"`
	TerminalEOL    string `yaml:"terminal_eol"`
	AX25T1Seconds  int    `yaml:"ax25_t1_seconds"`
	AX25T3Seconds  int    `yaml:"ax25_t3_seconds"`
	AX25N2         int    `yaml:"ax25_n2"`
	AX25N1         int    `yaml:"ax25_n1"`
}
type Beacon struct {
	Enabled         bool   `yaml:"enabled"`
	Destination     string `yaml:"destination"`
	Via             string `yaml:"via,omitempty"`
	Text            string `yaml:"text"`
	IntervalMinutes int    `yaml:"interval_minutes"`
}
type UPRD struct {
	Enabled         bool `yaml:"enabled"`
	IntervalSeconds int  `yaml:"interval_seconds"`
	MHeardLimit     int  `yaml:"mheard_limit"`
}

// Experimental controls whether optional subsystems exist at runtime. Service
// configuration is intentionally kept in the service-specific sections below.
type Experimental struct {
	UPRD     bool `yaml:"uprd"`
	Map      bool `yaml:"map"`
	Services bool `yaml:"services"`
	// Legacy fields are retained so existing dev configurations migrate.
	Node bool `yaml:"node,omitempty"`
	BBS  bool `yaml:"bbs,omitempty"`
	AI   bool `yaml:"ai,omitempty"`
}
type AI struct {
	Enabled           bool   `yaml:"enabled"`
	Callsign          string `yaml:"callsign"`
	SSID              uint8  `yaml:"ssid"`
	Provider          string `yaml:"provider"`
	URL               string `yaml:"url"`
	Model             string `yaml:"model"`
	TimeoutSeconds    int    `yaml:"timeout_seconds"`
	MaxResponseChars  int    `yaml:"max_response_chars"`
	MaxContext        int    `yaml:"max_context"`
	SystemPrompt      string `yaml:"system_prompt"`
	QueueSize         int    `yaml:"queue_size"`
	Concurrency       int    `yaml:"concurrency"`
	WelcomeMessage    string `yaml:"welcome_message"`
	ProcessingMessage string `yaml:"processing_message"`
	GoodbyeMessage    string `yaml:"goodbye_message"`
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
	Enabled        bool          `yaml:"enabled"`
	Listen         string        `yaml:"listen"`
	ForwardListen  string        `yaml:"forward_listen"`
	Database       string        `yaml:"database"`
	Title          string        `yaml:"title"`
	Callsign       string        `yaml:"callsign"`
	SSID           uint8         `yaml:"ssid"`
	Address        string        `yaml:"hierarchical_address"`
	Language       string        `yaml:"language"`
	BeaconVia      string        `yaml:"beacon_via,omitempty"`
	Forwarding     BBSForwarding `yaml:"forwarding"`
	WelcomeMessage string        `yaml:"welcome_message"`
	GoodbyeMessage string        `yaml:"goodbye_message"`
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
	Enabled        bool           `yaml:"enabled"`
	Alias          string         `yaml:"alias"`
	Listen         string         `yaml:"listen"`
	Language       string         `yaml:"language"`
	Neighbors      []NodeNeighbor `yaml:"neighbors"`
	Routes         []NodeRoute    `yaml:"routes"`
	Services       []NodeService  `yaml:"services"`
	WelcomeMessage string         `yaml:"welcome_message"`
	GoodbyeMessage string         `yaml:"goodbye_message"`
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
	Enabled          *bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Host             string   `yaml:"host"`
	Port             uint16   `yaml:"port"`
	MaxFrameBytes    int      `yaml:"max_frame_bytes"`
	ReconnectSeconds int      `yaml:"reconnect_seconds"`
	KISSPort         uint8    `yaml:"kiss_port"`
	KISSTXDelay      *uint8   `yaml:"kiss_txdelay,omitempty"`
	KISSPersistence  *uint8   `yaml:"kiss_persistence,omitempty"`
	KISSSlotTime     *uint8   `yaml:"kiss_slottime,omitempty"`
	KISSTXTail       *uint8   `yaml:"kiss_txtail,omitempty"`
	KISSFullDuplex   *bool    `yaml:"kiss_full_duplex,omitempty"`
	TNCProxyEnabled  bool     `yaml:"tncproxy_enabled,omitempty" json:"tncproxy_enabled,omitempty"`
	TNCProxyPort     uint16   `yaml:"tncproxy_port,omitempty" json:"tncproxy_port,omitempty"`
	TNCProxyListen   string   `yaml:"tncproxy_listen,omitempty" json:"-"` // legacy address form
	Listen           string   `yaml:"listen"`
	RemoteHost       string   `yaml:"remote_host"`
	RemotePort       uint16   `yaml:"remote_port"`
	FCS              bool     `yaml:"fcs"`
	AllowFrom        []string `yaml:"allow_from"`
}
type Config struct {
	Application  Application  `yaml:"application"`
	Server       Station      `yaml:"server"`
	Web          Web          `yaml:"web"`
	API          API          `yaml:"api"`
	Ports        []Port       `yaml:"ports"`
	Terminal     Station      `yaml:"terminal"`
	Beacon       Beacon       `yaml:"beacon"`
	UPRD         UPRD         `yaml:"uprd"`
	Experimental Experimental `yaml:"experimental"`
	History      History      `yaml:"history"`
	BBS          BBS          `yaml:"bbs"`
	Node         Node         `yaml:"node"`
	AI           AI           `yaml:"ai"`
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
	var presence struct {
		Experimental *yaml.Node `yaml:"experimental"`
	}
	_ = yaml.Unmarshal(b, &presence)
	c.applyDefaults(presence.Experimental != nil)
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c *Config) applyDefaults(hasExperimental bool) {
	if !hasExperimental {
		c.Experimental = Experimental{UPRD: c.UPRD.Enabled, Map: c.UPRD.Enabled, Node: c.Node.Enabled, BBS: c.BBS.Enabled}
	}
	if !c.Experimental.Services && (c.Experimental.Node || c.Experimental.BBS || c.Experimental.AI) {
		c.Experimental.Services = true
		if c.Experimental.AI {
			c.AI.Enabled = true
		}
	}
	if strings.TrimSpace(c.Application.Mode) == "" {
		if c.Node.Enabled || c.BBS.Enabled {
			c.Application.Mode = "station-node-bbs"
		} else {
			c.Application.Mode = "station"
		}
	}
	if !validLanguage(c.Application.Language) {
		c.Application.Language = "pl"
	}
	if strings.TrimSpace(c.Application.UpdateChannel) == "" {
		c.Application.UpdateChannel = "main"
	}
	if c.Application.UpdateChannel == "stable" {
		c.Application.UpdateChannel = "main"
	}
	if c.Application.TerminalEOL == "" {
		c.Application.TerminalEOL = "cr"
	}
	if c.Application.AX25T1Seconds == 0 {
		c.Application.AX25T1Seconds = 10
	}
	if c.Application.AX25T3Seconds == 0 {
		c.Application.AX25T3Seconds = 300
	}
	if c.Application.AX25N2 == 0 {
		c.Application.AX25N2 = 10
	}
	if c.Application.AX25N1 == 0 {
		c.Application.AX25N1 = 256
	}
	if !c.UPRD.Enabled && c.UPRD.IntervalSeconds == 0 && c.UPRD.MHeardLimit == 0 {
		// UPRD was added after older configuration files were created.
		c.UPRD.Enabled = true
	}
	if c.UPRD.IntervalSeconds == 0 {
		c.UPRD.IntervalSeconds = 600
	}
	if c.UPRD.MHeardLimit == 0 {
		c.UPRD.MHeardLimit = 5
	}
	if strings.TrimSpace(c.Web.Listen) == "" {
		c.Web.Listen = "0.0.0.0:8080"
	}
	if strings.TrimSpace(c.Web.Username) == "" {
		c.Web.Username = "admin"
	}
	if len(c.Web.AllowedAddresses) == 0 {
		c.Web.AllowedAddresses = []string{"0.0.0.0"}
	}
	for i := range c.Ports {
		p := &c.Ports[i]
		if p.TNCProxyPort == 0 && p.TNCProxyListen != "" {
			if _, rawPort, err := net.SplitHostPort(p.TNCProxyListen); err == nil {
				if port, err := strconv.ParseUint(rawPort, 10, 16); err == nil {
					p.TNCProxyPort = uint16(port)
				}
			}
		}
		if p.TNCProxyEnabled && p.TNCProxyPort == 0 {
			p.TNCProxyPort = 8101
		}
	}
	c.applyTerminalMessageDefaults()
	if strings.TrimSpace(c.AI.Callsign) == "" {
		c.AI.Callsign = c.Terminal.Callsign
	}
	if c.AI.SSID == 0 {
		c.AI.SSID = 12
	}
	if c.AI.Provider == "" {
		c.AI.Provider = "ollama"
	}
	if c.AI.URL == "" {
		c.AI.URL = "http://192.168.1.50:11434"
	}
	if c.AI.Model == "" {
		c.AI.Model = "qwen3:8b"
	}
	if c.AI.TimeoutSeconds == 0 {
		c.AI.TimeoutSeconds = 120
	}
	if c.AI.MaxResponseChars == 0 {
		c.AI.MaxResponseChars = 2000
	}
	if c.AI.MaxContext == 0 {
		c.AI.MaxContext = 20
	}
	if c.AI.QueueSize == 0 {
		c.AI.QueueSize = 8
	}
	if c.AI.Concurrency == 0 {
		c.AI.Concurrency = 1
	}
	if strings.TrimSpace(c.Node.WelcomeMessage) == "" {
		c.Node.WelcomeMessage = "Witaj {REMOTE} w NODE {CALL}.\r\nDostepne funkcje: NODES, ROUTES, PORTS, SERVICES, BBS, AI, CONNECT i HELP."
	}
	if strings.TrimSpace(c.Node.GoodbyeMessage) == "" {
		c.Node.GoodbyeMessage = "73 {REMOTE}, NODE {CALL}."
	}
	if strings.TrimSpace(c.BBS.WelcomeMessage) == "" {
		c.BBS.WelcomeMessage = "Witaj {REMOTE} w BBS {CALL}.\r\nTutaj mozesz czytac, wysylac i przekazywac wiadomosci packet radio. Wpisz H, aby zobaczyc pomoc."
	}
	if strings.TrimSpace(c.BBS.GoodbyeMessage) == "" {
		c.BBS.GoodbyeMessage = "73 {REMOTE}, BBS {CALL}."
	}
	const legacyAIWelcome = "Witaj {REMOTE} w usludze IA {CALL}.\r\nWpisz pytanie, a otrzymasz krotka odpowiedz. Q konczy rozmowe."
	if current := strings.TrimSpace(c.AI.WelcomeMessage); current == "" || current == legacyAIWelcome {
		c.AI.WelcomeMessage = "Witaj {REMOTE} w usludze IA {CALL}.\r\nWpisz pytanie w jednej lub kilku liniach i zakoncz je komenda /EX. Mozesz tez dopisac /EX na koncu pytania. Q konczy rozmowe."
	}
	if strings.TrimSpace(c.AI.ProcessingMessage) == "" {
		c.AI.ProcessingMessage = "Dziekuje za wiadomosc. Rozpoczynam jej przetwarzanie. Prosze o cierpliwosc — odpowiedz moze zajac do kilku minut."
	}
	if strings.TrimSpace(c.AI.GoodbyeMessage) == "" {
		c.AI.GoodbyeMessage = "73 {REMOTE}, IA {CALL}."
	}
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
	if c.Application.Mode != "station" && c.Application.Mode != "station-node-bbs" {
		return fmt.Errorf("application.mode must be station or station-node-bbs")
	}
	// Legacy application.mode remains accepted, but optional services are now
	// independently gated by experimental.*.
	if c.Application.UpdateChannel != "main" && c.Application.UpdateChannel != "dev" {
		return fmt.Errorf("application.update_channel must be main or dev")
	}
	if c.Application.TerminalEOL != "cr" && c.Application.TerminalEOL != "crlf" && c.Application.TerminalEOL != "lf" {
		return fmt.Errorf("application.terminal_eol must be cr, crlf or lf")
	}
	if c.Application.AX25T1Seconds < 1 || c.Application.AX25T1Seconds > 60 {
		return fmt.Errorf("application.ax25_t1_seconds must be 1..60")
	}
	if c.Application.AX25T3Seconds <= c.Application.AX25T1Seconds || c.Application.AX25T3Seconds > 86400 {
		return fmt.Errorf("application.ax25_t3_seconds must be greater than T1 and at most 86400")
	}
	if c.Application.AX25N2 < 1 || c.Application.AX25N2 > 255 {
		return fmt.Errorf("application.ax25_n2 must be 1..255")
	}
	if c.Application.AX25N1 < 16 || c.Application.AX25N1 > 2048 {
		return fmt.Errorf("application.ax25_n1 must be 16..2048")
	}
	if locator := strings.ToUpper(strings.TrimSpace(c.Application.Locator)); locator != "" && !validLocator(locator) {
		return fmt.Errorf("application.locator: invalid Maidenhead locator")
	}
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
	if _, err := ax25.ParseDigipeaters(c.Beacon.Via); err != nil {
		return fmt.Errorf("beacon.via: %w", err)
	}
	if c.Beacon.Enabled && (strings.TrimSpace(c.Beacon.Text) == "" || c.Beacon.IntervalMinutes < 1) {
		return fmt.Errorf("beacon: text and interval >= 1 minute are required")
	}
	if c.UPRD.IntervalSeconds < 1 || c.UPRD.IntervalSeconds > 86400 {
		return fmt.Errorf("uprd.interval_seconds must be 1..86400")
	}
	if c.UPRD.MHeardLimit < 1 || c.UPRD.MHeardLimit > 10 {
		return fmt.Errorf("uprd.mheard_limit must be 1..10")
	}
	if c.History.Enabled {
		if strings.TrimSpace(c.History.Database) == "" || c.History.MaxStations < 1 || c.History.MaxSessionsPerStation < 1 || c.History.MaxLinesPerStation < 1 || c.History.MaxBytes < 1024 || c.History.RetentionDays < 1 {
			return fmt.Errorf("history: database and positive limits are required")
		}
	}
	if _, err := ax25.ParseDigipeaters(c.BBS.BeaconVia); err != nil {
		return fmt.Errorf("bbs.beacon_via: %w", err)
	}
	_, _, err := net.SplitHostPort(c.Web.Listen)
	if err != nil {
		return fmt.Errorf("web.listen: %w", err)
	}
	if strings.TrimSpace(c.Web.Username) == "" {
		return fmt.Errorf("web.username is required")
	}
	seenTokens := map[string]bool{}
	for i, token := range c.API.Tokens {
		name := strings.TrimSpace(token.Name)
		hash := strings.ToLower(strings.TrimSpace(token.Hash))
		if name == "" || seenTokens[name] {
			return fmt.Errorf("api.tokens[%d]: name is empty or duplicated", i)
		}
		seenTokens[name] = true
		if len(hash) != 64 {
			return fmt.Errorf("api.tokens[%d].hash must be a SHA-256 hex digest", i)
		}
		if _, err := strconv.ParseUint(hash[:16], 16, 64); err != nil {
			return fmt.Errorf("api.tokens[%d].hash must be hexadecimal", i)
		}
	}
	for i, address := range c.Web.AllowedAddresses {
		address = strings.TrimSpace(address)
		if address == "0.0.0.0" || address == "::" {
			continue
		}
		if !netallow.ValidRule(address) {
			return fmt.Errorf("web.allowed_addresses[%d]: expected IP address, CIDR network, or hostname", i)
		}
	}
	if c.BBS.Enabled {
		if _, _, err := net.SplitHostPort(c.BBS.Listen); err != nil {
			return fmt.Errorf("bbs.listen: %w", err)
		}
		if strings.TrimSpace(c.BBS.Database) == "" {
			return fmt.Errorf("bbs.database is required")
		}
	}
	if c.Experimental.Services && c.AI.Enabled {
		if err := validStation(Station{Callsign: c.AI.Callsign, SSID: c.AI.SSID}); err != nil {
			return fmt.Errorf("ai: %w", err)
		}
		if c.AI.Provider != "ollama" {
			return fmt.Errorf("ai.provider: only ollama is currently supported")
		}
		u, err := neturl.Parse(c.AI.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("ai.url: valid HTTP(S) URL is required")
		}
		if strings.TrimSpace(c.AI.Model) == "" {
			return fmt.Errorf("ai.model is required")
		}
		if c.AI.TimeoutSeconds < 1 || c.AI.MaxResponseChars < 1 || c.AI.MaxContext < 1 || c.AI.QueueSize < 1 || c.AI.Concurrency < 1 {
			return fmt.Errorf("ai: timeout and limits must be positive")
		}
	}
	seen := map[string]bool{}
	for i, p := range c.Ports {
		if p.Enabled == nil {
			enabled := true
			c.Ports[i].Enabled = &enabled
			p.Enabled = c.Ports[i].Enabled
		}
		if p.ID == "" || seen[p.ID] {
			return fmt.Errorf("ports[%d]: id is empty or duplicated", i)
		}
		seen[p.ID] = true
		if p.Type != "kiss-tcp" && p.Type != "axudp" {
			return fmt.Errorf("ports[%d]: unsupported type %q", i, p.Type)
		}
		if p.MaxFrameBytes < 256 || p.MaxFrameBytes > 65535 {
			return fmt.Errorf("ports[%d]: max_frame_bytes out of range", i)
		}
		if p.Type == "kiss-tcp" && (p.ReconnectSeconds < 1 || p.ReconnectSeconds > 3600) {
			return fmt.Errorf("ports[%d]: reconnect_seconds out of range", i)
		}
		if p.Type == "kiss-tcp" && p.KISSPort > 15 {
			return fmt.Errorf("ports[%d]: kiss_port must be 0..15", i)
		}
		if p.Type == "kiss-tcp" && p.TNCProxyEnabled {
			if p.TNCProxyPort == 0 {
				return fmt.Errorf("ports[%d]: tncproxy_port must be between 1 and 65535", i)
			}
		}
		if p.Enabled != nil && !*p.Enabled {
			continue
		}
		if p.Type == "kiss-tcp" && (p.Host == "" || p.Port == 0) {
			return fmt.Errorf("ports[%d]: invalid KISS host or port", i)
		}
		if p.Type == "axudp" {
			if _, _, err := net.SplitHostPort(p.Listen); err != nil {
				return fmt.Errorf("ports[%d].listen: %w", i, err)
			}
			if p.RemoteHost == "" || p.RemotePort == 0 {
				return fmt.Errorf("ports[%d]: AXUDP remote host and port required", i)
			}
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
			if strings.TrimSpace(c.BBS.Address) == "" {
				return fmt.Errorf("bbs.hierarchical_address is required for TAPR forwarding")
			}
		}
		if c.BBS.Callsign != "" {
			if err := validStation(Station{Callsign: c.BBS.Callsign, SSID: c.BBS.SSID}); err != nil {
				return fmt.Errorf("bbs: %w", err)
			}
		}
		if strings.TrimSpace(c.BBS.Address) != "" {
			if _, err := bbscore.ParseHierarchicalAddress(c.BBS.Address); err != nil {
				return fmt.Errorf("bbs.hierarchical_address: %w", err)
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
			for j, scope := range p.BulletinScopes {
				if scope != "*" {
					if _, err := bbscore.ParseDistributionDesignator(scope); err != nil {
						return fmt.Errorf("bbs.forwarding.peers[%d].bulletin_scopes[%d]: %w", i, j, err)
					}
				}
			}
		}
	}
	return nil
}

func validLocator(v string) bool {
	v = strings.ToUpper(strings.TrimSpace(v))
	if len(v) != 6 {
		return false
	}
	for i, r := range v {
		switch {
		case i < 2:
			if r < 'A' || r > 'Z' {
				return false
			}
		case i < 4:
			if r < '0' || r > '9' {
				return false
			}
		default:
			if r < 'A' || r > 'Z' {
				return false
			}
		}
	}
	return true
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
