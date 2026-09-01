package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/packet-radio/ultimatepr/internal/ax25"
)

type EntryType string

const (
	EntryAX25  = EntryType("ax25")
	EntryLocal = EntryType("local")
	EntryTCP   = EntryType("tcp")
)

// ServiceContext is the transport-neutral context of one service invocation.
// It intentionally contains no session, KISS, TNC, or transport implementation.
type ServiceContext struct {
	Context     context.Context
	LocalCall   ax25.Address
	RemoteCall  ax25.Address
	PortID      string
	Digipeaters []ax25.Address
	Reader      io.Reader
	Writer      io.Writer
	EntryType   EntryType
	Metadata    map[string]string
	Cancel      context.CancelFunc
	Disconnect  func() error
}

type Service interface {
	ID() string
	Serve(ServiceContext) error
}

type ServiceRegistration struct {
	Service  Service
	Callsign ax25.Address
	Aliases  []string
	Enabled  bool
}

type Registry struct {
	mu       sync.RWMutex
	services map[string]ServiceRegistration
	calls    map[string]string
	aliases  map[string]string
}

var (
	ErrInvalidRegistration = errors.New("invalid service registration")
	ErrDuplicateServiceID  = errors.New("duplicate service id")
	ErrDuplicateCallsign   = errors.New("duplicate service callsign")
	ErrDuplicateAlias      = errors.New("duplicate service alias")
	ErrServiceUnavailable  = errors.New("service unavailable")
)

func NewRegistry() *Registry {
	return &Registry{services: map[string]ServiceRegistration{}, calls: map[string]string{}, aliases: map[string]string{}}
}

func (r *Registry) Register(reg ServiceRegistration) error {
	if r == nil || reg.Service == nil {
		return ErrInvalidRegistration
	}
	id := normalizeID(reg.Service.ID())
	if id == "" {
		return fmt.Errorf("%w: service id is empty", ErrInvalidRegistration)
	}
	if reg.Callsign.Callsign != "" {
		if err := reg.Callsign.Validate(); err != nil {
			return fmt.Errorf("%w: callsign: %v", ErrInvalidRegistration, err)
		}
	}
	aliases := make([]string, 0, len(reg.Aliases))
	seen := map[string]struct{}{}
	for _, alias := range reg.Aliases {
		alias = normalizeAlias(alias)
		if alias == "" {
			continue
		}
		if _, ok := seen[alias]; ok {
			return fmt.Errorf("%w: %q", ErrDuplicateAlias, alias)
		}
		seen[alias] = struct{}{}
		aliases = append(aliases, alias)
	}
	reg.Aliases = aliases

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.services[id]; ok {
		return fmt.Errorf("%w: %q", ErrDuplicateServiceID, id)
	}
	if reg.Enabled {
		call := normalizeCallsign(reg.Callsign.String())
		if call != "" {
			if owner := r.calls[call]; owner != "" {
				return fmt.Errorf("%w: %q already used by %q", ErrDuplicateCallsign, call, owner)
			}
		}
		for _, alias := range aliases {
			if owner := r.aliases[alias]; owner != "" {
				return fmt.Errorf("%w: %q already used by %q", ErrDuplicateAlias, alias, owner)
			}
		}
		if call != "" {
			r.calls[call] = id
		}
		for _, alias := range aliases {
			r.aliases[alias] = id
		}
	}
	r.services[id] = reg
	return nil
}

func (r *Registry) ByID(id string) (ServiceRegistration, bool) {
	return r.lookup(normalizeID(id), func(reg ServiceRegistration) bool { return reg.Enabled })
}

func (r *Registry) ByCallsign(callsign string) (ServiceRegistration, bool) {
	call := normalizeCallsign(callsign)
	if r == nil {
		return ServiceRegistration{}, false
	}
	r.mu.RLock()
	id := r.calls[call]
	r.mu.RUnlock()
	return r.lookup(id, func(reg ServiceRegistration) bool { return reg.Enabled })
}

func (r *Registry) ByAlias(alias string) (ServiceRegistration, bool) {
	if r == nil {
		return ServiceRegistration{}, false
	}
	r.mu.RLock()
	id := r.aliases[normalizeAlias(alias)]
	r.mu.RUnlock()
	return r.lookup(id, func(reg ServiceRegistration) bool { return reg.Enabled })
}

// Serve invokes an enabled service for a local, TCP, or other non-AX.25 entry.
// AX.25 sessions are invoked by session.InboundMux after endpoint routing.
func (r *Registry) Serve(key string, ctx ServiceContext) error {
	reg, ok := r.ByID(key)
	if !ok {
		reg, ok = r.ByAlias(key)
	}
	if !ok {
		return fmt.Errorf("%w: %q", ErrServiceUnavailable, key)
	}
	if ctx.Context == nil {
		ctx.Context = context.Background()
	}
	return reg.Service.Serve(ctx)
}

func (r *Registry) List() []ServiceRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ServiceRegistration, 0, len(r.services))
	for _, reg := range r.services {
		if reg.Enabled {
			out = append(out, cloneRegistration(reg))
		}
	}
	sort.Slice(out, func(i, j int) bool { return normalizeID(out[i].Service.ID()) < normalizeID(out[j].Service.ID()) })
	return out
}

func (r *Registry) lookup(id string, allowed func(ServiceRegistration) bool) (ServiceRegistration, bool) {
	if r == nil || id == "" {
		return ServiceRegistration{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.services[id]
	if !ok || !allowed(reg) {
		return ServiceRegistration{}, false
	}
	return cloneRegistration(reg), true
}

func cloneRegistration(reg ServiceRegistration) ServiceRegistration {
	reg.Aliases = append([]string(nil), reg.Aliases...)
	return reg
}

func normalizeID(value string) string       { return strings.ToLower(strings.TrimSpace(value)) }
func normalizeAlias(value string) string    { return strings.ToUpper(strings.TrimSpace(value)) }
func normalizeCallsign(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }

type Func struct {
	ServiceID string
	Handler   func(ServiceContext) error
}

func (f Func) ID() string { return f.ServiceID }
func (f Func) Serve(ctx ServiceContext) error {
	if f.Handler == nil {
		return ErrInvalidRegistration
	}
	return f.Handler(ctx)
}
