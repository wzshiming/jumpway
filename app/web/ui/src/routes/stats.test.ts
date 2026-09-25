import { flushSync, mount, unmount, tick } from 'svelte';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
	hostsTotals,
	rulesFixture,
	snapshotFixture,
	statsOf,
	statusFixture,
	virtualRulesFixture
} from '../../e2e/fixtures/api';
import App from '../App.svelte';
import { formatDateTime } from '../lib/format';
import { stats } from '../lib/stats.svelte';
import { toasts } from '../lib/toast.svelte';
import { DIRECTIONS, TRAFFIC_METRICS } from '../lib/traffic';
import type { Rule, Snapshot } from '../lib/types';

// Statistics, Hosts and Connections mounted in jsdom against an in-memory /apis stub whose
// snapshot the tests mutate between polls.

interface Call {
	method: string;
	url: string;
}

let target: HTMLElement;
let app: ReturnType<typeof mount> | null = null;
let calls: Call[];
let snapshot: Snapshot;
let rules: Rule[];
// `${method} ${url}` → 400 text.
let fail: Map<string, string>;
// `${method} ${url}` that fail at the network level.
let down: Set<string>;
// `${method} ${url}` whose next answer waits for the promise.
let delay: Map<string, Promise<void>>;

const NOW = Date.parse('2026-09-19T09:00:00Z');

const json = (body: unknown) =>
	new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});
const text = (status: number, body: string) => new Response(body, { status });

function stubApi() {
	calls = [];
	snapshot = structuredClone(snapshotFixture);
	rules = structuredClone(rulesFixture);
	fail = new Map();
	down = new Set();
	delay = new Map();
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string, init?: RequestInit) => {
			const method = init?.method ?? 'GET';
			calls.push({ method, url });
			const entry = `${method} ${url}`;
			if (down.has(entry)) throw new TypeError('Failed to fetch');
			const held = delay.get(entry);
			if (held) {
				delay.delete(entry);
				await held;
			}
			const injected = fail.get(entry);
			if (injected !== undefined) return text(400, injected);
			if (url === '/apis/configs/status') return json(statusFixture);
			if (url === '/apis/configs/rules') return json(rules);
			if (url === '/apis/stats') {
				if (method === 'GET') return json(snapshot);
				return json(null);
			}
			const match = /^\/apis\/stats\/connections\/(\d+)$/.exec(url);
			if (match && method === 'DELETE') {
				const id = Number(match[1]);
				for (const rule of snapshot.rules ?? []) {
					const index = rule.connections?.findIndex((connection) => connection.id === id) ?? -1;
					if (index >= 0) {
						rule.connections!.splice(index, 1);
						return json(null);
					}
				}
				return text(400, `connection ${id} not found`);
			}
			return text(404, 'not found');
		})
	);
}

const settle = () => vi.advanceTimersByTimeAsync(0);
const poll = () => vi.advanceTimersByTimeAsync(1_000);
const requested = (entry: string) =>
	calls.filter((call) => `${call.method} ${call.url}` === entry).length;
const compact = (element: Element | null | undefined) =>
	element?.textContent?.replace(/\s+/g, ' ').trim() ?? null;

const click = (element: Element | null | undefined) => {
	if (!element) throw new Error('missing element');
	element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
	flushSync();
};

const button = (name: string, root: ParentNode = target) =>
	Array.from(root.querySelectorAll('button')).find(
		(element) => element.textContent?.trim() === name || element.getAttribute('aria-label') === name
	);

const confirmDialog = () =>
	target.querySelector<HTMLDialogElement>('dialog[open][aria-describedby="confirm-message"]');
const rows = (selector = 'main article[data-rule]') =>
	Array.from(target.querySelectorAll<HTMLElement>(selector));
// The eight traffic counters in canonical order (upload first), read from the first band under
// the element whatever the band's own reading order.
const TRAFFIC_KEYS = DIRECTIONS.flatMap((direction) =>
	TRAFFIC_METRICS.map((metric) => direction[metric])
);
const traffic = (root: Element | null | undefined) =>
	TRAFFIC_KEYS.map((key) => compact(root?.querySelector(`[data-traffic] [data-metric="${key}"]`)));
// One band as its metric columns: each column names its counter and holds the upload value
// above the download value.
const columnsOf = (band: Element) =>
	Array.from(band.querySelectorAll<HTMLElement>('[data-column]')).map((column) => [
		column.dataset.column,
		Array.from(column.querySelectorAll<HTMLElement>('[data-metric]')).map(
			(element) => element.dataset.metric
		)
	]);
const METRIC_COLUMNS = TRAFFIC_METRICS.map((metric) => [
	metric,
	DIRECTIONS.map((direction) => direction[metric])
]);
// The expanded content a toggle controls.
const detailsOf = (toggle: Element) =>
	document.getElementById(toggle.getAttribute('aria-controls')!)!;
// The first detail band under an element (an item's own band, never a nested hop's).
const bandOf = (root: Element) => root.querySelector('[data-traffic]')!;
const labels = (root: Element) => Array.from(root.querySelectorAll('dt')).map(compact);
// The metrics an item shows outside its expanded content, keyed by counter.
const ownMetricsOf = (item: Element) => {
	const controls = item.querySelector('button[aria-controls]')?.getAttribute('aria-controls');
	const expanded = controls ? document.getElementById(controls) : null;
	return Object.fromEntries(
		Array.from(item.querySelectorAll<HTMLElement>('[data-metric]'))
			.filter((element) => !expanded?.contains(element))
			.map((element) => [element.dataset.metric, compact(element)])
	);
};
const FOUR_LABELS = ['now', 'peak', 'total', 'last'];

async function render(hash: string) {
	history.replaceState(null, '', '/' + hash);
	target = document.body.appendChild(document.createElement('div'));
	app = mount(App, { target });
	flushSync();
	await settle();
}

beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(NOW);
	stubApi();
});

afterEach(() => {
	if (app) unmount(app);
	app = null;
	target?.remove();
	toasts.clear();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
	localStorage.clear();
	document.body.innerHTML = '';
	history.replaceState(null, '', '/');
});

const OFFICE_METRICS = {
	rate_up: '12.0 KB/s',
	peak_rate_up: '64.0 KB/s',
	up: '5.0 MB',
	last_up: '\u2014',
	rate_down: '1.0 MB/s',
	peak_rate_down: '4.0 MB/s',
	down: '100.0 MB',
	last_down: '\u2014'
};

test('a collapsed rule shows all eight traffic values with connections, latency and failures; expanding adds the chain and its subordinate statistics only', async () => {
	await render('#/stats');
	const office = rows()[0];
	const toggle = button('Expand: office', office)!;
	expect(toggle.getAttribute('aria-expanded')).toBe('false');
	const band = bandOf(office);
	expect(band.getAttribute('role')).toBe('group');
	expect(band.getAttribute('aria-label')).toBe('Traffic');
	expect(labels(band)).toEqual(FOUR_LABELS);
	expect(columnsOf(band)).toEqual(METRIC_COLUMNS);
	expect(ownMetricsOf(office)).toEqual(OFFICE_METRICS);
	expect(compact(office.querySelector('[data-connections] dd'))).toBe('3 / 120');
	expect(office.querySelector('[data-connections] a')?.getAttribute('href')).toBe(
		'#/connections?rule=office'
	);
	expect(compact(office.querySelector('[data-latency] dd'))).toBe('41.2 ms / 38.7 ms');
	expect(compact(office.querySelector('[data-failures] dd'))).toBe('2 / 118');
	// The band is not part of the toggle.
	expect(band.closest('button')).toBeNull();

	click(toggle);
	const details = detailsOf(toggle);
	expect(details.querySelector('ol[aria-label="Chain"]')).not.toBeNull();
	expect(ownMetricsOf(office)).toEqual(OFFICE_METRICS);
	// Every value and count inside the expansion belongs to a hop, one of its URLs or a target.
	const stray = Array.from(
		details.querySelectorAll('[data-metric], [data-connections], [data-latency], [data-failures]')
	).filter((element) => !element.closest('[data-hop-stats], [data-url], [data-target]'));
	expect(stray).toEqual([]);
	expect(details.querySelectorAll('[data-hop-stats] [data-metric]')).toHaveLength(16);
	expect(details.querySelectorAll('[data-target] [data-metric]')).toHaveLength(24);
});

test('a single-endpoint host keeps its statistics only in the collapsed summary', async () => {
	await render('#/hosts');
	const bastion = rows('main article[data-host]')[1];
	expect(compact(bastion.querySelector('[data-endpoints]'))).toBe('1');
	const band = bandOf(bastion);
	expect(labels(band)).toEqual(
		FOUR_LABELS.map((label) => (label === 'peak' ? 'peak (sum)' : label))
	);
	const expected = {
		rate_up: '11.8 KB/s',
		peak_rate_up: '66.6 KB/s',
		up: hostsTotals.bastionUp,
		last_up: '3 s ago',
		rate_down: '785.3 KB/s',
		peak_rate_down: hostsTotals.bastionPeakDown,
		down: hostsTotals.bastionDown,
		last_down: '2 s ago'
	};
	expect(ownMetricsOf(bastion)).toEqual(expected);
	expect(compact(bastion.querySelector('[data-connections] dd'))).toBe(
		hostsTotals.bastionConnections
	);
	expect(compact(bastion.querySelector('[data-latency] dd'))).toBe(
		'18.4 ms / ' + hostsTotals.bastionAvgLatency
	);
	expect(compact(bastion.querySelector('[data-failures] dd'))).toBe('0 / 10');

	const toggle = button('Expand: bastion.example', bastion)!;
	click(toggle);
	const details = detailsOf(toggle);
	expect(ownMetricsOf(bastion)).toEqual(expected);
	expect(details.querySelectorAll('[data-endpoint-row]')).toHaveLength(1);
	expect(details.querySelector('[data-endpoint]')?.textContent?.trim()).toBe(
		'ssh://bastion.example:22'
	);
	expect(details.querySelectorAll('[data-use]').length).toBeGreaterThan(0);
	expect(
		details.querySelectorAll('[data-traffic], [data-connections], [data-latency], [data-failures]')
	).toHaveLength(0);
	expect(bastion.querySelectorAll('[data-metric]')).toHaveLength(8);
});

