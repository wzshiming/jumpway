import { ApiError, errorMessage } from './api';
import { t } from './i18n.svelte';
import type { Address } from './types';

// Address and form helpers for the Settings and YAML pages.

const WILDCARDS = new Set(['0.0.0.0', '::', '[::]']);

// host:port a browser at `hostname` can use once the web UI listens on `address`; '' when the
// server picks the port (0). Wildcard binds answer on the name this page was loaded from.
export function reachableAddress(address: Address, hostname: string): string {
	if (!address.port) return '';
	let host = address.host || '127.0.0.1';
	if (WILDCARDS.has(host)) host = hostname || '127.0.0.1';
	if (host.includes(':') && !host.startsWith('[')) host = '[' + host + ']';
	return host + ':' + address.port;
}

// A hostname, IPv4 or bracketed IPv6 with an optional port. Checked before parsing because the
// URL parser repairs slashes, backslashes, quotes and credentials into a host that was never typed.
const HOST_PORT = /^(\[[0-9a-f:.]+\]|[^\s\x00-\x1f\x7f[\]/\\?#@:"'`<>]+)(?::(\d{1,5}))?$/i;

// Parses host[:port] as an http origin; an explicit port must be usable (1-65535).
export function addressURL(address: string): URL | null {
	const match = HOST_PORT.exec(address);
	if (!match) return null;
	if (match[2] !== undefined && (Number(match[2]) < 1 || Number(match[2]) > 65535)) return null;
	try {
		const url = new URL('http://' + address);
		return url.username || url.password || url.pathname !== '/' || url.search || url.hash
			? null
			: url;
	} catch {
		return null;
	}
}

const canonicalHost = (url: URL) => (url.hostname === 'localhost' ? '127.0.0.1' : url.hostname);

// The listener as a browser at `origin` reaches it: a wildcard bind answers on the origin's name.
function listenerURL(address: string, origin: URL | null): URL | null {
	const url = addressURL(address);
	if (url && origin && WILDCARDS.has(url.hostname)) url.hostname = origin.hostname;
	return url;
}

const samePort = (a: URL, b: URL) => (a.port || '80') === (b.port || '80');

// True when `address` is where this page is served from. Only names are compared: a loopback
// bind is not reachable from a LAN browser, however local it is to the server.
export function sameAddress(address: string, currentHost: string): boolean {
	const origin = addressURL(currentHost);
	const listener = listenerURL(address, origin);
	if (!listener || !origin) return false;
	return samePort(listener, origin) && canonicalHost(listener) === canonicalHost(origin);
}

const LOOPBACKS = new Set(['127.0.0.1', 'localhost', '[::1]']);

// What a status answer received at `currentHost` stands for; '' when it has no usable port yet.
// The backend reports wildcard binds as 127.0.0.1, so a loopback host on the current port names
// the listener that answered, not a move.
export function reportedAddress(address: string, currentHost: string): string {
	const url = addressURL(address);
	if (!url) return '';
	const origin = addressURL(currentHost);
	return origin && LOOPBACKS.has(url.hostname) && samePort(url, origin) ? currentHost : address;
}

export interface MovedLinkOptions {
	currentHost: string;
	lang: string | null;
	hash: string;
}

// Absolute URL of this page once served from `address`, or null when it already is.
export function movedHref(
	address: string,
	{ currentHost, lang, hash }: MovedLinkOptions
): string | null {
	const url = listenerURL(address, addressURL(currentHost));
	if (!url || sameAddress(address, currentHost)) return null;
	if (lang) url.searchParams.set('lang', lang);
	url.hash = hash;
	return url.href;
}

export function parsePort(value: string): number | null {
	const trimmed = value.trim();
	if (!/^\d{1,5}$/.test(trimmed)) return null;
	const port = Number(trimmed);
	return port > 65535 ? null : port;
}

export const readLines = (value: string): string[] =>
	value
		.split(/\r?\n/)
		.map((line) => line.trim())
		.filter(Boolean);

export interface WriteFailure {
	title: string;
	message: string;
}

// A write without an answer may still have been applied; only an answered failure is a plain error.
export const writeFailure = (reason: unknown): WriteFailure =>
	reason instanceof ApiError && reason.unreachable
		? { title: t('saveUncertain'), message: t('recoveryHint') }
		: { title: t('requestFailed'), message: errorMessage(reason) };
