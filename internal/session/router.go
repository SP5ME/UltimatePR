package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/node"
	"github.com/packet-radio/ultimatepr/internal/service"
)

// Router adapts the existing Hub/Manager pair to the service-level session
// contract. It does not encode frames or own a second transport stack.
type Router struct {
	Hub                *Hub
	Node               *node.Router
	Registry           *service.Registry
	NodeEnabled        bool
	UnavailableTargets []string
}

type RouteExplanation struct {
	Target      string   `json:"target"`
	Resolution  string   `json:"resolution"`
	ServiceID   string   `json:"service_id,omitempty"`
	State       string   `json:"state,omitempty"`
	Transport   string   `json:"transport,omitempty"`
	Port        string   `json:"port,omitempty"`
	Via         string   `json:"via,omitempty"`
	Destination string   `json:"destination,omitempty"`
	Error       string   `json:"error,omitempty"`
	Steps       []string `json:"steps,omitempty"`
}

// ExplainRoute applies the same local-service and NODE/AX.25 lookup used by
// DialSession without creating a connection or sending a frame.
func (r *Router) ExplainRoute(target string) RouteExplanation {
	result := RouteExplanation{Target: target}
	if r == nil || r.Hub == nil {
		result.Resolution, result.Error = "transport error", service.ErrTransport.Error()
		return result
	}
	for _, unavailable := range r.UnavailableTargets {
		if strings.EqualFold(strings.TrimSpace(unavailable), strings.TrimSpace(target)) {
			result.Resolution, result.Error = "local service", service.ErrServiceUnavailable.Error()
			return result
		}
	}
	for _, unavailable := range r.UnavailableTargets {
		if strings.EqualFold(strings.TrimSpace(unavailable), strings.TrimSpace(target)) {
			result.Resolution, result.Error = "local service", service.ErrServiceUnavailable.Error()
			return result
		}
	}
	for _, unavailable := range r.UnavailableTargets {
		if strings.EqualFold(strings.TrimSpace(unavailable), strings.TrimSpace(target)) {
			result.Resolution, result.Error = "local service", service.ErrServiceUnavailable.Error()
			return result
		}
	}
	if r.Registry != nil && r.Registry.Has(target) {
		if reg, ok := r.Registry.Resolve(target); ok {
			result.Resolution = "local service"
			result.ServiceID = reg.Service.ID()
			result.State = string(reg.State)
			return result
		}
		result.Resolution, result.Error = "local service", service.ErrServiceUnavailable.Error()
		return result
	}
	if !r.NodeEnabled {
		result.Resolution, result.Error = "service unavailable", service.ErrServiceUnavailable.Error()
		return result
	}
	if r.Node == nil {
		result.Resolution, result.Error = "route not found", service.ErrRouteNotFound.Error()
		return result
	}
	neighbor, route, err := r.resolve(target, "")
	if err != nil {
		result.Resolution, result.Error = "route not found", service.ErrRouteNotFound.Error()
		return result
	}
	result.Resolution = "NODE / AX.25 route"
	result.Port = neighbor.Port
	result.Via = route.Via
	result.Destination = route.Destination
	return result
}

func (r *Router) DialSession(ctx context.Context, req service.SessionRequest) (service.Session, error) {
	if r == nil || r.Hub == nil {
		return nil, service.ErrTransport
	}
	for _, unavailable := range r.UnavailableTargets {
		if strings.EqualFold(strings.TrimSpace(unavailable), strings.TrimSpace(req.Target)) {
			return nil, service.ErrServiceUnavailable
		}
	}
	if !r.NodeEnabled {
		if !req.FallbackToRF || strings.TrimSpace(req.RFPort) == "" {
			return nil, service.ErrServiceUnavailable
		}
		return &routerSession{hub: r.Hub, target: req.Target, neighbor: node.Neighbor{Callsign: req.Target, Port: req.RFPort}, route: node.Route{Destination: strings.ToUpper(strings.TrimSpace(req.Target)), Via: "explicit-rf", Quality: 0}, status: service.SessionIdle}, nil
	}
	if r.Registry != nil && r.Registry.Has(req.Target) {
		if _, ok := r.Registry.Resolve(req.Target); !ok {
			return nil, service.ErrServiceUnavailable
		}
	}
	if strings.TrimSpace(req.Transport) != "" && !strings.EqualFold(req.Transport, "node") && !strings.EqualFold(req.Transport, "ax25") {
		return nil, fmt.Errorf("%w: unsupported session transport %q", service.ErrTransport, req.Transport)
	}
	if r.Node == nil {
		return nil, service.ErrRouteNotFound
	}
	neighbor, route, err := r.resolve(req.Target, req.ViaNode)
	if err != nil {
		if req.FallbackToRF && strings.TrimSpace(req.RFPort) != "" {
			return &routerSession{hub: r.Hub, target: req.Target, neighbor: node.Neighbor{Callsign: req.Target, Port: req.RFPort}, route: node.Route{Destination: strings.ToUpper(strings.TrimSpace(req.Target)), Via: "explicit-rf", Quality: 0}, status: service.SessionIdle}, nil
		}
		return nil, fmt.Errorf("%w: %v", service.ErrRouteNotFound, err)
	}
	return &routerSession{hub: r.Hub, target: req.Target, neighbor: neighbor, route: route, status: service.SessionIdle}, nil
}

