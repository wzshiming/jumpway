import { expect, test } from 'vitest';
import {
	addressURL,
	movedHref,
	parsePort,
	reachableAddress,
	readLines,
	reportedAddress,
	sameAddress
} from './settings';

// Ports of the legacy submittedAddress/addressURL/sameAddress/showMoved, with the wildcard and
// hash cases the old UI did not cover.

test('reachableAddress: loopback default, wildcard binds use the current hostname, IPv6 is bracketed, port 0 is unknown', () => {
	expect(reachableAddress({ host: '127.0.0.1', port: 1088 }, '127.0.0.1')).toBe('127.0.0.1:1088');
	expect(reachableAddress({ host: '', port: 1099 }, '192.168.1.5')).toBe('127.0.0.1:1099');
	expect(reachableAddress({ host: '0.0.0.0', port: 1099 }, '192.168.1.5')).toBe('192.168.1.5:1099');
	expect(reachableAddress({ host: '[::]', port: 1099 }, 'localhost')).toBe('localhost:1099');
	expect(reachableAddress({ host: '::', port: 1099 }, '')).toBe('127.0.0.1:1099');
	expect(reachableAddress({ host: '::1', port: 1099 }, 'ignored')).toBe('[::1]:1099');
	expect(reachableAddress({ host: '[::1]', port: 1099 }, 'ignored')).toBe('[::1]:1099');
	expect(reachableAddress({ host: '127.0.0.1', port: 0 }, '127.0.0.1')).toBe('');
});

test('addressURL accepts a bare host[:port] as an http origin and nothing else', () => {
	expect(addressURL('127.0.0.1:1099')?.href).toBe('http://127.0.0.1:1099/');
	expect(addressURL('[::1]:1099')?.href).toBe('http://[::1]:1099/');
	expect(addressURL('[2001:db8::1]:8080')?.href).toBe('http://[2001:db8::1]:8080/');
	expect(addressURL('[::]:1099')?.href).toBe('http://[::]:1099/');
	expect(addressURL('0.0.0.0:1099')?.href).toBe('http://0.0.0.0:1099/');
	expect(addressURL('Example.test')?.href).toBe('http://example.test/');
	expect(addressURL('my-host.local:65535')?.href).toBe('http://my-host.local:65535/');
	for (const bad of [
		'',
		'user:pw@host:1',
		'host:1/path',
		'host:1?x=1',
		'host:1#frag',
		'javascript:alert(1)',
		'::1:1099',
		'host:99999',
		'a b',
		'evil.test/x@127.0.0.1:1099',
		// The URL parser would silently repair these into another host; they never were one.
		'//other.example:1099',
		'//other.example',
		'\\other.example:1099',
		'other.example\\x:1099',
		'other.example/:1099',
		'"other.example":1099',
		"other'.example:1099",
		'other.example\t:1099',
		'other.example\u0000:1099',
		'[::1:1099',
		'[::1]x:1099',
		'host:',
		'host:1:2'
	]) {
		expect(addressURL(bad), JSON.stringify(bad)).toBeNull();
	}
});

test('sameAddress: default port 80, localhost is 127.0.0.1, wildcards answer on the current name, loopback is never a LAN name', () => {
	expect(sameAddress('127.0.0.1:1088', '127.0.0.1:1088')).toBe(true);
	expect(sameAddress('localhost:1088', '127.0.0.1:1088')).toBe(true);
	expect(sameAddress('127.0.0.1:1088', 'localhost:1088')).toBe(true);
	expect(sameAddress('192.168.1.5:1088', '192.168.1.5:1088')).toBe(true);
	expect(sameAddress('example.test', 'example.test:80')).toBe(true);
	expect(sameAddress('0.0.0.0:1088', '192.168.1.5:1088')).toBe(true);
	expect(sameAddress('[::]:1088', '[::1]:1088')).toBe(true);
	// Submitting an empty host from a LAN browser binds loopback: that page is no longer served here.
	expect(sameAddress('127.0.0.1:1088', '192.168.1.5:1088')).toBe(false);
	expect(sameAddress('localhost:1088', '192.168.1.5:1088')).toBe(false);
	expect(sameAddress('127.0.0.1:1099', '127.0.0.1:1088')).toBe(false);
	expect(sameAddress('192.168.1.5:1088', '127.0.0.1:1088')).toBe(false);
	expect(sameAddress('[::1]:1088', '127.0.0.1:1088')).toBe(false);
	expect(sameAddress('127.0.0.1:0', '127.0.0.1:0')).toBe(false);
	expect(sameAddress('', '127.0.0.1:1088')).toBe(false);
	expect(sameAddress('127.0.0.1:1088', '')).toBe(false);
});

