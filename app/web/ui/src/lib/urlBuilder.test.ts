import { describe, expect, test } from 'vitest';
import { buildURL, fieldsOf, initialValues, layoutFor, layouts, parseURL } from './urlBuilder';

const layout = (name: string) => {
	const found = layoutFor(name);
	if (!found) throw new Error('missing layout ' + name);
	return found;
};

describe('layouts', () => {
	test('ship the eight protocols in order with ss as an alias', () => {
		expect(layouts.map((entry) => entry.name)).toEqual([
			'https',
			'http',
			'socks4',
			'socks4a',
			'socks5',
			'socks5h',
			'ssh',
			'shadowsocks'
		]);
		expect(layoutFor('ss')?.name).toBe('shadowsocks');
		expect(layoutFor('SOCKS5')?.name).toBe('socks5');
		expect(layoutFor('cmd')).toBeUndefined();
	});

	test('fields exclude spans and initial values apply defaults under prefill', () => {
		expect(fieldsOf(layout('ssh')).map((field) => field.name)).toEqual([
			'username',
			'password',
			'host',
			'port',
			'identity'
		]);
		expect(initialValues(layout('shadowsocks'))).toEqual({
			encrypto: 'aes-256-gcm',
			password: '',
			host: '',
			port: '8379'
		});
		expect(initialValues(layout('http'), { host: 'h', port: undefined })).toEqual({
			username: '',
			password: '',
			host: 'h',
			port: '80'
		});
	});
});

describe('buildURL goldens', () => {
	test('encodes credentials', () => {
		expect(
			buildURL(layout('socks5'), { username: 'u', password: 'p@ss', host: 'h', port: '1080' })
		).toEqual({ url: 'socks5://u:p%40ss@h:1080', valid: true, hint: null });
	});

	test('shadowsocks puts the cipher in the userinfo and falls back to the default port', () => {
		expect(
			buildURL(layout('shadowsocks'), { encrypto: 'aes-256-gcm', password: 'secret', host: 'host' })
		).toEqual({ url: 'ss://aes-256-gcm:secret@host:8379', valid: true, hint: null });
	});

	test('ssh appends an encoded identity_file', () => {
		expect(
			buildURL(layout('ssh'), {
				username: 'user',
				host: 'host',
				port: '22',
				identity: '~/.ssh/id_ed25519'
			}).url
		).toBe('ssh://user@host:22?identity_file=~%2F.ssh%2Fid_ed25519');
		expect(buildURL(layout('ssh'), { host: 'host', port: '22' }).url).toBe('ssh://host:22');
	});

	test('brackets bare IPv6 hosts once', () => {
		expect(buildURL(layout('socks5'), { host: '::1', port: '1080' }).url).toBe(
			'socks5://[::1]:1080'
		);
		expect(buildURL(layout('socks5'), { host: ' [::1] ', port: '1080' }).url).toBe(
			'socks5://[::1]:1080'
		);
	});

	test('username-only layouts and password-only credentials', () => {
		expect(buildURL(layout('socks4'), { username: 'u', host: 'h', port: '1080' }).url).toBe(
			'socks4://u@h:1080'
		);
		expect(buildURL(layout('http'), { password: 'p', host: 'h' }).url).toBe('http://:p@h:80');
	});
});

describe('buildURL validation', () => {
	test('requires a host and a port within 1..65535', () => {
		const socks5 = layout('socks5');
		expect(buildURL(socks5, { host: '', port: '1080' }).valid).toBe(false);
		expect(buildURL(socks5, { host: 'h', port: 'abc' }).valid).toBe(false);
		expect(buildURL(socks5, { host: 'h', port: '0' }).valid).toBe(false);
		expect(buildURL(socks5, { host: 'h', port: '70000' }).valid).toBe(false);
		expect(buildURL(socks5, { host: 'h', port: '65535' }).valid).toBe(true);
		expect(buildURL(socks5, { host: 'h', port: '1' }).hint).toBeNull();
		expect(buildURL(socks5, { host: 'h', port: '' })).toMatchObject({
			url: 'socks5://h:1080',
			valid: true
		});
		expect(buildURL(socks5, { host: 'h', port: 'abc' }).hint).toBe('builderHint');
	});

	test('shadowsocks also needs a cipher and a password', () => {
		const ss = layout('shadowsocks');
		expect(buildURL(ss, { encrypto: 'aes-256-gcm', host: 'h', port: '1' })).toMatchObject({
			valid: false,
			hint: 'builderShadowsocksHint'
		});
		expect(buildURL(ss, { encrypto: '', password: 'x', host: 'h', port: '1' }).valid).toBe(false);
	});
});

describe('parseURL', () => {
	test('prefills every golden and round-trips through buildURL', () => {
		for (const golden of [
			'socks5://u:p%40ss@h:1080',
			'ss://aes-256-gcm:secret@host:8379',
			'ssh://user@host:22?identity_file=~%2F.ssh%2Fid_ed25519',
			'socks5://[::1]:1080'
		]) {
			const parsed = parseURL(golden);
			expect(parsed).not.toBeNull();
			expect(buildURL(parsed!.layout, parsed!.values).url).toBe(golden);
		}
	});

	test('decodes credentials and reports the cipher for shadowsocks', () => {
		expect(parseURL('socks5://u:p%40ss@h:1080')).toEqual({
			layout: layout('socks5'),
			values: {
				username: 'u',
				password: 'p@ss',
				host: 'h',
				port: '1080',
				identity: '',
				encrypto: 'u'
			}
		});
		expect(parseURL('ss://aes-256-gcm:secret@host:8379')?.values).toMatchObject({
			encrypto: 'aes-256-gcm',
			password: 'secret'
		});
	});

	test('missing ports stay undefined so the layout default applies', () => {
		expect(parseURL('http://h')?.values.port).toBeUndefined();
		expect(initialValues(layout('http'), parseURL('http://h')!.values).port).toBe('80');
	});

	test('keeps malformed percent-encodings verbatim instead of throwing', () => {
		expect(parseURL('socks5://%E0%A4%A:x@h:1080')?.values.username).toBe('%E0%A4%A');
	});

	test('returns null for unknown schemes and non-URLs', () => {
		expect(parseURL('cmd:ssh -W %h:%p jump')).toBeNull();
		expect(parseURL('not a url')).toBeNull();
		expect(parseURL('')).toBeNull();
	});
});
