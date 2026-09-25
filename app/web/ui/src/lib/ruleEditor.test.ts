import { describe, expect, test } from 'vitest';
import { rulesFixture, virtualRulesFixture } from '../../e2e/fixtures/api';
import {
	draftFrom,
	duplicateName,
	emptyDraft,
	newHop,
	PROTOCOLS,
	readRule,
	snapshot,
	type DraftErrors,
	type ProtocolDraft,
	type ReadContext,
	type RuleDraft
} from './ruleEditor';
import type { Address, Listen, Protocol, Rule } from './types';

const [office, mirror, dbTunnel, lab] = rulesFixture;

const urlsOf = (draft: RuleDraft, side: 'listen' | 'forward') =>
	draft[side].way.map((hop) => hop.urls.map((url) => url.value));

// What a legacy rule without protocols serves; readRule always spells it out.
const LEGACY: Protocol[] = [
	{ type: 'http' },
	{ type: 'socks5' },
	{ type: 'socks4' },
	{ type: 'ssh' }
];
const row = (draft: RuleDraft, type: string): ProtocolDraft =>
	draft.listen.protocols.find((entry) => entry.type === type)!;

describe('draftFrom', () => {
	test('deep-copies a proxy rule into editable strings with unique ids per hop and URL', () => {
		const draft = draftFrom(office);
		expect(draft.name).toBe('office');
		expect(draft.enabled).toBe(true);
		expect(draft.mode).toBe('proxy');
		expect(draft.listen).toMatchObject({
			host: '127.0.0.1',
			port: '18097',
			username: '',
			password: '',
			way: []
		});
		expect(draft.target).toEqual({ kind: 'address', host: '', port: '', virtual: '' });
		expect(urlsOf(draft, 'forward')).toEqual([
			['socks5://demo:placeholder@hop-a.example:1080'],
			['ssh://ops@bastion.example:22', 'ssh://ops@bastion-2.example:22']
		]);
		const ids = draft.forward.way.flatMap((hop) => [hop.id, ...hop.urls.map((url) => url.id)]);
		expect(new Set(ids).size).toBe(ids.length);
		draft.forward.way[0].urls[0].value = 'changed';
		expect(office.forward.way?.[0]).toBe('socks5://demo:placeholder@hop-a.example:1080');
	});

	test('forward rules expose the target; listen-through hops and credentials come along', () => {
		expect(draftFrom(mirror)).toMatchObject({
			mode: 'forward',
			target: { host: '10.0.0.5', port: '5432' }
		});
		const tunnel = draftFrom(dbTunnel);
		expect(tunnel.mode).toBe('forward');
		expect(tunnel.target).toEqual({
			kind: 'address',
			host: '127.0.0.1',
			port: '5432',
			virtual: ''
		});
		expect(urlsOf(tunnel, 'listen')).toEqual([['ssh://ops@edge.example:22']]);
		expect(urlsOf(tunnel, 'forward')).toEqual([]);
		expect(draftFrom(lab)).toMatchObject({
			enabled: false,
			mode: 'proxy',
			listen: { username: 'demo', password: 'placeholder' }
		});
	});

	test('a missing forward host becomes an empty field and a bare "a|b" node splits', () => {
		const rule: Rule = {
			name: 'x',
			listen: { host: '', port: 0, way: null },
			forward: { port: 8080, way: ['ssh://a:22|ssh://b:22'] }
		};
		const draft = draftFrom(rule);
		expect(draft.listen.host).toBe('');
		expect(draft.target).toEqual({ kind: 'address', host: '', port: '8080', virtual: '' });
		expect(urlsOf(draft, 'forward')).toEqual([['ssh://a:22', 'ssh://b:22']]);
	});

	test('emptyDraft is a proxy rule on 127.0.0.1:0 without hops, as the legacy new-rule form', () => {
		const draft = emptyDraft();
		expect(draft).toMatchObject({
			name: '',
			enabled: true,
			mode: 'proxy',
			listen: { host: '127.0.0.1', port: '0', username: '', password: '', way: [] },
			target: { kind: 'address', host: '', port: '', virtual: '' },
			forward: { way: [] }
		});
		expect(draft.listen).not.toHaveProperty('cipher');
	});
});

