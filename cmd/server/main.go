package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/bbs"
	"github.com/packet-radio/ultimatepr/internal/config"
	"github.com/packet-radio/ultimatepr/internal/digipeater"
	"github.com/packet-radio/ultimatepr/internal/history"
	"github.com/packet-radio/ultimatepr/internal/mheard"
	"github.com/packet-radio/ultimatepr/internal/monitor"
	nodecore "github.com/packet-radio/ultimatepr/internal/node"
	"github.com/packet-radio/ultimatepr/internal/session"
	"github.com/packet-radio/ultimatepr/internal/transport"
	"github.com/packet-radio/ultimatepr/internal/transport/axudp"
	"github.com/packet-radio/ultimatepr/internal/transport/kiss"
	webui "github.com/packet-radio/ultimatepr/internal/web"
)

func main() {
	path := flag.String("config", "config.yaml", "configuration file")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load(*path)
	if os.IsNotExist(err) {
		if err = webui.RunSetup(ctx, "0.0.0.0:8080", *path, log); err != nil {
			log.Error("first-run setup failed", "error", err)
			os.Exit(2)
		}
		if ctx.Err() != nil {
			return
		}
		cfg, err = config.Load(*path)
	}
	if err != nil {
		log.Error("configuration failed", "error", err)
		os.Exit(2)
	}
	digiAliases := []ax25.Address{{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}}
	if cfg.BBS.Enabled {
		digiAliases = append(digiAliases, ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID})
	}
	digi := digipeater.New(digiAliases...)
	log.Info("AX.25 digipeater enabled", "aliases", func() []string {
		out := make([]string, len(digiAliases))
		for i, a := range digiAliases {
			out[i] = a.String()
		}
		return out
	}())
	var restartRequested atomic.Bool
	rx := make(chan transport.Packet, 256)
	portIDs := make([]string, 0, len(cfg.Ports))
	senders := make(map[string]session.Sender, len(cfg.Ports))
	mon := monitor.New(300)
	ports := make([]transport.Port, 0, len(cfg.Ports))
	for _, pc := range cfg.Ports {
		portIDs = append(portIDs, pc.ID)
		var p transport.Port
		if pc.Type == "axudp" {
			p = axudp.New(axudp.Config{ID: pc.ID, Listen: pc.Listen, RemoteHost: pc.RemoteHost, RemotePort: pc.RemotePort, FCS: pc.FCS, AllowFrom: pc.AllowFrom, MaxFrame: pc.MaxFrameBytes, Queue: 256}, log)
		} else {
			p = kiss.NewTCPPort(kiss.TCPConfig{ID: pc.ID, Address: net.JoinHostPort(pc.Host, fmtPort(pc.Port)), Channel: pc.Channel, MaxFrame: pc.MaxFrameBytes, Reconnect: time.Duration(pc.ReconnectSeconds) * time.Second, Queue: 256}, log)
		}
		senders[pc.ID] = func(port transport.Port, channel uint8) session.Sender {
			return func(ctx context.Context, b []byte) error {
				if f, err := ax25.Decode(b); err == nil {
					mon.Add("TX", port.ID(), f, len(b))
				}
				return port.Send(ctx, transport.Packet{PortID: port.ID(), Channel: channel, Data: b})
			}
		}(p, pc.Channel)
		ports = append(ports, p)
		go func() {
			if e := p.Run(ctx, rx); e != nil && ctx.Err() == nil {
				log.Error("port stopped", "port", p.ID(), "error", e)
			}
		}()
	}
	var nodeRouter *nodecore.Router
	if cfg.Node.Enabled {
		neighbors := make([]nodecore.Neighbor, 0, len(cfg.Node.Neighbors))
		for _, n := range cfg.Node.Neighbors {
			if !n.Enabled {
				continue
			}
			neighbors = append(neighbors, nodecore.Neighbor{ID: n.ID, Callsign: n.Callsign, Port: n.Port, Quality: n.Quality, Locked: n.Locked})
		}
		routes := make([]nodecore.Route, 0, len(cfg.Node.Routes))
		for _, r := range cfg.Node.Routes {
			if !r.Enabled {
				continue
			}
			routes = append(routes, nodecore.Route{Destination: r.Destination, Via: r.Via, Quality: r.Quality})
		}
		services := make([]nodecore.Service, 0, len(cfg.Node.Services))
		for _, s := range cfg.Node.Services {
			services = append(services, nodecore.Service{Name: s.Name, Callsign: s.Callsign, Command: s.Command, Enabled: s.Enabled})
		}
		nodeRouter = nodecore.New(neighbors, routes, services)
		log.Info("node routing configured", "alias", cfg.Node.Alias, "neighbors", len(neighbors), "routes", len(routes), "services", len(services))
	}
	radio := session.NewHub(ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}, senders)
	heard := mheard.New(200)
	var historyStore *history.Store
	if cfg.History.Enabled {
		historyStore, err = history.Open(cfg.History.Database, history.Limits{MaxStations: cfg.History.MaxStations, MaxSessions: cfg.History.MaxSessionsPerStation, MaxLines: cfg.History.MaxLinesPerStation, MaxBytes: cfg.History.MaxBytes, RetentionDays: cfg.History.RetentionDays})
		if err != nil {
			log.Error("history database failed", "error", err)
			os.Exit(2)
		}
	}
	beaconPort := cfg.Beacon.Port
	if beaconPort == "" && len(portIDs) > 0 {
		beaconPort = portIDs[0]
	}
	beaconDestination := cfg.Beacon.Destination
	if strings.TrimSpace(beaconDestination) == "" {
		beaconDestination = "BEACON"
	}
	sendBeaconFrame := func(sendCtx context.Context, source ax25.Address, text string) error {
		send := senders[beaconPort]
		if send == nil {
			return fmt.Errorf("beacon port %q unavailable", beaconPort)
		}
		dst, err := ax25.ParseAddress(beaconDestination)
		if err != nil {
			return err
		}
		pid := byte(0xF0)
		f := ax25.Frame{Destination: dst, Source: source, Type: ax25.TypeUI, PID: &pid, Payload: []byte(text)}
		b, err := ax25.Encode(f)
		if err != nil {
			return err
		}
		return send(sendCtx, b)
	}
	sendBeacon := func(sendCtx context.Context) error {
		source := ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}
		return sendBeaconFrame(sendCtx, source, beaconText(source, cfg.Application.Locator, cfg.Application.QTH, heard.List(), digiAliases...))
	}
	sendBBSBeacon := func(sendCtx context.Context) error {
		source := ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID}
		return sendBeaconFrame(sendCtx, source, beaconText(source, cfg.Application.Locator, cfg.Application.QTH, heard.List(), digiAliases...))
	}
	inbound := session.NewInboundMux(senders, log)
	web := webui.New(webui.Config{
		Listen:           cfg.Web.Listen,
		Username:         cfg.Web.Username,
		PasswordHash:     cfg.Web.PasswordHash,
		AllowedAddresses: cfg.Web.AllowedAddresses,
		NodeCallsign:     cfg.Server.Callsign,
		NodeSSID:         cfg.Server.SSID,
		BBSCallsign:      cfg.BBS.Callsign,
		BSSSID:           cfg.BBS.SSID,
		TerminalCallsign: cfg.Terminal.Callsign,
		TerminalSSID:     cfg.Terminal.SSID,
		Ports:            portIDs,
		NodeEnabled:      cfg.Node.Enabled,
		PortStatus: func() []transport.Status {
			result := make([]transport.Status, 0, len(ports))
			for _, p := range ports {
				result = append(result, p.Status())
			}
			return result
		},
		Radio:      radio,
		MHeard:     heard,
		History:    historyStore,
		Monitor:    mon,
		SendBeacon: sendBeacon,
		BBSListen: func() string {
			if cfg.BBS.Enabled {
				return cfg.BBS.Listen
			}
			return ""
		}(),
		ConfigPath: *path,
		RequestRestart: func() {
			restartRequested.Store(true)
			stop()
		},
		Version: bbs.BuildVersion,
	}, log)
	inbound.Register(ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}, web.ServeOperatorAX25)
	go runBeaconSchedule(ctx, web, true, cfg.BBS.Enabled, sendBeacon, sendBBSBeacon, log)
	var bbsServer *bbs.Server
	if cfg.BBS.Enabled {
		store, err := bbs.Open(cfg.BBS.Database)
		if err != nil {
			log.Error("BBS database failed", "error", err)
			os.Exit(2)
		}
		bbsServer = &bbs.Server{Listen: cfg.BBS.Listen, Title: cfg.BBS.Title, Node: ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID}.String(), Address: cfg.BBS.Address, Language: cfg.BBS.Language, Store: store, Log: log}
		if cfg.BBS.Forwarding.Enabled {
			peers := make([]bbs.ForwardPeer, 0, len(cfg.BBS.Forwarding.Peers))
			for _, p := range cfg.BBS.Forwarding.Peers {
				peers = append(peers, bbs.ForwardPeer{ID: p.ID, Callsign: p.Callsign, Transport: p.Transport, Host: p.Host, Port: p.Port, ViaNode: p.ViaNode, PrivateRoutes: p.PrivateRoutes, BulletinScopes: p.BulletinScopes, Enabled: p.Enabled})
			}
			planner := &bbs.QueuePlanner{Store: store, Peers: peers, Interval: time.Duration(cfg.BBS.Forwarding.IntervalMinutes) * time.Minute, MaxPerPeer: cfg.BBS.Forwarding.MaxMessages, Log: log}
			go planner.Run(ctx)
			forwarder := &bbs.Forwarder{Store: store, Peers: peers, Interval: time.Duration(cfg.BBS.Forwarding.IntervalMinutes) * time.Minute, ConnectTimeout: time.Duration(cfg.BBS.Forwarding.ConnectTimeoutSeconds) * time.Second, SessionTimeout: time.Duration(cfg.BBS.Forwarding.SessionTimeoutSeconds) * time.Second, MaxMessages: cfg.BBS.Forwarding.MaxMessages, LocalCall: bbsServer.Node, Log: log}
			bbsServer.OnMessage = forwarder.Trigger
			go forwarder.Run(ctx)
			go func() {
				if err := bbsServer.RunForward(ctx, cfg.BBS.ForwardListen); err != nil && ctx.Err() == nil {
					log.Error("BBS forward listener stopped", "error", err)
					stop()
				}
			}()
			log.Info("BBS forwarding started", "listen", cfg.BBS.ForwardListen, "peers", len(peers))
		}
		web.SetBBS(store)
		go func() {
			if err := bbsServer.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("BBS server stopped", "error", err)
				stop()
			}
		}()
		log.Info("BBS service started", "listen", cfg.BBS.Listen, "database", cfg.BBS.Database)
	}
	if cfg.Node.Enabled {
		nodeServer := &nodecore.Server{Listen: cfg.Node.Listen, Callsign: ax25.Address{Callsign: cfg.Server.Callsign, SSID: cfg.Server.SSID}.String(), Alias: cfg.Node.Alias, Language: cfg.Node.Language, Router: nodeRouter, BBS: bbsServer, Ports: portIDs, Log: log}
		inbound.Register(ax25.Address{Callsign: cfg.Server.Callsign, SSID: cfg.Server.SSID}, nodeServer.ServeAX25)
		if bbsServer != nil {
			inbound.Register(ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID}, bbsServer.ServeAX25)
		}
		go func() {
			if err := nodeServer.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("node server stopped", "error", err)
				stop()
			}
		}()
		log.Info("node service started", "listen", cfg.Node.Listen, "alias", cfg.Node.Alias)
	}
	go func() {
		if err := web.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("web server stopped", "error", err)
			stop()
		}
	}()
	log.Info("server started", "callsign", cfg.Server.Callsign, "ssid", cfg.Server.SSID, "web", cfg.Web.Listen)
	for {
		select {
		case pkt := <-rx:
			f, e := ax25.Decode(pkt.Data)
			if e != nil {
				log.Warn("invalid AX.25 frame", "port", pkt.PortID, "error", e)
				continue
			}
			log.Info("frame rx", "port", pkt.PortID, "source", f.Source.String(), "destination", f.Destination.String(), "type", f.Type, "bytes", len(f.Payload))
			heard.Heard(f.Source.String(), pkt.PortID)
			if f.Type == ax25.TypeUI && strings.EqualFold(f.Destination.String(), "BEACON") && len(f.Payload) > 0 {
				heard.Beacon(f.Source.String(), pkt.PortID, string(f.Payload))
				reported := parseUltimatePRBeacon(string(f.Payload))
				reported = withoutCallsigns(reported, digiAliases...)
				heard.Reported(reported, f.Source.String(), pkt.PortID)
			}
			mon.Add("RX", pkt.PortID, f, len(pkt.Data))
			if repeated, ok := digi.Repeat(f, pkt.Data); ok {
				if send := senders[pkt.PortID]; send != nil {
					if err := send(ctx, repeated); err != nil {
						log.Warn("digipeater transmit failed", "port", pkt.PortID, "error", err)
					} else {
						log.Info("frame digipeated", "port", pkt.PortID, "source", f.Source.String())
					}
				}
				continue
			}
			if !inbound.Handle(pkt.PortID, f) {
				radio.Handle(pkt.PortID, f)
			}
		case <-ctx.Done():
			log.Info("server stopped")
			if restartRequested.Load() {
				log.Info("server restart requested")
				os.Exit(75)
			}
			return
		}
	}
}
func fmtPort(p uint16) string {
	const digits = "0123456789"
	if p == 0 {
		return "0"
	}
	var b [5]byte
	i := len(b)
	for p > 0 {
		i--
		b[i] = digits[p%10]
		p /= 10
	}
	return string(b[i:])
}

