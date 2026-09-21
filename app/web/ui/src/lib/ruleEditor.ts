import type { MessageKey } from './i18n/en';
import type { Forward, Listen, Rule, WayHop } from './types';
import { normalizeWay, serializeWay } from './way';

// The editor keeps ports as typed text so partial input never snaps to a number.

export type Mode = 'proxy' | 'forward';

export interface UrlDraft {
	id: number;
	value: string;
}

// Rows are keyed by id, never by their text: editing a URL must not remount its input.
export interface HopDraft {
	id: number;
	urls: UrlDraft[];
}

export interface RuleDraft {
	name: string;
	enabled: boolean;
	listen: { host: string; port: string; username: string; password: string; way: HopDraft[] };
	mode: Mode;
	target: { host: string; port: string };
	forward: { way: HopDraft[] };
}

let sequence = 0;

export const newUrl = (value = ''): UrlDraft => ({ id: ++sequence, value });

export const newHop = (urls: readonly string[] = ['']): HopDraft => ({
	id: ++sequence,
	urls: urls.map(newUrl)
});

const text = (value: unknown) => (value === undefined || value === null ? '' : String(value));

const hopsFrom = (way: Rule['listen']['way']): HopDraft[] =>
	normalizeWay(way).map((hop) => newHop(hop.lb));

export function draftFrom(rule: Rule): RuleDraft {
	const forwarding = (rule.forward.port ?? 0) > 0;
	return {
		name: rule.name,
		enabled: !rule.disabled,
		listen: {
			host: text(rule.listen.host),
			port: text(rule.listen.port),
			username: text(rule.listen.username),
			password: text(rule.listen.password),
			way: hopsFrom(rule.listen.way)
		},
		mode: forwarding ? 'forward' : 'proxy',
		target: {
			host: text(rule.forward.host),
			port: forwarding ? text(rule.forward.port) : ''
		},
		forward: { way: hopsFrom(rule.forward.way) }
	};
}

export const emptyDraft = (): RuleDraft =>
	draftFrom({ name: '', listen: { host: '127.0.0.1', port: 0 }, forward: {} });

export const wayOf = (hops: readonly HopDraft[]): WayHop[] =>
	serializeWay(hops.map((hop) => ({ lb: hop.urls.map((url) => url.value) })));

export interface DraftErrors {
	name?: MessageKey;
	listenPort?: MessageKey;
	targetPort?: MessageKey;
}

export type ReadResult = { ok: true; rule: Rule } | { ok: false; errors: DraftErrors };

function parsePort(value: string, minimum: 0 | 1): number | null {
	const trimmed = value.trim();
	const port = Number(trimmed);
	if (!trimmed || !Number.isInteger(port) || port < minimum || port > 65535) return null;
	return port;
}

// Port of the legacy readRule/readPort: trimmed name and hosts, credentials only for proxies,
// no target for proxies, ways omitted when empty, disabled only when true.
export function readRule(draft: RuleDraft): ReadResult {
	const errors: DraftErrors = {};
	const name = draft.name.trim();
	if (!name) errors.name = 'ruleNameRequired';
	const listenPort = parsePort(draft.listen.port, 0);
	if (listenPort === null) errors.listenPort = 'invalidPort';
	const forwarding = draft.mode === 'forward';
	const targetPort = forwarding ? parsePort(draft.target.port, 1) : 0;
	if (targetPort === null) errors.targetPort = 'invalidForwardPort';
	if (listenPort === null || targetPort === null || errors.name) return { ok: false, errors };

	const listen: Listen = { host: draft.listen.host.trim(), port: listenPort };
	if (!forwarding) {
		if (draft.listen.username) listen.username = draft.listen.username;
		if (draft.listen.password) listen.password = draft.listen.password;
	}
	const listenWay = wayOf(draft.listen.way);
	if (listenWay.length) listen.way = listenWay;

	const forward: Forward = {};
	if (forwarding) {
		const host = draft.target.host.trim();
		if (host) forward.host = host;
		forward.port = targetPort;
	}
	const forwardWay = wayOf(draft.forward.way);
	if (forwardWay.length) forward.way = forwardWay;

	const rule: Rule = { name, listen, forward };
	if (!draft.enabled) rule.disabled = true;
	return { ok: true, rule };
}

// Dirty tracking compares snapshots; ids are layout state, not content.
export const snapshot = (draft: RuleDraft): string =>
	JSON.stringify(draft, (key, value: unknown) => (key === 'id' ? undefined : value));
