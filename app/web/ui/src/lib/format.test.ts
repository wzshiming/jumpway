import { afterEach, describe, expect, test } from 'vitest';
import {
	displayClient,
	displayURL,
	formatAgo,
	formatBytes,
	formatCount,
	formatDateTime,
	formatDuration,
	formatElapsed,
	formatLatency,
	formatRate,
	redactCredentials
} from './format';
import { i18n } from './i18n.svelte';
import { normalizeWay, serializeWay } from './way';

const NOW = Date.parse('2026-09-18T12:00:00Z');

afterEach(() => i18n.dispose());

describe('normalizeWay (legacy shapes)', () => {
	test('accepts null, "a|b" strings, arrays and {lb} objects', () => {
		expect(normalizeWay(null)).toEqual([]);
		expect(normalizeWay(undefined)).toEqual([]);
		expect(normalizeWay(['a|b', ['c'], { lb: ['d', 'e'] }, {}, { lb: null }])).toEqual([
			{ lb: ['a', 'b'] },
			{ lb: ['c'] },
			{ lb: ['d', 'e'] },
			{ lb: [] },
			{ lb: [] }
		]);
	});

	test('returns fresh arrays so the editor can mutate them', () => {
		const source = [{ lb: ['x'] }];
		const hops = normalizeWay(source);
		hops[0].lb.push('y');
		expect(source[0].lb).toEqual(['x']);
	});
});

describe('serializeWay', () => {
	test('trims URLs, drops blank URLs and hops left empty', () => {
		expect(
			serializeWay([
				{ lb: [' socks5://a:1080 ', '', '  '] },
				{ lb: ['', ' '] },
				{ lb: ['ssh://b'] }
			])
		).toEqual([{ lb: ['socks5://a:1080'] }, { lb: ['ssh://b'] }]);
		expect(serializeWay([{ lb: [''] }])).toEqual([]);
	});
});

describe('formatBytes / formatRate', () => {
	test.each([
		[0, '0 B'],
		[1023, '1023 B'],
		[1024, '1.0 KB'],
		[1536, '1.5 KB'],
		[1048570, '1.0 MB'],
		[5 * 1024 ** 3, '5.0 GB'],
		[3 * 1024 ** 4, '3.0 TB'],
		[2 ** 60, '1048576.0 TB'],
		[-5, '0 B'],
		[NaN, '0 B'],
		[undefined, '0 B']
	])('%s → %s', (input, expected) => {
		expect(formatBytes(input)).toBe(expected);
	});

	test('rate appends /s', () => {
		expect(formatRate(2048)).toBe('2.0 KB/s');
	});
});

describe('formatLatency', () => {
	test('shows a dash unless at least one dial succeeded', () => {
		expect(formatLatency(undefined)).toBe('\u2014');
		expect(formatLatency({ dials: 3, dial_failures: 3, latency_ms: 12 })).toBe('\u2014');
		expect(formatLatency({ dials: 3, dial_failures: 1, latency_ms: 12.345 })).toBe('12.3 ms');
		expect(formatLatency({ dials: 3, dial_failures: 1, avg_latency_ms: 7 }, 'avg_latency_ms')).toBe(
			'7.0 ms'
		);
	});
});

describe('durations', () => {
	test.each([
		[0, '0 s', '0 s'],
		[59, '59 s', '59 s'],
		[60, '1 min 0 s', '1 min'],
		[3599, '59 min 59 s', '59 min'],
		[3600, '1 h 0 min', '1 h'],
		[90061, '25 h 1 min', '25 h']
	])('%d s → %s / short %s', (seconds, long, short) => {
		expect(formatElapsed(seconds)).toBe(long);
		expect(formatElapsed(seconds, true)).toBe(short);
	});

	test('formatDuration counts from the timestamp, clamps the future and rejects garbage', () => {
		expect(formatDuration('2026-09-18T10:58:30Z', false, NOW)).toBe('1 h 1 min');
		expect(formatDuration('2026-09-18T10:58:30Z', true, NOW)).toBe('1 h');
		expect(formatDuration('2026-09-18T12:00:05Z', false, NOW)).toBe('0 s');
		expect(formatDuration('garbage', false, NOW)).toBe('\u2014');
		expect(formatDuration(undefined, false, NOW)).toBe('\u2014');
	});

	test('formatAgo localises the suffix and keeps the dash', () => {
		i18n.init();
		expect(formatAgo('2026-09-18T11:59:15Z', NOW)).toBe('45 s ago');
		expect(formatAgo(null, NOW)).toBe('\u2014');
		i18n.setLanguage('zh');
		expect(formatAgo('2026-09-18T10:58:30Z', NOW)).toBe('1 h前');
	});

	test('formatDateTime renders a locale string or a dash', () => {
		expect(formatDateTime('2026-09-18T11:59:15Z')).toBe(
			new Date(NOW - 45_000).toLocaleString('en')
		);
		expect(formatDateTime('nope')).toBe('\u2014');
	});
});

