import { i18n } from './i18n.svelte';
import { router } from './router.svelte';
import { addressURL, movedHref, reportedAddress } from './settings';
import { status } from './status.svelte';
import type { Status } from './types';

// Where this page can be reopened after a write changed the web UI address. The notice outlives
// the page that caused it: once the old listener is gone the link is the only way forward.

export interface MovedLink {
	address: string;
	href: string;
}

// 'moved' is proven or implied by an applied write, 'maybe' got no answer, 'unconfirmed' is saved
// but not known to be running, 'unknown' applied to an address no link can name.
export type MovedNotice =
	| { kind: 'moved' | 'maybe'; link: MovedLink }
	| { kind: 'unconfirmed'; link: MovedLink | null }
	| { kind: 'unknown'; link: null };

let notice = $state.raw<MovedNotice | null>(null);
// Address the last write asked for; '' when the server picks the port.
let candidate = '';
// Whether the last write's answer said the reload applied.
let reloaded = true;
// Status loads numbered up to here were requested before the last write began.
let mark = 0;

function linkFor(address: string): MovedLink | null {
	const href = movedHref(address, {
		currentHost: location.host,
		lang: i18n.explicit ? i18n.language : null,
		hash: router.route.hash
	});
	return href === null ? null : { address, href };
}

// The listener is at `address`: a link when that is elsewhere, nothing when it is this page's own
// origin, and 'unknown' when no link can be made from it.
function claim(address: string): MovedNotice | null {
	if (!addressURL(address)) return { kind: 'unknown', link: null };
	const link = linkFor(address);
	return link ? { kind: 'moved', link } : null;
}

// What a status answer proves: the reachable address of a running listener, '' when nothing
// usable is bound (still starting, bind failed, or port 0 not yet assigned).
const bound = (current: Status): string =>
	current.running ? reportedAddress(current.address, location.host) : '';

// What a fresh status answer settles: nothing running leaves the write unconfirmed at the
// candidate (else at `link`, the address already shown); a running listener is where it says,
// named by the candidate when there is one; undefined when the answer proves nothing.
function fromStatus(current: Status, link: MovedLink | null): MovedNotice | null | undefined {
	if (!current.running) {
		return { kind: 'unconfirmed', link: (candidate && linkFor(candidate)) || link };
	}
	const actual = bound(current);
	if (!actual) return undefined;
	const proof = claim(actual);
	return proof && candidate ? claim(candidate) : proof;
}

// What the write's own answer says when no status does: an applied reload moved the web UI to
// the candidate or somewhere unknown; a failed one leaves the candidate unconfirmed.
const fromWrite = (): MovedNotice | null =>
	!reloaded
		? { kind: 'unconfirmed', link: candidate ? linkFor(candidate) : null }
		: candidate
			? claim(candidate)
			: { kind: 'unknown', link: null };

export const moved = {
	get notice(): MovedNotice | null {
		return notice;
	},
	clear() {
		notice = null;
		candidate = '';
		reloaded = true;
		mark = 0;
	},
	// Before a write that may change web_ui: a status answer to a request already on the wire can
	// only describe the old listener, however late it arrives.
	beginWrite() {
		mark = status.requested;
	},
	// After a persisted write that may have changed web_ui, `applied` being what its answer said
	// of the reload; true when this page must be reopened or its outcome is in doubt.
	async afterWrite(address: string, applied: boolean): Promise<boolean> {
		candidate = address;
		reloaded = applied;
		const current = await status.refresh();
		const settled = current ? fromStatus(current, null) : undefined;
		notice = settled === undefined ? fromWrite() : settled;
		return notice !== null;
	},
	// A write that got no answer may still have applied; true when a candidate link is shown.
	uncertain(address: string): boolean {
		candidate = address;
		const link = linkFor(address);
		notice = link ? { kind: 'maybe', link } : null;
		return notice !== null;
	},
	// A status answer from this origin, `request` being the poller's number for it. Answers to
	// requests that predate the write are ignored; a later one says whether anything runs, and where.
	reconcile(current: Status, request: number) {
		if (notice === null || request <= mark) return;
		// An unanswered write is not settled by a listener that is not running either.
		if (notice.kind === 'maybe' && !current.running) return;
		const next = fromStatus(current, notice.link);
		if (next !== undefined && JSON.stringify(next) !== JSON.stringify(notice)) notice = next;
	}
};