const CURL_METRICS = {
	rate_up: '1.0 KB/s',
	peak_rate_up: '4.0 KB/s',
	up: '12.0 KB',
	last_up: '3 s ago',
	rate_down: '512.0 KB/s',
	peak_rate_down: '1.0 MB/s',
	down: '1.0 MB',
	last_down: '2 s ago'
};

const helpNames = (root: ParentNode) =>
	Array.from(root.querySelectorAll('[data-help]')).map((element) =>
		element.getAttribute('aria-label')
	);
const tooltipText = () => target.querySelector('#tooltip')?.textContent ?? null;
const escape = () => {
	document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
	flushSync();
};

test('every statistics concept carries a circled question mark: its definition shows on hover, focus and click without toggling the item, and the arrows name their direction', async () => {
	await render('#/stats');
	// The heading explains the page's scope.
	const pageHelp = button('About Rule Traffic')!;
	expect(pageHelp.previousElementSibling?.tagName).toBe('H1');
	pageHelp.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toBe(
		'One item per rule with the counters of its listener. Expand a rule for its chain: each hop and proxy URL with its own counters, and the targets.'
	);
	pageHelp.dispatchEvent(new Event('pointerleave'));
	await tick();

	// One help per metric column and per meta cell of a collapsed item, each inside its label.
	const office = rows()[0];
	expect(helpNames(office)).toEqual([
		'About now',
		'About peak',
		'About total',
		'About last',
		'About Connections',
		'About Latency',
		'About Failures'
	]);
	expect(
		Array.from(office.querySelectorAll('[data-help]')).every((help) => help.closest('dt'))
	).toBe(true);
	expect(office.querySelectorAll('button button, button [data-help]')).toHaveLength(0);
	const toggle = button('Expand: office', office)!;
	const peak = button('About peak', office)!;
	click(peak);
	expect(tooltipText()).toBe(
		'Peak: the highest one-second rate seen since the statistics were last reset.'
	);
	expect(toggle.getAttribute('aria-expanded')).toBe('false');
	expect(peak.getAttribute('aria-describedby')).toBe('tooltip');
	// Escape hides it; the button is still there to reopen it.
	escape();
	expect(tooltipText()).toBeNull();
	peak.dispatchEvent(new Event('focusin', { bubbles: true }));
	flushSync();
	expect(tooltipText()).toContain('Peak:');
	peak.dispatchEvent(new Event('focusout', { bubbles: true }));
	await tick();
	expect(tooltipText()).toBeNull();
	button('About Latency', office)!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toBe(
		'Latency: how long the last successful dial took / the average over all successful dials. Connection setup time, not a ping.'
	);
	button('About Connections', office)!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toContain('Behind an SSH hop');
	// The arrows say which way is which.
	const arrows = Array.from(bandOf(office).querySelectorAll('[data-column="rate"] dd > span'));
	expect(arrows.map(compact)).toEqual(['Upload', '12.0 KB/s', 'Download', '1.0 MB/s']);
	arrows[0].dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toBe('Upload \u2014 from the client towards the target.');
	arrows[2].dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toBe('Download \u2014 from the target back to the client.');
	// Nested bands in the expansion repeat no help; only the dropped-targets hint has one.
	click(toggle);
	expect(helpNames(detailsOf(toggle))).toEqual(['About Targets']);

	// Hosts: the summed peak names its bound; the endpoint count is explained too.
	click(target.querySelector('nav a[href="#/hosts"]')!);
	await settle();
	expect(button('About Proxy Hosts')).toBeDefined();
	const bastion = rows('main article[data-host]')[1];
	expect(helpNames(bastion)).toEqual([
		'About Endpoints',
		'About now',
		'About peak (sum)',
		'About total',
		'About last',
		'About Connections',
		'About Latency',
		'About Failures'
	]);
	button('About peak (sum)', bastion)!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toContain('an upper bound');

	// Connections: the four metric helps per item and the path once expanded.
	click(target.querySelector('nav a[href="#/connections"]')!);
	await settle();
	expect(button('About Live Connections')).toBeDefined();
	const curl = rows('main [data-connection]')[2];
	expect(helpNames(curl)).toEqual(['About now', 'About peak', 'About total', 'About last']);
	click(button('Expand: example.com:443', curl));
	expect(helpNames(curl)).toEqual([
		'About now',
		'About peak',
		'About total',
		'About last',
		'About Path'
	]);
	button('About Path', curl)!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toContain('\u201creused\u201d');
});

test('the overview KPIs and the rule card facts are explained too, and the cards act through icon buttons', async () => {
	await render('#/');
	expect(helpNames(target.querySelector('main section[aria-label="Overview"]')!)).toEqual([
		'About Rules',
		'About Active connections',
		'About Current rate',
		'About Total traffic'
	]);
	button('About Rules')!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toBe(
		'Rules with a running listener out of all configured rules; a disabled rule is not running.'
	);
	// The KPI rates name their direction for assistive technology.
	expect(compact(target.querySelector('[data-kpi="rate"]'))).toMatch(/^Upload \S+ \S+ Download /);
	// Edit, Duplicate, Rule Traffic and Delete are icon controls to the same routes and action as before.
	const office = target.querySelector('main article[aria-labelledby]')!;
	const edit = office.querySelector('a[aria-label="Edit"]')!;
	expect(edit.getAttribute('href')).toBe('#/rules/office');
	expect(edit.classList.contains('icon-btn')).toBe(true);
	expect(edit.textContent?.trim()).toBe('');
	expect(edit.querySelector('svg')).not.toBeNull();
	const duplicate = office.querySelector('a[aria-label="Duplicate rule"]')!;
	expect(duplicate.getAttribute('href')).toBe('#/new?rule=office');
	expect(duplicate.classList.contains('icon-btn')).toBe(true);
	expect(duplicate.querySelector('svg')).not.toBeNull();
	const traffic = office.querySelector('a[aria-label="Rule Traffic"]')!;
	expect(traffic.getAttribute('href')).toBe('#/stats?rule=office');
	expect(traffic.querySelector('svg')).not.toBeNull();
	const remove = office.querySelector('button[aria-label="Delete"]')!;
	expect(remove.classList.contains('icon-btn')).toBe(true);
	expect(remove.querySelector('svg')).not.toBeNull();
	edit.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toBe('Edit');
	expect(
		Array.from(office.querySelectorAll('footer a, footer button')).map((control) =>
			control.textContent?.trim()
		)
	).toEqual(['', '', '', '']);
	// The footer rates use the shared arrows, named upload and download (not the hop moves).
	expect(compact(office.querySelector('footer'))).toMatch(/^Upload \S+ \S+ Download /);

	// The facts a rule is managed by carry the definitions the rule list's headers used to.
	expect(helpNames(office)).toEqual([
		'About Mode',
		'About State',
		'About Listen',
		'About Target',
		'About Exit chain'
	]);
	button('About Mode', office)!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toContain('port-forward rule pipes every connection to one target');
	button('About State', office)!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toContain('consecutive failed attempts');
});

const helpText = (name: string, root: ParentNode = target) => {
	button(name, root)!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	return tooltipText()!;
};
const switchLanguage = (label: string) => {
	click(target.querySelector(`[aria-label="${label}"]`)!);
	flushSync();
};

test('the connection counts say what they count: a reset keeps the live connections listed but counts only those opened since, in both languages and on the overview', async () => {
	await render('#/stats');
	const office = rows()[0];
	expect(compact(office.querySelector('[data-connections] dd'))).toBe('3 / 120');
	// The server's reset zeroes the aggregate counters yet keeps the live connections.
	snapshot.rules![0].stats.active = 0;
	snapshot.rules![0].stats.total = 0;
	await poll();
	expect(compact(office.querySelector('[data-connections] dd'))).toBe('0 / 0');
	click(button('Expand: office', office)!);
	expect(
		compact(
			detailsOf(button('Collapse: office', office)!).querySelector(
				'a[href="#/connections?rule=office"]'
			)
		)
	).toBe('3 connections');
	let text = helpText('About Connections', office);
	expect(text).toMatch(/still open \/ opened in total/);
	expect(text).toMatch(/since JumpWay started or the (statistics were )?last reset/);
	expect(text).toMatch(/already open at a reset stays listed but is not counted/);
	expect(text).not.toMatch(/open right now/);
	expect(text).toContain('Behind an SSH hop');
	switchLanguage('中文');
	text = helpText('连接说明', office);
	expect(text).toMatch(/仍然打开的 \/ 累计打开的/);
	expect(text).toContain('上次重置以来新建');
	expect(text).toContain('重置时已打开的连接仍会列出，但不计入');
	expect(text).not.toContain('当前打开');
	expect(text).toContain('SSH');

	click(target.querySelector('nav a[href="#/"]')!);
	await settle();
	text = helpText('活动连接说明');
	expect(text).toContain('上次重置以来新建且仍然打开');
	expect(text).not.toContain('当前打开');
	switchLanguage('English');
	text = helpText('About Active connections');
	expect(text).toMatch(/opened since JumpWay started or the last reset and (are )?still open/);
	expect(text).not.toMatch(/open right now/);
});

test('a host explains its latency as an aggregate: the last dial of the most recently active endpoint and the dial-weighted average, while a rule keeps the plain definition', async () => {
	// bastion.example is dialed by office (hop 2) and mirror (hop 1). Endpoint A took 10 ms and
	// has newer traffic; endpoint B took 90 ms with older activity: the host shows A's 10 ms.
	const [office, mirror] = snapshot.rules!;
	Object.assign(office.forward![1].urls![0].stats, { latency_ms: 10, avg_latency_ms: 10 });
	Object.assign(mirror.forward![0].urls![0].stats, { latency_ms: 90, avg_latency_ms: 90 });
	await render('#/hosts');
	const bastion = rows('main article[data-host]')[1];
	expect(bastion.dataset.host).toBe('bastion.example');
	expect(compact(bastion.querySelector('[data-latency] dd'))).toBe('10.0 ms / 42.0 ms');
	let text = helpText('About Latency', bastion);
	expect(text).toMatch(/^Latency: /);
	expect(text).toContain('most recently active endpoint');
	expect(text).toMatch(/weighted by successful dials/);
	expect(text).toContain('not a ping');
	// The endpoint bands inside the expansion carry no help of their own.
	click(button('Expand: bastion.example', bastion)!);
	expect(helpNames(detailsOf(button('Collapse: bastion.example', bastion)!))).toEqual([]);
	// The page help does not call the last value weighted either.
	expect(helpText('About Proxy Hosts')).toMatch(/average latency is weighted by successful dials/);
	switchLanguage('中文');
	text = helpText('延迟说明', bastion);
	expect(text).toContain('最近有活动');
	expect(text).toContain('按成功建连次数加权');
	expect(text).toContain('不是 ping');
	expect(helpText('跳板主机说明')).toContain('平均延迟按成功建连次数加权');

	// A rule's latency is its own last successful dial and average: the plain definition.
	switchLanguage('English');
	click(target.querySelector('nav a[href="#/stats"]')!);
	await settle();
	text = helpText('About Latency', rows()[0]);
	expect(text).toBe(
		'Latency: how long the last successful dial took / the average over all successful dials. Connection setup time, not a ping.'
	);
	expect(text).not.toContain('endpoint');
});

