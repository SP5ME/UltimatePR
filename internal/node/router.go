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
	Obsolescence     uint8
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
	routeDestination := up(destination)
	rs := append([]Route(nil), r.routes[routeDestination]...)
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].Quality > rs[j].Quality })
	for _, x := range rs {
		if n, ok := r.neighbors[x.Via]; ok && (x.Obsolescence > 0 || !x.Learned) {
			return n, x, nil
		}
	}
	for _, n := range r.neighbors {
		if n.Callsign == routeDestination {
			return n, Route{Destination: up(destination), Via: n.ID, Quality: n.Quality}, nil
		}
	}
	return Neighbor{}, Route{}, errors.New("no route to destination")
}

// Learn records a route advertised by a directly connected NET/ROM neighbor.
// Quality is reduced by the local link quality and stale learned routes are
// replaced only by a better route from the same neighbor.
func (r *Router) Learn(destination, neighbor string, quality uint8, obsolescence uint8, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	var via string
	for id, n := range r.neighbors {
		if n.Callsign == up(neighbor) {
			via = id
			if quality > 0 && n.Quality > 0 {
				quality = uint8(int(quality) * n.Quality / 255)
			}
			break
		}
	}
	if via == "" || up(destination) == "" || quality == 0 {
		return false
	}
	destination = up(destination)
	for i := range r.routes[destination] {
		x := &r.routes[destination][i]
		if x.Via == via {
			x.Quality, x.Obsolescence, x.Updated = int(quality), obsolescence, now
			x.Learned = true
			return true
		}
	}
	r.routes[destination] = append(r.routes[destination], Route{Destination: destination, Via: via, Quality: int(quality), Learned: true, Updated: now, Obsolescence: obsolescence})
	return true
}

// AgeLearned decrements automatic-route obsolescence and removes expired
// routes. Locked/static routes are never touched.
func (r *Router) AgeLearned() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for destination, routes := range r.routes {
		kept := routes[:0]
		for _, route := range routes {
			if route.Learned {
				if route.Obsolescence <= 1 {
					continue
				}
				route.Obsolescence--
			}
			kept = append(kept, route)
		}
		if len(kept) == 0 {
			delete(r.routes, destination)
		} else {
			r.routes[destination] = kept
		}
	}
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

func (r *Router) Neighbor(id string) (Neighbor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.neighbors[id]
	return n, ok
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
