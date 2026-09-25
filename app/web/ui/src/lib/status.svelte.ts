import { ApiError, configsApi } from './api';
import { createPoller } from './polling.svelte';
import type { Rule, RuleStatus, Status } from './types';

export const STATUS_INTERVAL_MS = 10_000;
export const STATUS_MAX_INTERVAL_MS = 60_000;

export const status = createPoller<Status>({
	load: (signal) => configsApi.status(signal),
	intervalMs: STATUS_INTERVAL_MS,
	maxIntervalMs: STATUS_MAX_INTERVAL_MS
});

export type RuntimeState = 'running' | 'stopped' | 'unknown';
export type RuleState = 'running' | 'retrying' | 'stopped';
export type RuleCardState = RuleState | 'unknown' | 'disabled';

// Unreachable overrides a stale snapshot: the dot goes back to unknown, as before.
export function runtimeState(): RuntimeState {
	if (status.error instanceof ApiError && status.error.unreachable) return 'unknown';
	const running = status.data?.running;
	return running === true ? 'running' : running === false ? 'stopped' : 'unknown';
}

export const ruleState = (rule: RuleStatus): RuleState =>
	rule.running ? 'running' : (rule.attempt ?? 0) > 0 ? 'retrying' : 'stopped';

// The configuration wins over a stale runtime entry: a disabled rule is disabled, whatever it says.
export function cardState(rule: Pick<Rule, 'disabled'>, runtime: RuleStatus | null): RuleCardState {
	if (rule.disabled) return 'disabled';
	if (!runtime) return 'unknown';
	return ruleState(runtime);
}

export interface RuleStateCounts {
	total: number;
	running: number;
	retrying: number;
	stopped: number;
	unknown: number;
	disabled: number;
}

// Runtime entries of rules that are not configured (e.g. just deleted) are not counted.
export function countRuleStates(
	rules: readonly Rule[] | null,
	runtime: readonly RuleStatus[] | null | undefined
): RuleStateCounts {
	const counts: RuleStateCounts = {
		total: 0,
		running: 0,
		retrying: 0,
		stopped: 0,
		unknown: 0,
		disabled: 0
	};
	const entries = runtime ?? [];
	const states = rules
		? rules.map((rule) =>
				cardState(rule, entries.find((entry) => entry.name === rule.name) ?? null)
			)
		: entries.map(ruleState);
	for (const state of states) counts[state]++;
	counts.total = states.length;
	return counts;
}
