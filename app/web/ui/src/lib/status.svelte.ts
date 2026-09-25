import { ApiError, configsApi } from './api';
import { createPoller } from './polling.svelte';
import type { RuleStatus, Status } from './types';

export const STATUS_INTERVAL_MS = 10_000;
export const STATUS_MAX_INTERVAL_MS = 60_000;

export const status = createPoller<Status>({
	load: (signal) => configsApi.status(signal),
	intervalMs: STATUS_INTERVAL_MS,
	maxIntervalMs: STATUS_MAX_INTERVAL_MS
});

export type RuntimeState = 'running' | 'stopped' | 'unknown';
export type RuleState = 'running' | 'retrying' | 'stopped';

// Unreachable overrides a stale snapshot: the dot goes back to unknown, as before.
export function runtimeState(): RuntimeState {
	if (status.error instanceof ApiError && status.error.unreachable) return 'unknown';
	const running = status.data?.running;
	return running === true ? 'running' : running === false ? 'stopped' : 'unknown';
}

export const ruleState = (rule: RuleStatus): RuleState =>
	rule.running ? 'running' : (rule.attempt ?? 0) > 0 ? 'retrying' : 'stopped';
