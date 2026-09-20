import { isAborted } from './api';
import { busy } from './busy.svelte';

export interface PollerOptions<T> {
	load: (signal: AbortSignal) => Promise<T>;
	intervalMs: number;
}

export interface Poller<T> {
	readonly data: T | null;
	readonly error: unknown;
	readonly loading: boolean;
	readonly active: boolean;
	// Loads started so far, numbered from 1; `answered` is the number of the load that produced
	// `data`. A consumer that noted `requested` before an action can tell answers to earlier loads
	// from those that could observe the action's outcome.
	readonly requested: number;
	readonly answered: number;
	subscribe(): () => void;
	refresh(): Promise<T | null>;
}

// Runs only while subscribed, visible and not busy; late answers after stop/hide are dropped.
export function createPoller<T>({ load, intervalMs }: PollerOptions<T>): Poller<T> {
	let data = $state.raw<T | null>(null);
	let error = $state.raw<unknown>(null);
	let loading = $state(false);
	let subscribers = $state(0);
	let timer: ReturnType<typeof setTimeout> | null = null;
	let controller: AbortController | null = null;
	let inFlight: Promise<T | null> | null = null;
	let generation = 0;
	let requested = 0;
	let answered = 0;

	const visible = () => document.visibilityState === 'visible';

	function clearTimer() {
		if (timer !== null) clearTimeout(timer);
		timer = null;
	}

	function invalidate() {
		generation++;
		controller?.abort();
		controller = null;
		inFlight = null;
		loading = false;
	}

	function schedule() {
		clearTimer();
		if (subscribers > 0 && visible()) timer = setTimeout(tick, intervalMs);
	}

	function fetchOnce(): Promise<T | null> {
		const own = new AbortController();
		const started = generation;
		const number = ++requested;
		controller = own;
		loading = true;
		const current = () => started === generation && controller === own;
		const run = load(own.signal)
			.then(
				(value) => {
					if (!current()) return null;
					data = value;
					answered = number;
					error = null;
					return value;
				},
				(reason: unknown) => {
					if (current() && !isAborted(reason)) error = reason;
					return null;
				}
			)
			.finally(() => {
				if (controller !== own) return;
				controller = null;
				inFlight = null;
				loading = false;
			});
		inFlight = run;
		return run;
	}

	function tick() {
		clearTimer();
		if (subscribers === 0 || !visible()) return;
		if (busy.value || inFlight) {
			schedule();
			return;
		}
		void fetchOnce().finally(schedule);
	}

	function onVisibility() {
		if (visible()) {
			if (!inFlight) tick();
		} else {
			clearTimer();
			invalidate();
		}
	}

	return {
		get data() {
			return data;
		},
		get error() {
			return error;
		},
		get loading() {
			return loading;
		},
		get active() {
			return subscribers > 0;
		},
		get requested() {
			return requested;
		},
		get answered() {
			return answered;
		},
		subscribe() {
			if (++subscribers === 1) {
				document.addEventListener('visibilitychange', onVisibility);
				tick();
			}
			let released = false;
			return () => {
				if (released) return;
				released = true;
				if (--subscribers === 0) {
					document.removeEventListener('visibilitychange', onVisibility);
					clearTimer();
					invalidate();
				}
			};
		},
		async refresh() {
			const started = generation;
			const wanted = () => started === generation && subscribers > 0 && visible() && !busy.value;
			if (!wanted()) return null;
			while (inFlight) {
				await inFlight;
				if (!wanted()) return null;
			}
			return fetchOnce();
		}
	};
}
