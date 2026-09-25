import { describe, expect, test } from 'vitest';
import { buildRows } from './statsRows';
import type { Rule, RuleStats, RuleStatus, Stats } from './types';

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

const entry = (name: string, up = 0): RuleStats => ({
	name,
	stats: { ...ZERO, up },
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
