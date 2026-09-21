import { describe, expect, test } from 'vitest';
import { buildURL, fieldsOf, initialValues, layoutFor, layouts, parseURL } from './urlBuilder';

const layout = (name: string) => {
	const found = layoutFor(name);
	if (!found) throw new Error('missing layout ' + name);
	return found;
};

describe('layouts', () => {
	test('ship the ten protocols in order with ss/cmd/nc as aliases', () => {
		expect(layouts.map((entry) => entry.name)).toEqual([
			'https',
			'http',
			'socks4',
			'socks4a',
			'socks5',
			'socks5h',
			'ssh',
			'shadowsocks',
			'command',
			'netcat'
		]);
		expect(layoutFor('ss')?.name).toBe('shadowsocks');
		expect(layoutFor('SOCKS5')?.name).toBe('socks5');
		for (const alias of ['cmd', 'CMD', 'command', 'Command']) {
			expect(layoutFor(alias)?.name).toBe('command');
		}
		for (const alias of ['nc', 'NC', 'netcat', 'NetCat']) {
			expect(layoutFor(alias)?.name).toBe('netcat');
		}
		expect(layoutFor('telnet')).toBeUndefined();
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

	test('command layouts expose a single command field with placeholders', () => {
		expect(fieldsOf(layout('command'))).toEqual([
			{ name: 'command', kind: 'text', placeholder: 'nc %h %p' }
		]);
		expect(fieldsOf(layout('netcat'))).toEqual([
			{
				name: 'command',
				kind: 'text',
				label: 'field.commandPrefix',
				option: true,
				placeholder: 'ssh jump'
			}
		]);
		expect(initialValues(layout('command'))).toEqual({ command: '' });
		expect(initialValues(layout('netcat'), { command: 'ssh jump' })).toEqual({
			command: 'ssh jump'
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
		expect(parseURL('telnet:ssh -W %h:%p jump')).toBeNull();
		expect(parseURL('not a url')).toBeNull();
		expect(parseURL('')).toBeNull();
		expect(parseURL(':')).toBeNull();
	});
});

describe('command layouts', () => {
	test('cmd: round-trips the raw command through parseURL and buildURL', () => {
		const parsed = parseURL('cmd:ssh -W %h:%p jump');
		expect(parsed?.layout.name).toBe('command');
		expect(parsed?.values.command).toBe('ssh -W %h:%p jump');
		expect(buildURL(parsed!.layout, parsed!.values)).toEqual({
			url: 'cmd:ssh -W %h:%p jump',
			valid: true,
			hint: null
		});
	});

	test('buildURL writes the command raw, trimmed only at the ends', () => {
		const cmd = layout('command');
		expect(buildURL(cmd, { command: '  ssh -W %h:%p "my jump"  ' }).url).toBe(
			'cmd:ssh -W %h:%p "my jump"'
		);
		expect(buildURL(cmd, { command: 'a  b?c#d/é' }).url).toBe('cmd:a  b?c#d/é');
		expect(buildURL(cmd, { command: 'x', host: 'h', port: '22' }).url).toBe('cmd:x');
	});

	test('command requires a non-blank command while netcat does not', () => {
		const cmd = layout('command');
		expect(buildURL(cmd, {})).toEqual({ url: 'cmd:', valid: false, hint: 'builderCommandHint' });
		expect(buildURL(cmd, { command: '   ' })).toEqual({
			url: 'cmd:',
			valid: false,
			hint: 'builderCommandHint'
		});
		const nc = layout('netcat');
		expect(buildURL(nc, {})).toEqual({ url: 'nc:', valid: true, hint: null });
		expect(buildURL(nc, { command: ' ssh jump ' })).toEqual({
			url: 'nc:ssh jump',
			valid: true,
			hint: null
		});
	});

	test('parseURL keeps the remainder verbatim and normalizes long aliases', () => {
		const raw = ' ssh  -W "%h:%p" %25 ?a=1#frag é ';
		expect(parseURL('cmd:' + raw)).toEqual({ layout: layout('command'), values: { command: raw } });
		expect(parseURL('CMD:x')?.layout.name).toBe('command');
		expect(parseURL('nc:')).toEqual({ layout: layout('netcat'), values: { command: '' } });
		for (const [input, url] of [
			['command:ssh -W %h:%p jump', 'cmd:ssh -W %h:%p jump'],
			['netcat:ssh jump', 'nc:ssh jump'],
			['NC:ssh jump', 'nc:ssh jump']
		]) {
			const parsed = parseURL(input)!;
			expect(buildURL(parsed.layout, parsed.values).url).toBe(url);
		}
	});
});
