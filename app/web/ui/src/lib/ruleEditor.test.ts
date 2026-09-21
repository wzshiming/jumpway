import { describe, expect, test } from 'vitest';
import { rulesFixture } from '../../e2e/fixtures/api';
import {
	draftFrom,
	emptyDraft,
	newHop,
	PROTOCOLS,
	readRule,
	snapshot,
	type ProtocolDraft,
	type RuleDraft
} from './ruleEditor';
import type { Protocol, Rule } from './types';

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
		});
		expect(passwordOnly.listen).toEqual({
			host: '127.0.0.1',
			port: 0,
			password: 'secret',
			protocols: LEGACY
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

	test('ways drop blank URLs and hops left without URLs, trimming what remains', () => {
		const rule = valid((draft) => {
			draft.listen.way = [newHop(['ssh://ops@edge.example:22 '])];
			draft.forward.way = [
				newHop(['', '  ']),
				newHop([' socks5://hop-a.example:1080', 'ssh://ops@bastion.example:22']),
				newHop([])
			];
		});
		expect(rule.listen.way).toEqual([{ lb: ['ssh://ops@edge.example:22'] }]);
		expect(rule.forward.way).toEqual([
			{ lb: ['socks5://hop-a.example:1080', 'ssh://ops@bastion.example:22'] }
		]);
		expect(valid((draft) => (draft.forward.way = [newHop([''])])).forward).toEqual({});
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
