import { text } from './format';
import type { MessageKey } from './i18n/en';
import { isVirtualChannel, joinHostPort, virtualAddress } from './rule';
import {
	list,
	type Address,
	type Forward,
	type Listen,
	type Protocol,
	type Rule,
	type WayHop
} from './types';
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

// The suggested name of a copy: "<name>-copy", counting up past taken ones; copying a copy
// counts from the original name rather than stacking suffixes.
export function duplicateName(name: string, taken: readonly string[]): string {
	const names = new Set(taken);
	const base = name.replace(/-copy(-\d+)?$/, '') + '-copy';
	if (!names.has(base)) return base;
	for (let n = 2; ; n++) {
		const candidate = `${base}-${n}`;
		if (!names.has(candidate)) return candidate;
	}
}

export interface ProtocolFieldErrors {
	username?: MessageKey;
	password?: MessageKey;
}

export interface DraftErrors {
	name?: MessageKey;
	listenPort?: MessageKey;
	listenVirtual?: MessageKey;
	// The rule named by a listenAddressTaken error.
	listenAddressOwner?: string;
	listenUsername?: MessageKey;
	listenPassword?: MessageKey;
	protocols?: MessageKey;
	protocolErrors?: Partial<Record<ProtocolType, ProtocolFieldErrors>>;
	ssCipher?: MessageKey;
	ssPassword?: MessageKey;
	targetPort?: MessageKey;
	targetVirtual?: MessageKey;
	// Keyed by UrlDraft.id.
	urls?: Record<number, MessageKey>;
}