test('reportedAddress: a loopback host on the current port is the wildcard listener serving this page; port 0 is unusable', () => {
	// The backend reports wildcard binds as 127.0.0.1, so from a LAN browser this is not a move.
	expect(reportedAddress('127.0.0.1:1088', '192.168.1.5:1088')).toBe('192.168.1.5:1088');
	expect(reportedAddress('localhost:1088', '192.168.1.5:1088')).toBe('192.168.1.5:1088');
	expect(reportedAddress('[::1]:1088', '192.168.1.5:1088')).toBe('192.168.1.5:1088');
	expect(reportedAddress('127.0.0.1:1088', '127.0.0.1:1088')).toBe('127.0.0.1:1088');
	// Another port or a real name is taken at face value.
	expect(reportedAddress('127.0.0.1:1099', '192.168.1.5:1088')).toBe('127.0.0.1:1099');
	expect(reportedAddress('192.168.1.5:1088', '10.0.0.5:1088')).toBe('192.168.1.5:1088');
	expect(reportedAddress('0.0.0.0:1099', '192.168.1.5:1088')).toBe('0.0.0.0:1099');
	// Nothing is bound yet, or the address is not one: no link can be made from it.
	expect(reportedAddress('127.0.0.1:0', '192.168.1.5:1088')).toBe('');
	expect(reportedAddress('', '192.168.1.5:1088')).toBe('');
	expect(reportedAddress('//other.example:1099', '192.168.1.5:1088')).toBe('');
});

test('movedHref keeps an explicit ?lang= and the route hash; the same address or unsafe input yields null', () => {
	const options = { currentHost: '127.0.0.1:1088', lang: null, hash: '#/settings' };
	expect(movedHref('127.0.0.1:1099', options)).toBe('http://127.0.0.1:1099/#/settings');
	expect(movedHref('127.0.0.1:1099', { ...options, lang: 'zh' })).toBe(
		'http://127.0.0.1:1099/?lang=zh#/settings'
	);
	expect(movedHref('[::1]:1099', { ...options, hash: '#/yaml' })).toBe('http://[::1]:1099/#/yaml');
	expect(movedHref('192.168.1.5:1088', { ...options, currentHost: 'localhost:1088' })).toBe(
		'http://192.168.1.5:1088/#/settings'
	);
	// A wildcard bind reported by the server answers on the name this page came from.
	expect(movedHref('0.0.0.0:1099', { ...options, currentHost: '192.168.1.5:1088' })).toBe(
		'http://192.168.1.5:1099/#/settings'
	);
	expect(movedHref('[::]:1099', { ...options, currentHost: '[::1]:1088' })).toBe(
		'http://[::1]:1099/#/settings'
	);
	// From a LAN browser, a loopback bind on the same port is a move; a wildcard one is not.
	expect(movedHref('127.0.0.1:1088', { ...options, currentHost: '192.0.2.20:1088' })).toBe(
		'http://127.0.0.1:1088/#/settings'
	);
	expect(movedHref('0.0.0.0:1088', { ...options, currentHost: '192.0.2.20:1088' })).toBeNull();
	// Port 0 is not bound yet: never a destination.
	expect(movedHref('127.0.0.1:0', options)).toBeNull();
	expect(movedHref('0.0.0.0:0', options)).toBeNull();
	expect(movedHref('127.0.0.1:1088', options)).toBeNull();
	expect(movedHref('localhost:1088', options)).toBeNull();
	expect(movedHref('0.0.0.0:1088', options)).toBeNull();
	expect(movedHref('', options)).toBeNull();
	expect(movedHref('evil.test/../x@127.0.0.1:1099', options)).toBeNull();
	expect(movedHref('javascript:alert(1)', options)).toBeNull();
	expect(movedHref('u:p@127.0.0.1:1099', options)).toBeNull();
	expect(movedHref('//other.example:1099', options)).toBeNull();
	expect(movedHref('\\\\other.example:1099', options)).toBeNull();
});

test('parsePort accepts whole decimal numbers from 0 to 65535 only', () => {
	expect(parsePort('0')).toBe(0);
	expect(parsePort(' 1088 ')).toBe(1088);
	expect(parsePort('65535')).toBe(65535);
	for (const bad of ['', ' ', '-1', '65536', '70000', '1.5', '1e3', '0x10', '+80', 'abc']) {
		expect(parsePort(bad), bad).toBeNull();
	}
});

test('readLines trims every line and drops blank ones, keeping order and duplicates', () => {
	expect(readLines(' a.example \r\n\n  \n10.0.0.0/8\n a.example\n')).toEqual([
		'a.example',
		'10.0.0.0/8',
		'a.example'
	]);
	expect(readLines('')).toEqual([]);
});
