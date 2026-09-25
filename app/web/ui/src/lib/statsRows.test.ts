import { afterEach, describe, expect, test } from 'vitest';
import { i18n } from './i18n.svelte';
import {
	DEFAULT_RULE_SORT,
	RULE_SORT_KEYS,
	buildRows,
	filterRows,
	ruleSortLabel,
	sortRows,
	visibleTargets,
	type StatsRow
} from './statsRows';
import type { Rule, RuleStats, RuleStatus, Stats, Target } from './types';

afterEach(() => {
	i18n.dispose();
	localStorage.clear();
});

const ZERO: Stats = {
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
	avg_latency_ms: 0
};

const rule = (name: string): Rule => ({ name, listen: { host: '', port: 0 }, forward: {} });

const runtime = (name: string): RuleStatus => ({
	name,
	address: '127.0.0.1:1080',
	remote: false,
	running: true
});

const entry = (name: string, up = 0, stats: Partial<Stats> = {}): RuleStats => ({
	name,
	stats: { ...ZERO, up, ...stats },
	listen: null,
	forward: null,
	targets: null,
	connections: null,
	targets_evicted: 0
});

describe('buildRows', () => {
	test('keeps the configured order and joins runtime and snapshot by name', () => {
		const rules = [rule('c'), rule('a'), rule('b')];
		const runtimes = [runtime('b'), runtime('a')];
		const entries = [entry('a'), entry('c')];
		const rows = buildRows(rules, runtimes, entries);
		expect(rows.map((row) => row.name)).toEqual(['c', 'a', 'b']);
		expect(rows[0]).toEqual({ name: 'c', rule: rules[0], runtime: null, entry: entries[1] });
		expect(rows[1]).toEqual({ name: 'a', rule: rules[1], runtime: runtimes[1], entry: entries[0] });
		expect(rows[2]).toEqual({ name: 'b', rule: rules[2], runtime: runtimes[0], entry: null });
	});

	test('appends snapshot-only names after the configured ones, in snapshot order and deduplicated', () => {
		const rules = [rule('a')];
		const entries = [entry('z', 1), entry('a'), entry('y'), entry('z', 2), entry('a', 3)];
		const rows = buildRows(rules, [], entries);
		expect(rows.map((row) => row.name)).toEqual(['a', 'z', 'y']);
		// The first snapshot entry of a name wins, as Array.prototype.find would pick it.
		expect(rows.map((row) => row.entry)).toEqual([entries[1], entries[0], entries[2]]);
		expect(rows.map((row) => row.rule)).toEqual([rules[0], null, null]);
		expect(rows.map((row) => row.runtime)).toEqual([null, null, null]);
	});

	test('a rule missing from runtime and snapshot yields nulls; no config yields snapshot rows only', () => {
		expect(buildRows([rule('lonely')], [], [])).toEqual([
			{ name: 'lonely', rule: rule('lonely'), runtime: null, entry: null }
		]);
		const entries = [entry('b'), entry('a')];
		expect(buildRows(null, [runtime('a')], entries)).toEqual([
			{ name: 'b', rule: null, runtime: null, entry: entries[0] },
			{ name: 'a', rule: null, runtime: runtime('a'), entry: entries[1] }
		]);
		expect(buildRows(null, [], [])).toEqual([]);
	});

	test('names like Object.prototype members are looked up by value, not by property', () => {
		const names = ['toString', '__proto__', 'constructor', 'hasOwnProperty'];
		const rows = buildRows(
			names.map(rule),
			names.map(runtime),
			names.map((name) => entry(name))
		);
		expect(rows.map((row) => row.name)).toEqual(names);
		for (const row of rows) {
			expect(row.rule?.name).toBe(row.name);
			expect(row.runtime?.name).toBe(row.name);
			expect(row.entry?.name).toBe(row.name);
		}
		// Unknown prototype names do not resolve to inherited members either.
		expect(buildRows([rule('valueOf')], [], [])[0]).toEqual({
			name: 'valueOf',
			rule: rule('valueOf'),
			runtime: null,
			entry: null
		});
		expect(buildRows(null, [], [entry('__proto__')])[0].rule).toBeNull();
	});

	test('duplicate configured and runtime names keep every configured row and resolve to the first match', () => {
		const first = { ...rule('dup'), disabled: true };
		const second = rule('dup');
		const runtimes = [
			{ ...runtime('dup'), address: '127.0.0.1:1' },
			{ ...runtime('dup'), address: '127.0.0.1:2' }
		];
		const rows = buildRows([first, second, rule('other')], runtimes, [entry('dup', 7)]);
		// The configured list is rendered as-is, so a duplicated name yields two rows.
		expect(rows.map((row) => row.name)).toEqual(['dup', 'dup', 'other']);
		for (const row of rows.slice(0, 2)) {
			expect(row.rule).toBe(first);
			expect(row.runtime).toBe(runtimes[0]);
			expect(row.entry?.stats.up).toBe(7);
		}
		expect(rows[2]).toEqual({ name: 'other', rule: rule('other'), runtime: null, entry: null });
	});
});

