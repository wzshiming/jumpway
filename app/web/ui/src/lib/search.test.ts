import { describe, expect, test } from 'vitest';
import { fuzzyMatch } from './search';

describe('fuzzyMatch', () => {
	test('matches when the query is a subsequence of any value', () => {
		expect(fuzzyMatch('1809', ['127.0.0.1:18094'], 'en')).toBe(true);
		expect(fuzzyMatch('z8', ['zeta.test:80'], 'en')).toBe(true);
		expect(fuzzyMatch('et:', ['example.com:443', 'zeta.test:80'], 'en')).toBe(true);
		expect(fuzzyMatch('9081', ['127.0.0.1:18094'], 'en')).toBe(false);
		expect(fuzzyMatch('zzz', ['example.com:443', 'curl'], 'en')).toBe(false);
	});

	test('is case-insensitive in the given language and ignores surrounding whitespace', () => {
		expect(fuzzyMatch('CURL', ['curl'], 'en')).toBe(true);
		expect(fuzzyMatch('  cu rl ', ['curl'], 'en')).toBe(false);
		expect(fuzzyMatch('  curl ', ['curl'], 'en')).toBe(true);
		// Turkish lowercases I to dotless ı.
		expect(fuzzyMatch('I', ['\u0131'], 'tr')).toBe(true);
		expect(fuzzyMatch('I', ['\u0131'], 'en')).toBe(false);
	});

	test('an empty or blank query matches everything, even without values', () => {
		expect(fuzzyMatch('', [], 'en')).toBe(true);
		expect(fuzzyMatch('   ', ['x'], 'en')).toBe(true);
		expect(fuzzyMatch('x', [], 'en')).toBe(false);
	});

	test('non-string values are matched by their text; null and undefined are empty', () => {
		expect(fuzzyMatch('42', [null, undefined, 4242], 'en')).toBe(true);
		expect(fuzzyMatch('a', [null, undefined], 'en')).toBe(false);
	});
});