describe('readRule', () => {
	const valid = (mutate: (draft: RuleDraft) => void): Rule => {
		const draft = emptyDraft();
		draft.name = 'r';
		mutate(draft);
		const result = readRule(draft);
		if (!result.ok) throw new Error('unexpected errors ' + JSON.stringify(result.errors));
		return result.rule;
	};

	test('a proxy rule trims the name and host and omits target, credentials and empty ways', () => {
		const draft = emptyDraft();
		draft.name = ' office ';
		draft.listen.host = ' 0.0.0.0 ';
		draft.listen.port = '18097';
		expect(readRule(draft)).toEqual({
			ok: true,
			rule: {
				name: 'office',
				listen: { host: '0.0.0.0', port: 18097, protocols: LEGACY },
				forward: {}
			}
		});
	});

	test('credentials are sent verbatim for proxy rules and excluded for forward rules', () => {
		const proxy = valid((draft) => {
			draft.listen.username = 'demo';
			draft.listen.password = ' p@ss ';
		});
		expect(proxy.listen).toEqual({
			host: '127.0.0.1',
			port: 0,
			username: 'demo',
			password: ' p@ss ',
			protocols: LEGACY
		});
		const passwordOnly = valid((draft) => {
			draft.listen.password = 'secret';
			for (const entry of draft.listen.protocols) entry.enabled = entry.type === 'socks4';
			row(draft, 'socks4').custom = true;
			row(draft, 'socks4').username = 'legacy';
		});
		expect(passwordOnly.listen).toEqual({
			host: '127.0.0.1',
			port: 0,
			password: 'secret',
			protocols: [{ type: 'socks4', username: 'legacy' }]
		});
		const forward = valid((draft) => {
			draft.listen.username = 'demo';
			draft.listen.password = 'placeholder';
			draft.mode = 'forward';
			draft.target.port = '5432';
		});
		expect(forward.listen).toEqual({ host: '127.0.0.1', port: 0 });
		expect(forward.forward).toEqual({ port: 5432 });
	});

	test('forward rules carry the trimmed target host only when given', () => {
		const rule = valid((draft) => {
			draft.mode = 'forward';
			draft.target.host = ' 10.0.0.5 ';
			draft.target.port = '5432';
		});
		expect(rule.forward).toEqual({ host: '10.0.0.5', port: 5432 });
	});

	test('a proxy rule never sends the target even if the fields still hold values', () => {
		const rule = valid((draft) => {
			draft.target.host = '10.0.0.5';
			draft.target.port = '5432';
		});
		expect(rule.forward).toEqual({});
	});

	test('disabled is present only when the rule is disabled', () => {
		expect(valid(() => {})).not.toHaveProperty('disabled');
		expect(valid((draft) => (draft.enabled = false)).disabled).toBe(true);
	});

	test('ways trim URLs and drop blank rows beside a URL and hops without rows; a hop left blank is an error', () => {
		const rule = valid((draft) => {
			draft.listen.way = [newHop(['ssh://ops@edge.example:22 '])];
			draft.forward.way = [
				newHop([' socks5://hop-a.example:1080', '', 'ssh://ops@bastion.example:22', '  ']),
				newHop([])
			];
		});
		expect(rule.listen.way).toEqual([{ lb: ['ssh://ops@edge.example:22'] }]);
		expect(rule.forward.way).toEqual([
			{ lb: ['socks5://hop-a.example:1080', 'ssh://ops@bastion.example:22'] }
		]);
		expect(valid((draft) => (draft.forward.way = [newHop([])])).forward).toEqual({});
		// Unlike a blank row beside a URL, a hop whose rows are all blank is a mistake, not a no-op.
		const blank = emptyDraft();
		blank.name = 'r';
		blank.forward.way = [newHop(['', '  ']), newHop(['socks5://hop-a.example:1080'])];
		const [first, second] = blank.forward.way[0].urls;
		expect(readRule(blank)).toEqual({
			ok: false,
			errors: { urls: { [first.id]: 'hopUrlEmpty', [second.id]: 'hopUrlEmpty' } }
		});
	});

	test('reports every invalid field at once with the legacy messages', () => {
		const draft = emptyDraft();
		draft.name = '   ';
		draft.listen.port = '65536';
		draft.mode = 'forward';
		draft.target.port = '0';
		expect(readRule(draft)).toEqual({
			ok: false,
			errors: {
				name: 'ruleNameRequired',
				listenPort: 'invalidPort',
				targetPort: 'invalidForwardPort'
			}
		});
	});

	test('ports must be whole numbers; listen allows 0 and forward starts at 1', () => {
		const listenPort = (value: string) => {
			const draft = emptyDraft();
			draft.name = 'r';
			draft.listen.port = value;
			const result = readRule(draft);
			return result.ok ? result.rule.listen.port : result.errors.listenPort;
		};
		expect(listenPort('0')).toBe(0);
		expect(listenPort(' 1080 ')).toBe(1080);
		expect(listenPort('65535')).toBe(65535);
		for (const bad of ['', ' ', 'abc', '-1', '65536', '1.5', '1e3x']) {
			expect(listenPort(bad)).toBe('invalidPort');
		}
		const targetPort = (value: string) => {
			const draft = emptyDraft();
			draft.name = 'r';
			draft.mode = 'forward';
			draft.target.port = value;
			const result = readRule(draft);
			return result.ok ? result.rule.forward.port : result.errors.targetPort;
		};
		expect(targetPort('1')).toBe(1);
		expect(targetPort('0')).toBe('invalidForwardPort');
		expect(targetPort('')).toBe('invalidForwardPort');
		expect(targetPort('70000')).toBe('invalidForwardPort');
		// The target port is not validated while the rule is a proxy.
		const proxy = emptyDraft();
		proxy.name = 'r';
		proxy.target.port = 'junk';
		expect(readRule(proxy).ok).toBe(true);
	});
});

