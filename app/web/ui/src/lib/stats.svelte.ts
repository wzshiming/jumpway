import { statsApi } from './api';
import { busy } from './busy.svelte';
import { createPoller } from './polling.svelte';
import type { Snapshot } from './types';

export const STATS_INTERVAL_MS = 1_000;

export const stats = createPoller<Snapshot>({
	load: (signal) => statsApi.get(signal),
	intervalMs: STATS_INTERVAL_MS
});

export async function resetStats(): Promise<void> {
	try {
		await busy.run(() => statsApi.reset());
	} finally {
		await stats.refresh();
	}
}

export async function disconnectConnection(id: number): Promise<void> {
	try {
		await statsApi.disconnect(id);
	} finally {
		await stats.refresh();
	}
}
