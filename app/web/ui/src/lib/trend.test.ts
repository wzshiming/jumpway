import { afterEach, describe, expect, test, vi } from 'vitest';
import { statsOf } from '../../e2e/fixtures/api';
import { displayURL } from './format';
import { aggregateHosts, sumStats } from './hosts';
import {
	EMPTY_TREND,
	TREND_GAP_MS,
	TREND_MIN_STEP_MS,
	TREND_SAMPLES,
	TREND_TOTAL,
	recordTrend,
	type RateSample,
	type TrendState
} from './trend';
import { trend } from './trend.svelte';
import type { Connection, Hop, RuleStats, Snapshot, Stats } from './types';

const rule = (name: string, stats: Partial<Stats>, extra: Partial<RuleStats> = {}): RuleStats => ({
	name,
	stats: statsOf(stats),
	listen: null,
	forward: null,
	targets: null,
	connections: null,
	targets_evicted: 0,
	...extra
});

const hop = (urls: Record<string, Partial<Stats>>): Hop => ({
	index: 0,
	parent_index: -1,
	stats: statsOf({}),
	urls: Object.entries(urls).map(([url, stats]) => ({ url, stats: statsOf(stats) }))
});

const connection = (id: number, stats: Partial<Stats>): Connection => ({
	id,
	target: 'example.com:443',
	via: 'direct',
	path: null,
	started: '2026-01-01T00:00:00Z',
	stats: statsOf(stats)
});

const SOCKS_URL = 'socks5://hop-a.example:1080';
const SSH_URL = 'ssh://hop-a.example:22';

const snapshot = (rules: RuleStats[] | null, since = 'since-1'): Snapshot => ({ since, rules });

const pairs = (samples: readonly RateSample[] | undefined) =>
	samples?.map((sample) => [sample.up, sample.down]);

const rates = (state: TrendState, name: string) => pairs(state.series.get(name));
const hostRates = (state: TrendState, host: string) => pairs(state.hosts.get(host)?.samples);
const endpointRates = (state: TrendState, host: string, endpoint: string) =>
	pairs(state.hosts.get(host)?.endpoints.get(endpoint));
const connectionRates = (state: TrendState, id: number) => pairs(state.connections.get(id));

const names = (state: TrendState) => [...state.series.keys()].sort();