describe('snapshot', () => {
	test('ignores ids, so a re-read draft is clean while edits and reorders are dirty', () => {
		const before = snapshot(draftFrom(office));
		expect(snapshot(draftFrom(office))).toBe(before);
		const edited = draftFrom(office);
		edited.forward.way[0].urls[0].value += 'x';
		expect(snapshot(edited)).not.toBe(before);
		const reordered = draftFrom(office);
		reordered.forward.way.reverse();
		expect(snapshot(reordered)).not.toBe(before);
		const extraHop = draftFrom(office);
		extraHop.forward.way.push(newHop());
		expect(snapshot(extraHop)).not.toBe(before);
		expect(before).not.toContain('"id"');
	});
});

describe('virtual endpoints', () => {
	const sharedExit: Rule = {
		name: 'shared-exit',
		listen: {
			host: '',
			port: 0,
			virtual: 'exit',
			username: 'demo',
			password: 'placeholder',
			protocols: LEGACY
		},
		forward: { way: [{ lb: ['ssh://ops@bastion.example:22'] }] }
	};
	const lanEntry: Rule = {
		name: 'lan-entry',
		listen: { host: '0.0.0.0', port: 18100 },
		forward: { virtual: 'exit' }
	};
	const tcpTunnel: Rule = {
		name: 'db-tunnel',
		listen: { host: '0.0.0.0', port: 18099, way: [{ lb: ['ssh://ops@edge.example:22'] }] },
		forward: { host: '127.0.0.1', port: 5432 }
	};
	const read = (draft: RuleDraft) => {
		const result = readRule(draft);
		if (!result.ok) throw new Error('unexpected errors ' + JSON.stringify(result.errors));
		return result.rule;
	};
	const errorsOf = (draft: RuleDraft) => {
		const result = readRule(draft);
		return result.ok ? {} : result.errors;
	};

	test('draftFrom marks a virtual listen and keeps the rule a proxy', () => {
		const draft = draftFrom(sharedExit);
		expect(draft.mode).toBe('proxy');
		expect(draft.listen).toMatchObject({ kind: 'virtual', virtual: 'exit', username: 'demo' });
		expect(draft.target).toEqual({ kind: 'address', host: '', port: '', virtual: '' });
	});

	test('draftFrom marks a virtual target as a forward without a port', () => {
		const draft = draftFrom(lanEntry);
		expect(draft.mode).toBe('forward');
		expect(draft.listen).toMatchObject({
			kind: 'address',
			host: '0.0.0.0',
			port: '18100',
			virtual: ''
		});
		expect(draft.target).toEqual({ kind: 'virtual', host: '', port: '', virtual: 'exit' });
		expect(emptyDraft().listen).toMatchObject({ kind: 'address', virtual: '' });
		expect(emptyDraft().target).toEqual({ kind: 'address', host: '', port: '', virtual: '' });
	});

	test('a virtual listen sends host "" port 0 and the channel, skipping hidden port and hops', () => {
		const draft = draftFrom(sharedExit);
		draft.listen.port = 'junk';
		draft.listen.way = [newHop(['ssh://ops@edge.example:22'])];
		draft.listen.virtual = ' exit ';
		const rule = read(draft);
		expect(rule.listen).toEqual({
			host: '',
			port: 0,
			virtual: 'exit',
			username: 'demo',
			password: 'placeholder',
			protocols: LEGACY
		});
		expect(rule.forward).toEqual({ way: [{ lb: ['ssh://ops@bastion.example:22'] }] });
	});

	test('a virtual target sends only the channel, skipping the hidden port and exit hops', () => {
		const draft = draftFrom(lanEntry);
		draft.target.port = 'junk';
		draft.forward.way = [newHop(['ssh://ops@bastion.example:22'])];
		expect(read(draft).forward).toEqual({ virtual: 'exit' });
		const proxy = draftFrom(lanEntry);
		proxy.mode = 'proxy';
		expect(read(proxy).forward).toEqual({});
	});

	test('address endpoints never send virtual fields even when the inactive drafts hold text', () => {
		const draft = draftFrom(tcpTunnel);
		draft.listen.virtual = 'entry';
		draft.target.virtual = 'exit';
		expect(read(draft)).toEqual(tcpTunnel);
	});

	test('channels must be non-empty without whitespace or slashes; a direct self loop is refused', () => {
		for (const bad of ['', '  ', 'a b', 'a/b']) {
			const draft = draftFrom(sharedExit);
			draft.listen.virtual = bad;
			expect(errorsOf(draft)).toEqual({ listenVirtual: 'invalidVirtualChannel' });
			const target = draftFrom(lanEntry);
			target.target.virtual = bad;
			expect(errorsOf(target)).toEqual({ targetVirtual: 'invalidVirtualChannel' });
		}
		const loop = draftFrom(sharedExit);
		loop.mode = 'forward';
		loop.target = { kind: 'virtual', host: '', port: '', virtual: 'exit' };
		expect(errorsOf(loop)).toEqual({ targetVirtual: 'virtualSelfLoop' });
		loop.target.virtual = 'Exit';
		expect(read(loop).forward).toEqual({ virtual: 'Exit' });
		// A virtual target is neither validated nor sent while the rule is a proxy.
		const proxy = draftFrom(lanEntry);
		proxy.mode = 'proxy';
		proxy.target.virtual = 'a/b';
		expect(read(proxy).forward).toEqual({});
	});

	test('mixed kinds and proxy transitions round-trip through readRule and draftFrom', () => {
		for (const rule of [sharedExit, lanEntry]) {
			const draft = draftFrom(rule);
			expect(read(draft)).toEqual(rule);
			expect(snapshot(draftFrom(read(draft)))).toBe(snapshot(draft));
		}
	});
});