test('a collapsed connection is the same kind of item as a rule or host at the desktop layout: target, rule, full client address, process, duration and all eight traffic values; its expansion holds the start time and path only', async () => {
	await render('#/connections');
	expect(target.querySelector('main table, main thead')).toBeNull();
	const curl = rows('main [data-connection]')[2];
	expect(curl.tagName).toBe('ARTICLE');
	expect(compact(curl.querySelector('[data-client]'))).toBe('127.0.0.1:18094 curl (4242)');
	expect(compact(curl.querySelector('[data-duration]'))).toBe('1 min 30 s');
	expect(ownMetricsOf(curl)).toEqual(CURL_METRICS);
	expect(curl.querySelector('[data-connections], [data-latency], [data-failures]')).toBeNull();

	const toggle = button('Expand: example.com:443', curl)!;
	click(toggle);
	const details = detailsOf(toggle);
	expect(details.querySelectorAll('[data-metric], [data-traffic]')).toHaveLength(0);
	expect(labels(details)).toEqual(['Started', 'Path']);
	expect(details.textContent).not.toContain('18094');
	expect(ownMetricsOf(curl)).toEqual(CURL_METRICS);
});

test('#/stats lists every configured rule with status, traffic, connections, latency and failures', async () => {
	await render('#/stats');
	expect(target.querySelector('h1')?.textContent).toBe('Rule Traffic');
	expect(document.title).toBe('Rule Traffic \u00b7 JumpWay');
	expect(requested('GET /apis/stats')).toBe(1);
	expect(requested('GET /apis/configs/rules')).toBe(1);
	const list = rows();
	expect(list.map((row) => row.dataset.rule)).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	expect(target.querySelector('main table')).toBeNull();

	// Collapsed: identity, state, mode, address and the full statistics band.
	const office = list[0];
	expect(office.tagName).toBe('ARTICLE');
	expect(compact(office.querySelector('[data-state]'))).toBe('Running');
	expect(office.querySelector('[data-state]')?.getAttribute('data-state')).toBe('running');
	expect(compact(office.querySelector('[data-mode]'))).toBe('Proxy');
	expect(compact(office.querySelector('[data-address]'))).toBe('127.0.0.1:18097');
	// The name links within statistics and is not nested in the toggle.
	const name = office.querySelector('a[href="#/stats?rule=office"]')!;
	expect(name.textContent?.trim()).toBe('office');
	expect(name.closest('button')).toBeNull();
	expect(office.querySelectorAll('button a, a button')).toHaveLength(0);

	// The band names all eight traffic values plus connections, latency and failures.
	const block = office.querySelector('[data-stats]')!;
	const band = bandOf(block);
	expect(band.getAttribute('role')).toBe('group');
	expect(band.getAttribute('aria-label')).toBe('Traffic');
	expect(columnsOf(band)).toEqual(METRIC_COLUMNS);
	expect(labels(block)).toEqual([
		...FOUR_LABELS,
		'Connections Active / total',
		'Latency Last / average',
		'Failures Failed / attempted'
	]);
	expect(traffic(band)).toEqual([
		'12.0 KB/s',
		'64.0 KB/s',
		'5.0 MB',
		'\u2014',
		'1.0 MB/s',
		'4.0 MB/s',
		'100.0 MB',
		'\u2014'
	]);
	const connections = block.querySelector('[data-connections] dd')!;
	expect(compact(connections)).toBe('3 / 120');
	expect(connections.querySelector('a')?.getAttribute('href')).toBe('#/connections?rule=office');
	expect(compact(block.querySelector('[data-latency] dd'))).toBe('41.2 ms / 38.7 ms');
	expect(compact(block.querySelector('[data-failures] dd'))).toBe('2 / 118');
	// Expanding adds the chain below the band and repeats none of it.
	const toggle = button('Expand: office', office)!;
	click(toggle);
	const details = detailsOf(toggle);
	expect(details.closest('article')).toBe(office);
	expect(
		details.querySelector('[data-stats]')?.closest('[data-hop-stats], [data-url]')
	).not.toBeNull();
	expect(ownMetricsOf(office)).toEqual(OFFICE_METRICS);

	const mirror = list[1];
	expect(compact(mirror.querySelector('[data-mode]'))).toBe('Port forward');
	expect(compact(mirror.querySelector('[data-address]'))).toBe(
		'127.0.0.1:18098 \u2192 10.0.0.5:5432'
	);
	expect(compact(mirror.querySelector('[data-connections] dd'))).toBe('1 / 4');

	// db-tunnel: all dials failed, null slices; the listen chain makes it remote.
	const tunnel = list[2];
	expect(tunnel.querySelector('[data-state]')?.getAttribute('data-state')).toBe('retrying');
	expect(compact(tunnel.querySelector('[data-mode]'))).toBe('Port forward \u00b7 remote');
	expect(compact(tunnel.querySelector('[data-latency] dd'))).toBe('\u2014 / \u2014');
	expect(compact(tunnel.querySelector('[data-failures] dd'))).toBe('3 / 3');
	expect(compact(tunnel.querySelector('[data-connections] dd'))).toBe('0 / 0');
	expect(tunnel.querySelector('[data-connections] a')).toBeNull();

	// lab: disabled, has no stats entry.
	const lab = list[3];
	expect(lab.querySelector('[data-state]')?.getAttribute('data-state')).toBe('disabled');
	expect(traffic(bandOf(lab))).toEqual(Array(8).fill('\u2014'));
	expect(compact(lab.querySelector('[data-connections] dd'))).toBe('\u2014');
	expect(compact(lab.querySelector('[data-failures] dd'))).toBe('\u2014');

	// Toolbar: since, reset and Prometheus.
	expect(target.querySelector('main')?.textContent).toContain('Since ');
	expect(target.querySelector('main a[href="/metrics"]')?.textContent?.trim()).toBe(
		'Prometheus metrics'
	);
	expect(button('Reset statistics')).not.toBeUndefined();
	expect(stats.active).toBe(true);
});

test('the last-activity value shows "ago" text and names the absolute time as its tooltip', async () => {
	await render('#/stats');
	// The poll itself moved the fake clock one second on.
	snapshot.rules![0].stats.last_up = '2026-09-19T08:59:57Z';
	snapshot.rules![0].stats.last_down = '2026-09-19T08:59:58Z';
	await poll();
	const office = rows()[0];
	const band = bandOf(office);
	expect(traffic(band).slice(3, 4)).toEqual(['4 s ago']);
	expect(traffic(band).slice(7)).toEqual(['3 s ago']);
	const ago = band.querySelector('[data-metric="last_down"]')!;
	ago.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe(
		new Date('2026-09-19T08:59:58Z').toLocaleString('en')
	);
});

test('expanding a rule shows its chain with per-hop and per-URL statistics and the targets stage', async () => {
	await render('#/stats');
	const office = rows()[0];
	const toggle = button('Expand: office', office)!;
	expect(toggle.getAttribute('aria-expanded')).toBe('false');
	const controls = toggle.getAttribute('aria-controls')!;
	expect(target.querySelector('#' + controls)).toBeNull();

	click(toggle);
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(toggle.getAttribute('aria-label')).toBe('Collapse: office');
	const details = target.querySelector('#' + controls)!;
	expect(details.closest('article[data-rule]')).toBe(office);
	expect(rows()[1].dataset.rule).toBe('mirror');
	// The chain follows the item's own band as a vertical timeline.
	expect(
		bandOf(office).compareDocumentPosition(details.querySelector('ol[aria-label="Chain"]')!)
	).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
	expect(details.querySelector('[data-traffic]')?.closest('[data-hop-stats]')).not.toBeNull();
	const stages = Array.from(details.querySelectorAll('ol[aria-label="Chain"] > li'));
	expect(stages.map((stage) => stage.getAttribute('data-stage'))).toEqual([
		'clients',
		'local',
		'forward-1',
		'forward-0',
		'target'
	]);
	const titles = stages.map((stage) => compact(stage.querySelector('p')));
	expect(titles).toEqual([
		'Clients',
		'This machine',
		'Hop 2 \u00b7 entry node \u00b7 dialed from this machine',
		'Hop 1 \u00b7 exit node',
		'Targets'
	]);
	// Hop 2 is dialed from this machine and carries the true (server-side) peak, not a sum.
	const entry = stages[2];
	expect(compact(entry.querySelector('[data-parent]'))).toBe('via this machine');
	expect(traffic(entry.querySelector('[data-hop-stats]')).slice(4, 6)).toEqual([
		'1.0 MB/s',
		'4.1 MB/s'
	]);
	const urls = Array.from(entry.querySelectorAll('[data-url]'));
	expect(urls.map((url) => compact(url.querySelector('[data-endpoint]')))).toEqual([
		'ssh://bastion.example:22',
		'ssh://bastion-2.example:22'
	]);
	expect(traffic(urls[1]).slice(4, 8)).toEqual(['244.1 KB/s', '1.9 MB/s', '23.8 MB', '15 min ago']);
	expect(compact(urls[1].querySelector('[data-connections] dd'))).toBe('1 / 3');
	expect(compact(urls[1].querySelector('[data-latency] dd'))).toBe('25.0 ms / 22.3 ms');
	expect(compact(urls[1].querySelector('[data-failures] dd'))).toBe('1 / 3');
	// Hop 1 sits behind hop 2.
	expect(compact(stages[3].querySelector('[data-parent]'))).toBe('via Hop 2');
	expect(compact(stages[3].querySelector('[data-endpoint]'))).toBe('socks5://hop-a.example:1080');
	expect(details.innerHTML).not.toContain('demo:placeholder');
	// Targets: distinct count, the live connections link, the dropped-targets hint and the list
	// itself, most recently active first, the never-active one last.
	const targets = stages[4];
	expect(compact(targets)).toContain('Targets 3 distinct targets 3 connections');
	expect(targets.querySelector('a')?.getAttribute('href')).toBe('#/connections?rule=office');
	expect(compact(targets.querySelector('[data-evicted]'))).toBe(
		'3 older targets were dropped from this list'
	);
	expect(
		Array.from(targets.querySelectorAll('[data-target] [data-target-address]')).map(compact)
	).toEqual(['cdn.example.net:443', 'example.com:443', '10.1.2.3:8080']);
	expect(targets.querySelector('[data-targets-more]')).toBeNull();

	// Items and the expansion survive a poll unchanged (same elements).
	snapshot.rules![0].stats.rate_up = 24_576;
	await poll();
	expect(target.querySelector('#' + controls)).toBe(details);
	expect(rows()[0]).toBe(office);
	expect(traffic(bandOf(office))[0]).toBe('24.0 KB/s');
	expect(
		details.querySelector('[data-metric="rate_up"]')?.closest('[data-hop-stats]')
	).not.toBeNull();

	click(toggle);
	expect(target.querySelector('#' + controls)).toBeNull();
	expect(toggle.getAttribute('aria-expanded')).toBe('false');
});