describe('visibleTargets', () => {
	const target = (address: string, last_active?: string, via = ''): Target => ({
		address,
		via,
		stats: { ...ZERO, ...(last_active ? { last_active } : {}) }
	});

	test('orders by last_active descending, never-active ones last in their given order, without mutating the input', () => {
		const targets = [
			target('quiet-a'),
			target('old', '2026-09-19T08:00:00Z'),
			target('quiet-b'),
			target('new', '2026-09-19T09:00:00Z'),
			target('mid', '2026-09-19T08:30:00Z'),
			target('broken', 'not a time')
		];
		const before = targets.map((item) => item.address);
		const { shown, total } = visibleTargets(targets);
		expect(shown.map((item) => item.address)).toEqual([
			'new',
			'mid',
			'old',
			'quiet-a',
			'quiet-b',
			'broken'
		]);
		expect(total).toBe(6);
		expect(targets.map((item) => item.address)).toEqual(before);
		// Equal timestamps keep their order too.
		expect(
			visibleTargets([
				target('b', '2026-09-19T09:00:00Z'),
				target('a', '2026-09-19T09:00:00Z')
			]).shown.map((item) => item.address)
		).toEqual(['b', 'a']);
	});

	test('caps the list at 20 rows by default while reporting the full count', () => {
		const targets = Array.from({ length: 25 }, (_, index) =>
			target(`t${index}`, new Date(Date.UTC(2026, 8, 19, 8, index)).toISOString())
		);
		const { shown, total } = visibleTargets(targets);
		expect(shown).toHaveLength(20);
		expect(total).toBe(25);
		expect(shown[0].address).toBe('t24');
		expect(shown[19].address).toBe('t5');
		expect(visibleTargets(targets, 3).shown.map((item) => item.address)).toEqual([
			't24',
			't23',
			't22'
		]);
	});

	test('null and undefined lists are empty', () => {
		expect(visibleTargets(null)).toEqual({ shown: [], total: 0 });
		expect(visibleTargets(undefined)).toEqual({ shown: [], total: 0 });
	});
});

