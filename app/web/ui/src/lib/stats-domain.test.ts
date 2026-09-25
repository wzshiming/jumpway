import { afterEach, describe, expect, test } from 'vitest';
import {
	CONNECTION_PAGE_SIZE,
	CONNECTION_SORT_KEYS,
	DEFAULT_CONNECTION_SORT,
	clientLabel,
	connectionSortLabel,
	currentConnections,
	filterConnections,
	formatConnectionPath,
	sortConnections
} from './connections';
import {
	DEFAULT_HOST_SORT,
	HOST_SORT_KEYS,
	aggregateHosts,
	filterHosts,
	hostSortLabel,
	sortHosts,
	sumStats,
	type HostAggregate
} from './hosts';
import { i18n } from './i18n.svelte';
import type { RuleStats, Stats } from './types';

afterEach(() => {
	i18n.dispose();
	localStorage.clear();
});

const stats = (partial: Partial<Stats>): Stats => ({
	up: 0,
	down: 0,
	rate_up: 0,
	rate_down: 0,
	peak_rate_up: 0,
	peak_rate_down: 0,
	active: 0,
	total: 0,
	dials: 0,
	dial_failures: 0,
	latency_ms: 0,
	avg_latency_ms: 0,
	...partial
});

const ZERO = sumStats([]);

describe('sumStats', () => {
	test('sums counters, weights the average latency by successful dials and keeps the newest timestamps', () => {
		const total = sumStats([
			{
				up: 1,
				down: 2,
				rate_up: 3,
				rate_down: 4,
				active: 1,
				total: 2,
				dials: 4,
				dial_failures: 1,
				latency_ms: 10,
				avg_latency_ms: 20,
				peak_rate_up: 5,
				peak_rate_down: 6,
				last_active: '2026-09-18T11:00:00Z',
				last_up: '2026-09-18T11:00:00Z'
			},
			{
				up: 10,
				down: 20,
				rate_up: 30,
				rate_down: 40,
				active: 2,
				total: 3,
				dials: 2,
				dial_failures: 0,
				latency_ms: 50,
				avg_latency_ms: 60,
				peak_rate_up: 7,
				peak_rate_down: 8,
				last_active: '2026-09-18T11:30:00Z',
				last_up: '2026-09-18T10:00:00Z',
				last_down: 'bad'
			}
		]);
		expect(total).toEqual({
			up: 11,
			down: 22,
			rate_up: 33,
			rate_down: 44,
			active: 3,
			total: 5,
			dials: 6,
			dial_failures: 1,
			latency_ms: 50,
			avg_latency_ms: 36,
			peak_rate_up: 12,
			peak_rate_down: 14,
			last_up: '2026-09-18T11:00:00Z',
			last_active: '2026-09-18T11:30:00Z'
		});
	});

	test('all-failed dials yield zero average latency and nulls count as zero', () => {
		expect(sumStats([{ dials: 2, dial_failures: 2, avg_latency_ms: 9, latency_ms: 3 }])).toEqual({
			...ZERO,
			dials: 2,
			dial_failures: 2
		});
		expect(sumStats([{}, { up: null as unknown as number }])).toEqual(ZERO);
	});
});

