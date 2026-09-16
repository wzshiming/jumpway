package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type collector struct {
	registry *Registry
}

type counterDescs struct {
	bytes         *prometheus.Desc
	bandwidth     *prometheus.Desc
	peakBandwidth *prometheus.Desc
	lastTransfer  *prometheus.Desc
	connections   *prometheus.Desc
	active        *prometheus.Desc
	dials         *prometheus.Desc
	dialFailures  *prometheus.Desc
	dialDuration  *prometheus.Desc
	lastDial      *prometheus.Desc
	lastActivity  *prometheus.Desc
}

var (
	ruleDescs   = newCounterDescs("rule", []string{"rule"})
	hopDescs    = newCounterDescs("hop", []string{"rule", "way", "hop", "url"})
	targetDescs = newCounterDescs("target", []string{"rule", "target", "via"})
	hopInfo     = prometheus.NewDesc("jumpway_hop_info", "Proxy URL and parent hop for each configured hop.",
		[]string{"rule", "way", "hop", "url", "parent"}, nil)
	targetsEvicted = prometheus.NewDesc("jumpway_rule_targets_evicted_total", "Total number of targets evicted from the rule.",
		[]string{"rule"}, nil)
	resetTimestamp = prometheus.NewDesc("jumpway_stats_reset_timestamp_seconds", "Unix timestamp of the last statistics reset or registry creation.", nil, nil)
)

func NewCollector(r *Registry) prometheus.Collector {
	return &collector{registry: r}
}

func NewHandler(r *Registry) http.Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(NewCollector(r), collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

func newCounterDescs(level string, labels []string) counterDescs {
	prefix := "jumpway_" + level + "_"
	return counterDescs{
		bytes: prometheus.NewDesc(prefix+"bytes_total", "Total number of bytes transferred in each direction.",
			append(labels, "direction"), nil),
		bandwidth: prometheus.NewDesc(prefix+"bandwidth_bytes_per_second", "Bytes transferred per second in the last completed sampling window.",
			append(labels, "direction"), nil),
		peakBandwidth: prometheus.NewDesc(prefix+"peak_bandwidth_bytes_per_second", "Highest bytes per second observed in a completed sampling window since the last reset.",
			append(labels, "direction"), nil),
		lastTransfer: prometheus.NewDesc(prefix+"last_transfer_timestamp_seconds", "Unix timestamp of the last byte transferred in each direction.",
			append(labels, "direction"), nil),
		connections:  prometheus.NewDesc(prefix+"connections_total", "Total number of established connections.", labels, nil),
		active:       prometheus.NewDesc(prefix+"active_connections", "Number of currently active connections.", labels, nil),
		dials:        prometheus.NewDesc(prefix+"dials_total", "Total number of dial attempts.", labels, nil),
		dialFailures: prometheus.NewDesc(prefix+"dial_failures_total", "Total number of failed dial attempts.", labels, nil),
		dialDuration: prometheus.NewDesc(prefix+"dial_duration_seconds", "Duration of successful dials in seconds.", labels, nil),
		lastDial:     prometheus.NewDesc(prefix+"last_dial_duration_seconds", "Duration of the last successful dial in seconds.", labels, nil),
		lastActivity: prometheus.NewDesc(prefix+"last_activity_timestamp_seconds", "Unix timestamp of the last connection or dial activity.", labels, nil),
	}
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for _, descs := range []counterDescs{ruleDescs, hopDescs, targetDescs} {
		for _, desc := range []*prometheus.Desc{
			descs.bytes, descs.bandwidth, descs.peakBandwidth, descs.lastTransfer, descs.connections, descs.active, descs.dials,
			descs.dialFailures, descs.dialDuration, descs.lastDial, descs.lastActivity,
		} {
			ch <- desc
		}
	}
	ch <- hopInfo
	ch <- targetsEvicted
	ch <- resetTimestamp
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.registry.mu.RLock()
	defer c.registry.mu.RUnlock()
	ch <- prometheus.MustNewConstMetric(resetTimestamp, prometheus.GaugeValue,
		float64(c.registry.since.UnixNano())/float64(time.Second))
	for _, name := range c.registry.order {
		rule := c.registry.rules[name]
		ruleDescs.collect(ch, rule.count, name)
		ch <- prometheus.MustNewConstMetric(targetsEvicted, prometheus.CounterValue, float64(rule.targetsEvicted), name)
		for role, wayName := range []string{"listen", "forward"} {
			way := rule.ways[role]
			for index, hop := range way {
				parent := "local"
				if index+1 < len(way) {
					parent = strconv.Itoa(index + 1)
				}
				seen := make(map[string]bool, len(hop.urls))
				for _, entry := range hop.urls {
					if seen[entry.url] {
						continue
					}
					seen[entry.url] = true
					labels := []string{name, wayName, strconv.Itoa(index), entry.url}
					hopDescs.collect(ch, entry.count, labels...)
					ch <- prometheus.MustNewConstMetric(hopInfo, prometheus.GaugeValue, 1, append(labels, parent)...)
				}
			}
		}
		for key, count := range rule.targets {
			targetDescs.collect(ch, count, name, key.address, key.via)
		}
	}
}

func (d counterDescs) collect(ch chan<- prometheus.Metric, count *counter, labels ...string) {
	count.mu.Lock()
	dials, failures := count.dials.Load(), count.dialFailures.Load()
	latency, latencySum := count.latency.Load(), count.latencySum.Load()
	lastDial := count.lastDial.Load()
	count.mu.Unlock()
	for _, direction := range []struct {
		name  string
		bytes int64
		rate  int64
		peak  int64
		last  int64
	}{
		{name: "up", bytes: count.up.Load(), rate: count.rateUp.Load(), peak: count.peakUp.Load(), last: count.lastUp.Load()},
		{name: "down", bytes: count.down.Load(), rate: count.rateDown.Load(), peak: count.peakDown.Load(), last: count.lastDown.Load()},
	} {
		values := append(labels, direction.name)
		ch <- prometheus.MustNewConstMetric(d.bytes, prometheus.CounterValue, float64(direction.bytes), values...)
		ch <- prometheus.MustNewConstMetric(d.bandwidth, prometheus.GaugeValue, float64(direction.rate), values...)
		ch <- prometheus.MustNewConstMetric(d.peakBandwidth, prometheus.GaugeValue, float64(direction.peak), values...)
		if direction.last != 0 {
			ch <- prometheus.MustNewConstMetric(d.lastTransfer, prometheus.GaugeValue, float64(direction.last)/float64(time.Second), values...)
		}
	}
	ch <- prometheus.MustNewConstMetric(d.connections, prometheus.CounterValue, float64(count.total.Load()), labels...)
	ch <- prometheus.MustNewConstMetric(d.active, prometheus.GaugeValue, float64(count.active.Load()), labels...)
	ch <- prometheus.MustNewConstMetric(d.dials, prometheus.CounterValue, float64(dials), labels...)
	ch <- prometheus.MustNewConstMetric(d.dialFailures, prometheus.CounterValue, float64(failures), labels...)
	ch <- prometheus.MustNewConstSummary(d.dialDuration, uint64(dials-failures), float64(latencySum)/float64(time.Second), nil, labels...)
	if lastDial != 0 {
		ch <- prometheus.MustNewConstMetric(d.lastDial, prometheus.GaugeValue, float64(latency)/float64(time.Second), labels...)
	}
	if lastActive := count.lastActive.Load(); lastActive != 0 {
		ch <- prometheus.MustNewConstMetric(d.lastActivity, prometheus.GaugeValue, float64(lastActive)/float64(time.Second), labels...)
	}
}
