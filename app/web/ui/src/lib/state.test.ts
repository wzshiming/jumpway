import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { rulesFixture, statusFixture } from '../../e2e/fixtures/api';
import { ApiError } from './api';
import { busy } from './busy.svelte';
import { confirm, confirmService } from './confirm';
import { createPoller, type Poller } from './polling.svelte';
import { disconnectConnection, resetStats, stats } from './stats.svelte';
import { cardState, countRuleStates, ruleState, runtimeState, status } from './status.svelte';
import { THEME_STORAGE_KEY, theme } from './theme.svelte';
import type { RuleStatus } from './types';
import { SUCCESS_TOAST_MS, toasts } from './toast.svelte';

function setVisibility(state: 'visible' | 'hidden') {
	Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
	document.dispatchEvent(new Event('visibilitychange'));
}

type Pending<T> = {
	resolve: (value: T) => void;
	reject: (error: unknown) => void;
	signal: AbortSignal;
};

function fakeLoader<T>() {
	const pending: Pending<T>[] = [];
	const load = vi.fn(
		(signal: AbortSignal) =>
			new Promise<T>((resolve, reject) => {
				pending.push({ resolve, reject, signal });
				signal.addEventListener('abort', () => reject(new ApiError('aborted', 'aborted', null)));
			})
	);
	return { load, pending };
}

const tick = () => vi.advanceTimersByTimeAsync(0);

beforeEach(() => {
	vi.useFakeTimers();
});

afterEach(() => {
	delete (document as { visibilityState?: unknown }).visibilityState;
	vi.useRealTimers();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
	toasts.clear();
	localStorage.clear();
});