describe('formatCount', () => {
	test('uses the active locale with no fraction digits', () => {
		i18n.init();
		expect(formatCount(1234567.8)).toBe('1,234,568');
		expect(formatCount(undefined)).toBe('0');
	});
});

describe('displayURL', () => {
	test.each([
		['socks5://user:p%40ss@example.com:1080', 'socks5://example.com:1080'],
		['http://h:80', 'http://h:80'],
		['https://h:443', 'https://h:443'],
		['https://h', 'https://h'],
		['ssh://user@[::1]:22?identity_file=x', 'ssh://[::1]:22'],
		['ss://aes-256-gcm:secret@host:8379', 'ss://host:8379'],
		['socks5://h:1080/path?x=1#f', 'socks5://h:1080'],
		['cmd:ssh -W %h:%p jump', 'cmd:ssh -W %h:%p jump'],
		['nc:host:22', 'nc:host:22'],
		['not a url', 'not a url'],
		['', ''],
		[null, '']
	])('%s → %s', (input, expected) => {
		expect(displayURL(input)).toBe(expected);
	});
});

describe('redactCredentials', () => {
	test.each([
		['socks5://user:secret@example.com:1080', 'socks5://xxxxx@example.com:1080'],
		['socks5://review-user:review-secret@host:bad', 'socks5://xxxxx@host:bad'],
		["socks5://user:secret'one@host:1080", 'socks5://xxxxx@host:1080'],
		["socks5://user:secret'one@host:bad", 'socks5://xxxxx@host:bad'],
		['socks5://user:S3c/r3t@host:1080', 'socks5://xxxxx@host:1080'],
		['socks5://user:S3c?r3t@host:1080', 'socks5://xxxxx@host:1080'],
		['socks5://user:S3c#r3t@host:1080', 'socks5://xxxxx@host:1080'],
		[String.raw`socks5://user:secret\"one@host:bad`, 'socks5://xxxxx@host:bad'],
		['socks5://u:p%40ss@h:1080', 'socks5://xxxxx@h:1080'],
		['socks5://u:p@ss@h:1080', 'socks5://xxxxx@h:1080'],
		['ss://aes-256-gcm:secret@host:8379', 'ss://xxxxx@host:8379'],
		['ssh://ops@bastion.example:22', 'ssh://xxxxx@bastion.example:22'],
		[
			'ssh://user:pw@[::1]:22?identity_file=~%2F.ssh%2Fid',
			'ssh://xxxxx@[::1]:22?identity_file=~%2F.ssh%2Fid'
		],
		['SOCKS5://User:Secret@Host:1080', 'SOCKS5://xxxxx@Host:1080'],
		['socks5://host:1080', 'socks5://host:1080'],
		['http://h:80/path?x=1', 'http://h:80/path?x=1'],
		['socks5://@host:1080', 'socks5://@host:1080'],
		['cmd:ssh -W %h:%p jump', 'cmd:ssh -W %h:%p jump'],
		['nc:host:22', 'nc:host:22'],
		['mail ops@example.com', 'mail ops@example.com'],
		['', ''],
		[null, '']
	])('%s → %s', (input, expected) => {
		expect(redactCredentials(input)).toBe(expected);
	});

	test('masks every URL in a Go validation error and leaves the rest of the text intact', () => {
		const url = 'socks5://review-user:review-secret@host:bad';
		const error = `rules[3].forward.way[0]: invalid proxy URL "${url}": parse "${url}": invalid port ":bad" after host (e.g. socks5://host:1080)`;
		expect(redactCredentials(error)).toBe(
			'rules[3].forward.way[0]: invalid proxy URL "socks5://xxxxx@host:bad": parse "socks5://xxxxx@host:bad": invalid port ":bad" after host (e.g. socks5://host:1080)'
		);
		expect(
			redactCredentials('saved, but dial ssh://ops:pw@edge.example:22: connection refused.')
		).toBe('saved, but dial ssh://xxxxx@edge.example:22: connection refused.');
	});
});

describe('displayClient', () => {
	test.each([
		['127.0.0.1:53422', '127.0.0.1'],
		['[::1]:53422', '::1'],
		['::1', '::1'],
		['host:1', 'host'],
		['bad host:xx', 'bad host:xx'],
		['', '\u2014'],
		[undefined, '\u2014']
	])('%s → %s', (input, expected) => {
		expect(displayClient(input)).toBe(expected);
	});
});