describe('aggregateHosts', () => {
	const rules: RuleStats[] = [
		{
			name: 'a',
			stats: ZERO,
			listen: null,
			forward: [
				{
					index: 0,
					parent_index: -1,
					stats: ZERO,
					urls: [
						{
							url: 'socks5://u:p@h1:1080',
							stats: stats({ up: 1, down: 1, dials: 2, avg_latency_ms: 10 })
						},
						{ url: 'socks5://x:y@h1:1080', stats: stats({ up: 5 }) }
					]
				},
				{
					index: 1,
					parent_index: 0,
					stats: ZERO,
					urls: [
						{ url: 'ssh://u@h2:22', stats: stats({ up: 100 }) },
						{ url: 'ssh://u@h2:2222', stats: stats({ up: 1 }) }
					]
				}
			],
			targets: null,
			connections: null,
			targets_evicted: 0
		},
		{
			name: 'b',
			stats: ZERO,
			listen: [
				{
					index: 0,
					parent_index: -1,
					stats: ZERO,
					urls: [{ url: 'ssh://h2:22', stats: stats({ down: 100 }) }]
				}
			],
			forward: [
				{ index: 0, parent_index: -1, stats: ZERO, urls: [{ url: 'cmd:ssh jump', stats: ZERO }] }
			],
			targets: null,
			connections: null,
			targets_evicted: 0
		},
		{
			name: 'c',
			stats: ZERO,
			listen: null,
			forward: [
				{
					index: 0,
					parent_index: -1,
					stats: ZERO,
					urls: [{ url: 'socks5://h1:1080', stats: ZERO }]
				}
			],
			targets: null,
			connections: null,
			targets_evicted: 0
		}
	];

	test('groups by hostname, merges redacted duplicates and orders by bytes then name', () => {
		const hosts = aggregateHosts(rules);
		expect(hosts.map((host) => host.host)).toEqual(['h2', 'h1', 'cmd:ssh jump']);
		expect(hosts[0].rules).toEqual(['a', 'b']);
		expect(hosts[0].stats).toEqual({ ...ZERO, up: 101, down: 100 });
		expect(hosts[0].endpoints.map((endpoint) => endpoint.endpoint)).toEqual([
			'ssh://h2:22',
			'ssh://h2:2222'
		]);
		expect(hosts[0].endpoints[0]).toEqual({
			endpoint: 'ssh://h2:22',
			urls: ['ssh://u@h2:22', 'ssh://h2:22'],
			uses: [
				{ rule: 'a', way: 'forward', index: 1 },
				{ rule: 'b', way: 'listen', index: 0 }
			],
			stats: { ...ZERO, up: 100, down: 100 }
		});
		expect(hosts[1].endpoints[0].urls).toEqual([
			'socks5://u:p@h1:1080',
			'socks5://x:y@h1:1080',
			'socks5://h1:1080'
		]);
		expect(hosts[1].stats).toEqual({ ...ZERO, up: 6, down: 1, dials: 2, avg_latency_ms: 10 });
		expect(hosts[2].endpoints[0].uses).toEqual([{ rule: 'b', way: 'forward', index: 0 }]);
	});

	test('tolerates null snapshots', () => {
		expect(aggregateHosts(null)).toEqual([]);
	});
});

