package metrics

import (
	"sort"
	"time"
)

type Snapshot struct {
	Since string      `json:"since"`
	Rules []RuleStats `json:"rules"`
}

type Stats struct {
	Up           int64   `json:"up"`
	Down         int64   `json:"down"`
	RateUp       int64   `json:"rate_up"`
	RateDown     int64   `json:"rate_down"`
	PeakRateUp   int64   `json:"peak_rate_up"`
	PeakRateDown int64   `json:"peak_rate_down"`
	Active       int64   `json:"active"`
	Total        int64   `json:"total"`
	Dials        int64   `json:"dials"`
	DialFailures int64   `json:"dial_failures"`
	LatencyMs    float64 `json:"latency_ms"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	LastActive   string  `json:"last_active,omitempty"`
	LastUp       string  `json:"last_up,omitempty"`
	LastDown     string  `json:"last_down,omitempty"`
}

type RuleStats struct {
	Name           string       `json:"name"`
	Stats          Stats        `json:"stats"`
	Listen         []Hop        `json:"listen"`
	Forward        []Hop        `json:"forward"`
	Targets        []Target     `json:"targets"`
	Connections    []Connection `json:"connections"`
	TargetsEvicted int64        `json:"targets_evicted"`
}

type Hop struct {
	Index       int        `json:"index"`
	ParentIndex int        `json:"parent_index"`
	Stats       Stats      `json:"stats"`
	URLs        []URLStats `json:"urls"`
}

type URLStats struct {
	URL   string `json:"url"`
	Stats Stats  `json:"stats"`
}

type Target struct {
	Address string `json:"address"`
	Via     string `json:"via"`
	Stats   Stats  `json:"stats"`
}

type Connection struct {
	ID      uint64    `json:"id"`
	Client  string    `json:"client,omitempty"`
	Target  string    `json:"target"`
	Via     string    `json:"via"`
	Path    []PathHop `json:"path"`
	Started string    `json:"started"`
	Stats   Stats     `json:"stats"`
}

// PathHop is one hop of a connection's actual route; Dialed is false when a cached transport was reused and URL was inferred (or unknown).
type PathHop struct {
	Index  int    `json:"index"`
	URL    string `json:"url"`
	Dialed bool   `json:"dialed"`
}

type counterSnapshot struct {
	stats      Stats
	lastDial   int64
	lastActive int64
	lastUp     int64
	lastDown   int64
	latencySum int64
}

func (r *Registry) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := Snapshot{Since: r.since.Format(time.RFC3339), Rules: make([]RuleStats, 0, len(r.order))}
	connections := make(map[*Rule][]Connection)
	for _, entry := range r.live {
		connections[entry.rule] = append(connections[entry.rule], Connection{
			ID:      entry.id,
			Client:  entry.client,
			Target:  entry.target,
			Via:     entry.via,
			Path:    append([]PathHop{}, entry.path...),
			Started: entry.started.UTC().Format(time.RFC3339Nano),
			Stats:   entry.count.snapshot().stats,
		})
	}
	for _, name := range r.order {
		rule := r.rules[name]
		current := connections[rule]
		if current == nil {
			current = []Connection{}
		}
		sort.Slice(current, func(left, right int) bool { return current[left].ID < current[right].ID })
		snapshot.Rules = append(snapshot.Rules, RuleStats{
			Name:           name,
			Stats:          rule.count.snapshot().stats,
			Listen:         snapshotWay(rule.ways[Listen]),
			Forward:        snapshotWay(rule.ways[Forward]),
			Targets:        snapshotTargets(rule.targets),
			Connections:    current,
			TargetsEvicted: rule.targetsEvicted,
		})
	}
	return snapshot
}

func snapshotWay(way []hop) []Hop {
	hops := make([]Hop, len(way))
	for index, entry := range way {
		hop := Hop{Index: index, ParentIndex: index + 1, URLs: make([]URLStats, 0, len(entry.urls))}
		if index == len(way)-1 {
			hop.ParentIndex = -1
		}
		for _, url := range entry.urls {
			hop.URLs = append(hop.URLs, URLStats{URL: url.url, Stats: url.count.snapshot().stats})
		}
		var aggregate counterSnapshot
		for _, count := range entry.uniqueCounters() {
			aggregate.add(count.snapshot())
		}
		hop.Stats = aggregate.stats
		hop.Stats.PeakRateUp = entry.peakUp
		hop.Stats.PeakRateDown = entry.peakDown
		hops[index] = hop
	}
	return hops
}

func snapshotTargets(targets map[targetKey]*counter) []Target {
	result := make([]Target, 0, len(targets))
	for key, count := range targets {
		result = append(result, Target{Address: key.address, Via: key.via, Stats: count.snapshot().stats})
	}
	sort.Slice(result, func(left, right int) bool {
		first, second := result[left], result[right]
		if first.Stats.Active != second.Stats.Active {
			return first.Stats.Active > second.Stats.Active
		}
		firstBytes := first.Stats.Up + first.Stats.Down
		secondBytes := second.Stats.Up + second.Stats.Down
		if firstBytes != secondBytes {
			return firstBytes > secondBytes
		}
		if first.Address != second.Address {
			return first.Address < second.Address
		}
		return first.Via < second.Via
	})
	return result
}

func (c *counter) snapshot() counterSnapshot {
	snapshot := counterSnapshot{
		stats: Stats{
			Up:           c.up.Load(),
			Down:         c.down.Load(),
			RateUp:       c.rateUp.Load(),
			RateDown:     c.rateDown.Load(),
			PeakRateUp:   c.peakUp.Load(),
			PeakRateDown: c.peakDown.Load(),
			Active:       c.active.Load(),
			Total:        c.total.Load(),
			Dials:        c.dials.Load(),
			DialFailures: c.dialFailures.Load(),
			LatencyMs:    float64(c.latency.Load()) / float64(time.Millisecond),
		},
		lastDial:   c.lastDial.Load(),
		lastActive: c.lastActive.Load(),
		lastUp:     c.lastUp.Load(),
		lastDown:   c.lastDown.Load(),
		latencySum: c.latencySum.Load(),
	}
	snapshot.finish()
	return snapshot
}

func (c *counterSnapshot) add(other counterSnapshot) {
	c.stats.Up += other.stats.Up
	c.stats.Down += other.stats.Down
	c.stats.RateUp += other.stats.RateUp
	c.stats.RateDown += other.stats.RateDown
	c.stats.PeakRateUp += other.stats.PeakRateUp
	c.stats.PeakRateDown += other.stats.PeakRateDown
	c.stats.Active += other.stats.Active
	c.stats.Total += other.stats.Total
	c.stats.Dials += other.stats.Dials
	c.stats.DialFailures += other.stats.DialFailures
	c.latencySum += other.latencySum
	if other.lastDial > c.lastDial {
		c.lastDial = other.lastDial
		c.stats.LatencyMs = other.stats.LatencyMs
	}
	if other.lastActive > c.lastActive {
		c.lastActive = other.lastActive
	}
	if other.lastUp > c.lastUp {
		c.lastUp = other.lastUp
	}
	if other.lastDown > c.lastDown {
		c.lastDown = other.lastDown
	}
	c.finish()
}

func (c *counterSnapshot) finish() {
	if successes := c.stats.Dials - c.stats.DialFailures; successes > 0 {
		c.stats.AvgLatencyMs = float64(c.latencySum) / float64(successes) / float64(time.Millisecond)
	}
	if c.lastActive != 0 {
		c.stats.LastActive = time.Unix(0, c.lastActive).UTC().Format(time.RFC3339Nano)
	}
	if c.lastUp != 0 {
		c.stats.LastUp = time.Unix(0, c.lastUp).UTC().Format(time.RFC3339Nano)
	}
	if c.lastDown != 0 {
		c.stats.LastDown = time.Unix(0, c.lastDown).UTC().Format(time.RFC3339Nano)
	}
}
