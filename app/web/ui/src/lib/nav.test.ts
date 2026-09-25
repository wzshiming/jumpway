import { afterEach, beforeEach, expect, test } from 'vitest';
import { i18n } from './i18n.svelte';
import { NAV_ITEMS, pageTitle } from './nav';
import { routeFor, type PageKind } from './routes';

// Record<PageKind, …> keeps this list exhaustive at compile time.
const KINDS = Object.keys({
	overview: true,
	new: true,
	rule: true,
	stats: true,
	hosts: true,
	connections: true,
	settings: true,
	yaml: true
} satisfies Record<PageKind, true>) as PageKind[];

beforeEach(() => {
	history.replaceState(null, '', '/');
	localStorage.clear();
	i18n.init();
});

afterEach(() => {
	i18n.dispose();
});

test('every page kind belongs to exactly one navigation entry, and each entry is current for its own hash', () => {
	for (const kind of KINDS) {
		expect(NAV_ITEMS.filter((item) => item.kinds.includes(kind))).toHaveLength(1);
	}
	expect(new Set(NAV_ITEMS.map((item) => item.hash)).size).toBe(NAV_ITEMS.length);
	expect(new Set(NAV_ITEMS.map((item) => item.icon)).size).toBe(NAV_ITEMS.length);
	for (const item of NAV_ITEMS) {
		expect(item.kinds).toContain(routeFor(item.hash).kind);
		expect(typeof item.icon).toBe('function');
	}
	// Rules are managed on the overview, so its entry stays current while one is being edited.
	expect(NAV_ITEMS).toHaveLength(6);
	expect(NAV_ITEMS.map((item) => item.hash)).not.toContain('#/rules');
	const hashFor = (kind: PageKind) => NAV_ITEMS.find((item) => item.kinds.includes(kind))?.hash;
	expect(hashFor('rule')).toBe('#/');
	expect(hashFor('new')).toBe('#/');
});

test('pageTitle names ordinary pages by their entry, the new-rule page by itself and the editor by its rule', () => {
	const titles = (hashes: string[]) => hashes.map((hash) => pageTitle(routeFor(hash)));
	expect(titles(NAV_ITEMS.map((item) => item.hash))).toEqual([
		'Overview',
		'Rule Traffic',
		'Proxy Hosts',
		'Live Connections',
		'Global Settings',
		'Configuration File'
	]);
	expect(titles(['#/new', '#/rules/office', '#/rules/a%2Fb', '#/nowhere'])).toEqual([
		'New rule',
		'office',
		'a/b',
		'Overview'
	]);
	i18n.setLanguage('zh');
	expect(titles(['#/', '#/connections', '#/new', '#/rules/office'])).toEqual([
		'概览',
		'活动连接',
		'新建规则',
		'office'
	]);
});