test('the listen chain of a remote rule is shown from the clients side and a rule without stats still expands', async () => {
	await render('#/stats');
	click(button('Expand: db-tunnel')!);
	const details = detailsOf(button('Collapse: db-tunnel')!);
	const stages = Array.from(details.querySelectorAll('ol[aria-label="Chain"] > li'));
	expect(stages.map((stage) => stage.getAttribute('data-stage'))).toEqual([
		'clients',
		'listen-0',
		'local',
		'direct',
		'target'
	]);
	expect(compact(stages[0])).toBe('Clients 0.0.0.0:18099');
	expect(compact(stages[1].querySelector('p'))).toBe(
		'Hop 1 \u00b7 binds the port \u00b7 dialed from this machine'
	);
	expect(compact(stages[1].querySelector('[data-endpoint]'))).toBe('ssh://edge.example:22');
	expect(compact(stages[1].querySelector('[data-failures] dd'))).toBe('3 / 3');
	// A forward rule names its fixed target; no connection has been seen yet.
	expect(compact(stages[4])).toBe('Targets 127.0.0.1:5432 No connections yet');

	click(button('Expand: lab')!);
	const lab = detailsOf(button('Collapse: lab')!);
	expect(compact(lab.querySelector('ol[aria-label="Chain"]'))).toBe(
		'Clients This machine 127.0.0.1:18100 Direct Targets No connections yet'
	);
});

test('?rule= pre-expands and focuses that rule once; later polls and query changes do not steal focus', async () => {
	await render('#/stats?rule=mirror');
	const toggle = button('Collapse: mirror')!;
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(document.activeElement).toBe(toggle);
	expect(button('Expand: office')!.getAttribute('aria-expanded')).toBe('false');

	const link = target.querySelector<HTMLAnchorElement>('main a[href="#/connections?rule=mirror"]')!;
	link.focus();
	await poll();
	expect(document.activeElement).toBe(link);

	// The rule name links within statistics; only the query changes: no remount, the new rule
	// expands and takes focus, the old one stays open.
	click(target.querySelector('main a[href="#/stats?rule=office"]'));
	await settle();
	await tick();
	expect(location.hash).toBe('#/stats?rule=office');
	expect(button('Collapse: office')!.getAttribute('aria-expanded')).toBe('true');
	expect(document.activeElement).toBe(button('Collapse: office'));
	expect(button('Collapse: mirror')!.getAttribute('aria-expanded')).toBe('true');

	// Back/Forward to another query does the same through popstate.
	history.pushState({ jumpwayHistoryIndex: 2 }, '', '/#/stats?rule=db-tunnel');
	window.dispatchEvent(new PopStateEvent('popstate'));
	flushSync();
	await tick();
	expect(button('Collapse: db-tunnel')!.getAttribute('aria-expanded')).toBe('true');
	expect(document.activeElement).toBe(button('Collapse: db-tunnel'));
	expect(requested('GET /apis/configs/rules')).toBe(1);
});

test('rules named like Object.prototype members expand and focus from ?rule= once their row exists, also when the config arrives late', async () => {
	const listen = (port: number) => ({ host: '127.0.0.1', port });
	rules.push(
		{ name: 'toString', listen: listen(18101), forward: {} },
		{ name: '__proto__', listen: listen(18102), forward: {} },
		{ name: 'constructor', listen: listen(18103), forward: {} }
	);
	let release!: () => void;
	delay.set('GET /apis/configs/rules', new Promise<void>((resolve) => (release = resolve)));
	await render('#/stats?rule=toString');
	// Only the snapshot's rules are rows so far: nothing to focus yet, nothing thrown.
	expect(rows().map((row) => row.dataset.rule)).toEqual(['office', 'mirror', 'db-tunnel']);
	expect(document.activeElement).toBe(document.body);

	release();
	await settle();
	expect(rows().map((row) => row.dataset.rule)).toEqual([
		'office',
		'mirror',
		'db-tunnel',
		'lab',
		'toString',
		'__proto__',
		'constructor'
	]);
	const toggle = button('Collapse: toString')!;
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(document.activeElement).toBe(toggle);
	expect(button('Expand: __proto__')!.getAttribute('aria-expanded')).toBe('false');
	expect(button('Expand: constructor')!.getAttribute('aria-expanded')).toBe('false');

	// Focused once: a poll leaves focus where the user put it.
	const link = target.querySelector<HTMLAnchorElement>('main a[href="#/stats?rule=__proto__"]')!;
	link.focus();
	await poll();
	expect(document.activeElement).toBe(link);

	// Query-only navigation and Back/Forward to the other two names.
	click(link);
	await settle();
	await tick();
	expect(location.hash).toBe('#/stats?rule=__proto__');
	expect(button('Collapse: __proto__')!.getAttribute('aria-expanded')).toBe('true');
	expect(document.activeElement).toBe(button('Collapse: __proto__'));
	expect(button('Collapse: toString')!.getAttribute('aria-expanded')).toBe('true');

	history.pushState({ jumpwayHistoryIndex: 2 }, '', '/#/stats?rule=constructor');
	window.dispatchEvent(new PopStateEvent('popstate'));
	flushSync();
	await tick();
	expect(document.activeElement).toBe(button('Collapse: constructor'));
	await poll();
	expect(document.activeElement).toBe(button('Collapse: constructor'));
	expect(requested('GET /apis/configs/rules')).toBe(1);
});

test('an unknown ?rule= and a rule missing from the runtime status render without errors', async () => {
	await render('#/stats?rule=ghost');
	expect(rows().map((row) => row.dataset.rule)).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	expect(target.querySelector('main [role="alert"]')).toBeNull();
	// The snapshot may name a rule the config no longer has: it is listed from the snapshot.
	snapshot.rules!.push({
		name: 'orphan',
		stats: snapshot.rules![1].stats,
		listen: null,
		forward: null,
		targets: null,
		connections: null,
		targets_evicted: 0
	});
	await poll();
	const orphan = rows().find((row) => row.dataset.rule === 'orphan')!;
	expect(orphan.querySelector('[data-state]')?.getAttribute('data-state')).toBe('unknown');
	expect(compact(orphan.querySelector('[data-address]'))).toBe('\u2014');
});

test('reset asks first, DELETEs /apis/stats, toasts and re-reads the snapshot', async () => {
	await render('#/stats');
	const office = rows()[0];
	click(button('Expand: office', office)!);
	const details = detailsOf(button('Collapse: office', office)!);
	const band = bandOf(office);
	expect(traffic(band)[2]).toBe('5.0 MB');
	click(button('Reset statistics'));
	await settle();
	const dialog = confirmDialog()!;
	expect(dialog.textContent).toContain('Reset statistics for all rules?');
	click(dialog.querySelectorAll('button')[0]);
	await settle();
	expect(requested('DELETE /apis/stats')).toBe(0);

	click(button('Reset statistics'));
	await settle();
	snapshot.rules![0].stats.up = 0;
	click(button('Confirm', confirmDialog()!));
	await settle();
	expect(requested('DELETE /apis/stats')).toBe(1);
	expect(requested('GET /apis/stats')).toBe(2);
	expect(toasts.list.map((toast) => [toast.kind, toast.message])).toEqual([
		['success', 'Statistics reset.']
	]);
	// The expansion survives the re-read; the band shows the zeroed counter without reopening.
	expect(detailsOf(button('Collapse: office', office)!)).toBe(details);
	expect(bandOf(office)).toBe(band);
	expect(traffic(band)[2]).toBe('0 B');

	fail.set('DELETE /apis/stats', 'stats: locked');
	click(button('Reset statistics'));
	await settle();
	click(button('Confirm', confirmDialog()!));
	await settle();
	expect(toasts.list.map((toast) => toast.kind)).toEqual(['success', 'error']);
	expect(toasts.list[1].message).toBe('stats: locked');
});

test('a failing poll keeps the last snapshot on screen behind a retry banner; the config error is redacted', async () => {
	await render('#/stats');
	expect(rows()).toHaveLength(4);
	down.add('GET /apis/stats');
	await poll();
	const banner = target.querySelector('main [role="alert"]')!;
	expect(banner.textContent).toContain('Cannot reach JumpWay.');
	expect(rows()).toHaveLength(4);
	expect(compact(rows()[0].querySelector('[data-metric="rate_up"]'))).toBe('12.0 KB/s');
	down.clear();
	click(banner.querySelector('button')!);
	await settle();
	expect(target.querySelector('main [role="alert"]')).toBeNull();

	if (app) unmount(app);
	app = null;
	target.remove();
	fail.set(
		'GET /apis/configs/rules',
		'rules[0].forward.way[0]: invalid proxy URL "socks5://demo:secret@bad:port": parse error'
	);
	await render('#/stats');
	const configBanner = target.querySelector('main [role="alert"]')!;
	expect(configBanner.textContent).toContain('socks5://xxxxx@bad:port');
	expect(target.innerHTML).not.toContain('demo:secret');
	// The snapshot alone still lists the rules it knows.
	expect(rows().map((row) => row.dataset.rule)).toEqual(['office', 'mirror', 'db-tunnel']);
	fail.clear();
	click(configBanner.querySelector('button')!);
	await settle();
	expect(target.querySelector('main [role="alert"]')).toBeNull();
	expect(rows()).toHaveLength(4);
});

