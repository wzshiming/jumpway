import { busy } from './busy.svelte';
import { confirm } from './confirm';
import { t } from './i18n.svelte';
import { routeFor, type Route } from './routes';

export type DirtyPredicate = () => boolean;

// Each entry carries its position so a refused Back/Forward is undone with history.go()
// instead of overwriting the destination entry.
export const HISTORY_INDEX_KEY = 'jumpwayHistoryIndex';

let route = $state<Route>(routeFor(''));
const guards = new Set<DirtyPredicate>();
// Position of the entry showing `route`.
let acceptedIdx = 0;
// Position a pending history.go() correction will land on.
let expectedIdx: number | null = null;
// A navigate() confirmation is open.
let prompting = false;
// Bumped by start/stop so an answer from an earlier lifecycle is ignored.
let epoch = 0;

function stateWith(position: number): Record<string, unknown> {
	const current: unknown = history.state;
	const base = typeof current === 'object' && current !== null ? current : {};
	return { ...base, [HISTORY_INDEX_KEY]: position };
}

function readPosition(): number | null {
	const current: unknown = history.state;
	if (typeof current !== 'object' || current === null) return null;
	const value = (current as Record<string, unknown>)[HISTORY_INDEX_KEY];
	return typeof value === 'number' ? value : null;
}

// Position of the entry the browser is on; an unstamped one is a fresh push after the accepted entry.
function sync(): number {
	let idx = readPosition();
	if (idx === null) {
		idx = acceptedIdx + 1;
		history.replaceState(stateWith(idx), '');
	}
	return idx;
}

// Rewrites only the entry the browser is on.
function canonicalise(idx: number, hash: string) {
	if (location.hash === hash && readPosition() === idx) return;
	history.replaceState(stateWith(idx), '', hash);
}

const dirty = () => Array.from(guards).some((isDirty) => isDirty());

// One history write back to the accepted entry; a stamp collision can only rewrite the current entry.
function restore(idx: number) {
	const delta = acceptedIdx - idx;
	if (delta === 0) {
		canonicalise(idx, route.hash);
		return;
	}
	expectedIdx = acceptedIdx;
	history.go(delta);
}

function onPopstate() {
	const idx = sync();
	const to = routeFor(location.hash);
	const landing = expectedIdx === idx;
	expectedIdx = null;
	if (landing || to.hash === route.hash) {
		acceptedIdx = idx;
		canonicalise(idx, route.hash);
		return;
	}
	if (busy.value || prompting || (dirty() && !window.confirm(t('discardChanges')))) {
		restore(idx);
		return;
	}
	route = to;
	acceptedIdx = idx;
	canonicalise(idx, to.hash);
}

export const router = {
	get route() {
		return route;
	},
	start(): () => void {
		epoch++;
		expectedIdx = null;
		route = routeFor(location.hash);
		acceptedIdx = readPosition() ?? 0;
		canonicalise(acceptedIdx, route.hash);
		window.addEventListener('popstate', onPopstate);
		return () => {
			window.removeEventListener('popstate', onPopstate);
			guards.clear();
			epoch++;
			prompting = false;
		};
	},
	async navigate(hash: string, { replace = false } = {}): Promise<boolean> {
		const to = routeFor(hash);
		if (to.hash === route.hash) {
			if (expectedIdx === null) canonicalise(sync(), route.hash);
			return true;
		}
		if (busy.value || prompting || expectedIdx !== null) return false;
		if (dirty()) {
			const started = epoch;
			prompting = true;
			let ok = false;
			try {
				ok = await confirm({ message: t('discardChanges'), danger: true });
			} catch {
				ok = false;
			} finally {
				if (epoch === started) prompting = false;
			}
			if (epoch !== started || !ok || busy.value || expectedIdx !== null) return false;
		}
		const idx = sync();
		if (replace) history.replaceState(stateWith(idx), '', to.hash);
		else history.pushState({ [HISTORY_INDEX_KEY]: idx + 1 }, '', to.hash);
		route = to;
		acceptedIdx = readPosition()!;
		return true;
	},
	registerLeaveGuard(isDirty: DirtyPredicate): () => void {
		guards.add(isDirty);
		return () => {
			guards.delete(isDirty);
		};
	}
};
