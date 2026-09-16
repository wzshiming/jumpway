package metrics

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wzshiming/bridge"
	bridgeconfig "github.com/wzshiming/bridge/config"
	"github.com/wzshiming/jumpway/config"
)

type Role int

const (
	Listen Role = iota
	Forward
)

const MaxTargets = 1000

type Registry struct {
	mu       sync.RWMutex
	nextID   atomic.Uint64
	since    time.Time
	lastTick time.Time
	rules    map[string]*Rule
	order    []string
	live     map[uint64]*live
}

type Rule struct {
	registry       *Registry
	name           string
	count          *counter
	ways           [2][]hop
	counters       map[hopKey]*counter
	targets        map[targetKey]*counter
	targetsEvicted int64
}

type hopKey struct {
	role  Role
	index int
	url   string
}

type hop struct {
	urls             []urlCounter
	peakUp, peakDown int64
}

// URLs that redact to the same string share one counter.
func (h *hop) uniqueCounters() []*counter {
	counters := make([]*counter, 0, len(h.urls))
	seen := make(map[*counter]bool, len(h.urls))
	for _, entry := range h.urls {
		if !seen[entry.count] {
			seen[entry.count] = true
			counters = append(counters, entry.count)
		}
	}
	return counters
}

type urlCounter struct {
	url   string
	count *counter
}

type targetKey struct {
	address string
	via     string
}

type live struct {
	id      uint64
	rule    *Rule
	client  string
	target  string
	via     string
	path    []PathHop
	started time.Time
	count   *counter
	conn    *conn
}

func NewRegistry() *Registry {
	now := time.Now()
	return &Registry{since: now, lastTick: now, rules: make(map[string]*Rule), live: make(map[uint64]*live)}
}

func (r *Registry) Sync(rules []config.Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	updated := make(map[string]*Rule, len(rules))
	order := make([]string, 0, len(rules))
	for _, rule := range rules {
		entry := r.rules[rule.Name]
		if entry == nil {
			entry = &Rule{registry: r, name: rule.Name, count: newCounter(r.lastTick), targets: make(map[targetKey]*counter)}
		}
		counters := make(map[hopKey]*counter)
		entry.ways[Listen] = entry.syncWay(Listen, rule.Listen.Way, counters)
		entry.ways[Forward] = entry.syncWay(Forward, rule.Forward.Way, counters)
		entry.counters = counters
		updated[rule.Name] = entry
		order = append(order, rule.Name)
	}
	r.rules = updated
	r.order = order
	for id, entry := range r.live {
		if updated[entry.rule.name] != entry.rule {
			delete(r.live, id)
		}
	}
}

func (r *Rule) syncWay(role Role, nodes []bridgeconfig.Node, counters map[hopKey]*counter) []hop {
	way := make([]hop, len(nodes))
	for index, node := range nodes {
		if index < len(r.ways[role]) {
			way[index].peakUp = r.ways[role][index].peakUp
			way[index].peakDown = r.ways[role][index].peakDown
		}
		way[index].urls = make([]urlCounter, 0, len(node.LB))
		for _, raw := range node.LB {
			key := hopKey{role: role, index: index, url: redact(raw)}
			count := counters[key]
			if count == nil {
				count = r.counters[key]
				if count == nil {
					count = newCounter(r.registry.lastTick)
				}
				counters[key] = count
			}
			way[index].urls = append(way[index].urls, urlCounter{url: key.url, count: count})
		}
	}
	return way
}

func (r *Registry) Rule(name string) *Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rules[name]
}

func (r *Registry) Disconnect(id uint64) error {
	r.mu.RLock()
	entry := r.live[id]
	r.mu.RUnlock()
	if entry == nil {
		return fmt.Errorf("connection %d not found", id)
	}
	return entry.conn.Close()
}

func (r *Rule) WrapListener(inner net.Listener) net.Listener {
	return &listener{Listener: inner, count: r.count}
}

func (r *Rule) HopWrapper(role Role) func(index int, url string, d bridge.Dialer) bridge.Dialer {
	return func(index int, raw string, inner bridge.Dialer) bridge.Dialer {
		key := hopKey{role: role, index: index, url: redact(raw)}
		r.registry.mu.Lock()
		defer r.registry.mu.Unlock()
		count := r.counters[key]
		if count == nil {
			count = newCounter(r.registry.lastTick)
			r.counters[key] = count
			for len(r.ways[role]) <= index {
				r.ways[role] = append(r.ways[role], hop{})
			}
			r.ways[role][index].urls = append(r.ways[role][index].urls, urlCounter{url: key.url, count: count})
		}
		return &dialer{inner: inner, count: count, url: key.url, index: index, role: role}
	}
}

func (r *Rule) target(key targetKey) *counter {
	if count := r.targets[key]; count != nil {
		return count
	}
	if len(r.targets) == MaxTargets {
		var oldest targetKey
		var oldestTime int64
		var oldestIdle bool
		found := false
		for key, count := range r.targets {
			idle := count.active.Load() == 0
			lastActive := count.lastActive.Load()
			if !found || idle && !oldestIdle || idle == oldestIdle && lastActive < oldestTime {
				oldest, oldestTime, oldestIdle = key, lastActive, idle
				found = true
			}
		}
		delete(r.targets, oldest)
		r.targetsEvicted++
	}
	count := newCounter(r.registry.lastTick)
	r.targets[key] = count
	return count
}

func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.since = now
	r.eachCounter(func(count *counter) { count.reset(r.lastTick) })
	kept := make(map[*counter]bool, len(r.live))
	for _, entry := range r.live {
		entry.count.reset(r.lastTick)
		entry.started = now
		kept[entry.rule.targets[targetKey{address: entry.target, via: entry.via}]] = true
	}
	for _, rule := range r.rules {
		for key, count := range rule.targets {
			if !kept[count] {
				delete(rule.targets, key)
			}
		}
		rule.targetsEvicted = 0
		for _, way := range rule.ways {
			for index := range way {
				way[index].peakUp, way[index].peakDown = 0, 0
			}
		}
	}
}

func (r *Registry) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			r.tick(now)
		}
	}
}

func (r *Registry) tick(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastTick = now
	r.eachCounter(func(count *counter) { count.tick(now) })
	for _, entry := range r.live {
		entry.count.tick(now)
	}
	for _, rule := range r.rules {
		for _, way := range rule.ways {
			for index := range way {
				hop := &way[index]
				var up, down int64
				for _, count := range hop.uniqueCounters() {
					up += count.rateUp.Load()
					down += count.rateDown.Load()
				}
				hop.peakUp = max(hop.peakUp, up)
				hop.peakDown = max(hop.peakDown, down)
			}
		}
	}
}

func (r *Registry) eachCounter(visit func(*counter)) {
	for _, rule := range r.rules {
		visit(rule.count)
		for _, count := range rule.counters {
			visit(count)
		}
		for _, count := range rule.targets {
			visit(count)
		}
	}
}

func redact(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<invalid url>"
	}
	return parsed.Redacted()
}

// endpoint keeps scheme and host:port only, so targets group by the exit server they went through.
func endpoint(redacted string) string {
	parsed, err := url.Parse(redacted)
	if err != nil || parsed.Host == "" {
		return redacted
	}
	return parsed.Scheme + "://" + parsed.Host
}
