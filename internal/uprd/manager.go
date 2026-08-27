package uprd

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/packet-radio/ultimatepr/internal/ax25"
	"github.com/packet-radio/ultimatepr/internal/mheard"
)

type Config struct {
	Enabled     bool
	Interval    time.Duration
	MHeardLimit int
}

type Edge struct {
	From        string    `json:"from"`
	To          string    `json:"to"`
	InterfaceID string    `json:"interface_id"`
	SourceType  string    `json:"source_type"`
	LastSeen    time.Time `json:"last_seen"`
	ReportOrder int       `json:"report_order"`
}

type Node struct {
	Callsign      string    `json:"callsign"`
	Locator       string    `json:"locator,omitempty"`
	LastSeen      time.Time `json:"last_seen"`
	EffectiveSeen time.Time `json:"effective_seen"`
	Interfaces    []string  `json:"interfaces,omitempty"`
	Sources       []string  `json:"sources,omitempty"`
}

type Route struct {
	Destination   string    `json:"destination"`
	Path          []string  `json:"path"`
	TNC           string    `json:"tnc"`
	Via           []string  `json:"via,omitempty"`
	EffectiveSeen time.Time `json:"effective_seen"`
	SourceType    string    `json:"source_type"`
}

type Snapshot struct {
	Root      string         `json:"root"`
	Locator   string         `json:"locator,omitempty"`
	Generated time.Time      `json:"generated"`
	MHeard    []mheard.Entry `json:"mheard"`
	Nodes     []Node         `json:"nodes"`
	Edges     []Edge         `json:"edges"`
	Routes    []Route        `json:"routes"`
}

type reportState struct {
	Port     string
	Reporter string
	Locator  string
	Heard    []string
	LastSeen time.Time
	Order    uint64
}

type queuedFrame struct {
	order uint64
	port  string
	frame ax25.Frame
}

type Manager struct {
	cfg     Config
	ctx     context.Context
	local   ax25.Address
	locator string
	heard   *mheard.Store

	mu      sync.RWMutex
	reports map[string]reportState
	queues  map[string]chan queuedFrame
	active  map[string]struct{}
}

func New(ctx context.Context, local ax25.Address, locator string, heard *mheard.Store, cfg Config, ports []string) *Manager {
	m := &Manager{
		cfg:     cfg,
		ctx:     ctx,
		local:   local,
		locator: NormalizeLocator(locator),
		heard:   heard,
		reports: make(map[string]reportState),
		queues:  make(map[string]chan queuedFrame, len(ports)),
		active:  make(map[string]struct{}, len(ports)),
	}
	for _, port := range ports {
		m.startWorker(ctx, port)
	}
	return m
}