test('stats polling runs only while a stats page is mounted and visible', async () => {
	await render('#/stats');
	expect(requested('GET /apis/stats')).toBe(1);
	await poll();
	expect(requested('GET /apis/stats')).toBe(2);
	click(target.querySelector('nav a[href="#/settings"]')!);
	await settle();
	expect(stats.active).toBe(false);
	await poll();
	await poll();
	expect(requested('GET /apis/stats')).toBe(2);
	click(target.querySelector('nav a[href="#/hosts"]')!);
	await settle();
	expect(stats.active).toBe(true);
	expect(requested('GET /apis/stats')).toBe(3);
	expect(target.querySelector('h1')?.textContent).toBe('Proxy Hosts');
});

test('#/hosts aggregates hop URLs by hostname with usage links that stay inside statistics', async () => {
	await render('#/hosts');
	expect(document.title).toBe('Proxy Hosts \u00b7 JumpWay');
	expect(target.querySelector('main table')).toBeNull();
	const list = rows('main article[data-host]');
	expect(list.map((row) => row.dataset.host)).toEqual(hostsTotals.order);
	const bastion = list[1];
	expect(compact(bastion.querySelector('[data-host-name]'))).toBe('bastion.example');
	// Usage links sit in their own line, never inside the toggle.
	const usage = Array.from(bastion.querySelectorAll('[data-usage] a'));
	expect(usage.map((link) => [link.textContent?.trim(), link.getAttribute('href')])).toEqual([
		['office', '#/stats?rule=office'],
		['mirror', '#/stats?rule=mirror']
	]);
	expect(compact(bastion.querySelector('[data-usage] > span'))).toBe('Used by');
	expect(bastion.querySelectorAll('button a')).toHaveLength(0);
	const endpointCount = bastion.querySelector('[data-endpoints]')!;
	expect(compact(endpointCount.closest('dl'))).toBe('Endpoints 1');
	expect(bastion.querySelector('[data-summary]')).toBeNull();
	// The band is visible while collapsed.
	const block = bastion.querySelector('[data-stats]')!;
	const band = bandOf(block);
	expect(traffic(band)).toEqual([
		'11.8 KB/s',
		'66.6 KB/s',
		hostsTotals.bastionUp,
		'3 s ago',
		'785.3 KB/s',
		hostsTotals.bastionPeakDown,
		hostsTotals.bastionDown,
		'2 s ago'
	]);
	expect(compact(block.querySelector('[data-connections] dd'))).toBe(
		hostsTotals.bastionConnections
	);
	expect(block.querySelector('[data-connections] a')).toBeNull();
	expect(compact(block.querySelector('[data-latency] dd'))).toBe(
		'18.4 ms / ' + hostsTotals.bastionAvgLatency
	);
	expect(compact(block.querySelector('[data-failures] dd'))).toBe('0 / 10');
	// Summed peaks are an upper bound: labelled as a sum, said so on the peak values.
	expect(labels(band).filter((label) => label?.startsWith('peak'))).toEqual(['peak (sum)']);
	const peak = band.querySelector('[data-metric="peak_rate_down"]')!;
	peak.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe(
		'Sum of endpoint peaks \u2014 an upper bound'
	);

	click(button('Expand: bastion.example', bastion)!);
	const details = detailsOf(button('Collapse: bastion.example', bastion)!);
	expect(details.querySelector('[data-stats]')).toBeNull();
	const endpoints = Array.from(details.querySelectorAll('[data-endpoint-row]'));
	expect(endpoints).toHaveLength(1);
	expect(compact(endpoints[0].querySelector('[data-endpoint]'))).toBe('ssh://bastion.example:22');
	expect(
		Array.from(endpoints[0].querySelectorAll('[data-use]')).map((chip) => compact(chip))
	).toEqual(['office \u00b7 Exit chain \u00b7 Hop 2', 'mirror \u00b7 Exit chain \u00b7 Hop 1']);
	expect(endpoints[0].querySelector('[data-traffic], [data-latency]')).toBeNull();
	const label = endpoints[0].querySelector('[data-endpoint]')!;
	label.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe('ssh://xxxxx@bastion.example:22');

	// edge.example: listen-through use and all dials failed.
	const edge = list[3];
	const edgeBlock = edge.querySelector('[data-stats]')!;
	expect(compact(edgeBlock.querySelector('[data-latency] dd'))).toBe('\u2014 / \u2014');
	expect(compact(edgeBlock.querySelector('[data-failures] dd'))).toBe('3 / 3');
	click(button('Expand: edge.example', edge)!);
	const edgeDetails = detailsOf(button('Collapse: edge.example', edge)!);
	expect(compact(edgeDetails.querySelector('[data-use]'))).toBe(
		'db-tunnel \u00b7 Listen through \u00b7 Hop 1'
	);
	expect(edgeDetails.querySelector('[data-endpoint-row] [data-failures]')).toBeNull();
	// Both stay open independently.
	expect(button('Collapse: bastion.example', bastion)!.getAttribute('aria-expanded')).toBe('true');
});

test('host endpoint breakdown follows one-to-many-to-one changes without collapsing', async () => {
	await render('#/hosts');
	const host = target.querySelector('article[data-host="bastion.example"]')!;
	click(button('Expand: bastion.example', host)!);
	const toggle = button('Collapse: bastion.example', host)!;
	const details = detailsOf(toggle);
	expect(details.querySelectorAll('[data-metric]')).toHaveLength(0);

	const urls = snapshot.rules![0].forward![1].urls!;
	const additional = structuredClone(urls[0]);
	additional.url = 'ssh://ops@bastion.example:2222';
	additional.stats.down = 2048;
	urls.push(additional);
	await poll();
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(compact(host.querySelector('[data-endpoints]'))).toBe('2');
	const endpoints = Array.from(details.querySelectorAll('[data-endpoint-row]'));
	expect(endpoints).toHaveLength(2);
	expect(endpoints.map((endpoint) => compact(endpoint.querySelector('[data-endpoint]')))).toEqual([
		'ssh://bastion.example:22',
		'ssh://bastion.example:2222'
	]);
	expect(
		endpoints.map((endpoint) => compact(endpoint.querySelector('[data-metric="down"]')))
	).toEqual([hostsTotals.bastionDown, '2.0 KB']);
	for (const endpoint of endpoints) {
		expect(endpoint.querySelectorAll('[data-metric]')).toHaveLength(8);
		expect(
			endpoint.querySelectorAll('[data-connections], [data-latency], [data-failures]')
		).toHaveLength(3);
	}

	urls.pop();
	await poll();
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(compact(host.querySelector('[data-endpoints]'))).toBe('1');
	expect(details.querySelectorAll('[data-endpoint-row]')).toHaveLength(1);
	expect(details.querySelectorAll('[data-stats], [data-metric]')).toHaveLength(0);
	expect(host.querySelectorAll('[data-metric]')).toHaveLength(8);
});

test('a focused host toggle keeps focus when a poll re-sorts its row; focus elsewhere and a removed host are left alone', async () => {
	await render('#/hosts');
	const order = () => rows('main article[data-host]').map((row) => row.dataset.host);
	expect(order()).toEqual(hostsTotals.order);
	const row = rows('main article[data-host]')[2];
	click(button('Expand: bastion-2.example', row)!);
	const toggle = button('Collapse: bastion-2.example', row)!;
	toggle.focus();
	flushSync();
	expect(document.activeElement).toBe(toggle);
	expect(target.querySelector('#tooltip')?.textContent).toBe('Collapse: bastion-2.example');

	// Its endpoint's download total grows past every other host: the poll moves the row (same element) first.
	snapshot.rules![0].forward![1].urls![1].stats.down = 300_000_000;
	await poll();
	expect(order()).toEqual([
		'bastion-2.example',
		'hop-a.example',
		'bastion.example',
		'edge.example'
	]);
	expect(rows('main article[data-host]')[0]).toBe(row);
	expect(button('Collapse: bastion-2.example', row)).toBe(toggle);
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(document.activeElement).toBe(toggle);
	expect(target.querySelector('#tooltip')?.textContent).toBe('Collapse: bastion-2.example');

	// Focus outside the rows is not touched by a re-sort.
	const nav = target.querySelector<HTMLAnchorElement>('nav a[href="#/hosts"]')!;
	nav.focus();
	snapshot.rules![0].forward![1].urls![1].stats.down = 25_000_000;
	await poll();
	expect(order()).toEqual(hostsTotals.order);
	expect(document.activeElement).toBe(nav);

	// The host's only URL disappears: its row is gone and focus is not sent to the detached button.
	toggle.focus();
	flushSync();
	expect(document.activeElement).toBe(toggle);
	snapshot.rules![0].forward![1].urls!.splice(1, 1);
	await poll();
	expect(order()).toEqual(['hop-a.example', 'bastion.example', 'edge.example']);
	expect(toggle.isConnected).toBe(false);
	expect(document.activeElement).toBe(document.body);
	expect(target.querySelector('#tooltip')).toBeNull();
});

test('#/hosts without any hop URL shows the empty state and never throws on odd hop strings', async () => {
	for (const rule of snapshot.rules!) {
		rule.listen = null;
		rule.forward = null;
	}
	await render('#/hosts');
	expect(target.querySelector('main')?.textContent).toContain('No proxy hosts.');
	snapshot.rules![0].forward = [
		{
			index: 0,
			parent_index: -1,
			stats: snapshot.rules![0].stats,
			urls: [{ url: 'cmd:ssh -W %h:%p jump', stats: snapshot.rules![0].stats }]
		}
	];
	await poll();
	expect(rows('main article[data-host]').map((row) => row.dataset.host)).toEqual([
		'cmd:ssh -W %h:%p jump'
	]);
});

