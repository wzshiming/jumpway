import { displayURL, text } from './format';
import { t } from './i18n.svelte';
import { fuzzyMatch, type ListSort } from './search';
import { list, type Nullable, type RuleStats, type Stats } from './types';

const SUMMED = [
	'up',
	'down',
	'rate_up',
	'rate_down',
	'active',
	'total',
	'dials',
	'dial_failures',
	'peak_rate_up',
	'peak_rate_down'
] as const;

// avg latency is weighted by successful dials; last latency follows the newest last_active.
export function sumStats(entries: readonly Partial<Stats>[]): Stats {
	const stats: Stats = {
		up: 0,
		down: 0,
		rate_up: 0,
		rate_down: 0,
		active: 0,
		total: 0,
		dials: 0,
		dial_failures: 0,
		latency_ms: 0,
		avg_latency_ms: 0,
		peak_rate_up: 0,
		peak_rate_down: 0
	};
	let weightedLatency = 0;
	let latest = -Infinity;
	for (const entry of entries) {
		for (const key of SUMMED) stats[key] += Number(entry[key]) || 0;
		for (const key of ['last_up', 'last_down'] as const) {
			const value = entry[key];
			const current = stats[key];
			if (
				Number.isFinite(Date.parse(text(value))) &&
				(!current || Date.parse(text(value)) > Date.parse(current))
			) {
				stats[key] = value;
			}
		}
		weightedLatency +=
			Math.max(0, (entry.dials || 0) - (entry.dial_failures || 0)) * (entry.avg_latency_ms || 0);
		const timestamp = Date.parse(text(entry.last_active));
		if (timestamp > latest) {
			latest = timestamp;
			stats.last_active = entry.last_active;
			stats.latency_ms = entry.latency_ms || 0;
		}
	}
	const successes = stats.dials - stats.dial_failures;
	stats.avg_latency_ms = successes > 0 ? weightedLatency / successes : 0;
	return stats;
}

export interface EndpointUse {
	rule: string;
	way: 'listen' | 'forward';
	index: number;
}

export interface HostEndpoint {
	endpoint: string;
	urls: string[];
	uses: EndpointUse[];
	stats: Stats;
}

export interface HostAggregate {
	host: string;
	rules: string[];
	stats: Stats;
	endpoints: HostEndpoint[];
}

interface EndpointBucket {
	endpoint: string;
	urls: Set<string>;
	uses: EndpointUse[];
	samples: Partial<Stats>[];
}

export function aggregateHosts(rules: Nullable<RuleStats[]>): HostAggregate[] {
	const hosts = new Map<
		string,
		{ host: string; rules: Set<string>; endpoints: Map<string, EndpointBucket> }
	>();
	for (const rule of list(rules)) {
		for (const way of ['listen', 'forward'] as const) {
			for (const hop of list(rule[way])) {
				for (const entry of list(hop.urls)) {
					let hostname = entry.url;
					try {
						hostname = new URL(entry.url).hostname || entry.url;
					} catch {
						// cmd:/nc: hops are not URLs; the raw string is the host key.
					}
					let host = hosts.get(hostname);
					if (!host) {
						host = { host: hostname, rules: new Set(), endpoints: new Map() };
						hosts.set(hostname, host);
					}
					const label = displayURL(entry.url);
					let endpoint = host.endpoints.get(label);
					if (!endpoint) {
						endpoint = { endpoint: label, urls: new Set(), uses: [], samples: [] };
						host.endpoints.set(label, endpoint);
					}
					host.rules.add(rule.name);
					endpoint.urls.add(entry.url);
					endpoint.samples.push(entry.stats || {});
					if (
						!endpoint.uses.some(
							(use) => use.rule === rule.name && use.way === way && use.index === hop.index
						)
					) {
						endpoint.uses.push({ rule: rule.name, way, index: hop.index });
					}
				}
			}
		}
	}
	const totalBytes = (value: { stats: Stats }) => value.stats.up + value.stats.down;
	return Array.from(hosts.values(), (host) => {
		const buckets = Array.from(host.endpoints.values());
		return {
			host: host.host,
			rules: Array.from(host.rules),
			stats: sumStats(buckets.flatMap((endpoint) => endpoint.samples)),
			endpoints: buckets
				.map((endpoint) => ({
					endpoint: endpoint.endpoint,
					urls: Array.from(endpoint.urls),
					uses: endpoint.uses,
					stats: sumStats(endpoint.samples)
				}))
				.sort((left, right) => totalBytes(right) - totalBytes(left))
		};
	}).sort(
		(left, right) => totalBytes(right) - totalBytes(left) || left.host.localeCompare(right.host)
	);
}

export type HostSortKey = 'traffic' | 'host' | 'rate' | 'active' | 'dial_failures' | 'latency';
export type HostSort = ListSort<HostSortKey>;

// Descending traffic is the order aggregateHosts already yields.
export const DEFAULT_HOST_SORT: HostSort = { key: 'traffic', direction: 'descending' };

export const HOST_SORT_KEYS: readonly HostSortKey[] = [
	'traffic',
	'host',
	'rate',
	'active',
	'dial_failures',
	'latency'
];

export function hostSortLabel(key: HostSortKey): string {
	switch (key) {
		case 'traffic':
			return t('traffic');
		case 'host':
			return t('host');
		case 'rate':
			return t('currentRate');
		case 'active':
			return t('connections');
		case 'dial_failures':
			return t('dialFailures');
		case 'latency':
			return t('latency');
	}
}

export function filterHosts(
	hosts: readonly HostAggregate[],
	query: string,
	language: string
): HostAggregate[] {
	return hosts.filter((host) =>
		fuzzyMatch(query, [host.host, ...host.endpoints.map((endpoint) => endpoint.endpoint)], language)
	);
}

const hostValue = (host: HostAggregate, key: Exclude<HostSortKey, 'host'>): number => {
	switch (key) {
		case 'traffic':
			return host.stats.up + host.stats.down;
		case 'rate':
			return host.stats.rate_up + host.stats.rate_down;
		case 'active':
			return host.stats.active;
		case 'dial_failures':
			return host.stats.dial_failures;
		case 'latency':
			return host.stats.avg_latency_ms;
	}
};

// Ties keep the aggregate order (traffic, then host name) in either direction.
export function sortHosts(
	hosts: readonly HostAggregate[],
	sort: HostSort,
	language: string
): HostAggregate[] {
	const direction = sort.direction === 'ascending' ? 1 : -1;
	const key = sort.key;
	const compare =
		key === 'host'
			? (left: HostAggregate, right: HostAggregate) =>
					left.host.localeCompare(right.host, language, { numeric: true })
			: (left: HostAggregate, right: HostAggregate) => hostValue(left, key) - hostValue(right, key);
	return hosts.slice().sort((left, right) => compare(left, right) * direction);
}
