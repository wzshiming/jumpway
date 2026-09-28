import { afterEach, describe, expect, test, vi } from 'vitest';
import { statsOf } from '../../e2e/fixtures/api';
import { sumStats } from './hosts';
import {
	EMPTY_TREND,
	TREND_GAP_MS,
	TREND_MIN_STEP_MS,
	TREND_SAMPLES,
	TREND_TOTAL,
	recordTrend,
	type TrendState
} from './trend';
import { trend } from './trend.svelte';
import type { RuleStats, Snapshot, Stats } from './types';

const rule = (name: string, stats: Partial<Stats>): RuleStats => ({
	name,
	stats: statsOf(stats),
	listen: null,
	forward: null,
	targets: null,
	connections: null,
	targets_evicted: 0
});

const snapshot = (rules: RuleStats[] | null, since = 'since-1'): Snapshot => ({ since, rules });

const rates = (state: TrendState, name: string) =>
	state.series.get(name)?.map((sample) => [sample.up, sample.down]);

const names = (state: TrendState) => [...state.series.keys()].sort();

describe('recordTrend', () => {
	test('starts empty', () => {
		expect(EMPTY_TREND).toEqual({ since: null, lastAt: null, series: new Map() });
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
});