const CURL_PATH =
	'this machine \u2192 ssh://bastion.example:22 \u2192 socks5://hop-a.example:1080 \u2192 example.com:443';
const CHROME_PATH =
	'this machine \u2192 ssh://bastion-2.example:22 (reused) \u2192 Hop 1 (reused) \u2192 cdn.example.net:443';
// The metrics of one item, keyed by counter.
const metricsOf = (root: Element) =>
	Object.fromEntries(
		Array.from(root.querySelectorAll<HTMLElement>('[data-metric]')).map((element) => [
			element.dataset.metric,
			compact(element)
		])
	);
const connectionIds = () => rows('main [data-connection]').map((row) => row.dataset.connection);
const expandedIds = () =>
	rows('main [data-connection]')
		.filter(
			(row) => row.querySelector('button[aria-controls]')?.getAttribute('aria-expanded') === 'true'
		)
		.map((row) => row.dataset.connection);

test('#/connections lists every connection as an item: target, rule link, full client with process, all eight traffic values and duration; expanding adds the start time and path; items open independently and survive polls', async () => {
	await render('#/connections');
	expect(document.title).toBe('Live Connections \u00b7 JumpWay');
	const list = rows('main [data-connection]');
	expect(list.map((row) => row.dataset.connection)).toEqual(['103', '102', '101', '201']);
	expect(list.map((row) => row.tagName)).toEqual(['ARTICLE', 'ARTICLE', 'ARTICLE', 'ARTICLE']);
	expect(compact(target.querySelector('[data-connection-count]'))).toBe('4 connections');
	// The sort controls stand alone: there is no table header.
	expect(target.querySelector('main table, main thead')).toBeNull();
	const sortBy = target.querySelector<HTMLSelectElement>('main select[aria-label="Sort by"]')!;
	expect(sortBy.value).toBe('started');
	expect(button('Descending')).toBeDefined();

	const curl = list[2];
	expect(compact(curl.querySelector('[data-target]'))).toBe('example.com:443');
	// The client keeps its port: nothing about it is left for the expansion.
	expect(compact(curl.querySelector('[data-client]'))).toBe('127.0.0.1:18094 curl (4242)');
	const rule = curl.querySelector('[data-rule-link]')!;
	expect(rule.textContent?.trim()).toBe('office');
	expect(rule.getAttribute('href')).toBe('#/stats?rule=office');
	expect(curl.querySelectorAll('button a, a button')).toHaveLength(0);
	// The item carries all eight traffic values in one labelled band, each exactly once, the
	// upload value above the download value of the same counter.
	const band = bandOf(curl);
	expect(band.getAttribute('role')).toBe('group');
	expect(labels(band)).toEqual(FOUR_LABELS);
	expect(columnsOf(band)).toEqual(METRIC_COLUMNS);
	expect(metricsOf(curl)).toEqual(CURL_METRICS);
	expect(Object.keys(metricsOf(curl)).sort()).toEqual([...TRAFFIC_KEYS].sort());
	expect(band.querySelector('[data-connections], [data-latency], [data-failures]')).toBeNull();
	const last = band.querySelector('[data-metric="last_down"]')!;
	last.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe(
		new Date('2026-09-19T08:59:58Z').toLocaleString('en')
	);
	expect(compact(curl.querySelector('[data-duration]'))).toBe('1 min 30 s');
	expect(button('Disconnect', curl)).toBeDefined();
	const path = button('Path', curl)!;
	path.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe(CURL_PATH);

	// Expanded: only what the item leaves out — the start time and the path. Traffic, client
	// and process are the item's alone.
	const toggle = button('Expand: example.com:443', curl)!;
	expect(toggle.getAttribute('aria-expanded')).toBe('false');
	click(toggle);
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(toggle.getAttribute('aria-label')).toBe('Collapse: example.com:443');
	const details = detailsOf(toggle);
	expect(details.closest('article')).toBe(curl);
	expect(details.querySelectorAll('[data-traffic], [data-metric]')).toHaveLength(0);
	expect(metricsOf(curl)).toEqual(CURL_METRICS);
	const facts = details.querySelector('[data-connection-facts]')!;
	expect(labels(details)).toEqual(['Started', 'Path']);
	expect(compact(facts.querySelector('[data-started]'))).toBe(
		formatDateTime('2026-09-19T08:58:30Z')
	);
	expect(facts.querySelector('[data-started] time')?.getAttribute('datetime')).toBe(
		'2026-09-19T08:58:30.000Z'
	);
	expect(compact(facts.querySelector('[data-path]'))).toBe(CURL_PATH);
	expect(details.querySelector('[data-client-address], [data-client]')).toBeNull();
	expect(curl.querySelectorAll('[data-client]')).toHaveLength(1);
	expect(details.textContent).not.toContain('curl');
	expect(details.textContent).not.toContain('127.0.0.1');

	// A reused transport and an unknown path; items stay open independently of each other and
	// a poll keeps them open.
	const chrome = list[1];
	button('Path', chrome)!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe(CHROME_PATH);
	expect(compact(chrome.querySelector('[data-client]'))).toBe(
		'127.0.0.1:18095 Google Chrome Helper (777)'
	);
	click(button('Expand: cdn.example.net:443', chrome));
	expect(expandedIds()).toEqual(['102', '101']);
	expect(
		compact(
			detailsOf(button('Collapse: cdn.example.net:443', chrome)!).querySelector('[data-path]')
		)
	).toBe(CHROME_PATH);
	await poll();
	expect(expandedIds()).toEqual(['102', '101']);
	expect(detailsOf(toggle)).toBe(details);
	click(button('Collapse: example.com:443', curl));
	expect(expandedIds()).toEqual(['102']);
	expect(connectionIds()).toEqual(['103', '102', '101', '201']);

	button('Path', list[0])!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe('direct');
	expect(compact(list[0].querySelector('[data-client]'))).toBe('[::1]:52000');
	expect(compact(list[0].querySelector('[data-rule-link]'))).toBe('office');
	// A connection without traffic yet shows zero rates and totals and no last activity.
	expect(traffic(list[0])).toEqual([
		'0 B/s',
		'0 B/s',
		'0 B',
		'\u2014',
		'0 B/s',
		'0 B/s',
		'0 B',
		'\u2014'
	]);
	click(button('Expand: 10.1.2.3:8080', list[0]));
	const direct = detailsOf(button('Collapse: 10.1.2.3:8080', list[0])!);
	expect(compact(direct.querySelector('[data-path]'))).toBe('direct');
	expect(direct.textContent).not.toContain('::1');
	expect(target.innerHTML).not.toContain('demo:placeholder');

	// A connection without a client address shows a dash; the facts are the same two.
	delete snapshot.rules![1].connections![0].client;
	await poll();
	const psql = rows('main [data-connection]')[3];
	expect(compact(psql.querySelector('[data-client]'))).toBe('\u2014 psql (9)');
	click(button('Expand: 10.0.0.5:5432', psql));
	const anonymous = detailsOf(button('Collapse: 10.0.0.5:5432', psql)!);
	expect(labels(anonymous.querySelector('[data-connection-facts]')!)).toEqual(['Started', 'Path']);
	// The polls aged the "ago" values by their seconds.
	expect(traffic(psql)).toEqual([
		'2.0 KB/s',
		'8.0 KB/s',
		'1.0 MB',
		'1 min ago',
		'4.0 KB/s',
		'16.0 KB/s',
		'2.0 MB',
		'32 s ago'
	]);
});

test('the rule select and the fuzzy search filter; ?rule= preselects and the query follows the select', async () => {
	await render('#/connections?rule=mirror');
	expect(location.hash).toBe('#/connections?rule=mirror');
	const select = target.querySelector<HTMLSelectElement>('main select#connections-rule')!;
	expect(select.value).toBe('mirror');
	expect(Array.from(select.options).map((option) => option.value)).toEqual([
		'',
		'office',
		'mirror',
		'db-tunnel'
	]);
	expect(connectionIds()).toEqual(['201']);
	expect(compact(target.querySelector('[data-connection-count]'))).toBe('1 of 4 connections');

	select.value = '';
	select.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
	await settle();
	expect(location.hash).toBe('#/connections');
	expect(connectionIds()).toHaveLength(4);

	const search = target.querySelector<HTMLInputElement>('main input[type="search"]')!;
	expect(button('Clear search')).toBeUndefined();
	search.value = '1809';
	search.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
	expect(connectionIds()).toEqual(['102', '101']);
	expect(compact(target.querySelector('[data-connection-count]'))).toBe('2 of 4 connections');
	// The process name is searched too.
	search.value = 'psql';
	search.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
	expect(connectionIds()).toEqual(['201']);
	search.value = 'zzz';
	search.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
	expect(connectionIds()).toHaveLength(0);
	expect(target.querySelector('main')?.textContent).toContain('No current connections');
	click(button('Clear search'));
	expect(search.value).toBe('');
	expect(connectionIds()).toHaveLength(4);

	// A query-only route change updates the select without a remount or focus change.
	search.focus();
	history.pushState({ jumpwayHistoryIndex: 1 }, '', '/#/connections?rule=office');
	window.dispatchEvent(new PopStateEvent('popstate'));
	flushSync();
	await settle();
	expect(select.value).toBe('office');
	expect(connectionIds()).toHaveLength(3);
	expect(document.activeElement).toBe(search);
	expect(requested('GET /apis/stats')).toBe(1);

	// An unknown rule is still selectable and simply matches nothing.
	history.pushState({ jumpwayHistoryIndex: 2 }, '', '/#/connections?rule=ghost');
	window.dispatchEvent(new PopStateEvent('popstate'));
	flushSync();
	await settle();
	expect(select.value).toBe('ghost');
	expect(connectionIds()).toHaveLength(0);
	expect(target.querySelector('main [role="alert"]')).toBeNull();
});