func (m *Manager) startWorker(ctx context.Context, port string) {
	port = strings.TrimSpace(port)
	if port == "" {
		return
	}
	m.mu.Lock()
	if _, exists := m.queues[port]; exists {
		m.mu.Unlock()
		return
	}
	q := make(chan queuedFrame, 64)
	m.queues[port] = q
	m.active[port] = struct{}{}
	m.mu.Unlock()
	go func() {
		for {
			select {
			case job := <-q:
				m.apply(job)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *Manager) Submit(port string, order uint64, frame ax25.Frame) {
	port = strings.TrimSpace(port)
	if port == "" {
		return
	}
	m.mu.RLock()
	q := m.queues[port]
	m.mu.RUnlock()
	if q == nil {
		return
	}
	select {
	case q <- queuedFrame{order: order, port: port, frame: frame}:
	case <-m.ctx.Done():
	default:
		// Discovery is advisory; never block the main RX loop when overloaded.
	}
}

func (m *Manager) apply(job queuedFrame) {
	parsed, ok := ParseFrame(job.frame)
	if !ok {
		return
	}
	reporter := BaseCall(job.frame.Source.String())
	if reporter == "" {
		return
	}
	key := job.port + "\x00" + reporter
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, exists := m.reports[key]; exists && prev.Order > job.order {
		return
	}
	m.reports[key] = reportState{
		Port:     job.port,
		Reporter: reporter,
		Locator:  parsed.Locator,
		Heard:    append([]string(nil), parsed.MHeard...),
		LastSeen: now,
		Order:    job.order,
	}
}

func ParseFrame(frame ax25.Frame) (Payload, bool) {
	if !strings.EqualFold(frame.Destination.String(), "UPR") {
		return Payload{}, false
	}
	if frame.Type != ax25.TypeUI || frame.PID == nil || *frame.PID != 0xF0 || len(frame.Digipeaters) != 0 {
		return Payload{}, false
	}
	parsed, err := ParsePayload(string(frame.Payload), frame.Source)
	if err != nil {
		return Payload{}, false
	}
	return parsed, true
}

func (m *Manager) BuildFrame(port string) ([]byte, bool, error) {
	if m == nil || !m.cfg.Enabled {
		return nil, false, nil
	}
	_ = port
	entries := m.heard.List()
	filtered := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.SourceType == "reported" {
			continue
		}
		call := BaseCall(entry.Callsign)
		if call == "" || call == BaseCall(m.local.String()) {
			continue
		}
		if _, ok := seen[call]; ok {
			continue
		}
		seen[call] = struct{}{}
		filtered = append(filtered, call)
		if len(filtered) == m.limit() {
			break
		}
	}
	payload, err := EncodePayload(m.local, m.locator, filtered, m.limit())
	if err != nil {
		return nil, false, err
	}
	pid := byte(0xF0)
	frame := ax25.Frame{
		Destination: ax25.Address{Callsign: "UPR"},
		Source:      m.local,
		Type:        ax25.TypeUI,
		PID:         &pid,
		Payload:     []byte(payload),
	}
	b, err := ax25.Encode(frame)
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (m *Manager) limit() int {
	limit := m.cfg.MHeardLimit
	if limit < 1 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	return limit
}

func (m *Manager) Snapshot(activePorts []string) Snapshot {
	portSet := make(map[string]struct{}, len(activePorts))
	for _, port := range activePorts {
		port = strings.TrimSpace(port)
		if port == "" {
			continue
		}
		portSet[port] = struct{}{}
	}
	m.mu.RLock()
	reports := make([]reportState, 0, len(m.reports))
	for _, r := range m.reports {
		if len(portSet) > 0 {
			if _, ok := portSet[r.Port]; !ok {
				continue
			}
		}
		reports = append(reports, r)
	}
	m.mu.RUnlock()

	nodes := make(map[string]*Node)
	edges := make([]Edge, 0, len(reports)*4)
	root := m.local.String()
	now := time.Now()
	rootNode := ensureNode(nodes, root)
	rootNode.Callsign = root
	rootNode.LastSeen = now
	rootNode.EffectiveSeen = now
	rootNode.Interfaces = uniqueAppend(rootNode.Interfaces, "LOCAL")

	direct := m.heard.ListByPortFilter(func(e mheard.Entry) bool {
		if len(portSet) > 0 {
			if _, ok := portSet[e.Port]; !ok {
				return false
			}
		}
		return true
	})
	directCalls := make(map[string]struct{})
	for _, entry := range direct {
		if entry.SourceType == "reported" {
			continue
		}
		call := strings.TrimSpace(entry.Callsign)
		if call == "" {
			continue
		}
		if !entry.Indirect {
			directCalls[BaseCall(call)] = struct{}{}
		}
		n := ensureNode(nodes, call)
		n.Callsign = call
		n.LastSeen = maxTime(n.LastSeen, entry.LastSeen)
		n.Interfaces = uniqueAppend(n.Interfaces, entry.Port)
		if entry.SourceType == "reported" {
			n.Sources = uniqueAppend(n.Sources, "direct")
		} else {
			n.Sources = uniqueAppend(n.Sources, entry.SourceType)
		}
		edges = append(edges, Edge{
			From:        root,
			To:          call,
			InterfaceID: entry.Port,
			SourceType:  sourceType(entry),
			LastSeen:    entry.LastSeen,
		})
	}
	for _, report := range reports {
		reporter := report.Reporter
		if reporter == "" {
			continue
		}
		reporterNode := ensureNode(nodes, reporter)
		reporterNode.Callsign = reporter
		reporterNode.Locator = report.Locator
		reporterNode.LastSeen = maxTime(reporterNode.LastSeen, report.LastSeen)
		reporterNode.Interfaces = uniqueAppend(reporterNode.Interfaces, report.Port)
		reporterNode.Sources = uniqueAppend(reporterNode.Sources, "uprd")

		for idx, heardCall := range report.Heard {
			heardCall = BaseCall(heardCall)
			if heardCall == "" {
				continue
			}
			if _, directlyHeard := directCalls[heardCall]; directlyHeard {
				continue
			}
			heardNode := ensureNode(nodes, heardCall)
			heardNode.Callsign = heardCall
			heardNode.LastSeen = maxTime(heardNode.LastSeen, report.LastSeen)
			heardNode.Interfaces = uniqueAppend(heardNode.Interfaces, report.Port)
			heardNode.Sources = uniqueAppend(heardNode.Sources, "uprd")
			edges = append(edges, Edge{
				From:        reporter,
				To:          heardCall,
				InterfaceID: report.Port,
				SourceType:  "uprd",
				LastSeen:    report.LastSeen,
				ReportOrder: idx,
			})
		}
	}

	routes := bestRoutes(root, now, nodes, edges)
	for _, route := range routes {
		if node := nodes[route.Destination]; node != nil {
			node.EffectiveSeen = route.EffectiveSeen
		}
	}

	outNodes := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		node.Interfaces = sortedUnique(node.Interfaces)
		node.Sources = sortedUnique(node.Sources)
		outNodes = append(outNodes, *node)
	}
	sort.Slice(outNodes, func(i, j int) bool { return outNodes[i].Callsign < outNodes[j].Callsign })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].InterfaceID != edges[j].InterfaceID {
			return edges[i].InterfaceID < edges[j].InterfaceID
		}
		if edges[i].ReportOrder != edges[j].ReportOrder {
			return edges[i].ReportOrder < edges[j].ReportOrder
		}
		return edges[i].To < edges[j].To
	})
	sort.Slice(routes, func(i, j int) bool { return routes[i].Destination < routes[j].Destination })

	return Snapshot{
		Root:      root,
		Locator:   m.locator,
		Generated: now,
		MHeard:    m.heardView(activePorts),
		Nodes:     outNodes,
		Edges:     edges,
		Routes:    routes,
	}
}