describe('createPoller', () => {
	test('loads on first subscribe, repeats every interval and stops with the last subscriber', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		expect(poller.active).toBe(false);
		const unsubscribe = poller.subscribe();
		const other = poller.subscribe();
		expect(load).toHaveBeenCalledTimes(1);
		expect(poller.loading).toBe(true);
		pending[0].resolve(1);
		await tick();
		expect(poller.data).toBe(1);
		expect(poller.loading).toBe(false);
		await vi.advanceTimersByTimeAsync(999);
		expect(load).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(load).toHaveBeenCalledTimes(2);
		other();
		pending[1].resolve(2);
		await vi.advanceTimersByTimeAsync(1000);
		expect(load).toHaveBeenCalledTimes(3);
		unsubscribe();
		expect(pending[2].signal.aborted).toBe(true);
		expect(poller.active).toBe(false);
		await vi.advanceTimersByTimeAsync(5000);
		expect(load).toHaveBeenCalledTimes(3);
		expect(poller.data).toBe(2);
		expect(poller.error).toBeNull();
	});

	test('a response that arrives after unsubscribe is dropped', async () => {
		const { load, pending } = fakeLoader<string>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		unsubscribe();
		pending[0].resolve('late');
		await tick();
		expect(poller.data).toBeNull();
		expect(poller.loading).toBe(false);
		expect(load).toHaveBeenCalledTimes(1);
	});

	test('busy skips ticks without aborting an in-flight request and resumes afterwards', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		let release!: () => void;
		const task = busy.run(() => new Promise<void>((resolve) => (release = resolve)));
		await vi.advanceTimersByTimeAsync(1000);
		expect(pending[0].signal.aborted).toBe(false);
		pending[0].resolve(7);
		await tick();
		expect(poller.data).toBe(7);
		await vi.advanceTimersByTimeAsync(3000);
		expect(load).toHaveBeenCalledTimes(1);
		release();
		await task;
		await vi.advanceTimersByTimeAsync(1000);
		expect(load).toHaveBeenCalledTimes(2);
		unsubscribe();
	});

	test('hiding the document aborts the request and pauses; showing it refreshes at once', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		setVisibility('hidden');
		expect(pending[0].signal.aborted).toBe(true);
		await vi.advanceTimersByTimeAsync(5000);
		expect(load).toHaveBeenCalledTimes(1);
		expect(poller.error).toBeNull();
		setVisibility('visible');
		expect(load).toHaveBeenCalledTimes(2);
		pending[1].resolve(3);
		await vi.advanceTimersByTimeAsync(1000);
		expect(load).toHaveBeenCalledTimes(3);
		unsubscribe();
	});

	test('never overlaps requests: a slow response delays the next tick', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		await vi.advanceTimersByTimeAsync(3500);
		expect(load).toHaveBeenCalledTimes(1);
		pending[0].resolve(1);
		await tick();
		expect(load).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(1000);
		expect(load).toHaveBeenCalledTimes(2);
		unsubscribe();
	});

	test('errors are exposed, keep the last data and clear on the next success', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		pending[0].resolve(1);
		await vi.advanceTimersByTimeAsync(1000);
		const failure = new ApiError('network', 'Failed to fetch', null);
		pending[1].reject(failure);
		await tick();
		expect(poller.error).toBe(failure);
		expect(poller.data).toBe(1);
		await vi.advanceTimersByTimeAsync(1000);
		pending[2].resolve(2);
		await tick();
		expect(poller.error).toBeNull();
		expect(poller.data).toBe(2);
		unsubscribe();
	});

	test('refresh waits for the in-flight request, then fetches again and returns the fresh value', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		const refreshed = poller.refresh();
		await tick();
		expect(load).toHaveBeenCalledTimes(1);
		pending[0].resolve(1);
		await tick();
		expect(load).toHaveBeenCalledTimes(2);
		pending[1].resolve(2);
		await expect(refreshed).resolves.toBe(2);
		expect(poller.data).toBe(2);

		const failing = poller.refresh();
		await tick();
		pending[2].reject(new ApiError('http', 'boom', 500));
		await expect(failing).resolves.toBeNull();
		expect(poller.error).toMatchObject({ message: 'boom' });
		unsubscribe();
	});

	test('a refresh queued behind an in-flight request is dropped once the last subscriber leaves', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		const refreshed = poller.refresh();
		await tick();
		expect(load).toHaveBeenCalledTimes(1);
		unsubscribe();
		await tick();
		expect(load).toHaveBeenCalledTimes(1);
		await expect(refreshed).resolves.toBeNull();
		pending[0].resolve(1);
		await tick();
		expect(poller.data).toBeNull();
		expect(poller.loading).toBe(false);
	});

	test('a refresh queued behind an in-flight request is dropped while the document is hidden', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		const refreshed = poller.refresh();
		await tick();
		setVisibility('hidden');
		await tick();
		expect(load).toHaveBeenCalledTimes(1);
		await expect(refreshed).resolves.toBeNull();
		expect(pending[0].signal.aborted).toBe(true);
		expect(poller.data).toBeNull();
		setVisibility('visible');
		expect(load).toHaveBeenCalledTimes(2);
		unsubscribe();
	});

	test('a refresh queued behind an in-flight request is skipped while busy; polling resumes later', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		const unsubscribe = poller.subscribe();
		const refreshed = poller.refresh();
		await tick();
		let release!: () => void;
		const task = busy.run(() => new Promise<void>((resolve) => (release = resolve)));
		pending[0].resolve(1);
		await tick();
		expect(load).toHaveBeenCalledTimes(1);
		await expect(refreshed).resolves.toBeNull();
		expect(poller.data).toBe(1);
		release();
		await task;
		await vi.advanceTimersByTimeAsync(1000);
		expect(load).toHaveBeenCalledTimes(2);
		unsubscribe();
	});

	test('refresh without subscribers or while busy does not load', async () => {
		const { load } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		await expect(poller.refresh()).resolves.toBeNull();
		expect(load).not.toHaveBeenCalled();
		expect(poller.loading).toBe(false);
		const unsubscribe = poller.subscribe();
		expect(load).toHaveBeenCalledTimes(1);
		let release!: () => void;
		const task = busy.run(() => new Promise<void>((resolve) => (release = resolve)));
		await expect(poller.refresh()).resolves.toBeNull();
		expect(load).toHaveBeenCalledTimes(1);
		release();
		await task;
		unsubscribe();
	});

	test('numbers every load and tags data with the load that produced it; dropped and failed loads leave the tag alone', async () => {
		const { load, pending } = fakeLoader<number>();
		const poller = createPoller({ load, intervalMs: 1000 });
		expect(poller.requested).toBe(0);
		expect(poller.answered).toBe(0);
		const unsubscribe = poller.subscribe();
		expect(poller.requested).toBe(1);
		expect(poller.answered).toBe(0);
		pending[0].resolve(1);
		await tick();
		expect(poller.answered).toBe(1);
		await vi.advanceTimersByTimeAsync(1000);
		expect(poller.requested).toBe(2);
		pending[1].reject(new ApiError('network', 'Failed to fetch', null));
		await tick();
		expect(poller.answered).toBe(1);
		await vi.advanceTimersByTimeAsync(1000);
		expect(poller.requested).toBe(3);
		setVisibility('hidden');
		pending[2].resolve(3);
		await tick();
		expect(poller.answered).toBe(1);
		expect(poller.data).toBe(1);
		setVisibility('visible');
		expect(poller.requested).toBe(4);
		// A refresh is a load like any other: what it returns is what `answered` then points at.
		const refreshed = poller.refresh();
		pending[3].resolve(4);
		await tick();
		expect(poller.answered).toBe(4);
		expect(poller.requested).toBe(5);
		pending[4].resolve(5);
		await expect(refreshed).resolves.toBe(5);
		expect(poller.answered).toBe(5);
		unsubscribe();
	});

	describe('backoff', () => {
		const failure = new ApiError('network', 'Failed to fetch', null);

		// Fails the load under way and checks that the next one starts exactly `wait` ms later.
		async function failThenWait(
			poller: Poller<number>,
			fake: ReturnType<typeof fakeLoader<number>>,
			wait: number
		) {
			const started = fake.load.mock.calls.length;
			fake.pending[started - 1].reject(failure);
			await tick();
			expect(poller.error).toBe(failure);
			await vi.advanceTimersByTimeAsync(wait - 1);
			expect(fake.load).toHaveBeenCalledTimes(started);
			await vi.advanceTimersByTimeAsync(1);
			expect(fake.load).toHaveBeenCalledTimes(started + 1);
		}

		test('consecutive failures double the delay up to maxIntervalMs', async () => {
			const fake = fakeLoader<number>();
			const poller = createPoller({ load: fake.load, intervalMs: 1000, maxIntervalMs: 16_000 });
			const unsubscribe = poller.subscribe();
			for (const [index, wait] of [2000, 4000, 8000, 16_000, 16_000].entries()) {
				await failThenWait(poller, fake, wait);
				expect(poller.failures).toBe(index + 1);
			}
			unsubscribe();
		});

		test('a success resets the delay to intervalMs', async () => {
			const fake = fakeLoader<number>();
			const poller = createPoller({ load: fake.load, intervalMs: 1000, maxIntervalMs: 16_000 });
			const unsubscribe = poller.subscribe();
			await failThenWait(poller, fake, 2000);
			await failThenWait(poller, fake, 4000);
			expect(poller.failures).toBe(2);
			fake.pending[2].resolve(1);
			await tick();
			expect(poller.failures).toBe(0);
			expect(poller.error).toBeNull();
			await vi.advanceTimersByTimeAsync(999);
			expect(fake.load).toHaveBeenCalledTimes(3);
			await vi.advanceTimersByTimeAsync(1);
			expect(fake.load).toHaveBeenCalledTimes(4);
			unsubscribe();
		});

		test('refresh loads at once and restarts the backoff from its own outcome', async () => {
			const fake = fakeLoader<number>();
			const poller = createPoller({ load: fake.load, intervalMs: 1000, maxIntervalMs: 16_000 });
			const unsubscribe = poller.subscribe();
			await failThenWait(poller, fake, 2000);
			await failThenWait(poller, fake, 4000);
			fake.pending[2].reject(failure);
			await tick();
			expect(poller.failures).toBe(3);
			const refreshed = poller.refresh();
			expect(fake.load).toHaveBeenCalledTimes(4);
			expect(poller.failures).toBe(0);
			fake.pending[3].reject(failure);
			await expect(refreshed).resolves.toBeNull();
			expect(poller.failures).toBe(1);
			// One failure since the retry: the next poll is 2 s away, not 16 s.
			await vi.advanceTimersByTimeAsync(1999);
			expect(fake.load).toHaveBeenCalledTimes(4);
			await vi.advanceTimersByTimeAsync(1);
			expect(fake.load).toHaveBeenCalledTimes(5);
			unsubscribe();
		});

		test('showing the document again restarts the backoff', async () => {
			const fake = fakeLoader<number>();
			const poller = createPoller({ load: fake.load, intervalMs: 1000, maxIntervalMs: 4000 });
			const unsubscribe = poller.subscribe();
			await failThenWait(poller, fake, 2000);
			await failThenWait(poller, fake, 4000);
			fake.pending[2].reject(failure);
			await tick();
			expect(poller.failures).toBe(3);
			setVisibility('hidden');
			await vi.advanceTimersByTimeAsync(10_000);
			expect(fake.load).toHaveBeenCalledTimes(3);
			setVisibility('visible');
			expect(fake.load).toHaveBeenCalledTimes(4);
			expect(poller.failures).toBe(0);
			await failThenWait(poller, fake, 2000);
			expect(poller.failures).toBe(1);
			unsubscribe();
		});

		test('without maxIntervalMs failures keep the fixed cadence', async () => {
			const fake = fakeLoader<number>();
			const poller = createPoller({ load: fake.load, intervalMs: 1000 });
			const unsubscribe = poller.subscribe();
			for (let index = 0; index < 3; index++) {
				await failThenWait(poller, fake, 1000);
				expect(poller.failures).toBe(index + 1);
			}
			unsubscribe();
		});

		test('an aborted load is not a failure', async () => {
			const fake = fakeLoader<number>();
			const poller = createPoller({ load: fake.load, intervalMs: 1000, maxIntervalMs: 16_000 });
			const unsubscribe = poller.subscribe();
			fake.pending[0].reject(new ApiError('aborted', 'aborted', null));
			await tick();
			expect(poller.failures).toBe(0);
			expect(poller.error).toBeNull();
			await vi.advanceTimersByTimeAsync(999);
			expect(fake.load).toHaveBeenCalledTimes(1);
			await vi.advanceTimersByTimeAsync(1);
			expect(fake.load).toHaveBeenCalledTimes(2);
			unsubscribe();
		});
	});
});

