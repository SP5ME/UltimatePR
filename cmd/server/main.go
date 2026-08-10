package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/packet-radio/modernbbs/internal/ax25"
	"github.com/packet-radio/modernbbs/internal/bbs"
	"github.com/packet-radio/modernbbs/internal/config"
	"github.com/packet-radio/modernbbs/internal/session"
	"github.com/packet-radio/modernbbs/internal/transport"
	"github.com/packet-radio/modernbbs/internal/transport/kiss"
	webui "github.com/packet-radio/modernbbs/internal/web"
)

func main() {
	path := flag.String("config", "config.yaml", "configuration file")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(*path)
	if err != nil {
		log.Error("configuration failed", "error", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rx := make(chan transport.Packet, 256)
	portIDs := make([]string, 0, len(cfg.Ports))
	senders := make(map[string]session.Sender, len(cfg.Ports))
	for _, pc := range cfg.Ports {
		portIDs = append(portIDs, pc.ID)
		p := kiss.NewTCPPort(kiss.TCPConfig{ID: pc.ID, Address: net.JoinHostPort(pc.Host, fmtPort(pc.Port)), Channel: pc.Channel, MaxFrame: pc.MaxFrameBytes, Reconnect: time.Duration(pc.ReconnectSeconds) * time.Second, Queue: 256}, log)
		senders[pc.ID] = func(port *kiss.TCPPort, channel uint8) session.Sender {
			return func(ctx context.Context, b []byte) error {
				return port.Send(ctx, transport.Packet{PortID: port.ID(), Channel: channel, Data: b})
			}
		}(p, pc.Channel)
		go func() {
			if e := p.Run(ctx, rx); e != nil && ctx.Err() == nil {
				log.Error("port stopped", "port", p.ID(), "error", e)
			}
		}()
	}
	radio := session.New(ax25.Address{Callsign: cfg.Terminal.Callsign, SSID: cfg.Terminal.SSID}, senders)
	web := webui.New(webui.Config{
		Listen:           cfg.Web.Listen,
		NodeCallsign:     cfg.Server.Callsign,
		NodeSSID:         cfg.Server.SSID,
		TerminalCallsign: cfg.Terminal.Callsign,
		TerminalSSID:     cfg.Terminal.SSID,
		Ports:            portIDs,
		Radio:            radio,
		BBSListen: func() string {
			if cfg.BBS.Enabled {
				return cfg.BBS.Listen
			}
			return ""
		}(),
	}, log)
	if cfg.BBS.Enabled {
		store, err := bbs.Open(cfg.BBS.Database)
		if err != nil {
			log.Error("BBS database failed", "error", err)
			os.Exit(2)
		}
		bbsServer := &bbs.Server{Listen: cfg.BBS.Listen, Title: cfg.BBS.Title, Node: ax25.Address{Callsign: cfg.Server.Callsign, SSID: cfg.Server.SSID}.String(), Store: store, Log: log}
		web.SetBBS(store)
		go func() {
			if err := bbsServer.Run(ctx); err != nil && ctx.Err() == nil {
				log.Error("BBS server stopped", "error", err)
				stop()
			}
		}()
		log.Info("BBS service started", "listen", cfg.BBS.Listen, "database", cfg.BBS.Database)
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
			radio.Handle(pkt.PortID, f)
		case <-ctx.Done():
			log.Info("server stopped")
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