describe('host sort and filter', () => {
	const host = (
		name: string,
		partial: Partial<Stats>,
		endpoints: string[] = [`ssh://${name}:22`]
	): HostAggregate => ({
		host: name,
		rules: [],
		stats: stats(partial),
		endpoints: endpoints.map((endpoint) => ({ endpoint, urls: [endpoint], uses: [], stats: ZERO }))
	});
	const names = (hosts: readonly HostAggregate[]) => hosts.map((item) => item.host);
	// In aggregate order: bytes descending, then name.
	const hosts = [
		host('hop-10.example', { up: 50, down: 250, rate_down: 1, active: 3, avg_latency_ms: 30 }),
		host(
			'bastion.example',
			{ up: 100, down: 100, rate_up: 5, dial_failures: 2, avg_latency_ms: 10 },
			['ssh://bastion.example:22', 'ssh://bastion.example:2222']
		),
		host('edge.example', { up: 100, down: 100, rate_down: 5, active: 1, avg_latency_ms: 20 }),
		host('hop-9.example', { dial_failures: 7 }, ['socks5://hop-9.example:1080'])
	];

	test('the sort menu offers traffic, host, rate, connections, failures and latency, labelled in the active language', () => {
		i18n.init();
		expect(DEFAULT_HOST_SORT).toEqual({ key: 'traffic', direction: 'descending' });
		expect(HOST_SORT_KEYS).toEqual([
			'traffic',
			'host',
			'rate',
			'active',
			'dial_failures',
			'latency'
		]);
		expect(HOST_SORT_KEYS.map(hostSortLabel)).toEqual([
			'Traffic',
			'Host',
			'Current rate',
			'Connections',
			'Failures',
			'Latency'
		]);
		i18n.setLanguage('zh');
		expect(HOST_SORT_KEYS.map(hostSortLabel)).toEqual([
			'\u6d41\u91cf',
			'\u4e3b\u673a',
			'\u5f53\u524d\u901f\u7387',
			'\u8fde\u63a5',
			'\u5931\u8d25',
			'\u5ef6\u8fdf'
		]);
	});

	test('filterHosts matches the host name and every endpoint label as a case-insensitive subsequence', () => {
		expect(names(filterHosts(hosts, '', 'en'))).toEqual(names(hosts));
		expect(names(filterHosts(hosts, 'BASTION', 'en'))).toEqual(['bastion.example']);
		expect(names(filterHosts(hosts, '2222', 'en'))).toEqual(['bastion.example']);
		expect(names(filterHosts(hosts, 'socks', 'en'))).toEqual(['hop-9.example']);
		expect(names(filterHosts(hosts, 'hop-', 'en'))).toEqual(['hop-10.example', 'hop-9.example']);
		// "hop" is also a subsequence of ssh://bastion.example:22.
		expect(names(filterHosts(hosts, 'hop', 'en'))).toEqual([
			'hop-10.example',
			'bastion.example',
			'hop-9.example'
		]);
		expect(names(filterHosts(hosts, 'zzz', 'en'))).toEqual([]);
	});

	test('the default traffic sort keeps the aggregate order and reverses it ascending; ties keep that order', () => {
		expect(names(sortHosts(hosts, DEFAULT_HOST_SORT, 'en'))).toEqual(names(hosts));
		expect(sortHosts(hosts, DEFAULT_HOST_SORT, 'en')).not.toBe(hosts);
		expect(names(sortHosts(hosts, { key: 'traffic', direction: 'ascending' }, 'en'))).toEqual([
			'hop-9.example',
			'bastion.example',
			'edge.example',
			'hop-10.example'
		]);
	});

	test('host sorts by locale with numeric collation; the other keys by their counter', () => {
		expect(names(sortHosts(hosts, { key: 'host', direction: 'ascending' }, 'en'))).toEqual([
			'bastion.example',
			'edge.example',
			'hop-9.example',
			'hop-10.example'
		]);
		expect(names(sortHosts(hosts, { key: 'host', direction: 'descending' }, 'en'))).toEqual([
			'hop-10.example',
			'hop-9.example',
			'edge.example',
			'bastion.example'
		]);
		// rate = up + down; bastion and edge tie at 5 and keep the aggregate order.
		expect(names(sortHosts(hosts, { key: 'rate', direction: 'descending' }, 'en'))).toEqual([
			'bastion.example',
			'edge.example',
			'hop-10.example',
			'hop-9.example'
		]);
		expect(names(sortHosts(hosts, { key: 'active', direction: 'descending' }, 'en'))).toEqual([
			'hop-10.example',
			'edge.example',
			'bastion.example',
			'hop-9.example'
		]);
		expect(
			names(sortHosts(hosts, { key: 'dial_failures', direction: 'descending' }, 'en'))
		).toEqual(['hop-9.example', 'bastion.example', 'hop-10.example', 'edge.example']);
		expect(names(sortHosts(hosts, { key: 'latency', direction: 'ascending' }, 'en'))).toEqual([
			'hop-9.example',
			'bastion.example',
			'edge.example',
			'hop-10.example'
		]);
		expect(names(hosts)).toEqual([
			'hop-10.example',
			'bastion.example',
			'edge.example',
			'hop-9.example'
		]);
	});
});

