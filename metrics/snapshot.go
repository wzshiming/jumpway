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
	Active       int64   `json:"active"`
	Total        int64   `json:"total"`
	Dials        int64   `json:"dials"`
	DialFailures int64   `json:"dial_failures"`
	LatencyMs    float64 `json:"latency_ms"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	LastActive   string  `json:"last_active,omitempty"`
}

type RuleStats struct {
	Name           string   `json:"name"`
	Stats          Stats    `json:"stats"`
	Listen         []Hop    `json:"listen"`
	Forward        []Hop    `json:"forward"`
	Targets        []Target `json:"targets"`
	TargetsEvicted int64    `json:"targets_evicted"`
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

type counterSnapshot struct {
	stats      Stats
	lastDial   int64
	lastActive int64
	latencySum int64
}

func (r *Registry) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := Snapshot{Since: r.since.Format(time.RFC3339), Rules: make([]RuleStats, 0, len(r.order))}
	for _, name := range r.order {
		rule := r.rules[name]
		snapshot.Rules = append(snapshot.Rules, RuleStats{
			Name:           name,
			Stats:          rule.count.snapshot().stats,
			Listen:         snapshotWay(rule.ways[Listen]),
			Forward:        snapshotWay(rule.ways[Forward]),
			Targets:        snapshotTargets(rule.targets),
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
		var aggregate counterSnapshot
		for _, entry := range entry.urls {
			count := entry.count.snapshot()
			hop.URLs = append(hop.URLs, URLStats{URL: entry.url, Stats: count.stats})
			aggregate.add(count)
		}
		hop.Stats = aggregate.stats
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
			Active:       c.active.Load(),
			Total:        c.total.Load(),
			Dials:        c.dials.Load(),
			DialFailures: c.dialFailures.Load(),
			LatencyMs:    float64(c.latency.Load()) / float64(time.Millisecond),
		},
		lastDial:   c.lastDial.Load(),
		lastActive: c.lastActive.Load(),
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
	c.finish()
}

func (c *counterSnapshot) finish() {
	if successes := c.stats.Dials - c.stats.DialFailures; successes > 0 {
		c.stats.AvgLatencyMs = float64(c.latencySum) / float64(successes) / float64(time.Millisecond)
	}
	if c.lastActive != 0 {
		c.stats.LastActive = time.Unix(0, c.lastActive).UTC().Format(time.RFC3339Nano)
	}
}