describe('status and stats stores', () => {
	test('status polls /apis/configs/status every 10 s and derives runtime states', async () => {
		const fetchMock = vi.fn((url: string) =>
			Promise.resolve(
				new Response(
					JSON.stringify({
						address: '127.0.0.1:1088',
						running: true,
						rules: [
							{ name: 'a', address: ':1', remote: false, running: true },
							{ name: 'b', address: ':2', remote: true, running: false, attempt: 3 },
							{ name: 'c', address: ':3', remote: false, running: false }
						]
					}),
					{ status: 200, headers: { 'X-Url': url } }
				)
			)
		);
		vi.stubGlobal('fetch', fetchMock);
		expect(runtimeState()).toBe('unknown');
		const unsubscribe = status.subscribe();
		await tick();
		expect(fetchMock.mock.calls[0][0]).toBe('/apis/configs/status');
		expect(runtimeState()).toBe('running');
		expect(status.data?.rules?.map(ruleState)).toEqual(['running', 'retrying', 'stopped']);
		await vi.advanceTimersByTimeAsync(9_999);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(fetchMock).toHaveBeenCalledTimes(2);
		unsubscribe();
	});

	test('runtimeState is unknown while JumpWay is unreachable', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(() => Promise.reject(new TypeError('Failed to fetch')))
		);
		const unsubscribe = status.subscribe();
		await tick();
		expect(status.error).toMatchObject({ unreachable: true });
		expect(runtimeState()).toBe('unknown');
		unsubscribe();
	});

	const runtime = (partial: Partial<RuleStatus>): RuleStatus => ({
		name: 'x',
		address: ':1',
		remote: false,
		running: false,
		...partial
	});

	test('cardState: disabled wins over the runtime, no runtime is unknown, otherwise the rule state', () => {
		expect(cardState({ disabled: true }, runtime({ running: true }))).toBe('disabled');
		expect(cardState({ disabled: true }, null)).toBe('disabled');
		expect(cardState({}, null)).toBe('unknown');
		expect(cardState({ disabled: false }, runtime({ running: true }))).toBe('running');
		expect(cardState({}, runtime({ running: false, attempt: 2 }))).toBe('retrying');
		expect(cardState({}, runtime({ running: false }))).toBe('stopped');
		expect(cardState({}, runtime({ running: false, attempt: 0 }))).toBe('stopped');
	});

	test('countRuleStates counts every configured rule once by its card state', () => {
		expect(countRuleStates(rulesFixture, statusFixture.rules)).toEqual({
			total: 4,
			running: 2,
			retrying: 1,
			stopped: 0,
			unknown: 0,
			disabled: 1
		});
	});

	test('countRuleStates falls back to the runtime list while the rules are unknown', () => {
		// lab is listed running:false without an attempt: as an enabled rule it counts as stopped.
		expect(countRuleStates(null, statusFixture.rules)).toEqual({
			total: 4,
			running: 2,
			retrying: 1,
			stopped: 1,
			unknown: 0,
			disabled: 0
		});
		const zeros = { total: 0, running: 0, retrying: 0, stopped: 0, unknown: 0, disabled: 0 };
		expect(countRuleStates(null, null)).toEqual(zeros);
		expect(countRuleStates(null, undefined)).toEqual(zeros);
		expect(countRuleStates([], statusFixture.rules)).toEqual(zeros);
	});

	test('countRuleStates ignores status entries of rules that are no longer configured', () => {
		const withoutOffice = rulesFixture.filter((rule) => rule.name !== 'office');
		expect(countRuleStates(withoutOffice, statusFixture.rules)).toEqual({
			total: 3,
			running: 1,
			retrying: 1,
			stopped: 0,
			unknown: 0,
			disabled: 1
		});
	});

	test('countRuleStates: an enabled rule missing from the status is unknown; without a status all enabled rules are', () => {
		const withoutMirror = statusFixture.rules!.filter((entry) => entry.name !== 'mirror');
		expect(countRuleStates(rulesFixture, withoutMirror)).toEqual({
			total: 4,
			running: 1,
			retrying: 1,
			stopped: 0,
			unknown: 1,
			disabled: 1
		});
		expect(countRuleStates(rulesFixture, null)).toEqual({
			total: 4,
			running: 0,
			retrying: 0,
			stopped: 0,
			unknown: 3,
			disabled: 1
		});
	});

	test('status and stats back off while unreachable: 10 → 20 s and 1 → 2 s', async () => {
		const fetchMock = vi.fn(() => Promise.reject(new TypeError('Failed to fetch')));
		vi.stubGlobal('fetch', fetchMock);
		const unsubscribe = status.subscribe();
		await tick();
		expect(status.failures).toBe(1);
		await vi.advanceTimersByTimeAsync(19_999);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(fetchMock).toHaveBeenCalledTimes(2);
		unsubscribe();

		fetchMock.mockClear();
		const unsubscribeStats = stats.subscribe();
		await tick();
		expect(stats.failures).toBe(1);
		await vi.advanceTimersByTimeAsync(1999);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(fetchMock).toHaveBeenCalledTimes(2);
		unsubscribeStats();
	});

	test('stats polls /apis/stats every second; reset and disconnect refresh afterwards', async () => {
		const calls: [string, string | undefined][] = [];
		vi.stubGlobal(
			'fetch',
			vi.fn((url: string, init: RequestInit) => {
				calls.push([init.method ?? 'GET', url]);
				return Promise.resolve(
					new Response(JSON.stringify({ since: 's', rules: null }), { status: 200 })
				);
			})
		);
		const unsubscribe = stats.subscribe();
		await tick();
		expect(calls).toEqual([['GET', '/apis/stats']]);
		await vi.advanceTimersByTimeAsync(1000);
		expect(calls).toHaveLength(2);
		calls.length = 0;
		await resetStats();
		expect(calls).toEqual([
			['DELETE', '/apis/stats'],
			['GET', '/apis/stats']
		]);
		calls.length = 0;
		await disconnectConnection(42);
		expect(calls).toEqual([
			['DELETE', '/apis/stats/connections/42'],
			['GET', '/apis/stats']
		]);
		unsubscribe();
	});
});