describe('connections', () => {
	const snapshot: RuleStats[] = [
		{
			name: 'r1',
			stats: ZERO,
			listen: null,
			forward: null,
			targets: null,
			targets_evicted: 0,
			connections: [
				{
					id: 3,
					client: '127.0.0.1:18094',
					process: { pid: 42, name: 'curl' },
					target: 'example.com:443',
					via: '',
					path: [],
					started: '2026-09-18T11:00:00Z',
					stats: stats({ down: 10, rate_down: 1 })
				},
				{
					id: 1,
					client: '[::1]:5000',
					target: 'zeta.test:80',
					via: '',
					path: null,
					started: '2026-09-18T11:30:00Z',
					stats: stats({ down: 30 })
				}
			]
		},
		{
			name: 'r2',
			stats: ZERO,
			listen: null,
			forward: null,
			targets: null,
			targets_evicted: 0,
			connections: [
				{
					id: 2,
					client: '',
					target: 'alpha.test:22',
					via: '',
					path: null,
					started: 'bad',
					stats: stats({ down: 20 })
				}
			]
		},
		{
			name: 'r3',
			stats: ZERO,
			listen: null,
			forward: null,
			targets: null,
			targets_evicted: 0,
			connections: null
		}
	];
	const all = currentConnections(snapshot);
	const ids = (sorted: typeof all) => sorted.map((connection) => connection.id);

	test('flattens rules and tags each connection with its rule', () => {
		expect(all.map((connection) => [connection.id, connection.rule])).toEqual([
			[3, 'r1'],
			[1, 'r1'],
			[2, 'r2']
		]);
		expect(currentConnections(null)).toEqual([]);
	});

	test('filters by rule and by case-insensitive subsequence over target, client, rule and process', () => {
		i18n.init();
		expect(ids(filterConnections(all, '', '1809'))).toEqual([3]);
		expect(ids(filterConnections(all, '', 'zzz'))).toEqual([]);
		expect(ids(filterConnections(all, 'r1', ''))).toEqual([3, 1]);
		expect(ids(filterConnections(all, '', 'CURL'))).toEqual([3]);
		expect(ids(filterConnections(all, '', 'z8'))).toEqual([1]);
		expect(ids(filterConnections(all, '', 'et:'))).toEqual([1, 2]);
		expect(ids(filterConnections(all, 'r2', 'zeta'))).toEqual([]);
	});

	test('sorts by time, numbers and locale strings with the id as tie breaker', () => {
		i18n.init();
		expect(DEFAULT_CONNECTION_SORT).toEqual({ key: 'started', direction: 'descending' });
		expect(ids(sortConnections(all))).toEqual([1, 3, 2]);
		expect(ids(sortConnections(all, { key: 'started', direction: 'ascending' }))).toEqual([
			2, 3, 1
		]);
		expect(ids(sortConnections(all, { key: 'down', direction: 'descending' }))).toEqual([1, 2, 3]);
		expect(ids(sortConnections(all, { key: 'rate_down', direction: 'ascending' }))).toEqual([
			1, 2, 3
		]);
		expect(ids(sortConnections(all, { key: 'client', direction: 'ascending' }))).toEqual([2, 1, 3]);
		expect(ids(sortConnections(all, { key: 'target', direction: 'ascending' }))).toEqual([2, 3, 1]);
		expect(ids(sortConnections(all, { key: 'rule', direction: 'descending' }))).toEqual([2, 1, 3]);
		expect(all.map((connection) => connection.id)).toEqual([3, 1, 2]);
	});

	test('client label prefers the process name and rows are revealed 200 at a time', () => {
		expect(all.map(clientLabel)).toEqual(['curl', '::1', '\u2014']);
		expect(CONNECTION_PAGE_SIZE).toBe(200);
	});

	test('the sort menu lists the four identity keys, then each metric download first, labelled in the active language', () => {
		i18n.init();
		expect(CONNECTION_SORT_KEYS).toEqual([
			'started',
			'target',
			'client',
			'rule',
			'rate_down',
			'rate_up',
			'peak_rate_down',
			'peak_rate_up',
			'down',
			'up',
			'last_down',
			'last_up'
		]);
		expect(CONNECTION_SORT_KEYS.map(connectionSortLabel)).toEqual([
			'Duration',
			'Target',
			'Client',
			'Rule',
			'Download \u00b7 now',
			'Upload \u00b7 now',
			'Download \u00b7 peak',
			'Upload \u00b7 peak',
			'Download \u00b7 total',
			'Upload \u00b7 total',
			'Download \u00b7 last',
			'Upload \u00b7 last'
		]);
		i18n.setLanguage('zh');
		expect(connectionSortLabel('rate_down')).toBe('\u4e0b\u8f7d \u00b7 \u5f53\u524d');
		expect(connectionSortLabel('started')).toBe('\u6301\u7eed\u65f6\u95f4');
		// Keys without a caption (never offered) fall back to themselves.
		expect(connectionSortLabel('dials')).toBe('dials');
	});

	test('path formatting hides credentials, marks reused hops and falls back to hop numbers', () => {
		i18n.init();
		expect(formatConnectionPath({ path: null, target: 't:1' })).toBe('direct');
		expect(
			formatConnectionPath({
				path: [{ index: 0, url: 'socks5://u:p@h:1080', dialed: true }],
				target: 'a:1'
			})
		).toBe('this machine \u2192 socks5://h:1080 \u2192 a:1');
		expect(
			formatConnectionPath({
				path: [
					{ index: 0, url: 'ssh://u@exit:22', dialed: false },
					{ index: 1, url: '', dialed: false }
				],
				target: ''
			})
		).toBe('this machine \u2192 Hop 2 (reused) \u2192 ssh://exit:22 (reused) \u2192 target');
	});
});
