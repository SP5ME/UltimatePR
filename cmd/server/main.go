package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	aiservice "github.com/packet-radio/ultimatepr/internal/ai"
	publicapi "github.com/packet-radio/ultimatepr/internal/api"
	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/ax25core"
	"github.com/packet-radio/ultimatepr/internal/bbs"
	"github.com/packet-radio/ultimatepr/internal/config"
	"github.com/packet-radio/ultimatepr/internal/digipeater"
	"github.com/packet-radio/ultimatepr/internal/gamehall"
	"github.com/packet-radio/ultimatepr/internal/history"
	"github.com/packet-radio/ultimatepr/internal/lineinput"
	"github.com/packet-radio/ultimatepr/internal/mheard"
	"github.com/packet-radio/ultimatepr/internal/monitor"
	"github.com/packet-radio/ultimatepr/internal/netrom"
	nodecore "github.com/packet-radio/ultimatepr/internal/node"
	"github.com/packet-radio/ultimatepr/internal/service"
	"github.com/packet-radio/ultimatepr/internal/session"
	"github.com/packet-radio/ultimatepr/internal/tncproxy"
	"github.com/packet-radio/ultimatepr/internal/transport"
	"github.com/packet-radio/ultimatepr/internal/transport/axudp"
	"github.com/packet-radio/ultimatepr/internal/transport/kiss"
	"github.com/packet-radio/ultimatepr/internal/transport/loopback"
	"github.com/packet-radio/ultimatepr/internal/uprd"
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
	// NODE is an independent subsystem; the legacy services switch only
	// controls the optional BBS/AI/GameHall group.
	nodeEnabled := cfg.Node.Enabled
	bbsEnabled := cfg.Experimental.Services && cfg.BBS.Enabled
	aiEnabled := cfg.Experimental.Services && cfg.AI.Enabled
	gameHallEnabled := cfg.Experimental.Services && cfg.GameHall.Enabled
	// UPRD has its own visible configuration switch. The legacy experimental
	// flag gates the optional map, not status generation or transmission.
	uprdEnabled := cfg.UPRD.Enabled
	stationBeaconVia, err := ax25.ParseDigipeaters(cfg.Beacon.Via)
	if err != nil {
		log.Error("station beacon path failed", "error", err)
		os.Exit(2)
	}
	bbsBeaconVia, err := ax25.ParseDigipeaters(cfg.BBS.BeaconVia)
	if err != nil {
		log.Error("BBS beacon path failed", "error", err)
		os.Exit(2)
	}
	digiAliases := []ax25.Address{{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}}
	if bbsEnabled {
		digiAliases = append(digiAliases, ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID})
	}
	if aiEnabled {
		digiAliases = append(digiAliases, ax25.Address{Callsign: cfg.AI.Callsign, SSID: cfg.AI.SSID})
	}
	if gameHallEnabled {
		digiAliases = append(digiAliases, ax25.Address{Callsign: cfg.GameHall.Callsign, SSID: cfg.GameHall.SSID})
	}
	ownCalls := map[string]struct{}{
		ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}.String(): {},
	}
	if bbsEnabled {
		ownCalls[ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID}.String()] = struct{}{}
	}
	if aiEnabled {
		ownCalls[ax25.Address{Callsign: cfg.AI.Callsign, SSID: cfg.AI.SSID}.String()] = struct{}{}
	}
	if gameHallEnabled {
		ownCalls[ax25.Address{Callsign: cfg.GameHall.Callsign, SSID: cfg.GameHall.SSID}.String()] = struct{}{}
	}
	isOwnCallsign := func(call string) bool {
		_, ok := ownCalls[strings.ToUpper(strings.TrimSpace(call))]
		return ok
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
	var rxOrder atomic.Uint64
	rx := make(chan transport.Packet, 256)
	portIDs := make([]string, 0, len(cfg.Ports))
	senders := make(map[string]session.Sender, len(cfg.Ports))
	mon := monitor.New(300)
	events := publicapi.NewBroker()
	var digipeated atomic.Uint64
	var lastDigipeated atomic.Int64
	ports := make([]transport.Port, 0, len(cfg.Ports))
	type runningPort struct {
		port    transport.Port
		enabled bool
		mu      sync.Mutex
		cancel  context.CancelFunc
		done    chan struct{}
	}
	startPort := func(r *runningPort) {
		portCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		r.cancel = cancel
		r.done = done
		go func(p transport.Port, pctx context.Context, done chan struct{}) {
			defer close(done)
			if e := p.Run(pctx, rx); e != nil && pctx.Err() == nil {
				log.Error("port stopped", "port", p.ID(), "error", e)
			}
		}(r.port, portCtx, done)
	}
	restartPort := func(id string, runtimes map[string]*runningPort) error {
		r := runtimes[id]
		if r == nil {
			return fmt.Errorf("port %q not found", id)
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.enabled {
			return fmt.Errorf("port %q is disabled", id)
		}
		if r.cancel != nil {
			r.cancel()
			if r.done != nil {
				<-r.done
			}
		}
		startPort(r)
		return nil
	}
	runtimes := make(map[string]*runningPort, len(cfg.Ports))
	logicalPorts := make(map[string]struct{}, len(cfg.Ports))
	for _, pc := range cfg.Ports {
		cfgPort := pc
		interfaceID := cfgPort.InterfaceID
		if strings.TrimSpace(interfaceID) == "" {
			interfaceID = cfgPort.ID
		}
		channelPorts := cfgPort.Channels
		kissPortMap := cfgPort.Channels
		if cfgPort.Type == "axudp" {
			channelPorts = map[uint8]string{0: cfgPort.ID}
			kissPortMap = nil
		}
		if len(channelPorts) == 0 {
			channelPorts = map[uint8]string{cfgPort.KISSPort: cfgPort.ID}
		}
		for _, logicalID := range channelPorts {
			if _, exists := logicalPorts[logicalID]; !exists {
				portIDs = append(portIDs, logicalID)
				logicalPorts[logicalID] = struct{}{}
			}
		}
		enabled := cfgPort.Enabled == nil || *cfgPort.Enabled
		var p transport.Port
		if cfgPort.Type == "axudp" {
			p = axudp.New(axudp.Config{ID: cfgPort.ID, InterfaceID: interfaceID, Listen: cfgPort.Listen, RemoteHost: cfgPort.RemoteHost, RemotePort: cfgPort.RemotePort, FCS: cfgPort.FCS, AllowFrom: cfgPort.AllowFrom, MaxFrame: cfgPort.MaxFrameBytes, Queue: 256}, log)
		} else {
			address := net.JoinHostPort(cfgPort.Host, fmtPort(cfgPort.Port))
			if cfgPort.TNCProxyEnabled {
				listen := net.JoinHostPort("0.0.0.0", fmtPort(cfgPort.TNCProxyPort))
				if err := tncproxy.Start(ctx, listen, address, cfg.Web.AllowedAddresses, log); err != nil {
					log.Error("TNC proxy failed to start", "port", cfgPort.ID, "error", err)
					os.Exit(2)
				}
				address = net.JoinHostPort("127.0.0.1", fmtPort(cfgPort.TNCProxyPort))
			}
			p = kiss.NewTCPPort(kiss.TCPConfig{ID: cfgPort.ID, InterfaceID: interfaceID, Address: address, MaxFrame: cfgPort.MaxFrameBytes, Reconnect: time.Duration(cfgPort.ReconnectSeconds) * time.Second, Queue: 256, Port: cfgPort.KISSPort, PortMap: kissPortMap, TXDelay: cfgPort.KISSTXDelay, Persistence: cfgPort.KISSPersistence, SlotTime: cfgPort.KISSSlotTime, TXTail: cfgPort.KISSTXTail, FullDuplex: cfgPort.KISSFullDuplex}, log)
		}
		runtime := &runningPort{port: p, enabled: enabled}
		// Keep the configured transport ID addressable for existing status and
		// restart paths; channel mappings expose the logical PortIDs separately.
		runtimes[cfgPort.ID] = runtime
		for channel, logicalID := range channelPorts {
			runtimes[logicalID] = runtime
			senders[logicalID] = func(port transport.Port, active bool, id string, interfaceID string, channel uint8) session.Sender {
				return func(ctx context.Context, b []byte) error {
					if !active {
						return fmt.Errorf("port %q is disabled", id)
					}
					if f, err := ax25.Decode(b); err == nil {
						mon.Add("TX", id, f, len(b))
						events.Publish("frame.tx", map[string]any{"port": id, "source": f.Source.String(), "destination": f.Destination.String(), "bytes": len(b)})
					}
					return port.Send(ctx, transport.Packet{InterfaceID: interfaceID, PortID: id, Channel: channel, Data: b})
				}
			}(p, enabled, logicalID, interfaceID, channel)
		}
		ports = append(ports, p)
		if enabled {
			startPort(runtime)
		}
	}
	localPort := loopback.New(rx)
	localSend := session.LocalSender(func(port string) session.Sender {
		return func(ctx context.Context, b []byte) error {
			if f, err := ax25.Decode(b); err == nil {
				mon.Add("TX", port, f, len(b))
				events.Publish("frame.tx", map[string]any{"port": port, "source": f.Source.String(), "destination": f.Destination.String(), "bytes": len(b)})
			}
			return localPort.Send(ctx, transport.Packet{PortID: port, Data: b})
		}
	})
	var nodeRouter *nodecore.Router
	services := service.NewRegistry()
	if nodeEnabled {
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
		advertisedServices := make([]nodecore.Service, 0, len(cfg.Node.Services))
		for _, s := range cfg.Node.Services {
			advertisedServices = append(advertisedServices, nodecore.Service{Name: s.Name, Callsign: s.Callsign, Command: s.Command, Enabled: s.Enabled})
		}
		nodeRouter = nodecore.New(neighbors, routes, advertisedServices)
		log.Info("node routing configured", "alias", cfg.Node.Alias, "neighbors", len(neighbors), "routes", len(routes), "services", len(advertisedServices))
		if cfg.Node.NetROMEnabled {
			mnemonic := cfg.Node.NetROMMnemonic
			if strings.TrimSpace(mnemonic) == "" {
				mnemonic = cfg.Node.Alias
			}
			interval := time.Duration(cfg.Node.NetROMInterval) * time.Second
			if interval <= 0 {
				interval = time.Hour
			}
			obsolescence := cfg.Node.NetROMObsolescence
			if obsolescence == 0 {
				obsolescence = 6
			}
			minQuality := cfg.Node.NetROMMinQuality
			if minQuality == 0 {
				minQuality = 1
			}
			maxDestinations := cfg.Node.NetROMMaxDestinations
			if maxDestinations <= 0 {
				maxDestinations = 50
			}
			go func() {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						b := netrom.RoutingBroadcast{Sender: mnemonic}
						own := ax25.Address{Callsign: cfg.Server.Callsign, SSID: cfg.Server.SSID}
						b.Destinations = append(b.Destinations, netrom.Destination{Callsign: own, Mnemonic: mnemonic, Neighbor: own, Quality: 255})
						for _, route := range nodeRouter.Routes() {
							if len(b.Destinations) >= maxDestinations || route.Quality < int(minQuality) || (route.Learned && route.Obsolescence == 0) {
								continue
							}
							n, ok := nodeRouter.Neighbor(route.Via)
							if !ok {
								continue
							}
							name := route.Destination
							if len(name) > netrom.MnemonicSize {
								name = name[:netrom.MnemonicSize]
							}
							destination, err := ax25.ParseAddress(route.Destination)
							if err != nil {
								continue
							}
							quality := route.Quality
							if quality > 255 {
								quality = 255
							}
							neighborCall, err := ax25.ParseAddress(n.Callsign)
							if err != nil {
								continue
							}
							b.Destinations = append(b.Destinations, netrom.Destination{Callsign: destination, Mnemonic: name, Neighbor: neighborCall, Quality: uint8(quality)})
						}
						pid := netrom.PID
						for start := 0; start < len(b.Destinations); start += netrom.MaxRoutingDestinations {
							end := start + netrom.MaxRoutingDestinations
							if end > len(b.Destinations) {
								end = len(b.Destinations)
							}
							part := netrom.RoutingBroadcast{Sender: b.Sender, Destinations: b.Destinations[start:end]}
							payload, err := part.Encode()
							if err != nil {
								log.Warn("NET/ROM routing broadcast build failed", "error", err)
								continue
							}
							frame, err := ax25.Encode(ax25.Frame{Destination: ax25.Address{Callsign: "NODES"}, Source: own, Type: ax25.TypeUI, PID: &pid, Payload: payload})
							if err != nil {
								continue
							}
							for _, portID := range portIDs {
								runtime := runtimes[portID]
								if runtime != nil && runtime.enabled && runtime.port.Status().Connected {
									if send := senders[portID]; send != nil {
										_ = send(ctx, frame)
									}
								}
							}
						}
						nodeRouter.AgeLearned()
					}
				}
			}()
		}
	}
	outboundSenders := make(map[string]session.Sender, len(senders))
	for id, sender := range senders {
		if runtimes[id] != nil && runtimes[id].enabled {
			outboundSenders[id] = sender
		}
	}
	radio := session.NewHub(ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}, outboundSenders)
	radio.SetLocalDelivery(func(address ax25.Address) bool {
		_, ok := services.ByCallsign(address.String())
		return ok
	}, localSend)
	radio.Configure(time.Duration(cfg.Application.AX25T1Seconds)*time.Second, cfg.Application.AX25N2, cfg.Application.AX25N1)
	heard := mheard.New(200)
	mheardSnapshotPath := *path + ".mheard-snapshot.json"
	configuredPorts := make(map[string]bool, len(portIDs))
	for _, id := range portIDs {
		configuredPorts[id] = true
	}
	if err := heard.LoadSnapshot(mheardSnapshotPath, configuredPorts); err == nil {
		_ = os.Remove(mheardSnapshotPath)
		log.Info("MHEARD restored after planned restart")
	} else if !os.IsNotExist(err) {
		log.Warn("MHEARD snapshot could not be restored", "error", err)
	}
	var web *webui.Server
	var aiServer *aiservice.Service
	var gameHall *gamehall.Hall
	var bbsStore *bbs.Store
	uprdMgr := uprd.New(ctx, ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}, cfg.Application.Locator, heard, uprd.Config{Enabled: uprdEnabled, Interval: time.Duration(cfg.UPRD.IntervalSeconds) * time.Second, MHeardLimit: cfg.UPRD.MHeardLimit, OperatorPresent: func() bool {
		return web != nil && web.HasActiveBrowser()
	}}, portIDs)
	var historyStore *history.Store
	if cfg.History.Enabled {
		historyStore, err = history.Open(cfg.History.Database, history.Limits{MaxStations: cfg.History.MaxStations, MaxSessions: cfg.History.MaxSessionsPerStation, MaxLines: cfg.History.MaxLinesPerStation, MaxBytes: cfg.History.MaxBytes, RetentionDays: cfg.History.RetentionDays})
		if err != nil {
			log.Error("history database failed", "error", err)
			os.Exit(2)
		}
	}
	beaconDestination := cfg.Beacon.Destination
	if strings.TrimSpace(beaconDestination) == "" {
		beaconDestination = "BEACON"
	}
	sendBeaconFrame := func(sendCtx context.Context, source ax25.Address, via []ax25.Address, text string) error {
		dst, err := ax25.ParseAddress(beaconDestination)
		if err != nil {
			return err
		}
		pid := byte(0xF0)
		f := ax25.Frame{Destination: dst, Source: source, Digipeaters: via, Type: ax25.TypeUI, PID: &pid, Payload: []byte(text)}
		b, err := ax25.Encode(f)
		if err != nil {
			return err
		}
		sent := 0
		failed := make([]string, 0)
		for _, port := range ports {
			runtime := runtimes[port.ID()]
			if runtime == nil || !runtime.enabled || !port.Status().Connected {
				continue
			}
			send := senders[port.ID()]
			if send == nil {
				continue
			}
			if err := send(sendCtx, b); err != nil {
				failed = append(failed, port.ID()+": "+err.Error())
				continue
			}
			sent++
		}
		if sent == 0 {
			if len(failed) > 0 {
				return fmt.Errorf("beacon failed on active TNC ports: %s", strings.Join(failed, "; "))
			}
			return fmt.Errorf("no active TNC ports available for beacon")
		}
		if len(failed) > 0 {
			log.Warn("beacon failed on some active TNC ports", "errors", strings.Join(failed, "; "), "sent", sent)
		}
		return nil
	}
	sendBeacon := func(sendCtx context.Context) error {
		source := ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}
		return sendBeaconFrame(sendCtx, source, stationBeaconVia, expandBeaconText(cfg.Beacon.Text, source, cfg.Application.OperatorName, cfg.Application.Locator, cfg.Application.QTH))
	}
	sendBBSBeacon := func(sendCtx context.Context) error {
		source := ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID}
		return sendBeaconFrame(sendCtx, source, bbsBeaconVia, beaconText(source, cfg.Application.Locator, heard.List(), digiAliases...))
	}
	sendUPRD := func(sendCtx context.Context) error {
		frame, enabled, err := uprdMgr.BuildFrame("")
		if err != nil {
			return err
		}
		if !enabled || frame == nil {
			return fmt.Errorf("UPRD is disabled")
		}
		sent := 0
		failed := make([]string, 0)
		for _, portID := range portIDs {
			runtime := runtimes[portID]
			if runtime == nil || !runtime.enabled || !runtime.port.Status().Connected {
				continue
			}
			send := senders[portID]
			if send == nil {
				continue
			}
			if err := send(sendCtx, frame); err != nil {
				failed = append(failed, portID+": "+err.Error())
				continue
			}
			sent++
		}
		if sent == 0 {
			if len(failed) > 0 {
				return fmt.Errorf("UPRD failed on active TNC ports: %s", strings.Join(failed, "; "))
			}
			return fmt.Errorf("no active TNC ports available for UPRD")
		}
		if len(failed) > 0 {
			log.Warn("UPRD failed on some active TNC ports", "errors", strings.Join(failed, "; "), "sent", sent)
		}
		return nil
	}
	inbound := session.NewInboundMux(senders, log)
	inbound.SetRegistry(services)
	registerService := func(reg service.ServiceRegistration) {
		if err := services.Register(reg); err != nil {
			log.Error("service registration failed", "service", reg.Service.ID(), "error", err)
			os.Exit(2)
		}
	}
	inbound.Configure(time.Duration(cfg.Application.AX25T1Seconds)*time.Second, cfg.Application.AX25N2, cfg.Application.AX25N1)
	if nodeEnabled && cfg.Node.NetROMEnabled {
		var netromMu sync.Mutex
		netromCircuits := make(map[string]*netrom.Circuit)
		var nextNetROMIndex uint8 = 1
		localNode := ax25.Address{Callsign: cfg.Server.Callsign, SSID: cfg.Server.SSID}
		inbound.RegisterPacket(localNode, func(route session.InboundRoute, pid byte, data []byte, send func(context.Context, byte, []byte) error) {
			if pid != netrom.PID {
				return
			}
			frame, err := netrom.DecodeFrame(data)
			if err != nil {
				log.Warn("invalid NET/ROM frame", "remote", route.Remote, "error", err)
				return
			}
			key := fmt.Sprintf("%s/%d/%d", route.Remote, frame.Transport.CircuitIndex, frame.Transport.CircuitID)
			netromMu.Lock()
			circuit := netromCircuits[key]
			if circuit == nil && frame.Transport.Opcode == netrom.OpcodeConnectRequest {
				circuit, err = netrom.NewCircuit(nextNetROMIndex, nextNetROMIndex, 4)
				if err == nil {
					nextNetROMIndex++
					if nextNetROMIndex == 0 {
						nextNetROMIndex = 1
					}
					netromCircuits[key] = circuit
				}
			}
			netromMu.Unlock()
			if err != nil || circuit == nil {
				log.Warn("NET/ROM circuit allocation failed", "remote", route.Remote, "error", err)
				return
			}
			if frame.Transport.Opcode == netrom.OpcodeConnectRequest {
				ack, acceptErr := circuit.Accept(frame.Transport, 4)
				if acceptErr != nil {
					log.Warn("NET/ROM connect rejected", "remote", route.Remote, "error", acceptErr)
					return
				}
				frame.Transport = ack
			} else {
				event, handleErr := circuit.Handle(frame.Transport)
				if handleErr != nil {
					log.Warn("NET/ROM circuit frame rejected", "remote", route.Remote, "error", handleErr)
					return
				}
				for _, response := range event.Packets {
					responseFrame := netrom.Frame{Network: netrom.NetworkHeader{Origin: frame.Network.Destination, Destination: frame.Network.Origin, TTL: frame.Network.TTL}, Transport: response}
					responseData, encodeErr := responseFrame.Encode()
					if encodeErr == nil {
						_ = send(context.Background(), netrom.PID, responseData)
					}
				}
				if event.Closed {
					netromMu.Lock()
					delete(netromCircuits, key)
					netromMu.Unlock()
				}
				return
			}
			responseFrame := netrom.Frame{Network: netrom.NetworkHeader{Origin: frame.Network.Destination, Destination: frame.Network.Origin, TTL: frame.Network.TTL}, Transport: frame.Transport}
			responseData, err := responseFrame.Encode()
			if err == nil {
				_ = send(context.Background(), netrom.PID, responseData)
			}
		})
	}
	var webUPRD *uprd.Manager
	if uprdEnabled {
		webUPRD = uprdMgr
	}
	apiTokens := make([]publicapi.Token, 0, len(cfg.API.Tokens))
	for _, t := range cfg.API.Tokens {
		apiTokens = append(apiTokens, publicapi.Token{Name: t.Name, Hash: t.Hash, Scopes: t.Scopes})
	}
	var apiHandler http.Handler
	if cfg.API.Enabled {
		apiServer := publicapi.New(publicapi.Config{
			Callsign: ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}.String(), Mode: cfg.Application.Mode, Version: bbs.BuildVersion, Tokens: apiTokens,
			Ports: func() []transport.Status {
				result := make([]transport.Status, 0, len(ports))
				for _, p := range ports {
					st := p.Status()
					st.Enabled = runtimes[p.ID()].enabled
					result = append(result, st)
				}
				return result
			},
			MHeard: heard, Monitor: mon,
			Sessions: func() []publicapi.SessionDTO {
				snapshots := radio.Snapshot()
				out := make([]publicapi.SessionDTO, 0, len(snapshots))
				for _, x := range snapshots {
					connected := x.Created
					out = append(out, publicapi.SessionDTO{ID: x.ID, State: string(x.State), Direction: "outbound", ConnectedSince: &connected})
				}
				return out
			},
			Node: func() publicapi.NodeStatus {
				if !nodeEnabled || nodeRouter == nil {
					return publicapi.NodeStatus{Enabled: false}
				}
				return publicapi.NodeStatus{Enabled: true, Neighbors: len(nodeRouter.Neighbors()), Routes: len(nodeRouter.Routes()), Services: len(services.List())}
			},
			BBS: func() *bbs.Store { return bbsStore },
			Digipeater: func() publicapi.DigipeaterStatus {
				count := digipeated.Load()
				out := publicapi.DigipeaterStatus{Enabled: len(digiAliases) > 0, Repeated: count}
				if n := lastDigipeated.Load(); n > 0 {
					t := time.Unix(0, n).UTC()
					out.LastActivity = &t
				}
				return out
			}, Broker: events,
		}, log)
		apiHandler = apiServer.Handler()
		log.Info("public API enabled", "listener", cfg.Web.Listen, "prefix", "/api/v1")
	}
	terminalPorts := append([]string(nil), portIDs...)
	web = webui.New(webui.Config{
		Listen:             cfg.Web.Listen,
		Username:           cfg.Web.Username,
		PasswordHash:       cfg.Web.PasswordHash,
		AllowedAddresses:   cfg.Web.AllowedAddresses,
		NodeCallsign:       cfg.Server.Callsign,
		NodeSSID:           cfg.Server.SSID,
		BBSCallsign:        cfg.BBS.Callsign,
		BSSSID:             cfg.BBS.SSID,
		AICallsign:         cfg.AI.Callsign,
		AISSID:             cfg.AI.SSID,
		AIEnabled:          aiEnabled,
		GameHallCallsign:   cfg.GameHall.Callsign,
		GameHallSSID:       cfg.GameHall.SSID,
		GameHallEnabled:    gameHallEnabled,
		TerminalCallsign:   cfg.Terminal.Callsign,
		TerminalSSID:       cfg.Terminal.SSID,
		OperatorName:       cfg.Application.OperatorName,
		ApplicationLocator: cfg.Application.Locator,
		ApplicationQTH:     cfg.Application.QTH,
		TerminalWelcome:    cfg.Application.WelcomeMessage,
		TerminalAway:       cfg.Application.AwayMessage,
		TerminalGoodbye:    cfg.Application.GoodbyeMessage,
		TerminalInfo:       cfg.Application.InfoMessage,
		TerminalEOL:        cfg.Application.TerminalEOL,
		AX25T3:             time.Duration(cfg.Application.AX25T3Seconds) * time.Second,
		Ports:              terminalPorts,
		NodeEnabled:        nodeEnabled,
		PortStatus: func() []transport.Status {
			result := make([]transport.Status, 0, len(ports))
			for _, p := range ports {
				st := p.Status()
				st.Enabled = runtimes[p.ID()].enabled
				result = append(result, st)
			}
			return result
		},
		Radio:      radio,
		MHeard:     heard,
		History:    historyStore,
		Monitor:    mon,
		UPRD:       webUPRD,
		SendBeacon: sendBeacon,
		SendUPRD:   sendUPRD,
		PresenceChanged: func() {
			if !uprdEnabled {
				return
			}
			uprdMgr.ResetSchedule()
			go func() {
				if err := sendUPRD(context.Background()); err != nil {
					log.Warn("UPRD presence update failed", "error", err)
				}
			}()
		},
		BBSListen: func() string {
			if bbsEnabled {
				return cfg.BBS.Listen
			}
			return ""
		}(),
		ConfigPath: *path,
		ReconnectPort: func(id string) error {
			return restartPort(id, runtimes)
		},
		RequestRestart: func() {
			if err := heard.SaveSnapshot(mheardSnapshotPath); err != nil {
				log.Warn("MHEARD snapshot failed", "error", err)
			}
			restartRequested.Store(true)
			stop()
		},
		Version:   bbs.BuildVersion,
		PublicAPI: apiHandler,
	}, log)
	inbound.RegisterRouted(ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}, web.ServeOperatorAX25)
	go runBeaconSchedule(ctx, cfg.Beacon.Enabled, bbsEnabled, time.Duration(cfg.Beacon.IntervalMinutes)*time.Minute, sendBeacon, sendBBSBeacon, log)
	if uprdEnabled {
		runUPRDSchedule(ctx, time.Duration(cfg.UPRD.IntervalSeconds)*time.Second, uprdMgr, portIDs, func(portID string) bool {
			runtime := runtimes[portID]
			return runtime != nil && runtime.enabled && runtime.port.Status().Connected
		}, senders, log)
	}
	var bbsServer *bbs.Server
	if bbsEnabled {
		store, err := bbs.Open(cfg.BBS.Database)
		if err != nil {
			log.Error("BBS database failed", "error", err)
			os.Exit(2)
		}
		bbsStore = store
		bbsServer = &bbs.Server{Listen: cfg.BBS.Listen, Title: cfg.BBS.Title, Node: ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID}.String(), Address: cfg.BBS.Address, Language: cfg.BBS.Language, WelcomeMessage: cfg.BBS.WelcomeMessage, GoodbyeMessage: cfg.BBS.GoodbyeMessage, NewUserMessage: cfg.BBS.NewUserMessage, InfoMessage: cfg.BBS.InfoMessage, Prompt: cfg.BBS.Prompt, SysopCallsign: cfg.BBS.SysopCallsign, MaxSessions: cfg.BBS.MaxSessions, Store: store, Log: log}
		if cfg.BBS.Forwarding.Enabled {
			peers := make([]bbs.ForwardPeer, 0, len(cfg.BBS.Forwarding.Peers))
			for _, p := range cfg.BBS.Forwarding.Peers {
				send, receive := true, true
				if p.Send != nil {
					send = *p.Send
				}
				if p.Receive != nil {
					receive = *p.Receive
				}
				peers = append(peers, bbs.ForwardPeer{ID: p.ID, Callsign: p.Callsign, Transport: p.Transport, Host: p.Host, Port: p.Port, ViaNode: p.ViaNode, PrivateRoutes: p.PrivateRoutes, BulletinScopes: p.BulletinScopes, ToCalls: p.ToCalls, AtCalls: p.AtCalls, HierarchicalRoutes: p.HierarchicalRoutes, Enabled: p.Enabled, Send: send, Receive: receive, SendConfigured: p.Send != nil, ReceiveConfigured: p.Receive != nil})
			}
			planner := &bbs.QueuePlanner{Store: store, Peers: peers, Interval: time.Duration(cfg.BBS.Forwarding.IntervalMinutes) * time.Minute, MaxPerPeer: cfg.BBS.Forwarding.MaxMessages, Log: log}
			go planner.Run(ctx)
			bbsServer.ForwardPeers = peers
			bbsServer.MaxForwardMessages = cfg.BBS.Forwarding.MaxMessages
			forwarder := &bbs.Forwarder{Store: store, Peers: peers, Interval: time.Duration(cfg.BBS.Forwarding.IntervalMinutes) * time.Minute, ConnectTimeout: time.Duration(cfg.BBS.Forwarding.ConnectTimeoutSeconds) * time.Second, SessionTimeout: time.Duration(cfg.BBS.Forwarding.SessionTimeoutSeconds) * time.Second, MaxMessages: cfg.BBS.Forwarding.MaxMessages, LocalCall: bbsServer.Node, LocalAddress: bbsServer.Address, Log: log}
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
		registerService(service.ServiceRegistration{Service: service.Func{ServiceID: "bbs", Handler: func(ctx service.ServiceContext) error {
			bbsServer.ServeAX25(ctx.RemoteCall.String(), ctx.Reader, ctx.Writer)
			return nil
		}}, Callsign: ax25.Address{Callsign: cfg.BBS.Callsign, SSID: cfg.BBS.SSID}, Aliases: []string{"BBS"}, Enabled: true, NodeVisible: true})
	}
	if aiEnabled {
		provider := &aiservice.Ollama{URL: cfg.AI.URL, Model: cfg.AI.Model, Client: &http.Client{}}
		aiServer = aiservice.New(provider, aiservice.Config{Timeout: time.Duration(cfg.AI.TimeoutSeconds) * time.Second, MaxContext: cfg.AI.MaxContext, MaxResponseChars: cfg.AI.MaxResponseChars, SystemPrompt: cfg.AI.SystemPrompt, QueueSize: cfg.AI.QueueSize, WelcomeMessage: cfg.AI.WelcomeMessage, ProcessingMessage: cfg.AI.ProcessingMessage, GoodbyeMessage: cfg.AI.GoodbyeMessage, LocalCall: ax25.Address{Callsign: cfg.AI.Callsign, SSID: cfg.AI.SSID}.String()}, cfg.AI.Concurrency)
		registerService(service.ServiceRegistration{Service: service.Func{ServiceID: "ai", Handler: func(ctx service.ServiceContext) error {
			scanner := lineinput.NewScanner(ctx.Reader)
			aiServer.ServeSession(ctx.RemoteCall.String(), cfg.Application.Language, scanner, ctx.Writer)
			return nil
		}}, Callsign: ax25.Address{Callsign: cfg.AI.Callsign, SSID: cfg.AI.SSID}, Aliases: []string{"AI"}, Enabled: true, NodeVisible: true})
		log.Info("AI service enabled", "provider", cfg.AI.Provider, "model", cfg.AI.Model, "callsign", ax25.Address{Callsign: cfg.AI.Callsign, SSID: cfg.AI.SSID}.String())
	}
	if gameHallEnabled {
		gameHall = gamehall.New(time.Duration(cfg.GameHall.InviteTimeoutSeconds) * time.Second)
		registerService(service.ServiceRegistration{Service: service.Func{ServiceID: "gamehall", Handler: func(ctx service.ServiceContext) error {
			gameHall.ServeAX25(ctx.RemoteCall.String(), cfg.GameHall.Language, ctx.Reader, ctx.Writer)
			return nil
		}}, Callsign: ax25.Address{Callsign: cfg.GameHall.Callsign, SSID: cfg.GameHall.SSID}, Aliases: []string{"GAME", "GAMES", "GRY"}, Enabled: true, NodeVisible: true})
		log.Info("game hall service enabled", "callsign", ax25.Address{Callsign: cfg.GameHall.Callsign, SSID: cfg.GameHall.SSID}.String())
	}
	if nodeEnabled {
		nodeServer := &nodecore.Server{Listen: cfg.Node.Listen, Callsign: ax25.Address{Callsign: cfg.Server.Callsign, SSID: cfg.Server.SSID}.String(), Alias: cfg.Node.Alias, Language: cfg.Node.Language, Version: bbs.BuildVersion, Registry: services, Router: nodeRouter, Ports: portIDs, Log: log}
		nodeServer.LanguageLookup = func(call string) string {
			if bbsStore == nil {
				return ""
			}
			return bbsStore.Language(call)
		}
		if bbsStore != nil {
			nodeServer.SetLanguage = bbsStore.SetLanguage
		}
		nodeServer.Connect = func(target string, neighbor nodecore.Neighbor, route nodecore.Route, local io.Reader, terminal io.Writer) error {
			remoteCall, err := ax25.ParseAddress(neighbor.Callsign)
			if err != nil {
				return fmt.Errorf("invalid neighbor callsign: %w", err)
			}
			remote, release := radio.NewSession()
			defer release()
			bridgeCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			events, unsubscribe := remote.Subscribe()
			defer unsubscribe()
			if err := remote.Connect(bridgeCtx, neighbor.Port, remoteCall.String()); err != nil {
				return err
			}
			// A direct neighbor is already the requested node. For a routed
			// destination, ask that node to continue the path through its NODE UI.
			if !strings.EqualFold(target, neighbor.Callsign) {
				if err := remote.Send(bridgeCtx, []byte("C "+strings.TrimSpace(target)+"\r")); err != nil {
					return err
				}
			}
			go func() {
				buf := make([]byte, 512)
				for {
					n, readErr := local.Read(buf)
					if n > 0 {
						if sendErr := remote.Send(bridgeCtx, buf[:n]); sendErr != nil {
							cancel()
							return
						}
					}
					if readErr != nil {
						cancel()
						return
					}
				}
			}()
			for {
				select {
				case event, ok := <-events:
					if !ok {
						return nil
					}
					if event.Type == "data" && len(event.Data) > 0 {
						if _, err := terminal.Write(event.Data); err != nil {
							return err
						}
					}
					if event.Type == "state" && event.State == session.Disconnected {
						return nil
					}
				case <-bridgeCtx.Done():
					disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
					_ = remote.Disconnect(disconnectCtx)
					disconnectCancel()
					return nil
				}
			}
		}
		registerService(service.ServiceRegistration{Service: service.Func{ServiceID: "node", Handler: func(ctx service.ServiceContext) error {
			nodeServer.ServeContext(ctx)
			return nil
		}}, Callsign: ax25.Address{Callsign: cfg.Server.Callsign, SSID: cfg.Server.SSID}, Aliases: []string{cfg.Node.Alias}, Enabled: true})
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
	dispatcher := ax25core.New()
	dispatcher.AddObserver(func(frame ax25core.FrameContext) {
		f := frame.Frame
		log.Info("frame rx", "port", frame.PortID, "source", f.Source.String(), "destination", f.Destination.String(), "type", f.Type, "bytes", len(f.Payload))
		mon.Add("RX", frame.PortID, f, len(frame.Raw))
		events.Publish("frame.rx", map[string]any{"port": frame.PortID, "source": f.Source.String(), "destination": f.Destination.String(), "bytes": len(frame.Raw)})
	})
	dispatcher.RegisterPre(func(frame ax25core.FrameContext) bool {
		f := frame.Frame
		seq := rxOrder.Add(1)
		if !frame.Internal && !isOwnCallsign(f.Source.String()) {
			if via := mheardReturnPath(f); via != "" {
				heard.HeardVia(f.Source.String(), frame.PortID, via)
			} else if directlyHeard(f) {
				heard.Heard(f.Source.String(), frame.PortID)
			}
			if uprdMgr != nil {
				uprdMgr.Submit(frame.PortID, seq, f)
			}
		}
		if !frame.Internal && !isOwnCallsign(f.Source.String()) && directlyHeard(f) && f.Type == ax25.TypeUI && strings.EqualFold(f.Destination.String(), "BEACON") && len(f.Payload) > 0 {
			heard.Beacon(f.Source.String(), frame.PortID, string(f.Payload))
			reported := parseUltimatePRBeacon(string(f.Payload))
			reported = withoutCallsigns(reported, append(digiAliases, ownCallsAsAddresses(ownCalls)...)...)
			heard.Reported(reported, f.Source.String(), frame.PortID)
		}
		if frame.Internal {
			return false
		}
		repeated, ok := digi.Repeat(f, frame.Raw)
		if !ok {
			return false
		}
		available := make([]string, 0, len(runtimes))
		for id, runtime := range runtimes {
			if runtime != nil && runtime.enabled && runtime.port.Status().Connected {
				available = append(available, id)
			}
		}
		outputPorts := digipeaterOutputPorts(frame.PortID, f.Destination.String(), heard, available)
		for _, outputPort := range outputPorts {
			if send := senders[outputPort]; send != nil {
				if err := send(ctx, repeated); err != nil {
					log.Warn("digipeater transmit failed", "input_port", frame.PortID, "output_port", outputPort, "error", err)
				} else {
					digipeated.Add(1)
					lastDigipeated.Store(time.Now().UnixNano())
					events.Publish("digipeater.activity", map[string]any{"input_port": frame.PortID, "output_port": outputPort, "source": f.Source.String(), "destination": f.Destination.String()})
					log.Info("frame digipeated", "input_port", frame.PortID, "output_port", outputPort, "source", f.Source.String(), "destination", f.Destination.String())
				}
			}
		}
		return true
	})
	dispatcher.RegisterUI(func(frame ax25core.FrameContext) bool {
		f := frame.Frame
		if nodeRouter != nil && f.PID != nil && *f.PID == netrom.PID && strings.EqualFold(f.Destination.String(), "NODES") {
			minQuality := cfg.Node.NetROMMinQuality
			if minQuality == 0 {
				minQuality = 1
			}
			obsolescence := cfg.Node.NetROMObsolescence
			if obsolescence == 0 {
				obsolescence = 6
			}
			if broadcast, err := netrom.DecodeRouting(f.Payload); err == nil {
				for _, destination := range broadcast.Destinations {
					if destination.Quality >= minQuality {
						nodeRouter.Learn(destination.Callsign.String(), f.Source.String(), destination.Quality, obsolescence, time.Now().UTC())
					}
				}
			}
		}
		return false
	})
	dispatcher.RegisterConnected(func(frame ax25core.FrameContext) bool {
		if radio.Handle(frame.PortID, frame.Frame) {
			return true
		}
		return inbound.Handle(frame.PortID, frame.Frame)
	})
	log.Info("server started", "callsign", cfg.Server.Callsign, "ssid", cfg.Server.SSID, "web", cfg.Web.Listen)
	for {
		select {
		case pkt := <-rx:
			if _, err := dispatcher.Dispatch(pkt); err != nil {
				log.Warn("invalid AX.25 frame", "port", pkt.PortID, "error", err)
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
func mheardReturnPath(frame ax25.Frame) string {
	path := make([]string, 0, len(frame.Digipeaters))
	for i := len(frame.Digipeaters) - 1; i >= 0; i-- {
		if frame.Digipeaters[i].Repeated {
			path = append(path, frame.Digipeaters[i].String())
		}
	}
	return strings.Join(path, ",")
}
func directlyHeard(frame ax25.Frame) bool {
	for _, via := range frame.Digipeaters {
		if via.Repeated {
			return false
		}
	}
	return true
}
func digipeaterOutputPorts(input, destination string, heard *mheard.Store, available []string) []string {
	usable := make(map[string]struct{}, len(available))
	for _, port := range available {
		usable[port] = struct{}{}
	}
	if output, found := heard.DirectPort(destination); found {
		if _, ok := usable[output]; ok {
			return []string{output}
		}
	}
	// The destination has not been heard directly yet (or its old port is
	// unavailable). Fan out once through the independent queues of all active
	// TNCs. The response teaches MHEARD the direct port in both directions, so
	// subsequent connected-mode traffic is routed to a single TNC.
	outputs := make([]string, 0, len(usable))
	for port := range usable {
		outputs = append(outputs, port)
	}
	sort.Strings(outputs)
	return outputs
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

func runBeaconSchedule(ctx context.Context, stationEnabled, bbsEnabled bool, interval time.Duration, sendStation, sendBBS func(context.Context) error, log *slog.Logger) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
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
					if err := sendStation(ctx); err != nil {
						log.Warn("station beacon failed", "error", err)
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func runUPRDSchedule(ctx context.Context, interval time.Duration, manager *uprd.Manager, ports []string, isPortActive func(string) bool, senders map[string]session.Sender, log *slog.Logger) {
	if manager == nil || interval <= 0 {
		return
	}
	for _, portID := range ports {
		portID := portID
		go func() {
			timer := time.NewTimer(interval)
			defer timer.Stop()
			for {
				select {
				case <-timer.C:
					if isPortActive != nil && !isPortActive(portID) {
						timer.Reset(interval)
						continue
					}
					send := senders[portID]
					if send == nil {
						timer.Reset(interval)
						continue
					}
					frame, ok, err := manager.BuildFrame(portID)
					if err != nil {
						log.Warn("UPRD frame build failed", "port", portID, "error", err)
						timer.Reset(interval)
						continue
					}
					if !ok {
						timer.Reset(interval)
						continue
					}
					if err := send(ctx, frame); err != nil {
						log.Warn("UPRD transmit failed", "port", portID, "error", err)
					}
					timer.Reset(interval)
				case <-manager.ScheduleResetChannel():
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(interval)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func beaconText(source ax25.Address, locator string, entries []mheard.Entry, excluded ...ax25.Address) string {
	lines := make([]string, 0, 4)
	if locator = strings.ToUpper(strings.TrimSpace(locator)); locator != "" {
		lines = append(lines, locator)
	}
	lines = append(lines, "DIGI", "UltimatePR")

	calls := make([]string, 0, 5)
	seen := make(map[string]struct{}, 5)
	for _, entry := range entries {
		call := strings.ToUpper(strings.TrimSpace(entry.Callsign))
		if call == "" {
			continue
		}
		own := call == source.String()
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

func expandBeaconText(template string, source ax25.Address, name, locator, qth string) string {
	expanded := strings.NewReplacer(
		"{CALL}", strings.ToUpper(strings.TrimSpace(source.Callsign)),
		"{NAME}", strings.TrimSpace(name),
		"{LOC}", strings.ToUpper(strings.TrimSpace(locator)),
		"{QTH}", strings.TrimSpace(qth),
	).Replace(template)
	expanded = strings.ReplaceAll(expanded, "\r\n", "\n")
	expanded = strings.ReplaceAll(expanded, "\r", "\n")
	lines := strings.Split(expanded, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\r")
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

func ownCallsAsAddresses(calls map[string]struct{}) []ax25.Address {
	out := make([]ax25.Address, 0, len(calls))
	for call := range calls {
		addr, err := ax25.ParseAddress(call)
		if err != nil {
			continue
		}
		out = append(out, addr)
	}
	return out
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
