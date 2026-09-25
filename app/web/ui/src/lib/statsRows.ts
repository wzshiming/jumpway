import { text } from './format';
import { t } from './i18n.svelte';
import { fuzzyMatch, type ListSort } from './search';
import { TRAFFIC_SORT_KEYS, trafficSortLabel, type TrafficKey } from './traffic';
import {
	list,
	type Nullable,
	type Rule,
	type RuleStats,
	type RuleStatus,
	type Target
} from './types';

export interface StatsRow {
	name: string;
	rule: Rule | null;
	runtime: RuleStatus | null;
	entry: RuleStats | null;
}

// Maps: a rule named "toString" or "__proto__" must not resolve to an Object.prototype member.
function byName<T extends { name: string }>(entries: readonly T[]): Map<string, T> {
	const map = new Map<string, T>();
	for (const entry of entries) if (!map.has(entry.name)) map.set(entry.name, entry);
	return map;
}

// Configured rules first; a snapshot may still name a rule the config no longer has.
export function buildRows(
	rules: Rule[] | null,
	runtime: readonly RuleStatus[],
	snapshot: readonly RuleStats[]
): StatsRow[] {
	const names = rules ? rules.map((rule) => rule.name) : [];
	const seen = new Set(names);
	for (const entry of snapshot) {
		if (!seen.has(entry.name)) {
			seen.add(entry.name);
			names.push(entry.name);
		}
	}
	const ruleOf = byName(rules ?? []);
	const runtimeOf = byName(runtime);
	const entryOf = byName(snapshot);
	return names.map((name) => ({
		name,
		rule: ruleOf.get(name) ?? null,
		runtime: runtimeOf.get(name) ?? null,
		entry: entryOf.get(name) ?? null
	}));
}

const activeAt = (target: Target): number => {
	const timestamp = Date.parse(text(target.stats?.last_active));
	return Number.isFinite(timestamp) ? timestamp : -Infinity;
};

// The most recently active targets first (never active last, in their given order), at most `limit`.
export function visibleTargets(
	targets: Nullable<Target[]>,
	limit = 20
): { shown: Target[]; total: number } {
	const all = list(targets);
	const shown = all
		.slice()
		.sort((left, right) => {
			const [before, after] = [activeAt(left), activeAt(right)];
			return before === after ? 0 : before < after ? 1 : -1;
		})
		.slice(0, limit);
	return { shown, total: all.length };
}

export type RuleSortKey = 'configured' | 'name' | 'active' | 'dial_failures' | TrafficKey;
export type RuleSort = ListSort<RuleSortKey>;

export const DEFAULT_RULE_SORT: RuleSort = { key: 'configured', direction: 'ascending' };

// The sort options in menu order.
export const RULE_SORT_KEYS: readonly RuleSortKey[] = [
	'configured',
	'name',
	...TRAFFIC_SORT_KEYS,
	'active',
	'dial_failures'
];

export function ruleSortLabel(key: RuleSortKey): string {
	switch (key) {
		case 'configured':
			return t('sortConfigured');
		case 'name':
			return t('ruleLabel');
		case 'active':
			return t('connections');
		case 'dial_failures':
			return t('dialFailures');
	}
	return trafficSortLabel(key);
}

export function filterRows(
	rows: readonly StatsRow[],
	query: string,
	language: string,
	addressOf: (row: StatsRow) => string,
	targetOf: (row: StatsRow) => string
): StatsRow[] {
	return rows.filter((row) =>
		fuzzyMatch(query, [row.name, addressOf(row), targetOf(row)], language)
	);
}

const TIME_KEYS = new Set<RuleSortKey>(['last_up', 'last_down']);

// Rows without a snapshot entry count as zero; ties keep the configured order in either direction.
export function sortRows(rows: readonly StatsRow[], sort: RuleSort, language: string): StatsRow[] {
	const direction = sort.direction === 'ascending' ? 1 : -1;
	if (sort.key === 'configured') return direction > 0 ? rows.slice() : rows.slice().reverse();
	const key = sort.key;
	const compare =
		key === 'name'
			? (left: StatsRow, right: StatsRow) =>
					left.name.localeCompare(right.name, language, { numeric: true })
			: TIME_KEYS.has(key)
				? (left: StatsRow, right: StatsRow) =>
						(Date.parse(text(left.entry?.stats[key])) || 0) -
						(Date.parse(text(right.entry?.stats[key])) || 0)
				: (left: StatsRow, right: StatsRow) =>
						(Number(left.entry?.stats[key]) || 0) - (Number(right.entry?.stats[key]) || 0);
	return rows.slice().sort((left, right) => compare(left, right) * direction);
}
