import type { MessageKey } from './i18n/en';
import { isVirtualChannel } from './rule';
import { list, type Forward, type Listen, type Protocol, type Rule, type WayHop } from './types';
import { fieldsOf, layoutFor } from './urlBuilder';
import { normalizeWay, serializeWay } from './way';

// The editor keeps ports as typed text so partial input never snaps to a number.

export type Mode = 'proxy' | 'forward';

// Inactive kinds keep their drafts so toggling never loses typed input; only the active one is read.
export type EndpointKind = 'address' | 'virtual';

// Served protocols in display order with the credential fields a custom row exposes.
export const PROTOCOLS = [
	{ type: 'http', label: 'HTTP', fields: ['username', 'password'] },
	{ type: 'socks5', label: 'SOCKS5', fields: ['username', 'password'] },
	{ type: 'socks4', label: 'SOCKS4', fields: ['username'] },
	{ type: 'ssh', label: 'SSH', fields: ['username', 'password'] },
	{ type: 'ss', label: 'Shadowsocks', fields: ['password'] }
] as const satisfies readonly {
	type: string;
	label: string;
	fields: readonly ('username' | 'password')[];
}[];

export type ProtocolType = (typeof PROTOCOLS)[number]['type'];

const LEGACY_TYPES: readonly ProtocolType[] = ['http', 'socks5', 'socks4', 'ssh'];

const DEFAULT_CIPHER =
	fieldsOf(layoutFor('shadowsocks')!).find((input) => input.name === 'encrypto')?.value ?? '';

// One row per PROTOCOLS entry; custom off means the row inherits the shared credentials.
export interface ProtocolDraft {
	type: ProtocolType;
	enabled: boolean;
	custom: boolean;
	username: string;
	password: string;
	cipher: string;
}

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
		protocols: ProtocolDraft[];
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

// An empty stored list means the legacy set; a stored ss entry may still take the flat cipher.
function protocolsFrom(listen: Listen): ProtocolDraft[] {
	const stored = list(listen.protocols);
	return PROTOCOLS.map(({ type }) => {
		const entry = stored.find((candidate) => candidate.type === type);
		const username = text(entry?.username);
		const password = text(entry?.password);
		const cipher =
			type === 'ss' ? text(entry?.cipher) || text(listen.cipher) || DEFAULT_CIPHER : '';
		const enabled = stored.length
			? !!entry
			: LEGACY_TYPES.includes(type) || (type === 'ss' && !!listen.cipher);
		return { type, enabled, custom: !!(username || password), username, password, cipher };
	});
}

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
			protocols: protocolsFrom(rule.listen),
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
	protocols?: MessageKey;
	ssCipher?: MessageKey;
	ssPassword?: MessageKey;
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

// An empty protocol list enables legacy defaults, so proxy saves must list their selections.
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
	const protocols: Protocol[] = [];
	for (const row of draft.listen.protocols) {
		if (forwarding || !row.enabled) continue;
		const entry: Protocol = { type: row.type };
		if (row.custom && row.username) entry.username = row.username;
		if (row.custom && row.password) entry.password = row.password;
		if (row.type === 'ss') {
			if (row.cipher) entry.cipher = row.cipher;
			else errors.ssCipher = 'cipherRequired';
			if (!entry.password && !draft.listen.password) errors.ssPassword = 'passwordRequired';
		}
		protocols.push(entry);
	}
	if (!forwarding && !protocols.length) errors.protocols = 'protocolRequired';
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
		errors.protocols ||
		errors.ssCipher ||
		errors.ssPassword ||
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
		listen.protocols = protocols;
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