describe('listen protocols', () => {
	const read = (draft: RuleDraft) => {
		const result = readRule(draft);
		if (!result.ok) throw new Error('unexpected errors ' + JSON.stringify(result.errors));
		return result.rule;
	};
	const errorsOf = (draft: RuleDraft) => {
		const result = readRule(draft);
		return result.ok ? {} : result.errors;
	};
	const enabledTypes = (draft: RuleDraft) =>
		draft.listen.protocols.filter((entry) => entry.enabled).map((entry) => entry.type);
	const proxy = (listen: Partial<Rule['listen']>): Rule => ({
		name: 'r',
		listen: { host: '127.0.0.1', port: 18200, ...listen },
		forward: {}
	});

	test('PROTOCOLS is the fixed row order with the credential fields each scheme takes', () => {
		expect(PROTOCOLS.map((entry) => [entry.type, entry.label, [...entry.fields]])).toEqual([
			['http', 'HTTP', ['username', 'password']],
			['socks5', 'SOCKS5', ['username', 'password']],
			['socks4', 'SOCKS4', ['username']],
			['ssh', 'SSH', ['username', 'password']],
			['ss', 'Shadowsocks', ['password']]
		]);
	});

	test('a rule without protocols drafts the legacy four enabled and Shadowsocks off with a default cipher', () => {
		for (const rule of [office, lab, proxy({ protocols: [] }), proxy({ protocols: null })]) {
			const draft = draftFrom(rule);
			expect(draft.listen.protocols.map((entry) => entry.type)).toEqual(
				PROTOCOLS.map((entry) => entry.type)
			);
			expect(enabledTypes(draft)).toEqual(['http', 'socks5', 'socks4', 'ssh']);
			for (const entry of draft.listen.protocols) {
				expect(entry).toMatchObject({ custom: false, username: '', password: '' });
			}
			expect(row(draft, 'ss').cipher).toBe('aes-256-gcm');
			expect(row(draft, 'http').cipher).toBe('');
		}
		expect(enabledTypes(emptyDraft())).toEqual(['http', 'socks5', 'socks4', 'ssh']);
	});

	test('a legacy flat cipher enables the Shadowsocks row and lands in it, keeping the shared password', () => {
		const draft = draftFrom(proxy({ password: 'placeholder', cipher: 'aes-128-gcm' }));
		expect(enabledTypes(draft)).toEqual(['http', 'socks5', 'socks4', 'ssh', 'ss']);
		expect(row(draft, 'ss')).toEqual({
			type: 'ss',
			enabled: true,
			custom: false,
			username: '',
			password: '',
			cipher: 'aes-128-gcm'
		});
		expect(draft.listen).toMatchObject({ username: '', password: 'placeholder' });
		expect(draft.listen).not.toHaveProperty('cipher');
		// An alias the backend accepts is kept as text rather than dropped.
		expect(row(draftFrom(proxy({ password: 'p', cipher: 'AES_256_GCM' })), 'ss').cipher).toBe(
			'AES_256_GCM'
		);
	});

	test('an explicit list enables exactly its entries; own credentials mark the row custom, an ss cipher may still come from the flat field', () => {
		const draft = draftFrom(
			proxy({
				username: 'demo',
				password: 'placeholder',
				cipher: 'aes-128-gcm',
				protocols: [
					{ type: 'ss', password: 'own' },
					{ type: 'socks4', username: 'legacy-user' },
					{ type: 'http' }
				]
			})
		);
		expect(enabledTypes(draft)).toEqual(['http', 'socks4', 'ss']);
		expect(row(draft, 'http')).toMatchObject({ custom: false, username: '', password: '' });
		expect(row(draft, 'socks4')).toMatchObject({ custom: true, username: 'legacy-user' });
		expect(row(draft, 'ss')).toMatchObject({
			custom: true,
			password: 'own',
			cipher: 'aes-128-gcm'
		});
		expect(row(draft, 'socks5')).toMatchObject({ enabled: false, custom: false });
		const explicit = draftFrom(
			proxy({ protocols: [{ type: 'ss', password: 'own', cipher: 'chacha20-ietf-poly1305' }] })
		);
		expect(enabledTypes(explicit)).toEqual(['ss']);
		expect(row(explicit, 'ss').cipher).toBe('chacha20-ietf-poly1305');
	});

	test('a proxy rule always spells out its enabled protocols; one checked row is a single-protocol port', () => {
		const draft = emptyDraft();
		draft.name = 'r';
		expect(read(draft).listen.protocols).toEqual(LEGACY);
		for (const entry of draft.listen.protocols) entry.enabled = entry.type === 'socks5';
		expect(read(draft).listen).toEqual({
			host: '127.0.0.1',
			port: 0,
			protocols: [{ type: 'socks5' }]
		});
	});

	test('custom rows send only their non-empty fields; inheriting rows send the bare type even when they hold text', () => {
		const draft = emptyDraft();
		draft.name = 'r';
		draft.listen.username = 'demo';
		draft.listen.password = 'placeholder';
		Object.assign(row(draft, 'http'), { custom: true, username: 'web', password: 'web-pass' });
		Object.assign(row(draft, 'socks5'), { custom: true, username: 'only-user' });
		Object.assign(row(draft, 'socks4'), { custom: false, username: 'typed', password: 'typed' });
		expect(read(draft).listen).toEqual({
			host: '127.0.0.1',
			port: 0,
			username: 'demo',
			password: 'placeholder',
			protocols: [
				{ type: 'http', username: 'web', password: 'web-pass' },
				{ type: 'socks5', username: 'only-user' },
				{ type: 'socks4' },
				{ type: 'ssh' }
			]
		});
	});

	test('Shadowsocks sends its cipher in its own entry, never a flat listen.cipher, with the shared or its own password', () => {
		const draft = emptyDraft();
		draft.name = 'r';
		draft.listen.password = 'placeholder';
		row(draft, 'ss').enabled = true;
		const shared = read(draft).listen;
		expect(shared).not.toHaveProperty('cipher');
		expect(shared).toMatchObject({
			password: 'placeholder',
			protocols: [...LEGACY, { type: 'ss', cipher: 'aes-256-gcm' }]
		});
		// With its own password the shared one no longer serves ss, so it needs a username.
		draft.listen.username = 'demo';
		Object.assign(row(draft, 'ss'), { custom: true, password: 'own', cipher: 'AES_256_GCM' });
		expect(read(draft).listen.protocols?.at(-1)).toEqual({
			type: 'ss',
			password: 'own',
			cipher: 'AES_256_GCM'
		});
	});

	test('a legacy password-and-cipher rule migrates to an explicit list that reads back to the same draft', () => {
		const legacy = proxy({ password: 'placeholder', cipher: 'aes-256-gcm' });
		const draft = draftFrom(legacy);
		const rule = read(draft);
		expect(rule.listen).toEqual({
			host: '127.0.0.1',
			port: 18200,
			password: 'placeholder',
			protocols: [...LEGACY, { type: 'ss', cipher: 'aes-256-gcm' }]
		});
		expect(snapshot(draftFrom(rule))).toBe(snapshot(draft));
		const explicit = proxy({
			username: 'demo',
			password: 'placeholder',
			protocols: [
				{ type: 'socks4', username: 'legacy-user', password: 'kept' },
				{ type: 'ss', password: 'own', cipher: 'aes-128-gcm' }
			]
		});
		// Stored fields the form does not expose (a SOCKS4 password) survive the round trip.
		expect(read(draftFrom(explicit))).toEqual(explicit);
	});

	test('a forward rule omits protocols and credentials while the rows keep their drafts', () => {
		const draft = draftFrom(proxy({ password: 'placeholder', cipher: 'aes-256-gcm' }));
		Object.assign(row(draft, 'http'), { custom: true, username: 'web' });
		draft.mode = 'forward';
		draft.target.port = '5432';
		expect(read(draft).listen).toEqual({ host: '127.0.0.1', port: 18200 });
		draft.mode = 'proxy';
		expect(read(draft).listen.protocols).toEqual([
			{ type: 'http', username: 'web' },
			{ type: 'socks5' },
			{ type: 'socks4' },
			{ type: 'ssh' },
			{ type: 'ss', cipher: 'aes-256-gcm' }
		]);
	});

	test('a proxy rule needs at least one protocol; Shadowsocks needs a cipher and an effective password', () => {
		const none = emptyDraft();
		none.name = 'r';
		for (const entry of none.listen.protocols) entry.enabled = false;
		expect(errorsOf(none)).toEqual({ protocols: 'protocolRequired' });
		none.mode = 'forward';
		none.target.port = '5432';
		expect(errorsOf(none)).toEqual({});

		const ss = emptyDraft();
		ss.name = 'r';
		row(ss, 'ss').enabled = true;
		expect(errorsOf(ss)).toEqual({ ssPassword: 'passwordRequired' });
		row(ss, 'ss').cipher = '';
		expect(errorsOf(ss)).toEqual({ ssCipher: 'cipherRequired', ssPassword: 'passwordRequired' });
		ss.listen.password = 'placeholder';
		expect(errorsOf(ss)).toEqual({ ssCipher: 'cipherRequired' });
		row(ss, 'ss').cipher = 'aes-256-gcm';
		expect(errorsOf(ss)).toEqual({});
		// A custom row with an empty password still inherits the shared one.
		ss.listen.password = '';
		row(ss, 'ss').custom = true;
		expect(errorsOf(ss)).toEqual({ ssPassword: 'passwordRequired' });
		row(ss, 'ss').password = 'own';
		expect(errorsOf(ss)).toEqual({});
	});
});