// What the stored configuration already holds, for the checks the server runs across rules.
export interface ReadContext {
	others: readonly Rule[];
	// The saved name of the rule being edited; its stored copy is not a peer.
	self: string | null;
	// The configured (not the bound) web UI address, or null while unknown.
	webUI: Address | null;
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

// Go's url.Parse scheme: a letter, then letters, digits, "+", "-" or ".", up to the colon.
const SCHEME = /^[a-z][a-z0-9+.-]*:/i;

// Blank rows beside a usable URL are dropped as before; a hop left with none needs one.
function readWay(hops: readonly HopDraft[], errors: DraftErrors): WayHop[] {
	for (const hop of hops) {
		const usable = hop.urls.some((url) => url.value.trim());
		for (const url of hop.urls) {
			const value = url.value.trim();
			if (!value) {
				if (!usable) (errors.urls ??= {})[url.id] = 'hopUrlEmpty';
			} else if (!SCHEME.test(value)) {
				(errors.urls ??= {})[url.id] = 'hopUrlNoScheme';
			}
		}
	}
	return wayOf(hops);
}

// Rules the server checks listen addresses against: enabled, bound here, on a port or channel.
const listens = (rule: Rule): boolean =>
	!rule.disabled &&
	normalizeWay(rule.listen.way).length === 0 &&
	(rule.listen.port !== 0 || !!rule.listen.virtual);

// Mirrors Listen.Address(): the string Validate compares, brackets and all.
const configuredAddress = (listen: Listen): string =>
	listen.virtual ? virtualAddress(listen.virtual) : joinHostPort(listen.host, listen.port);

const protocolFields = (type: ProtocolType): readonly string[] =>
	PROTOCOLS.find((entry) => entry.type === type)!.fields;

// Mirrors validateListenAuth for an explicit protocol list: a row's own credentials win, empty
// ones inherit the shared pair, and the shared password may serve Shadowsocks alone.
function readAuth(listen: RuleDraft['listen'], errors: DraftErrors) {
	// Only a field the row shows can carry an error; anything else is left to the server's 400.
	const report = (row: ProtocolDraft, field: 'username' | 'password', key: MessageKey) => {
		if (!protocolFields(row.type).includes(field)) return;
		((errors.protocolErrors ??= {})[row.type] ??= {})[field] = key;
	};
	if (listen.username.includes(':')) errors.listenUsername = 'usernameColon';
	const rows = listen.protocols.filter((row) => row.enabled);
	const sharedForSS = rows.some((row) => row.type === 'ss' && !(row.custom && row.password));
	for (const row of rows) {
		const ownUsername = row.custom ? row.username : '';
		const ownPassword = row.custom ? row.password : '';
		if (ownUsername.includes(':')) report(row, 'username', 'usernameColon');
		if (row.type === 'ss') continue;
		if (!(ownPassword || listen.password) || ownUsername || listen.username) continue;
		if (ownPassword) {
			// SOCKS4 shows no password field; its stored password is reported on the username.
			const field = protocolFields(row.type).includes('password') ? 'password' : 'username';
			report(row, field, 'passwordNeedsUsername');
		} else if (!sharedForSS) {
			errors.listenPassword = 'passwordNeedsUsername';
		}
	}
}

// An empty protocol list enables legacy defaults, so proxy saves must list their selections.
// Without a context only the checks that need no other rule run.
export function readRule(draft: RuleDraft, context?: ReadContext): ReadResult {
	const errors: DraftErrors = {};
	const name = draft.name.trim();
	if (!name) errors.name = 'ruleNameRequired';
	else if (name.includes('/')) errors.name = 'ruleNameSlash';
	else if (context?.others.some((rule) => rule.name === name && rule.name !== context.self))
		errors.name = 'ruleNameTaken';
	const listenVirtual = draft.listen.kind === 'virtual';
	const listenPort = listenVirtual ? 0 : parsePort(draft.listen.port, 0);
	if (listenPort === null) errors.listenPort = 'invalidPort';
	const listenChannel = listenVirtual ? parseChannel(draft.listen.virtual) : '';
	if (listenChannel === null) errors.listenVirtual = 'invalidVirtualChannel';
	const listenWay = listenVirtual ? [] : readWay(draft.listen.way, errors);
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
	if (!forwarding) readAuth(draft.listen, errors);
	const targetVirtual = forwarding && draft.target.kind === 'virtual';
	const targetPort = forwarding && !targetVirtual ? parsePort(draft.target.port, 1) : 0;
	if (targetPort === null) errors.targetPort = 'invalidForwardPort';
	const targetChannel = targetVirtual ? parseChannel(draft.target.virtual) : '';
	if (targetChannel === null) errors.targetVirtual = 'invalidVirtualChannel';
	else if (targetChannel && targetChannel === listenChannel)
		errors.targetVirtual = 'virtualSelfLoop';
	const forwardWay = targetVirtual ? [] : readWay(draft.forward.way, errors);

	// Like Validate: disabled and remote rules and port 0 never clash, nor does a web UI on port 0.
	if (
		context &&
		draft.enabled &&
		listenPort !== null &&
		listenChannel !== null &&
		(listenVirtual || (listenPort !== 0 && listenWay.length === 0))
	) {
		const address = listenVirtual
			? virtualAddress(listenChannel)
			: joinHostPort(draft.listen.host.trim(), listenPort);
		const field = listenVirtual ? 'listenVirtual' : 'listenPort';
		const owner = context.others.find(
			(rule) =>
				rule.name !== context.self && listens(rule) && configuredAddress(rule.listen) === address
		);
		if (owner) {
			errors[field] = 'listenAddressTaken';
			errors.listenAddressOwner = owner.name;
		} else if (
			context.webUI?.port &&
			joinHostPort(context.webUI.host, context.webUI.port) === address
		) {
			errors[field] = 'listenAddressWebUI';
		}
	}
	if (
		listenPort === null ||
		listenChannel === null ||
		targetPort === null ||
		targetChannel === null ||
		Object.keys(errors).length
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
	if (listenWay.length) listen.way = listenWay;

	const forward: Forward = {};
	if (targetVirtual) {
		forward.virtual = targetChannel;
	} else if (forwarding) {
		const host = draft.target.host.trim();
		if (host) forward.host = host;
		forward.port = targetPort;
	}
	if (forwardWay.length) forward.way = forwardWay;

	const rule: Rule = { name, listen, forward };
	if (!draft.enabled) rule.disabled = true;
	return { ok: true, rule };
}

// Dirty tracking compares snapshots; ids are layout state, not content.
export const snapshot = (draft: RuleDraft): string =>
	JSON.stringify(draft, (key, value: unknown) => (key === 'id' ? undefined : value));