describe('theme', () => {
	function fakeMatchMedia(matches: boolean) {
		const listeners = new Set<(event: { matches: boolean }) => void>();
		const query = {
			matches,
			media: '(prefers-color-scheme: dark)',
			addEventListener: (_: string, listener: (event: { matches: boolean }) => void) =>
				listeners.add(listener),
			removeEventListener: (_: string, listener: (event: { matches: boolean }) => void) =>
				listeners.delete(listener)
		};
		vi.stubGlobal(
			'matchMedia',
			vi.fn(() => query)
		);
		return {
			listeners,
			set(next: boolean) {
				query.matches = next;
				listeners.forEach((listener) => listener({ matches: next }));
			}
		};
	}

	test('defaults to system, follows the media query and persists explicit choices', () => {
		const media = fakeMatchMedia(true);
		const dispose = theme.init();
		expect(theme.value).toBe('system');
		expect(theme.resolved).toBe('dark');
		expect(document.documentElement.dataset.theme).toBe('dark');
		media.set(false);
		expect(document.documentElement.dataset.theme).toBe('light');
		theme.set('dark');
		expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');
		media.set(true);
		media.set(false);
		expect(theme.resolved).toBe('dark');
		expect(document.documentElement.dataset.theme).toBe('dark');
		dispose();
		expect(media.listeners.size).toBe(0);
	});

	test('restores a stored theme, ignores junk and survives storage denial', () => {
		localStorage.setItem(THEME_STORAGE_KEY, 'light');
		fakeMatchMedia(true);
		let dispose = theme.init();
		expect(theme.value).toBe('light');
		expect(document.documentElement.dataset.theme).toBe('light');
		dispose();

		localStorage.setItem(THEME_STORAGE_KEY, 'neon');
		dispose = theme.init();
		expect(theme.value).toBe('system');
		dispose();

		vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
			throw new DOMException('denied', 'SecurityError');
		});
		dispose = theme.init();
		expect(() => theme.set('light')).not.toThrow();
		expect(theme.resolved).toBe('light');
		dispose();
	});

	test('works without matchMedia (system means light)', () => {
		vi.stubGlobal('matchMedia', undefined);
		const dispose = theme.init();
		expect(theme.resolved).toBe('light');
		dispose();
	});
});

