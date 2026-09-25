import { i18n, t } from './i18n.svelte';
import type { Stats } from './types';

export const DASH = '\u2014';

export const text = (value: unknown) => String(value ?? '');

const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'];

export function formatBytes(value: unknown): string {
	const bytes = Math.max(0, Number(value) || 0);
	let unit = Math.max(0, Math.min(4, Math.floor(Math.log2(bytes || 1) / 10)));
	// 1048570 B would otherwise print as "1024.0 KB".
	if (unit < 4 && (bytes / 1024 ** unit).toFixed(1) === '1024.0') unit++;
	return (bytes / 1024 ** unit).toFixed(unit ? 1 : 0) + ' ' + UNITS[unit];
}

export const formatRate = (value: unknown): string => formatBytes(value) + '/s';

export function formatLatency(
	stats: Partial<Stats> | null | undefined,
	key: 'latency_ms' | 'avg_latency_ms' = 'latency_ms'
): string {
	const entry = stats ?? {};
	return (entry.dials || 0) <= (entry.dial_failures || 0)
		? DASH
		: Number(entry[key] || 0).toFixed(1) + ' ms';
}

function formatElapsed(seconds: number, short = false): string {
	if (seconds < 60) return seconds + ' s';
	if (seconds < 3600) {
		return Math.floor(seconds / 60) + ' min' + (short ? '' : ' ' + (seconds % 60) + ' s');
	}
	return (
		Math.floor(seconds / 3600) +
		' h' +
		(short ? '' : ' ' + Math.floor((seconds % 3600) / 60) + ' min')
	);
}

export function formatDuration(value: unknown, short = false, now = Date.now()): string {
	const timestamp = Date.parse(text(value));
	if (!Number.isFinite(timestamp)) return DASH;
	return formatElapsed(Math.max(0, Math.floor((now - timestamp) / 1000)), short);
}

export function formatAgo(value: unknown, now = Date.now()): string {
	const duration = formatDuration(value, true, now);
	return duration === DASH ? duration : t('ago', { time: duration });
}

export function formatDateTime(value: unknown): string {
	const timestamp = Date.parse(text(value));
	return Number.isFinite(timestamp) ? new Date(timestamp).toLocaleString(i18n.language) : DASH;
}

const counters = new Map<string, Intl.NumberFormat>();

export function formatCount(value: unknown): string {
	const language = i18n.language;
	let formatter = counters.get(language);
	if (!formatter) {
		formatter = new Intl.NumberFormat(language, { maximumFractionDigits: 0 });
		counters.set(language, formatter);
	}
	return formatter.format(Number(value) || 0);
}

export function displayURL(value: unknown): string {
	const raw = text(value);
	try {
		const url = new URL(raw);
		if (!url.hostname) return raw;
		// Re-parse under an unknown scheme so default ports (http:80) survive.
		const authority = new URL(raw.trim().replace(/^[a-z][a-z\d+.-]*:/i, 'endpoint:'));
		return url.protocol + '//' + url.hostname + (authority.port ? ':' + authority.port : '');
	} catch {
		return raw;
	}
}

export function displayClient(value: unknown): string {
	const raw = text(value);
	if (!raw) return DASH;
	try {
		return new URL('http://' + raw).hostname.replace(/^\[|\]$/g, '');
	} catch {
		return raw;
	}
}

// Go %q escapes embedded quotes but leaves apostrophes in credentials unchanged.
const URL_TOKEN = /[a-z][a-z\d+.-]*:\/\/(?:\\"|[^\s"])+/gi;
const AUTHORITY = /^([a-z][a-z\d+.-]*:\/\/)([^/?#]*)/i;
const MASK = 'xxxxx';

// The whole userinfo is masked: for ss:// the "username" is the secret.
export function redactURL(token: string): string {
	const match = AUTHORITY.exec(token);
	if (!match) return token;
	const [, scheme] = match;
	let span = match[2];
	try {
		const url = new URL(token);
		if (url.username === '' && url.password === '') return token;
	} catch {
		span = token.slice(scheme.length);
	}
	const separator = span.lastIndexOf('@');
	if (separator <= 0) return token;
	return scheme + MASK + span.slice(separator) + token.slice(scheme.length + span.length);
}

export const redactCredentials = (value: unknown): string =>
	text(value).replace(URL_TOKEN, redactURL);