func runBeaconSchedule(ctx context.Context, web *webui.Server, stationEnabled, bbsEnabled bool, sendStation, sendBBS func(context.Context) error, log *slog.Logger) {
	const interval = 10 * time.Minute
	if bbsEnabled {
		go func() {
			timer := time.NewTimer(interval / 2)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				if err := sendBBS(ctx); err != nil {
					log.Warn("BBS beacon failed", "error", err)
				}
				select {
				case <-ticker.C:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	if stationEnabled {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if web.HasActiveBrowser() {
						if err := sendStation(ctx); err != nil {
							log.Warn("station beacon failed", "error", err)
						}
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func beaconText(source ax25.Address, locator, qth string, entries []mheard.Entry, excluded ...ax25.Address) string {
	lines := []string{source.String()}
	if locator = strings.ToUpper(strings.TrimSpace(locator)); locator != "" {
		lines = append(lines, locator)
	}
	if qth = strings.TrimSpace(qth); qth != "" {
		lines = append(lines, qth)
	}
	lines = append(lines, "DIGI", "UltimatePR")

	calls := make([]string, 0, 5)
	seen := make(map[string]struct{}, 5)
	for _, entry := range entries {
		call := strings.ToUpper(strings.TrimSpace(entry.Callsign))
		if call == "" {
			continue
		}
		own := false
		for _, alias := range excluded {
			if call == alias.String() {
				own = true
				break
			}
		}
		if own {
			continue
		}
		if _, exists := seen[call]; exists {
			continue
		}
		seen[call] = struct{}{}
		calls = append(calls, call)
		if len(calls) == 5 {
			break
		}
	}
	if len(calls) > 0 {
		lines = append(lines, strings.Join(calls, ","))
	}
	return strings.Join(lines, "\r")
}

func withoutCallsigns(calls []string, excluded ...ax25.Address) []string {
	blocked := make(map[string]struct{}, len(excluded))
	for _, address := range excluded {
		blocked[address.String()] = struct{}{}
	}
	result := calls[:0]
	for _, call := range calls {
		if _, found := blocked[call]; !found {
			result = append(result, call)
		}
	}
	return result
}

func parseUltimatePRBeacon(text string) []string {
	lines := strings.FieldsFunc(strings.ReplaceAll(text, "\r\n", "\n"), func(r rune) bool { return r == '\r' || r == '\n' })
	software := -1
	for i := 0; i+1 < len(lines); i++ {
		if strings.EqualFold(strings.TrimSpace(lines[i]), "DIGI") && strings.EqualFold(strings.TrimSpace(lines[i+1]), "UltimatePR") {
			software = i + 1
			break
		}
	}
	if software < 0 || software+1 >= len(lines) {
		return nil
	}
	result := make([]string, 0, 5)
	seen := make(map[string]struct{}, 5)
	for _, raw := range strings.Split(lines[software+1], ",") {
		call := strings.ToUpper(strings.TrimSpace(raw))
		address, err := ax25.ParseAddress(call)
		if err != nil || address.String() != call {
			continue
		}
		if _, exists := seen[call]; exists {
			continue
		}
		seen[call] = struct{}{}
		result = append(result, call)
		if len(result) == 5 {
			break
		}
	}
	return result
}
