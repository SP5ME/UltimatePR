package node

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

type Neighbor struct {
	ID, Callsign, Port string
	Quality            int
	Locked             bool
	LastHeard          time.Time
}
type Route struct {
	Destination, Via string
	Quality          int
	Learned          bool
	Updated          time.Time
}
type Service struct {
	Name, Callsign, Command string
	Enabled                 bool
}
type Router struct {
	mu        sync.RWMutex
	neighbors map[string]Neighbor
	routes    map[string][]Route
	services  map[string]Service
}

func New(neighbors []Neighbor, routes []Route, services []Service) *Router {
	r := &Router{neighbors: map[string]Neighbor{}, routes: map[string][]Route{}, services: map[string]Service{}}
	for _, n := range neighbors {
		n.Callsign = up(n.Callsign)
		r.neighbors[n.ID] = n
	}
	for _, x := range routes {
		x.Destination = up(x.Destination)
		r.routes[x.Destination] = append(r.routes[x.Destination], x)
	}
	for _, s := range services {
		r.services[up(s.Command)] = s
	}
	return r
}
func (r *Router) Resolve(destination string) (Neighbor, Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rs := append([]Route(nil), r.routes[up(destination)]...)
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].Quality > rs[j].Quality })
	for _, x := range rs {
		if n, ok := r.neighbors[x.Via]; ok {
			return n, x, nil
		}
	}
	for _, n := range r.neighbors {
		if n.Callsign == up(destination) {
			return n, Route{Destination: up(destination), Via: n.ID, Quality: n.Quality}, nil
		}
	}
	return Neighbor{}, Route{}, errors.New("no route to destination")
}
func (r *Router) Service(command string) (Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[up(command)]
	return s, ok && s.Enabled
}
func (r *Router) Neighbors() []Neighbor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Neighbor, 0, len(r.neighbors))
	for _, n := range r.neighbors {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Callsign < out[j].Callsign })
	return out
}
func (r *Router) Routes() []Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Route
	for _, rs := range r.routes {
		out = append(out, rs...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Destination < out[j].Destination })
	return out
}
func (r *Router) Services() []Service {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Service, 0, len(r.services))
	for _, s := range r.services {
		if s.Enabled {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Command < out[j].Command })
	return out
}
func up(v string) string { return strings.ToUpper(strings.TrimSpace(v)) }