test('the sort select offers every identity and traffic key and the direction button flips the order; the ordering matches each counter', async () => {
	await render('#/connections');
	const sortBy = target.querySelector<HTMLSelectElement>('main select[aria-label="Sort by"]')!;
	expect(sortBy.value).toBe('started');
	expect(Array.from(sortBy.options).map((option) => option.value)).toEqual([
		'started',
		'target',
		'client',
		'rule',
		'rate_down',
		'rate_up',
		'peak_rate_down',
		'peak_rate_up',
		'down',
		'up',
		'last_down',
		'last_up'
	]);
	expect(Array.from(sortBy.options).map((option) => option.textContent)).toEqual([
		'Duration',
		'Target',
		'Client',
		'Rule',
		'Download \u00b7 now',
		'Upload \u00b7 now',
		'Download \u00b7 peak',
		'Upload \u00b7 peak',
		'Download \u00b7 total',
		'Upload \u00b7 total',
		'Download \u00b7 last',
		'Upload \u00b7 last'
	]);
	const direction = button('Descending')!;
	direction.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe('Descending');
	const ids = connectionIds;
	const choose = (key: string) => {
		sortBy.value = key;
		sortBy.dispatchEvent(new Event('change', { bubbles: true }));
		flushSync();
	};
	expect(ids()).toEqual(['103', '102', '101', '201']);

	// A new key keeps the direction; the button flips it and its own name follows.
	choose('target');
	expect(direction.getAttribute('aria-label')).toBe('Descending');
	expect(ids()).toEqual(['101', '102', '103', '201']);
	click(direction);
	expect(direction.getAttribute('aria-label')).toBe('Ascending');
	expect(ids()).toEqual(['201', '103', '102', '101']);

	choose('rate_down');
	expect(ids()).toEqual(['103', '201', '102', '101']);
	click(direction);
	expect(ids()).toEqual(['101', '102', '201', '103']);

	choose('peak_rate_down');
	expect(direction.getAttribute('aria-label')).toBe('Descending');
	expect(ids()).toEqual(['101', '102', '201', '103']);
	click(direction);
	expect(direction.getAttribute('aria-label')).toBe('Ascending');
	expect(ids()).toEqual(['103', '201', '102', '101']);
	expect(sortBy.value).toBe('peak_rate_down');

	choose('rate_up');
	expect(ids()).toEqual(['103', '102', '101', '201']);
	click(direction);
	expect(ids()).toEqual(['201', '101', '102', '103']);

	choose('client');
	click(direction);
	expect(direction.getAttribute('aria-label')).toBe('Ascending');
	expect(ids()).toEqual(['103', '101', '102', '201']);
	choose('started');
	click(direction);
	expect(direction.getAttribute('aria-label')).toBe('Descending');
	expect(ids()).toEqual(['103', '102', '101', '201']);
});

test('a focused row control keeps focus when a poll moves its row; a removed or filtered-out row does not get it back', async () => {
	await render('#/connections');
	const ids = connectionIds;
	// Ascending download total, chosen through the toolbar.
	const sortBy = target.querySelector<HTMLSelectElement>('main select[aria-label="Sort by"]')!;
	sortBy.value = 'down';
	sortBy.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
	click(button('Descending'));
	expect(ids()).toEqual(['103', '102', '101', '201']);
	const chrome = rows('main [data-connection]')[1];
	const path = button('Path', chrome)!;
	path.focus();
	flushSync();
	expect(document.activeElement).toBe(path);
	expect(target.querySelector('#tooltip')?.textContent).toBe(CHROME_PATH);

	// Its download total grows past every other row: the poll moves the row (same element) last.
	snapshot.rules![0].connections![1].stats.down = 3_000_000;
	await poll();
	expect(ids()).toEqual(['103', '101', '201', '102']);
	expect(rows('main [data-connection]')[3]).toBe(chrome);
	expect(button('Path', chrome)).toBe(path);
	expect(document.activeElement).toBe(path);
	expect(target.querySelector('#tooltip')?.textContent).toBe(CHROME_PATH);

	// The connection closes: its control is gone and focus is not sent to the detached button.
	snapshot.rules![0].connections!.splice(1, 1);
	await poll();
	expect(ids()).toEqual(['103', '101', '201']);
	expect(path.isConnected).toBe(false);
	expect(document.activeElement).toBe(document.body);
	expect(target.querySelector('#tooltip')).toBeNull();

	// Filtering the focused row away is what the user asked for: no restore, nothing thrown.
	const disconnect = button('Disconnect', rows('main [data-connection]')[0])!;
	disconnect.focus();
	const select = target.querySelector<HTMLSelectElement>('main select#connections-rule')!;
	select.value = 'mirror';
	select.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
	await settle();
	expect(ids()).toEqual(['201']);
	expect(disconnect.isConnected).toBe(false);
	expect(document.activeElement).toBe(document.body);
	button('Disconnect', rows('main [data-connection]')[0])!.focus();
	const search = target.querySelector<HTMLInputElement>('main input[type="search"]')!;
	search.value = 'zzz';
	search.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
	expect(ids()).toEqual([]);
	expect(document.activeElement).toBe(document.body);
});

test('disconnect disables only that row, DELETEs its id, toasts and the row goes on the next read', async () => {
	await render('#/connections');
	const list = rows('main [data-connection]');
	const chrome = list[1];
	const disconnect = button('Disconnect', chrome)!;
	disconnect.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(target.querySelector('#tooltip')?.textContent).toBe('Disconnect');
	click(disconnect);
	expect(disconnect.disabled).toBe(true);
	expect(button('Disconnect', list[0])!.disabled).toBe(false);
	await settle();
	expect(requested('DELETE /apis/stats/connections/102')).toBe(1);
	expect(requested('GET /apis/stats')).toBe(2);
	expect(connectionIds()).toEqual(['103', '101', '201']);
	expect(toasts.list.map((toast) => [toast.kind, toast.message])).toEqual([
		['success', 'Connection closed.']
	]);
	// The hovered row was removed: its tooltip is gone too.
	expect(target.querySelector('#tooltip')).toBeNull();
	// The surviving rows kept their elements.
	expect(rows('main [data-connection]')[0]).toBe(list[0]);

	fail.set('DELETE /apis/stats/connections/101', 'connection 101 not found');
	click(button('Disconnect', list[2]));
	await settle();
	expect(toasts.list.at(-1)).toMatchObject({ kind: 'error', message: 'connection 101 not found' });
	expect(button('Disconnect', list[2])!.disabled).toBe(false);
});

test('at most 200 of many connections are rendered, with a note saying so', async () => {
	const template = snapshot.rules![0].connections![0];
	snapshot.rules![0].connections = Array.from({ length: 250 }, (_, index) => ({
		...template,
		id: 1_000 + index,
		client: `10.0.0.${index % 250}:${20_000 + index}`,
		started: new Date(NOW - (250 - index) * 1_000).toISOString()
	}));
	await render('#/connections');
	const list = rows('main [data-connection]');
	expect(list).toHaveLength(200);
	expect(list[0].dataset.connection).toBe('1249');
	expect(compact(target.querySelector('[data-connection-count]'))).toBe('251 connections');
	expect(target.querySelector('main')?.textContent).toContain('Showing 200 of 251');
});

test('virtual rows label channels as virtual:// and link peers from the loaded rule list; without that list the association stays unknown', async () => {
	rules.push(...structuredClone(virtualRulesFixture));
	await render('#/stats');
	const peers = (row: Element) =>
		Array.from(row.querySelectorAll('[data-virtual-peers] a')).map((link) => [
			link.textContent?.trim(),
			link.getAttribute('href')
		]);
	const list = rows();
	expect(list.map((row) => row.dataset.rule)).toEqual([
		'office',
		'mirror',
		'db-tunnel',
		'lab',
		'shared-exit',
		'lan-entry',
		'orphan'
	]);
	const [exit, entry, orphan] = list.slice(4);
	expect(compact(exit.querySelector('[data-mode]'))).toBe('Proxy');
	expect(compact(exit.querySelector('[data-address]'))).toBe('virtual://exit');
	expect(compact(exit.querySelector('[data-virtual-peers]'))).toBe('Incoming rules lan-entry');
	expect(peers(exit)).toEqual([['lan-entry', '#/rules/lan-entry']]);
	expect(compact(entry.querySelector('[data-mode]'))).toBe('Port forward');
	expect(compact(entry.querySelector('[data-address]'))).toBe(
		'0.0.0.0:18101 \u2192 virtual://exit'
	);
	expect(peers(entry)).toEqual([['shared-exit', '#/rules/shared-exit']]);
	expect(compact(orphan.querySelector('[data-virtual-peers]'))).toBe(
		'No enabled rule listens on this channel.'
	);
	expect(list[0].querySelector('[data-virtual-peers]')).toBeNull();
	expect(exit.querySelectorAll('button a, a button')).toHaveLength(0);

	unmount(app!);
	app = null;
	target.remove();
	snapshot.rules!.push({
		name: 'orphan',
		stats: statsOf({}),
		listen: null,
		forward: null,
		targets: null,
		connections: null,
		targets_evicted: 0
	});
	fail.set('GET /apis/configs/rules', 'boom');
	await render('#/stats');
	expect(rows().map((row) => row.dataset.rule)).toContain('orphan');
	expect(target.querySelector('main [data-virtual-peers]')).toBeNull();
});

const input = (element: HTMLInputElement | HTMLSelectElement, value: string) => {
	element.value = value;
	element.dispatchEvent(
		new Event(element instanceof HTMLSelectElement ? 'change' : 'input', { bubbles: true })
	);
	flushSync();
};
const searchBox = () => target.querySelector<HTMLInputElement>('main input[type="search"]')!;
const sortSelect = () =>
	target.querySelector<HTMLSelectElement>('main select[aria-label="Sort by"]')!;
const ruleNames = () => rows().map((row) => row.dataset.rule);
const hostNames = () => rows('main article[data-host]').map((row) => row.dataset.host);

test('the failures cell pairs failed with attempted dials and explains both numbers in both languages; compact bands carry the pair without help', async () => {
	await render('#/stats');
	const office = rows()[0];
	const cell = office.querySelector('[data-failures]')!;
	expect(compact(cell.querySelector('dt'))).toBe('Failures Failed / attempted');
	expect(compact(cell.querySelector('dd'))).toBe('2 / 118');
	expect(Array.from(cell.querySelectorAll('dd span')).map(compact)).toEqual(['2', '118']);
	let text = helpText('About Failures', office);
	expect(text).toMatch(/^Failures: /);
	expect(text).toContain('did not connect');
	expect(text).toContain('all dial attempts');
	switchLanguage('中文');
	expect(compact(cell.querySelector('dt'))).toBe('失败 失败 / 尝试');
	text = helpText('失败说明', office);
	expect(text).toContain('未能建连的尝试次数');
	expect(text).toContain('全部建连尝试次数');
	switchLanguage('English');
	// A URL band inside the expansion shows the same pair, without a help tip.
	click(button('Expand: office', office)!);
	const url = detailsOf(button('Collapse: office', office)!).querySelectorAll('[data-url]')[1];
	expect(compact(url.querySelector('[data-failures] dt'))).toBe('Failures Failed / attempted');
	expect(compact(url.querySelector('[data-failures] dd'))).toBe('1 / 3');
	expect(url.querySelector('[data-failures] [data-help]')).toBeNull();
});

