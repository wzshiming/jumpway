import { describe, expect, test } from 'vitest';
import { rulesFixture } from '../../e2e/fixtures/api';
import { draftFrom, emptyDraft, newHop, readRule, snapshot, type RuleDraft } from './ruleEditor';
import type { Rule } from './types';

const [office, mirror, dbTunnel, lab] = rulesFixture;

const urlsOf = (draft: RuleDraft, side: 'listen' | 'forward') =>
	draft[side].way.map((hop) => hop.urls.map((url) => url.value));

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
		expect(draft.target).toEqual({ host: '', port: '' });
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
		expect(tunnel.target).toEqual({ host: '127.0.0.1', port: '5432' });
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
		expect(draft.target).toEqual({ host: '', port: '8080' });
		expect(urlsOf(draft, 'forward')).toEqual([['ssh://a:22', 'ssh://b:22']]);
	});

	test('emptyDraft is a proxy rule on 127.0.0.1:0 without hops, as the legacy new-rule form', () => {
		const draft = emptyDraft();
		expect(draft).toMatchObject({
			name: '',
			enabled: true,
			mode: 'proxy',
			listen: { host: '127.0.0.1', port: '0', username: '', password: '', way: [] },
			target: { host: '', port: '' },
			forward: { way: [] }
		});
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
			rule: { name: 'office', listen: { host: '0.0.0.0', port: 18097 }, forward: {} }
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
			password: ' p@ss '
		});
		const passwordOnly = valid((draft) => {
			draft.listen.password = 'secret';
		});
		expect(passwordOnly.listen).toEqual({ host: '127.0.0.1', port: 0, password: 'secret' });
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