describe('duplicateName', () => {
	test('suggests "<name>-copy", counting up past taken names', () => {
		expect(duplicateName('office', ['office', 'mirror'])).toBe('office-copy');
		expect(duplicateName('office', ['office', 'office-copy'])).toBe('office-copy-2');
		expect(duplicateName('office', ['office', 'office-copy', 'office-copy-2'])).toBe(
			'office-copy-3'
		);
		expect(duplicateName('office', ['office', 'office-copy-2'])).toBe('office-copy');
		expect(duplicateName('office', [])).toBe('office-copy');
	});

	test('copying a copy counts from the original name instead of stacking suffixes', () => {
		expect(duplicateName('office-copy', ['office', 'office-copy'])).toBe('office-copy-2');
		expect(duplicateName('office-copy-2', ['office-copy-2'])).toBe('office-copy');
		expect(duplicateName('copy', ['copy'])).toBe('copy-copy');
		expect(duplicateName('-copy', ['-copy'])).toBe('-copy-2');
	});
});

// The server's TestValidate table for listen credentials, translated to drafts the editor would
// send: `null` means the server accepts the rule.
describe('readRule mirrors validateListenAuth', () => {
	const proxy = (listen: Partial<Listen>): Rule => ({
		name: 'a',
		listen: { host: '127.0.0.1', port: 18200, ...listen },
		forward: {}
	});
	const ss = { type: 'ss', cipher: 'aes-256-gcm' };
	const cases: [string, Partial<Listen>, DraftErrors | null][] = [
		[
			'password_without_username',
			{ password: 'secret' },
			{ listenPassword: 'passwordNeedsUsername' }
		],
		[
			'username_with_colon',
			{ username: 'us:er', password: 'secret' },
			{ listenUsername: 'usernameColon' }
		],
		['username_and_password', { username: 'user', password: 'secret' }, null],
		['username_only', { username: 'user' }, null],
		['explicit_protocols', { port: 1080, protocols: [{ type: 'http' }, { type: 'socks5' }] }, null],
		['ss_only', { protocols: [{ ...ss, password: 'secret' }] }, null],
		['socks4_username_only', { protocols: [{ type: 'socks4', username: 'alice' }] }, null],
		[
			'protocol_username_with_colon',
			{ protocols: [{ type: 'http', username: 'a:b', password: 'secret' }] },
			{ protocolErrors: { http: { username: 'usernameColon' } } }
		],
		['ss_inherits_shared_password', { password: 'secret', protocols: [ss] }, null],
		[
			'shared_password_for_ss_beside_unauthenticated_http',
			{ password: 'secret', protocols: [...LEGACY, ss] },
			null
		],
		[
			'protocol_password_without_username',
			{ protocols: [{ type: 'http', password: 'secret' }] },
			{ protocolErrors: { http: { password: 'passwordNeedsUsername' } } }
		],
		[
			'protocol_password_without_username_beside_ss',
			{ password: 'shared', protocols: [ss, { type: 'socks5', password: 'secret' }] },
			{ protocolErrors: { socks5: { password: 'passwordNeedsUsername' } } }
		],
		[
			'shared_password_without_ss',
			{ password: 'secret', protocols: [{ type: 'http' }] },
			{ listenPassword: 'passwordNeedsUsername' }
		],
		[
			'shared_password_unused_by_ss',
			{ password: 'secret', protocols: [{ type: 'http' }, { ...ss, password: 'other' }] },
			{ listenPassword: 'passwordNeedsUsername' }
		],
		[
			'protocol_password_with_inherited_username',
			{ username: 'alice', protocols: [{ type: 'http', password: 'secret' }] },
			null
		],
		[
			'protocol_usernames_with_shared_password',
			{
				password: 'secret',
				protocols: [
					{ type: 'http', username: 'alice' },
					{ type: 'socks5', username: 'bob' }
				]
			},
			null
		],
		// SOCKS4 has no password field, so a stored password without a username lands on the username.
		[
			'socks4_password_without_username',
			{ protocols: [{ type: 'socks4', password: 'secret' }] },
			{ protocolErrors: { socks4: { username: 'passwordNeedsUsername' } } }
		],
		// The ss row has no username field to mark, so its stored ":" is left to the server's 400.
		[
			'ss_username_with_colon',
			{ protocols: [{ ...ss, username: 'a:b', password: 'secret' }] },
			null
		]
	];

	test.each(cases)('%s', (_, listen, expected) => {
		const result = readRule(draftFrom(proxy(listen)));
		expect(result.ok ? null : result.errors).toEqual(expected);
	});

	test('a stored ss username the form cannot show is neither refused nor altered', () => {
		const stored = proxy({ protocols: [{ ...ss, username: 'a:b', password: 'secret' }] });
		expect(readRule(draftFrom(stored))).toEqual({ ok: true, rule: stored });
	});

	test('the credential checks apply to proxy rules only and read the typed rows, not stored text', () => {
		const draft = draftFrom(proxy({ username: 'us:er', password: 'secret' }));
		draft.mode = 'forward';
		draft.target.port = '5432';
		expect(readRule(draft).ok).toBe(true);
		const typed = draftFrom(proxy({}));
		Object.assign(row(typed, 'http'), { custom: false, username: 'a:b', password: 'x' });
		expect(readRule(typed).ok).toBe(true);
		row(typed, 'http').custom = true;
		expect(readRule(typed)).toEqual({
			ok: false,
			errors: { protocolErrors: { http: { username: 'usernameColon' } } }
		});
	});
});