test('an expanded rule lists its targets most recently active first with their hop and counters, notes dropped targets only when there are any, and caps the list at 20', async () => {
	await render('#/stats');
	const office = rows()[0];
	click(button('Expand: office', office)!);
	const details = detailsOf(button('Collapse: office', office)!);
	const stage = details.querySelector('[data-stage="target"]')!;
	// Dropped targets: the hint and its definition.
	const evicted = stage.querySelector('[data-evicted]')!;
	expect(compact(evicted)).toBe('3 older targets were dropped from this list');
	expect(helpText('About Targets', stage)).toContain('at most 1000 targets');
	expect(helpText('About Targets', stage)).toContain('stays in the rule totals');
	// The list: cdn (newest) first, the never-active 10.1.2.3 last; each with its own band.
	const items = Array.from(stage.querySelectorAll('[data-targets] > [data-target]'));
	expect(items.map((item) => compact(item.querySelector('[data-target-address]')))).toEqual([
		'cdn.example.net:443',
		'example.com:443',
		'10.1.2.3:8080'
	]);
	expect(items.map((item) => compact(item.querySelector('[data-via]')))).toEqual([
		'via ssh://bastion-2.example:22',
		'via ssh://bastion.example:22',
		null
	]);
	items[1].querySelector('[data-via]')!.dispatchEvent(new Event('pointerenter'));
	flushSync();
	expect(tooltipText()).toBe('ssh://xxxxx@bastion.example:22');
	expect(details.innerHTML).not.toContain('ops@');
	expect(traffic(items[0])).toEqual([
		'0 B/s',
		'0 B/s',
		'1.9 MB',
		'\u2014',
		'0 B/s',
		'0 B/s',
		'38.1 MB',
		'\u2014'
	]);
	expect(compact(items[0].querySelector('[data-connections] dd'))).toBe('1 / 30');
	expect(compact(items[0].querySelector('[data-latency] dd'))).toBe('22.0 ms / 24.5 ms');
	expect(compact(items[0].querySelector('[data-failures] dd'))).toBe('0 / 30');
	expect(compact(items[1].querySelector('[data-failures] dd'))).toBe('2 / 80');
	// Compact bands: no help tips of their own.
	expect(stage.querySelectorAll('[data-target] [data-help]')).toHaveLength(0);
	expect(stage.querySelector('[data-targets-more]')).toBeNull();

	// mirror has nothing dropped: no hint; its single target is listed.
	click(button('Expand: mirror')!);
	const mirror = detailsOf(button('Collapse: mirror')!).querySelector('[data-stage="target"]')!;
	expect(mirror.querySelector('[data-evicted]')).toBeNull();
	expect(
		Array.from(mirror.querySelectorAll('[data-target] [data-target-address]')).map(compact)
	).toEqual(['10.0.0.5:5432']);

	// 25 targets on the next poll: 20 listed, the note says so; the hint follows the count.
	const template = snapshot.rules![0].targets![0];
	snapshot.rules![0].targets = Array.from({ length: 25 }, (_, index) => ({
		...structuredClone(template),
		address: `10.9.0.${index}:443`,
		stats: statsOf({
			last_active: new Date(NOW - (25 - index) * 60_000).toISOString()
		})
	}));
	snapshot.rules![0].targets_evicted = 1;
	await poll();
	const listed = Array.from(stage.querySelectorAll('[data-target] [data-target-address]')).map(
		compact
	);
	expect(listed).toHaveLength(20);
	expect(listed[0]).toBe('10.9.0.24:443');
	expect(listed[19]).toBe('10.9.0.5:443');
	expect(compact(stage.querySelector('[data-targets-more]'))).toBe('Showing 20 of 25 targets');
	expect(compact(stage.querySelector('[data-evicted]'))).toBe(
		'1 older targets were dropped from this list'
	);
	expect(stage.textContent).toContain('25 distinct targets');
	snapshot.rules![0].targets_evicted = 0;
	await poll();
	expect(stage.querySelector('[data-evicted]')).toBeNull();
});

test('#/stats has a search box and sort controls: the search narrows the rows by name, address or target and reports the count; sorting reorders by a counter and the direction button flips it', async () => {
	await render('#/stats');
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	expect(target.querySelector('[data-rule-count]')).toBeNull();
	const search = searchBox();
	expect(search.getAttribute('aria-label')).toBe('Search');
	input(search, '1809');
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel']);
	expect(compact(target.querySelector('[data-rule-count]'))).toBe('3 of 4 rules');
	input(search, 'tun');
	expect(ruleNames()).toEqual(['db-tunnel']);
	expect(compact(target.querySelector('[data-rule-count]'))).toBe('1 of 4 rules');
	// The forward target is searched too.
	input(search, '5432');
	expect(ruleNames()).toEqual(['mirror', 'db-tunnel']);
	input(search, 'zzz');
	expect(ruleNames()).toEqual([]);
	expect(target.querySelector('main')?.textContent).toContain('No matches');
	expect(target.querySelector('main')?.textContent).not.toContain('No rules yet.');
	expect(compact(target.querySelector('[data-rule-count]'))).toBe('0 of 4 rules');
	click(button('Clear search'));
	expect(search.value).toBe('');
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	expect(target.querySelector('[data-rule-count]')).toBeNull();

	// Sorting: configured order by default; a counter reorders; the button flips.
	const sortBy = sortSelect();
	expect(sortBy.value).toBe('configured');
	expect(Array.from(sortBy.options).map((option) => option.value)).toEqual([
		'configured',
		'name',
		'rate_down',
		'rate_up',
		'peak_rate_down',
		'peak_rate_up',
		'down',
		'up',
		'last_down',
		'last_up',
		'active',
		'dial_failures'
	]);
	expect(Array.from(sortBy.options).map((option) => option.textContent)).toEqual([
		'Configured order',
		'Rule',
		'Download \u00b7 now',
		'Upload \u00b7 now',
		'Download \u00b7 peak',
		'Upload \u00b7 peak',
		'Download \u00b7 total',
		'Upload \u00b7 total',
		'Download \u00b7 last',
		'Upload \u00b7 last',
		'Connections',
		'Failures'
	]);
	const direction = button('Ascending')!;
	input(sortBy, 'down');
	expect(ruleNames()).toEqual(['db-tunnel', 'lab', 'mirror', 'office']);
	click(direction);
	expect(direction.getAttribute('aria-label')).toBe('Descending');
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	input(sortBy, 'dial_failures');
	expect(ruleNames()).toEqual(['db-tunnel', 'office', 'mirror', 'lab']);
	input(sortBy, 'name');
	expect(ruleNames()).toEqual(['office', 'mirror', 'lab', 'db-tunnel']);
	input(sortBy, 'configured');
	expect(ruleNames()).toEqual(['lab', 'db-tunnel', 'mirror', 'office']);
	click(direction);
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);

	// A poll that moves a focused row keeps its element, expansion and focus.
	input(sortBy, 'down');
	click(direction);
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	const mirror = rows()[1];
	click(button('Expand: mirror', mirror)!);
	const toggle = button('Collapse: mirror', mirror)!;
	toggle.focus();
	snapshot.rules![1].stats.down = 500_000_000;
	await poll();
	expect(ruleNames()).toEqual(['mirror', 'office', 'db-tunnel', 'lab']);
	expect(rows()[0]).toBe(mirror);
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(document.activeElement).toBe(toggle);
	// The search applies on top of the sort.
	input(search, 'o');
	expect(ruleNames()).toEqual(['mirror', 'office']);
});

test('#/hosts has the same controls: the search matches host names and endpoint labels, sorting by host name orders alphabetically and the count line appears while filtering', async () => {
	await render('#/hosts');
	expect(hostNames()).toEqual(hostsTotals.order);
	expect(target.querySelector('[data-host-count]')).toBeNull();
	const search = searchBox();
	input(search, 'bastion');
	expect(hostNames()).toEqual(['bastion.example', 'bastion-2.example']);
	expect(compact(target.querySelector('[data-host-count]'))).toBe('2 of 4 hosts');
	// Endpoint labels are searched: only the exit hop is a SOCKS URL.
	input(search, 'socks5');
	expect(hostNames()).toEqual(['hop-a.example']);
	input(search, 'zzz');
	expect(hostNames()).toEqual([]);
	expect(target.querySelector('main')?.textContent).toContain('No matches');
	expect(target.querySelector('main')?.textContent).not.toContain('No proxy hosts.');
	click(button('Clear search'));
	expect(hostNames()).toEqual(hostsTotals.order);
	expect(target.querySelector('[data-host-count]')).toBeNull();

	const sortBy = sortSelect();
	expect(sortBy.value).toBe('traffic');
	expect(Array.from(sortBy.options).map((option) => option.textContent)).toEqual([
		'Traffic',
		'Host',
		'Current rate',
		'Connections',
		'Failures',
		'Latency'
	]);
	const direction = button('Descending')!;
	input(sortBy, 'host');
	expect(hostNames()).toEqual([
		'hop-a.example',
		'edge.example',
		'bastion.example',
		'bastion-2.example'
	]);
	click(direction);
	expect(direction.getAttribute('aria-label')).toBe('Ascending');
	expect(hostNames()).toEqual([
		'bastion-2.example',
		'bastion.example',
		'edge.example',
		'hop-a.example'
	]);
	click(direction);
	input(sortBy, 'dial_failures');
	expect(hostNames()[0]).toBe('edge.example');
	input(sortBy, 'traffic');
	expect(hostNames()).toEqual(hostsTotals.order);
	click(direction);
	expect(hostNames()).toEqual([...hostsTotals.order].reverse());
});