func (r *Router) resolve(target, via string) (node.Neighbor, node.Route, error) {
	if strings.TrimSpace(via) == "" {
		return r.Node.Resolve(target)
	}
	for _, n := range r.Node.Neighbors() {
		if strings.EqualFold(n.ID, via) || strings.EqualFold(n.Callsign, via) {
			return n, node.Route{Destination: strings.ToUpper(strings.TrimSpace(target)), Via: n.ID, Quality: n.Quality}, nil
		}
	}
	return node.Neighbor{}, node.Route{}, errors.New("configured via_node is not a known neighbor")
}

type routerSession struct {
	mu        sync.Mutex
	hub       *Hub
	manager   *Manager
	release   func()
	events    <-chan Event
	unsub     func()
	read      chan []byte
	done      chan struct{}
	closeOnce sync.Once
	doneOnce  sync.Once
	status    service.SessionStatus
	target    string
	neighbor  node.Neighbor
	route     node.Route
}

func (s *routerSession) Connect(ctx context.Context, target string) error {
	s.mu.Lock()
	if s.status != "" && s.status != service.SessionIdle {
		s.mu.Unlock()
		return service.ErrSessionClosed
	}
	s.status = service.SessionConnecting
	s.mu.Unlock()
	if strings.TrimSpace(target) != "" {
		s.target = target
	}
	s.mu.Lock()
	s.read = make(chan []byte, 32)
	s.done = make(chan struct{})
	s.mu.Unlock()
	m, release := s.hub.NewSession()
	s.mu.Lock()
	s.manager, s.release = m, release
	s.mu.Unlock()
	if err := m.Connect(ctx, s.neighbor.Port, s.neighbor.Callsign); err != nil {
		s.Close()
		return classifyConnectError(ctx, err)
	}
	events, unsubscribe := m.Subscribe()
	s.mu.Lock()
	s.events, s.unsub = events, unsubscribe
	s.mu.Unlock()
	go s.forwardEvents()
	if !strings.EqualFold(strings.TrimSpace(s.target), strings.TrimSpace(s.neighbor.Callsign)) {
		if err := m.Send(ctx, []byte("C "+strings.TrimSpace(s.target)+"\r")); err != nil {
			s.Close()
			return fmt.Errorf("%w: %v", service.ErrTransport, err)
		}
	}
	s.mu.Lock()
	s.status = service.SessionConnected
	s.mu.Unlock()
	return nil
}

func classifyConnectError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %v", service.ErrConnectTimeout, ctx.Err())
	}
	if strings.Contains(strings.ToLower(err.Error()), "rejected") {
		return fmt.Errorf("%w: %v", service.ErrConnectionRejected, err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return fmt.Errorf("%w: %v", service.ErrConnectTimeout, err)
	}
	return fmt.Errorf("%w: %v", service.ErrTransport, err)
}

func (s *routerSession) forwardEvents() {
	for event := range s.events {
		if event.Type == "data" {
			select {
			case s.read <- append([]byte(nil), event.Data...):
			case <-s.done:
				return
			}
		}
		if event.State == Disconnected {
			s.mu.Lock()
			if s.status != service.SessionClosing {
				s.status = service.SessionClosed
			}
			s.mu.Unlock()
			s.doneOnce.Do(func() { close(s.done) })
			return
		}
	}
}

func (s *routerSession) Read(p []byte) (int, error) {
	select {
	case data := <-s.read:
		if len(data) == 0 {
			return 0, nil
		}
		n := copy(p, data)
		if n < len(data) {
			select {
			case s.read <- data[n:]:
			default:
			}
		}
		return n, nil
	case <-s.done:
		return 0, io.EOF
	}
}

func (s *routerSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	m := s.manager
	status := s.status
	s.mu.Unlock()
	if m == nil || status != service.SessionConnected {
		return 0, service.ErrSessionClosed
	}
	if err := m.Send(context.Background(), p); err != nil {
		if m.State() == Disconnected {
			return 0, service.ErrSessionClosed
		}
		return 0, fmt.Errorf("%w: %v", service.ErrTransport, err)
	}
	return len(p), nil
}

func (s *routerSession) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.status = service.SessionClosing
		m, release, unsub := s.manager, s.release, s.unsub
		s.mu.Unlock()
		s.doneOnce.Do(func() { close(s.done) })
		if unsub != nil {
			unsub()
		}
		if m != nil {
			closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = m.Disconnect(closeCtx)
			cancel()
		}
		if release != nil {
			release()
		}
		s.mu.Lock()
		s.status = service.SessionClosed
		s.mu.Unlock()
	})
	return nil
}

func (s *routerSession) Status() service.SessionStatus {
	s.mu.Lock()
	status, manager := s.status, s.manager
	s.mu.Unlock()
	if manager == nil {
		return status
	}
	switch manager.State() {
	case AwaitingConnection:
		return service.SessionConnecting
	case Connected, TimerRecovery:
		return service.SessionConnected
	case AwaitingRelease:
		return service.SessionClosing
	case Disconnected:
		return service.SessionClosed
	default:
		return status
	}
}

var _ service.SessionDialer = (*Router)(nil)