func (m *Manager) heardView(activePorts []string) []mheard.Entry {
	portSet := make(map[string]struct{}, len(activePorts))
	for _, port := range activePorts {
		port = strings.TrimSpace(port)
		if port == "" {
			continue
		}
		portSet[port] = struct{}{}
	}
	base := m.heard.ListByPortFilter(func(e mheard.Entry) bool {
		if len(portSet) > 0 {
			if _, ok := portSet[e.Port]; !ok {
				return false
			}
		}
		return true
	})
	seen := make(map[string]struct{}, len(base))
	out := make([]mheard.Entry, 0, len(base))
	for _, entry := range base {
		key := entry.Port + "\x00" + entry.Callsign
		seen[key] = struct{}{}
		out = append(out, entry)
	}
	for _, route := range m.bestRoutesForView(activePorts) {
		key := route.TNC + "\x00" + route.Destination
		if _, ok := seen[key]; ok {
			continue
		}
		out = append(out, mheard.Entry{
			Callsign:   route.Destination,
			Port:       route.TNC,
			LastSeen:   route.EffectiveSeen,
			Indirect:   true,
			Via:        strings.Join(route.Via, ","),
			SourceType: "uprd",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LastSeen.Equal(out[j].LastSeen) {
			if out[i].Port != out[j].Port {
				return out[i].Port < out[j].Port
			}
			return out[i].Callsign < out[j].Callsign
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}

func (m *Manager) bestRoutesForView(activePorts []string) []Route {
	return m.SnapshotRoutes(activePorts)
}

func (m *Manager) SnapshotRoutes(activePorts []string) []Route {
	portSet := make(map[string]struct{}, len(activePorts))
	for _, port := range activePorts {
		port = strings.TrimSpace(port)
		if port == "" {
			continue
		}
		portSet[port] = struct{}{}
	}
	m.mu.RLock()
	reports := make([]reportState, 0, len(m.reports))
	for _, r := range m.reports {
		if len(portSet) > 0 {
			if _, ok := portSet[r.Port]; !ok {
				continue
			}
		}
		reports = append(reports, r)
	}
	m.mu.RUnlock()
	nodes := make(map[string]*Node)
	edges := make([]Edge, 0, len(reports)*4)
	root := m.local.String()
	now := time.Now()
	rootNode := ensureNode(nodes, root)
	rootNode.LastSeen = now
	rootNode.EffectiveSeen = now
	for _, report := range reports {
		reporter := report.Reporter
		_ = ensureNode(nodes, reporter)
		for idx, heardCall := range report.Heard {
			heardCall = BaseCall(heardCall)
			if heardCall == "" {
				continue
			}
			_ = ensureNode(nodes, heardCall)
			edges = append(edges, Edge{
				From:        reporter,
				To:          heardCall,
				InterfaceID: report.Port,
				SourceType:  "uprd",
				LastSeen:    report.LastSeen,
				ReportOrder: idx,
			})
		}
	}
	for _, entry := range m.heard.ListByPortFilter(func(e mheard.Entry) bool {
		if len(portSet) > 0 {
			if _, ok := portSet[e.Port]; !ok {
				return false
			}
		}
		return true
	}) {
		if entry.SourceType == "reported" {
			continue
		}
		_ = ensureNode(nodes, entry.Callsign)
		edges = append(edges, Edge{
			From:        root,
			To:          entry.Callsign,
			InterfaceID: entry.Port,
			SourceType:  sourceType(entry),
			LastSeen:    entry.LastSeen,
		})
	}
	return bestRoutes(root, now, nodes, edges)
}

func bestRoutes(root string, now time.Time, nodes map[string]*Node, edges []Edge) []Route {
	adj := make(map[string][]Edge, len(nodes))
	for _, edge := range edges {
		adj[edge.From] = append(adj[edge.From], edge)
	}
	for key := range adj {
		sort.SliceStable(adj[key], func(i, j int) bool {
			if adj[key][i].LastSeen.Equal(adj[key][j].LastSeen) {
				if adj[key][i].To != adj[key][j].To {
					return adj[key][i].To < adj[key][j].To
				}
				return adj[key][i].ReportOrder < adj[key][j].ReportOrder
			}
			return adj[key][i].LastSeen.After(adj[key][j].LastSeen)
		})
	}

	type state struct {
		node      string
		score     time.Time
		path      []string
		firstPort string
		firstHop  string
	}
	best := map[string]state{
		root: {node: root, score: now, path: []string{root}},
	}
	queue := []state{best[root]}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, edge := range adj[cur.node] {
			child := edge.To
			childNode := nodes[child]
			if childNode == nil {
				childNode = &Node{Callsign: child}
			}
			score := minTime(cur.score, edge.LastSeen)
			score = minTime(score, childNode.LastSeen)
			nextPath := append(append([]string(nil), cur.path...), child)
			next := state{node: child, score: score, path: nextPath, firstPort: cur.firstPort, firstHop: cur.firstHop}
			if len(cur.path) == 1 {
				next.firstPort = edge.InterfaceID
				next.firstHop = child
			}
			if prev, ok := best[child]; !ok || len(next.path) < len(prev.path) || (len(next.path) == len(prev.path) && next.score.After(prev.score)) {
				best[child] = next
				queue = append(queue, next)
			}
		}
	}

	routes := make([]Route, 0, len(best))
	for node, st := range best {
		if node == root {
			continue
		}
		route := Route{
			Destination:   node,
			Path:          append([]string(nil), st.path...),
			TNC:           st.firstPort,
			EffectiveSeen: st.score,
		}
		if len(route.Path) > 2 {
			route.Via = append([]string(nil), route.Path[1:len(route.Path)-1]...)
		}
		if len(route.Path) > 1 {
			if e := firstEdge(route.Path[0], route.Path[1], edges); e != nil {
				route.SourceType = e.SourceType
			}
		}
		routes = append(routes, route)
	}
	return routes
}

func firstEdge(from, to string, edges []Edge) *Edge {
	for i := range edges {
		if edges[i].From == from && edges[i].To == to {
			return &edges[i]
		}
	}
	return nil
}

func ensureNode(nodes map[string]*Node, key string) *Node {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "?"
	}
	if n, ok := nodes[key]; ok {
		return n
	}
	n := &Node{Callsign: key}
	nodes[key] = n
	return n
}

func uniqueAppend(values []string, next string) []string {
	next = strings.TrimSpace(next)
	if next == "" {
		return values
	}
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	values = append([]string(nil), values...)
	sort.Strings(values)
	out := values[:0]
	var prev string
	for i, value := range values {
		if i == 0 || value != prev {
			out = append(out, value)
			prev = value
		}
	}
	return out
}

func maxTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.After(a) {
		return b
	}
	return a
}

func minTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	if b.Before(a) {
		return b
	}
	return a
}

func sourceType(e mheard.Entry) string {
	if e.SourceType != "" {
		return e.SourceType
	}
	if e.Indirect {
		return "reported"
	}
	return "direct"
}