describe('recordTrend', () => {
	test('starts empty', () => {
		expect(EMPTY_TREND).toEqual({
			since: null,
			lastAt: null,
			series: new Map(),
			hosts: new Map(),
			connections: new Map()
		});
		expect(TREND_TOTAL).toBe('');
	});

	test('appends one sample per rule and one aggregate per accepted snapshot', () => {
		const first = recordTrend(
			EMPTY_TREND,
			snapshot([
				rule('office', { rate_up: 10, rate_down: 100 }),
				rule('lab', { rate_up: 1, rate_down: 2 })
			]),
			1000
		);
		expect(first.since).toBe('since-1');
		expect(first.lastAt).toBe(1000);
		expect(names(first)).toEqual([TREND_TOTAL, 'lab', 'office']);
		expect(rates(first, 'office')).toEqual([[10, 100]]);
		expect(rates(first, 'lab')).toEqual([[1, 2]]);
		expect(rates(first, TREND_TOTAL)).toEqual([[11, 102]]);
		const second = recordTrend(
			first,
			snapshot([
				rule('office', { rate_up: 20, rate_down: 200 }),
				rule('lab', { rate_up: 3, rate_down: 4 })
			]),
			2000
		);
		expect(second.lastAt).toBe(2000);
		expect(rates(second, 'office')).toEqual([
			[10, 100],
			[20, 200]
		]);
		expect(rates(second, TREND_TOTAL)).toEqual([
			[11, 102],
			[23, 204]
		]);
	});

	test('the aggregate is sumStats over the snapshot rules', () => {
		const rules = [
			rule('a', { rate_up: 1.5, rate_down: 2.5, up: 7 }),
			rule('b', { rate_up: 4, rate_down: 8, down: 9 })
		];
		const state = recordTrend(EMPTY_TREND, snapshot(rules), 0);
		const total = sumStats(rules.map((entry) => entry.stats));
		expect(rates(state, TREND_TOTAL)).toEqual([[total.rate_up, total.rate_down]]);
		expect(rates(state, TREND_TOTAL)).toEqual([[5.5, 10.5]]);
	});

	test('keeps only the last TREND_SAMPLES samples', () => {
		expect(TREND_SAMPLES).toBe(60);
		let state = EMPTY_TREND;
		for (let index = 1; index <= TREND_SAMPLES + 1; index++) {
			state = recordTrend(state, snapshot([rule('office', { rate_up: index })]), index * 1000);
		}
		const office = state.series.get('office')!;
		expect(office).toHaveLength(TREND_SAMPLES);
		expect(office[0].up).toBe(2);
		expect(office.at(-1)!.up).toBe(TREND_SAMPLES + 1);
		expect(state.series.get(TREND_TOTAL)).toHaveLength(TREND_SAMPLES);
	});

	test('a snapshot less than TREND_MIN_STEP_MS after the last sample is skipped: same state back', () => {
		const first = recordTrend(EMPTY_TREND, snapshot([rule('office', { rate_up: 1 })]), 1000);
		const skipped = recordTrend(
			first,
			snapshot([rule('office', { rate_up: 2 })]),
			1000 + TREND_MIN_STEP_MS - 1
		);
		expect(skipped).toBe(first);
		expect(skipped.hosts).toBe(first.hosts);
		expect(skipped.connections).toBe(first.connections);
		const taken = recordTrend(
			first,
			snapshot([rule('office', { rate_up: 3 })]),
			1000 + TREND_MIN_STEP_MS
		);
		expect(taken).not.toBe(first);
		expect(rates(taken, 'office')).toEqual([
			[1, 0],
			[3, 0]
		]);
		expect(taken.lastAt).toBe(1000 + TREND_MIN_STEP_MS);
	});

	test('a changed since restarts every series, even right after the last sample', () => {
		let state = recordTrend(EMPTY_TREND, snapshot([rule('office', { rate_up: 1 })], 'a'), 1000);
		state = recordTrend(state, snapshot([rule('office', { rate_up: 2 })], 'a'), 2000);
		expect(rates(state, 'office')).toHaveLength(2);
		const reset = recordTrend(state, snapshot([rule('office', { rate_up: 0 })], 'b'), 2100);
		expect(reset.since).toBe('b');
		expect(reset.lastAt).toBe(2100);
		expect(rates(reset, 'office')).toEqual([[0, 0]]);
		expect(rates(reset, TREND_TOTAL)).toEqual([[0, 0]]);
	});

	test('a gap longer than TREND_GAP_MS restarts every series', () => {
		let state = recordTrend(EMPTY_TREND, snapshot([rule('office', { rate_up: 1 })]), 1000);
		state = recordTrend(state, snapshot([rule('office', { rate_up: 2 })]), 1000 + TREND_GAP_MS);
		expect(rates(state, 'office')).toEqual([
			[1, 0],
			[2, 0]
		]);
		const restarted = recordTrend(
			state,
			snapshot([rule('office', { rate_up: 3 })]),
			1000 + TREND_GAP_MS + TREND_GAP_MS + 1
		);
		expect(restarted.since).toBe('since-1');
		expect(rates(restarted, 'office')).toEqual([[3, 0]]);
		expect(rates(restarted, TREND_TOTAL)).toEqual([[3, 0]]);
	});

	test('drops the series of rules missing from the snapshot; a rule that returns starts afresh', () => {
		let state = recordTrend(
			EMPTY_TREND,
			snapshot([rule('office', { rate_up: 1 }), rule('lab', { rate_up: 2 })]),
			1000
		);
		state = recordTrend(state, snapshot([rule('office', { rate_up: 3 })]), 2000);
		expect(names(state)).toEqual([TREND_TOTAL, 'office']);
		expect(rates(state, 'office')).toEqual([
			[1, 0],
			[3, 0]
		]);
		state = recordTrend(
			state,
			snapshot([rule('office', { rate_up: 4 }), rule('lab', { rate_up: 5 })]),
			3000
		);
		expect(rates(state, 'lab')).toEqual([[5, 0]]);
		expect(rates(state, 'office')).toHaveLength(3);
	});

	test('null or empty rules keep the aggregate going with zeros and prune every rule series', () => {
		let state = recordTrend(
			EMPTY_TREND,
			snapshot([rule('office', { rate_up: 1, rate_down: 2 })]),
			1000
		);
		state = recordTrend(state, snapshot(null), 2000);
		expect(names(state)).toEqual([TREND_TOTAL]);
		expect(rates(state, TREND_TOTAL)).toEqual([
			[1, 2],
			[0, 0]
		]);
		state = recordTrend(state, snapshot([]), 3000);
		expect(rates(state, TREND_TOTAL)).toEqual([
			[1, 2],
			[0, 0],
			[0, 0]
		]);
	});

	test('non-finite or missing rates are recorded as 0', () => {
		const state = recordTrend(
			EMPTY_TREND,
			snapshot([
				rule('office', { rate_up: NaN, rate_down: Infinity }),
				rule('lab', { rate_up: undefined as unknown as number, rate_down: 5 })
			]),
			1000
		);
		expect(rates(state, 'office')).toEqual([[0, 0]]);
		expect(rates(state, 'lab')).toEqual([[0, 5]]);
		expect(rates(state, TREND_TOTAL)).toEqual([[0, 5]]);
	});

	test('never mutates the previous state', () => {
		const first = recordTrend(
			EMPTY_TREND,
			snapshot([rule('office', { rate_up: 1 }), rule('lab', { rate_up: 2 })]),
			1000
		);
		const office = first.series.get('office')!;
		const total = first.series.get(TREND_TOTAL)!;
		const second = recordTrend(first, snapshot([rule('office', { rate_up: 3 })]), 2000);
		expect(second).not.toBe(first);
		expect(first.series.get('office')).toBe(office);
		expect(office).toEqual([{ up: 1, down: 0 }]);
		expect(total).toEqual([{ up: 3, down: 0 }]);
		expect(names(first)).toEqual([TREND_TOTAL, 'lab', 'office']);
		expect(first.lastAt).toBe(1000);
		expect(second.series.get('office')).not.toBe(office);
		expect(EMPTY_TREND.series.size).toBe(0);
		expect(EMPTY_TREND.lastAt).toBeNull();
	});

	test('records one sample per host and per endpoint from the aggregated URL stats', () => {
		const rules = [
			rule(
				'office',
				{ rate_up: 1000, rate_down: 2000 },
				{
					forward: [
						hop({
							[SOCKS_URL]: { rate_up: 10, rate_down: 100 },
							[SSH_URL]: { rate_up: 5, rate_down: 50 }
						})
					]
				}
			),
			rule(
				'lab',
				{ rate_up: 3000, rate_down: 4000 },
				{ forward: [hop({ [SOCKS_URL]: { rate_up: 1, rate_down: 2 } })] }
			)
		];
		const state = recordTrend(EMPTY_TREND, snapshot(rules), 1000);
		expect(rates(state, 'office')).toEqual([[1000, 2000]]);
		expect(rates(state, TREND_TOTAL)).toEqual([[4000, 6000]]);
		expect([...state.hosts.keys()]).toEqual(['hop-a.example']);
		expect(hostRates(state, 'hop-a.example')).toEqual([[16, 152]]);
		const [host] = aggregateHosts(rules);
		expect(hostRates(state, 'hop-a.example')).toEqual([[host.stats.rate_up, host.stats.rate_down]]);
		expect([...state.hosts.get('hop-a.example')!.endpoints.keys()].sort()).toEqual(
			[displayURL(SOCKS_URL), displayURL(SSH_URL)].sort()
		);
		expect(endpointRates(state, 'hop-a.example', displayURL(SOCKS_URL))).toEqual([[11, 102]]);
		expect(endpointRates(state, 'hop-a.example', displayURL(SSH_URL))).toEqual([[5, 50]]);
		expect(connectionRates(state, 101)).toBeUndefined();
	});

	test('records one sample per connection id across rules and prunes ids that vanished', () => {
		const first = recordTrend(
			EMPTY_TREND,
			snapshot([
				rule(
					'office',
					{ rate_up: 1000 },
					{
						connections: [connection(101, { rate_up: 7, rate_down: 70 }), connection(102, {})]
					}
				),
				rule(
					'lab',
					{ rate_up: 2000 },
					{ connections: [connection(201, { rate_up: 3, rate_down: 30 })] }
				)
			]),
			1000
		);
		expect([...first.connections.keys()].sort((a, b) => a - b)).toEqual([101, 102, 201]);
		expect(connectionRates(first, 101)).toEqual([[7, 70]]);
		expect(connectionRates(first, 102)).toEqual([[0, 0]]);
		expect(connectionRates(first, 201)).toEqual([[3, 30]]);
		expect(first.hosts.size).toBe(0);
		const second = recordTrend(
			first,
			snapshot([
				rule(
					'office',
					{ rate_up: 1000 },
					{ connections: [connection(101, { rate_up: 8, rate_down: 80 })] }
				),
				rule(
					'lab',
					{ rate_up: 2000 },
					{ connections: [connection(201, { rate_up: 4, rate_down: 40 })] }
				)
			]),
			2000
		);
		expect([...second.connections.keys()].sort((a, b) => a - b)).toEqual([101, 201]);
		expect(connectionRates(second, 101)).toEqual([
			[7, 70],
			[8, 80]
		]);
		expect(connectionRates(second, 201)).toEqual([
			[3, 30],
			[4, 40]
		]);
		expect(second.connections.has(102)).toBe(false);
		expect(connectionRates(first, 101)).toEqual([[7, 70]]);
	});

	test('drops endpoints missing from the snapshot and hosts whose every URL vanished', () => {
		let state = recordTrend(
			EMPTY_TREND,
			snapshot([
				rule(
					'office',
					{},
					{
						listen: [hop({ 'http://hop-b.example:8080': { rate_up: 3 } })],
						forward: [hop({ [SOCKS_URL]: { rate_up: 1 }, [SSH_URL]: { rate_up: 2 } })]
					}
				)
			]),
			1000
		);
		expect([...state.hosts.keys()].sort()).toEqual(['hop-a.example', 'hop-b.example']);
		expect(hostRates(state, 'hop-b.example')).toEqual([[3, 0]]);
		state = recordTrend(
			state,
			snapshot([rule('office', {}, { forward: [hop({ [SOCKS_URL]: { rate_up: 4 } })] })]),
			2000
		);
		expect([...state.hosts.keys()]).toEqual(['hop-a.example']);
		expect([...state.hosts.get('hop-a.example')!.endpoints.keys()]).toEqual([
			displayURL(SOCKS_URL)
		]);
		expect(endpointRates(state, 'hop-a.example', displayURL(SOCKS_URL))).toEqual([
			[1, 0],
			[4, 0]
		]);
		expect(hostRates(state, 'hop-a.example')).toEqual([
			[3, 0],
			[4, 0]
		]);
	});

	test('a changed since or a long gap restarts the host and connection series too', () => {
		const rules = [
			rule(
				'office',
				{},
				{
					forward: [hop({ [SOCKS_URL]: { rate_up: 1 } })],
					connections: [connection(101, { rate_up: 2 })]
				}
			)
		];
		let state = recordTrend(EMPTY_TREND, snapshot(rules, 'a'), 1000);
		state = recordTrend(state, snapshot(rules, 'a'), 2000);
		expect(hostRates(state, 'hop-a.example')).toHaveLength(2);
		expect(endpointRates(state, 'hop-a.example', displayURL(SOCKS_URL))).toHaveLength(2);
		expect(connectionRates(state, 101)).toHaveLength(2);
		const reset = recordTrend(state, snapshot(rules, 'b'), 2100);
		expect(hostRates(reset, 'hop-a.example')).toEqual([[1, 0]]);
		expect(endpointRates(reset, 'hop-a.example', displayURL(SOCKS_URL))).toEqual([[1, 0]]);
		expect(connectionRates(reset, 101)).toEqual([[2, 0]]);
		state = recordTrend(reset, snapshot(rules, 'b'), 3000);
		expect(hostRates(state, 'hop-a.example')).toHaveLength(2);
		const restarted = recordTrend(state, snapshot(rules, 'b'), 3000 + TREND_GAP_MS + 1);
		expect(hostRates(restarted, 'hop-a.example')).toEqual([[1, 0]]);
		expect(endpointRates(restarted, 'hop-a.example', displayURL(SOCKS_URL))).toEqual([[1, 0]]);
		expect(connectionRates(restarted, 101)).toEqual([[2, 0]]);
	});
});

