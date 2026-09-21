import type { MessageKey } from './i18n/en';
import { isVirtualChannel } from './rule';
import type { Forward, Listen, Rule, WayHop } from './types';
import { normalizeWay, serializeWay } from './way';

// The editor keeps ports as typed text so partial input never snaps to a number.

export type Mode = 'proxy' | 'forward';

// Inactive kinds keep their drafts so toggling never loses typed input; only the active one is read.
export type EndpointKind = 'address' | 'virtual';

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
	listen: {
		kind: EndpointKind;
		host: string;
		port: string;
		virtual: string;
		username: string;
		password: string;
		way: HopDraft[];
	};
	mode: Mode;
	target: { kind: EndpointKind; host: string; port: string; virtual: string };
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

const kindOf = (virtual: string | undefined): EndpointKind => (virtual ? 'virtual' : 'address');

export function draftFrom(rule: Rule): RuleDraft {
	const port = rule.forward.port ?? 0;
	const forwarding = port > 0 || !!rule.forward.virtual;
	return {
		name: rule.name,
		enabled: !rule.disabled,
		listen: {
			kind: kindOf(rule.listen.virtual),
			host: text(rule.listen.host),
			port: text(rule.listen.port),
			virtual: text(rule.listen.virtual),
			username: text(rule.listen.username),
			password: text(rule.listen.password),
			way: hopsFrom(rule.listen.way)
		},
		mode: forwarding ? 'forward' : 'proxy',
		target: {
			kind: kindOf(rule.forward.virtual),
			host: text(rule.forward.host),
			port: port > 0 ? text(port) : '',
			virtual: text(rule.forward.virtual)
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
	listenVirtual?: MessageKey;
	targetPort?: MessageKey;
	targetVirtual?: MessageKey;
}

export type ReadResult = { ok: true; rule: Rule } | { ok: false; errors: DraftErrors };

function parsePort(value: string, minimum: 0 | 1): number | null {
	const trimmed = value.trim();
	const port = Number(trimmed);
	if (!trimmed || !Number.isInteger(port) || port < minimum || port > 65535) return null;
	return port;
}

function parseChannel(value: string): string | null {
	const trimmed = value.trim();
	return isVirtualChannel(trimmed) ? trimmed : null;
}

// Port of the legacy readRule/readPort: trimmed name and hosts, credentials only for proxies,
// no target for proxies, ways omitted when empty, disabled only when true. Virtual endpoints
// send only their channel; hidden ports and hops are neither validated nor sent.
export function readRule(draft: RuleDraft): ReadResult {
	const errors: DraftErrors = {};
	const name = draft.name.trim();
	if (!name) errors.name = 'ruleNameRequired';
	const listenVirtual = draft.listen.kind === 'virtual';
	const listenPort = listenVirtual ? 0 : parsePort(draft.listen.port, 0);
	if (listenPort === null) errors.listenPort = 'invalidPort';
	const listenChannel = listenVirtual ? parseChannel(draft.listen.virtual) : '';
	if (listenChannel === null) errors.listenVirtual = 'invalidVirtualChannel';
	const forwarding = draft.mode === 'forward';
	const targetVirtual = forwarding && draft.target.kind === 'virtual';
	const targetPort = forwarding && !targetVirtual ? parsePort(draft.target.port, 1) : 0;
	if (targetPort === null) errors.targetPort = 'invalidForwardPort';
	const targetChannel = targetVirtual ? parseChannel(draft.target.virtual) : '';
	if (targetChannel === null) errors.targetVirtual = 'invalidVirtualChannel';
	else if (targetChannel && targetChannel === listenChannel)
		errors.targetVirtual = 'virtualSelfLoop';
	if (
		listenPort === null ||
		listenChannel === null ||
		targetPort === null ||
		targetChannel === null ||
		errors.name ||
		errors.targetVirtual
	) {
		return { ok: false, errors };
	}

	const listen: Listen = listenVirtual
		? { host: '', port: 0, virtual: listenChannel }
		: { host: draft.listen.host.trim(), port: listenPort };
	if (!forwarding) {
		if (draft.listen.username) listen.username = draft.listen.username;
		if (draft.listen.password) listen.password = draft.listen.password;
	}
	const listenWay = listenVirtual ? [] : wayOf(draft.listen.way);
	if (listenWay.length) listen.way = listenWay;

	const forward: Forward = {};
	if (targetVirtual) {
		forward.virtual = targetChannel;
	} else if (forwarding) {
		const host = draft.target.host.trim();
		if (host) forward.host = host;
		forward.port = targetPort;
	}
	const forwardWay = targetVirtual ? [] : wayOf(draft.forward.way);
	if (forwardWay.length) forward.way = forwardWay;

	const rule: Rule = { name, listen, forward };
	if (!draft.enabled) rule.disabled = true;
	return { ok: true, rule };
}

// Dirty tracking compares snapshots; ids are layout state, not content.
export const snapshot = (draft: RuleDraft): string =>
	JSON.stringify(draft, (key, value: unknown) => (key === 'id' ? undefined : value));