describe('toasts', () => {
	test('success toasts expire after 6 s, errors stay until dismissed', async () => {
		const ok = toasts.success('Saved and applied.');
		const bad = toasts.error('port 70000 out of range');
		expect(toasts.list.map((toast) => [toast.id, toast.kind, toast.message])).toEqual([
			[ok, 'success', 'Saved and applied.'],
			[bad, 'error', 'port 70000 out of range']
		]);
		await vi.advanceTimersByTimeAsync(SUCCESS_TOAST_MS - 1);
		expect(toasts.list).toHaveLength(2);
		await vi.advanceTimersByTimeAsync(1);
		expect(toasts.list.map((toast) => toast.id)).toEqual([bad]);
		await vi.advanceTimersByTimeAsync(60_000);
		expect(toasts.list).toHaveLength(1);
		toasts.dismiss(bad);
		expect(toasts.list).toEqual([]);
	});

	test('dismiss cancels the expiry timer and warnings are persistent', () => {
		const id = toasts.success('x');
		toasts.dismiss(id);
		expect(vi.getTimerCount()).toBe(0);
		toasts.warning('saved, but ...');
		expect(vi.getTimerCount()).toBe(0);
		toasts.clear();
		expect(toasts.list).toEqual([]);
	});
});

describe('confirm', () => {
	test('falls back to window.confirm until a dialog host registers', async () => {
		const native = vi.spyOn(window, 'confirm').mockReturnValue(false);
		await expect(confirm({ message: 'Delete rule "a"?' })).resolves.toBe(false);
		expect(native).toHaveBeenCalledWith('Delete rule "a"?');
		const host = vi.fn(async () => true);
		const unregister = confirmService.register(host);
		await expect(confirm({ message: 'Reset?', danger: true })).resolves.toBe(true);
		expect(host).toHaveBeenCalledWith({ message: 'Reset?', danger: true });
		unregister();
		await expect(confirm({ message: 'again' })).resolves.toBe(false);
		expect(native).toHaveBeenCalledTimes(2);
	});
});