describe('trend store', () => {
	afterEach(() => {
		trend.reset();
		vi.useRealTimers();
	});

	test('record feeds of() and total; unknown names read as one frozen empty list; reset clears', () => {
		expect(trend.total).toEqual([]);
		expect(trend.of('office')).toEqual([]);
		expect(Object.isFrozen(trend.of('office'))).toBe(true);
		expect(trend.of('office')).toBe(trend.of('lab'));
		trend.record(snapshot([rule('office', { rate_up: 1, rate_down: 2 })]), 1000);
		trend.record(snapshot([rule('office', { rate_up: 3, rate_down: 4 })]), 2000);
		expect(trend.of('office')).toEqual([
			{ up: 1, down: 2 },
			{ up: 3, down: 4 }
		]);
		expect(trend.total).toEqual([
			{ up: 1, down: 2 },
			{ up: 3, down: 4 }
		]);
		trend.record(snapshot([rule('office', { rate_up: 5, rate_down: 6 })]), 2000 + 100);
		expect(trend.of('office')).toHaveLength(2);
		expect(trend.of('lab')).toEqual([]);
		trend.reset();
		expect(trend.total).toEqual([]);
		expect(trend.of('office')).toEqual([]);
	});

	test('record defaults now to Date.now()', () => {
		vi.useFakeTimers();
		trend.record(snapshot([rule('office', { rate_up: 1 })]));
		vi.advanceTimersByTime(1000);
		trend.record(snapshot([rule('office', { rate_up: 2 })]));
		vi.advanceTimersByTime(TREND_MIN_STEP_MS - 1);
		trend.record(snapshot([rule('office', { rate_up: 3 })]));
		expect(trend.of('office').map((sample) => sample.up)).toEqual([1, 2]);
		vi.advanceTimersByTime(1);
		trend.record(snapshot([rule('office', { rate_up: 4 })]));
		expect(trend.of('office').map((sample) => sample.up)).toEqual([1, 2, 4]);
	});

	test('ofHost, ofEndpoint and ofConnection share the frozen empty list and follow record and reset', () => {
		const empty = trend.of('nope');
		expect(trend.ofHost('nope')).toBe(empty);
		expect(trend.ofEndpoint('nope', 'x')).toBe(empty);
		expect(trend.ofConnection(999)).toBe(empty);
		const rules = (up: number) => [
			rule(
				'office',
				{},
				{
					forward: [hop({ [SOCKS_URL]: { rate_up: up } })],
					connections: [connection(101, { rate_up: up * 10 })]
				}
			)
		];
		trend.record(snapshot(rules(1)), 1000);
		trend.record(snapshot(rules(2)), 2000);
		const ups = (samples: readonly RateSample[]) => samples.map((sample) => sample.up);
		expect(ups(trend.ofHost('hop-a.example'))).toEqual([1, 2]);
		expect(ups(trend.ofEndpoint('hop-a.example', displayURL(SOCKS_URL)))).toEqual([1, 2]);
		expect(ups(trend.ofConnection(101))).toEqual([10, 20]);
		expect(trend.ofEndpoint('hop-a.example', 'nope')).toBe(empty);
		trend.reset();
		expect(trend.ofHost('hop-a.example')).toBe(empty);
		expect(trend.ofEndpoint('hop-a.example', displayURL(SOCKS_URL))).toBe(empty);
		expect(trend.ofConnection(101)).toBe(empty);
	});
});