describe('rule sort and filter', () => {
	const row = (
		name: string,
		stats: Partial<Stats> | null = {},
		status: Partial<RuleStatus> = {}
	): StatsRow => ({
		name,
		rule: rule(name),
		runtime: { ...runtime(name), ...status },
		entry: stats === null ? null : entry(name, 0, stats)
	});
	const names = (rows: readonly StatsRow[]) => rows.map((item) => item.name);

	const rows = [
		row('rule-10', { down: 300, rate_down: 1, active: 2, last_down: '2026-09-19T08:00:00Z' }),
		row('rule-9', { down: 100, rate_down: 5, active: 0, dial_failures: 4 }),
		row('ghost', null),
		row('Alpha', { down: 200, rate_down: 5, active: 1, last_down: '2026-09-19T09:00:00Z' })
	];

	test('the sort menu lists configured order, name, the eight traffic keys download first, then connections and failures, labelled in the active language', () => {
		i18n.init();
		expect(DEFAULT_RULE_SORT).toEqual({ key: 'configured', direction: 'ascending' });
		expect(RULE_SORT_KEYS).toEqual([
			'configured',
			'name',
			'rate_down',
			'rate_up',
			'peak_rate_down',
			'peak_rate_up',
			'down',
			'up',
			'last_down',
			'last_up',
			'active',
			'dial_failures'
		]);
		expect(RULE_SORT_KEYS.map(ruleSortLabel)).toEqual([
			'Configured order',
			'Rule',
			'Download \u00b7 now',
			'Upload \u00b7 now',
			'Download \u00b7 peak',
			'Upload \u00b7 peak',
			'Download \u00b7 total',
			'Upload \u00b7 total',
			'Download \u00b7 last',
			'Upload \u00b7 last',
			'Connections',
			'Failures'
		]);
		i18n.setLanguage('zh');
		expect(ruleSortLabel('configured')).toBe('\u914d\u7f6e\u987a\u5e8f');
		expect(ruleSortLabel('down')).toBe('\u4e0b\u8f7d \u00b7 \u7d2f\u8ba1');
	});

	test('filterRows matches the name, the address and the target as a case-insensitive subsequence', () => {
		const addressOf = (item: StatsRow) => item.runtime?.address ?? '';
		const targetOf = (item: StatsRow) => item.runtime?.target ?? '';
		const list = [
			row('office', {}, { address: '127.0.0.1:18097' }),
			row('mirror', {}, { address: '127.0.0.1:18098', target: '10.0.0.5:5432' }),
			row('lab', null, { address: 'virtual://exit' })
		];
		expect(names(filterRows(list, '', 'en', addressOf, targetOf))).toEqual([
			'office',
			'mirror',
			'lab'
		]);
		expect(names(filterRows(list, 'OFF', 'en', addressOf, targetOf))).toEqual(['office']);
		expect(names(filterRows(list, '1809', 'en', addressOf, targetOf))).toEqual([
			'office',
			'mirror'
		]);
		expect(names(filterRows(list, '5432', 'en', addressOf, targetOf))).toEqual(['mirror']);
		expect(names(filterRows(list, 'vexit', 'en', addressOf, targetOf))).toEqual(['lab']);
		expect(names(filterRows(list, 'zzz', 'en', addressOf, targetOf))).toEqual([]);
	});

	test('configured order is the given order or its reverse', () => {
		expect(names(sortRows(rows, DEFAULT_RULE_SORT, 'en'))).toEqual([
			'rule-10',
			'rule-9',
			'ghost',
			'Alpha'
		]);
		expect(names(sortRows(rows, { key: 'configured', direction: 'descending' }, 'en'))).toEqual([
			'Alpha',
			'ghost',
			'rule-9',
			'rule-10'
		]);
		expect(sortRows(rows, DEFAULT_RULE_SORT, 'en')).not.toBe(rows);
		expect(names(rows)).toEqual(['rule-10', 'rule-9', 'ghost', 'Alpha']);
	});

	test('name sorts by locale with numeric collation, case-insensitively', () => {
		expect(names(sortRows(rows, { key: 'name', direction: 'ascending' }, 'en'))).toEqual([
			'Alpha',
			'ghost',
			'rule-9',
			'rule-10'
		]);
		expect(names(sortRows(rows, { key: 'name', direction: 'descending' }, 'en'))).toEqual([
			'rule-10',
			'rule-9',
			'ghost',
			'Alpha'
		]);
	});

	test('traffic, connection and failure keys sort by the snapshot values; rows without an entry count as zero and ties keep the configured order in both directions', () => {
		expect(names(sortRows(rows, { key: 'down', direction: 'descending' }, 'en'))).toEqual([
			'rule-10',
			'Alpha',
			'rule-9',
			'ghost'
		]);
		expect(names(sortRows(rows, { key: 'down', direction: 'ascending' }, 'en'))).toEqual([
			'ghost',
			'rule-9',
			'Alpha',
			'rule-10'
		]);
		// rule-9 and Alpha tie on rate_down: configured order either way.
		expect(names(sortRows(rows, { key: 'rate_down', direction: 'descending' }, 'en'))).toEqual([
			'rule-9',
			'Alpha',
			'rule-10',
			'ghost'
		]);
		expect(names(sortRows(rows, { key: 'rate_down', direction: 'ascending' }, 'en'))).toEqual([
			'ghost',
			'rule-10',
			'rule-9',
			'Alpha'
		]);
		expect(names(sortRows(rows, { key: 'active', direction: 'descending' }, 'en'))).toEqual([
			'rule-10',
			'Alpha',
			'rule-9',
			'ghost'
		]);
		expect(names(sortRows(rows, { key: 'dial_failures', direction: 'descending' }, 'en'))).toEqual([
			'rule-9',
			'rule-10',
			'ghost',
			'Alpha'
		]);
		// Timestamps compare as times; a missing one is the oldest.
		expect(names(sortRows(rows, { key: 'last_down', direction: 'descending' }, 'en'))).toEqual([
			'Alpha',
			'rule-10',
			'rule-9',
			'ghost'
		]);
	});
});
