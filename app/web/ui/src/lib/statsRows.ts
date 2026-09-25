import type { Rule, RuleStats, RuleStatus } from './types';

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