describe('readRule with the stored rules', () => {
	const context = (
		others: readonly Rule[],
		self: string | null = null,
		webUI: Address | null = { host: '127.0.0.1', port: 1088 }
	): ReadContext => ({ others, self, webUI });
	const errorsOf = (draft: RuleDraft, ctx?: ReadContext): DraftErrors => {
		const result = readRule(draft, ctx);
		return result.ok ? {} : result.errors;
	};
	const listenOn = (host: string, port: string): RuleDraft => {
		const draft = emptyDraft();
		draft.name = 'r';
		draft.listen.host = host;
		draft.listen.port = port;
		return draft;
	};

	test('names must not contain "/" or repeat another rule; the saved name itself is no clash', () => {
		const draft = draftFrom(office);
		draft.name = 'a/b';
		expect(errorsOf(draft)).toEqual({ name: 'ruleNameSlash' });
		expect(errorsOf(draft, context(rulesFixture, 'office'))).toEqual({ name: 'ruleNameSlash' });
		draft.name = ' mirror ';
		expect(errorsOf(draft)).toEqual({});
		expect(errorsOf(draft, context(rulesFixture, 'office'))).toEqual({ name: 'ruleNameTaken' });
		draft.name = 'office';
		expect(errorsOf(draft, context(rulesFixture, 'office'))).toEqual({});
		// A copy has no saved name: the source is a peer for both its name and its address.
		expect(errorsOf(draft, context(rulesFixture))).toEqual({
			name: 'ruleNameTaken',
			listenPort: 'listenAddressTaken',
			listenAddressOwner: 'office'
		});
	});

	test('a listen address clashes with enabled, locally bound rules on the same host:port', () => {
		const taken = { listenPort: 'listenAddressTaken', listenAddressOwner: 'mirror' };
		expect(errorsOf(listenOn('127.0.0.1', '18098'), context(rulesFixture))).toEqual(taken);
		expect(errorsOf(listenOn('', ' 18098 '), context(rulesFixture))).toEqual(taken);
		expect(errorsOf(listenOn('127.0.0.1', '18098'))).toEqual({});
		// Names are compared as spelled, never resolved.
		expect(errorsOf(listenOn('localhost', '18098'), context(rulesFixture))).toEqual({});
		expect(errorsOf(listenOn('0.0.0.0', '18098'), context(rulesFixture))).toEqual({});
		// lab is disabled and db-tunnel listens through a hop; neither holds its port here.
		expect(errorsOf(listenOn('127.0.0.1', '18100'), context(rulesFixture))).toEqual({});
		expect(errorsOf(listenOn('0.0.0.0', '18099'), context(rulesFixture))).toEqual({});
		// Nor does a disabled or remote draft, or one on port 0.
		const disabled = listenOn('127.0.0.1', '18098');
		disabled.enabled = false;
		expect(errorsOf(disabled, context(rulesFixture))).toEqual({});
		const remote = listenOn('127.0.0.1', '18098');
		remote.listen.way = [newHop(['ssh://ops@edge.example:22'])];
		expect(errorsOf(remote, context(rulesFixture))).toEqual({});
		const anyPort = { name: 'z', listen: { host: '', port: 0 }, forward: {} };
		expect(errorsOf(listenOn('', '0'), context([anyPort, ...rulesFixture]))).toEqual({});
		// Addresses are compared as Go's net.JoinHostPort spells them: brackets are added to any host
		// with ':', so '[::1]' and '::1' are different addresses, as they are to the server.
		const six: Rule = { name: 'six', listen: { host: '::1', port: 18300 }, forward: {} };
		expect(errorsOf(listenOn('::1', '18300'), context([six]))).toEqual({
			listenPort: 'listenAddressTaken',
			listenAddressOwner: 'six'
		});
		expect(errorsOf(listenOn('[::1]', '18300'), context([six]))).toEqual({});
		expect(errorsOf(listenOn('127.0.0.1', '18300'), context([six]))).toEqual({});
		const bracketed: Rule = { name: 'b6', listen: { host: '[::1]', port: 18300 }, forward: {} };
		expect(errorsOf(listenOn('[::1]', '18300'), context([bracketed]))).toEqual({
			listenPort: 'listenAddressTaken',
			listenAddressOwner: 'b6'
		});
		expect(errorsOf(listenOn('::1', '18300'), context([bracketed]))).toEqual({});
		// An invalid port is reported as such before any clash.
		expect(errorsOf(listenOn('127.0.0.1', 'junk'), context(rulesFixture))).toEqual({
			listenPort: 'invalidPort'
		});
	});

	test('the web UI clashes by its configured address once its port is set; port 0, another host spelling or an unknown address never does', () => {
		expect(errorsOf(listenOn('127.0.0.1', '1088'), context(rulesFixture))).toEqual({
			listenPort: 'listenAddressWebUI'
		});
		expect(errorsOf(listenOn('', '1088'), context(rulesFixture))).toEqual({
			listenPort: 'listenAddressWebUI'
		});
		expect(errorsOf(listenOn('0.0.0.0', '1088'), context(rulesFixture))).toEqual({});
		expect(errorsOf(listenOn('127.0.0.1', '1088'), context(rulesFixture, null, null))).toEqual({});
		// What Validate compares is the configured address, not the one the runtime bound: a wildcard
		// web UI beside a loopback rule, or one on port 0 beside a rule on its ephemeral port, passes.
		const wildcard = { host: '0.0.0.0', port: 1088 };
		expect(errorsOf(listenOn('127.0.0.1', '1088'), context(rulesFixture, null, wildcard))).toEqual(
			{}
		);
		const ephemeral = { host: '127.0.0.1', port: 0 };
		expect(
			errorsOf(listenOn('127.0.0.1', '18080'), context(rulesFixture, null, ephemeral))
		).toEqual({});
		expect(errorsOf(listenOn('127.0.0.1', '1088'), context(rulesFixture, null, ephemeral))).toEqual(
			{}
		);
		// IPv6 hosts follow net.JoinHostPort on both sides.
		const six = { host: '::1', port: 1088 };
		expect(errorsOf(listenOn('::1', '1088'), context(rulesFixture, null, six))).toEqual({
			listenPort: 'listenAddressWebUI'
		});
		expect(errorsOf(listenOn('[::1]', '1088'), context(rulesFixture, null, six))).toEqual({});
		const bracketed = { host: '[::1]', port: 1088 };
		expect(errorsOf(listenOn('::1', '1088'), context(rulesFixture, null, bracketed))).toEqual({});
		expect(errorsOf(listenOn('[::1]', '1088'), context(rulesFixture, null, bracketed))).toEqual({
			listenPort: 'listenAddressWebUI'
		});
		// A rule wins over the web UI in naming the owner.
		const same: Rule = { name: 'ui-twin', listen: { host: '127.0.0.1', port: 1088 }, forward: {} };
		expect(errorsOf(listenOn('127.0.0.1', '1088'), context([same]))).toEqual({
			listenPort: 'listenAddressTaken',
			listenAddressOwner: 'ui-twin'
		});
	});

	test('a virtual listen clashes with an enabled rule on the same channel, on the channel field', () => {
		const draft = emptyDraft();
		draft.name = 'r';
		draft.listen.kind = 'virtual';
		draft.listen.virtual = ' exit ';
		expect(errorsOf(draft, context(virtualRulesFixture))).toEqual({
			listenVirtual: 'listenAddressTaken',
			listenAddressOwner: 'shared-exit'
		});
		expect(errorsOf(draft, context(virtualRulesFixture, 'shared-exit'))).toEqual({});
		const off = structuredClone(virtualRulesFixture);
		off[0].disabled = true;
		expect(errorsOf(draft, context(off))).toEqual({});
		draft.listen.virtual = 'other';
		expect(errorsOf(draft, context(virtualRulesFixture))).toEqual({});
		// A hidden port draft never takes part.
		draft.listen.port = '18098';
		expect(errorsOf(draft, context(rulesFixture))).toEqual({});
	});

	test('hop URLs need a scheme as Go parses one; blank rows are dropped or, alone in a hop, refused', () => {
		const check = (value: string, side: 'listen' | 'forward' = 'forward') => {
			const draft = listenOn('127.0.0.1', '18400');
			draft[side].way = [newHop([value])];
			const result = readRule(draft);
			return result.ok ? null : result.errors.urls?.[draft[side].way[0].urls[0].id];
		};
		for (const url of [
			'socks5://h:1',
			'cmd:nc %h %p',
			'ssh://u@h',
			'nc:ssh jump',
			'SOCKS5://H:1',
			'ss://aes-256-gcm:pass@host:8388',
			'host:1080',
			' socks5://h:1 '
		]) {
			expect(check(url), url).toBeNull();
			expect(check(url, 'listen'), url).toBeNull();
		}
		for (const url of ['127.0.0.1:1080', '//x', '/just/a/path', 'x', ':1080', 'a_b:1']) {
			expect(check(url), url).toBe('hopUrlNoScheme');
			expect(check(url, 'listen'), url).toBe('hopUrlNoScheme');
		}
		expect(check(' ')).toBe('hopUrlEmpty');
		expect(check('', 'listen')).toBe('hopUrlEmpty');
		// Every offending row is reported at once, by id, on both sides.
		const draft = listenOn('127.0.0.1', '18400');
		draft.listen.way = [newHop(['edge.example', 'ssh://ops@edge.example:22'])];
		draft.forward.way = [newHop(['', 'socks5://h:1']), newHop(['  '])];
		expect(errorsOf(draft)).toEqual({
			urls: {
				[draft.listen.way[0].urls[0].id]: 'hopUrlNoScheme',
				[draft.forward.way[1].urls[0].id]: 'hopUrlEmpty'
			}
		});
		// Hops hidden by a virtual endpoint are neither sent nor checked.
		draft.listen.kind = 'virtual';
		draft.listen.virtual = 'entry';
		draft.mode = 'forward';
		draft.target.kind = 'virtual';
		draft.target.virtual = 'exit';
		expect(errorsOf(draft)).toEqual({});
	});
});
