import { flushSync, mount, unmount } from 'svelte';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
	LEAKY_SSH_URL,
	LEAKY_URL,
	SECRET,
	rulesFixture,
	snapshotFixture,
	snapshotTotals,
	statusFixture
} from '../e2e/fixtures/api';
import App from './App.svelte';
import { confirm } from './lib/confirm';
import { formatCount, formatShortTime } from './lib/format';
import { sumStats } from './lib/hosts';
import { router } from './lib/router.svelte';
import { stats } from './lib/stats.svelte';
import { status } from './lib/status.svelte';
import { toasts } from './lib/toast.svelte';
import { list } from './lib/types';

let target: HTMLElement;
let app: ReturnType<typeof mount> | null = null;
let requests: string[];

const json = (body: unknown) =>
	new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});

const click = (element: Element) =>
	element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));

function stubApi(overrides: Record<string, () => Response | Promise<Response>> = {}) {
	requests = [];
	vi.stubGlobal(
		'fetch',
		vi.fn((url: string) => {
			requests.push(url);
			const override = overrides[url];
			if (override) return Promise.resolve().then(override);
			switch (url) {
				case '/apis/configs/status':
					return Promise.resolve(json(statusFixture));
				case '/apis/configs/rules':
					return Promise.resolve(json(rulesFixture));
				case '/apis/stats':
					return Promise.resolve(json(snapshotFixture));
			}
			return Promise.resolve(new Response('not found', { status: 404 }));
		})
	);
}

const settle = () => vi.advanceTimersByTimeAsync(0);

function render() {
	target = document.body.appendChild(document.createElement('div'));
	app = mount(App, { target });
	flushSync();
}

function teardown() {
	if (app) unmount(app);
	app = null;
	target?.remove();
}

// Below the desktop breakpoint: header + drawer instead of the sidebar.
function stubMobile() {
	vi.stubGlobal(
		'matchMedia',
		vi.fn((query: string) => ({
			matches: false,
			media: query,
			addEventListener: vi.fn(),
			removeEventListener: vi.fn()
		}))
	);
}

const prompt = () =>
	target.querySelector<HTMLDialogElement>('dialog[open]:not(#navigation-drawer)');

beforeEach(() => {
	vi.useFakeTimers();
	history.replaceState(null, '', '/');
	stubApi();
});

afterEach(() => {
	teardown();
	toasts.clear();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
	localStorage.clear();
	document.documentElement.removeAttribute('data-theme');
	document.documentElement.lang = 'en';
	document.body.innerHTML = '';
});

test('renders the brand with the bundled icon asset and one link per top-level route', () => {
	render();
	const brand = target.querySelector('a[href="#/"] img');
	expect(brand?.getAttribute('src')).toMatch(/icon/);
	expect(target.textContent).toContain('JumpWay');
	const links = Array.from(target.querySelectorAll('nav a')).map((a) => a.getAttribute('href'));
	expect(links).toEqual(['#/', '#/stats', '#/hosts', '#/connections', '#/settings', '#/yaml']);
	// The labels say what each page is for, not what it is made of.
	expect(Array.from(target.querySelectorAll('nav a')).map((a) => a.textContent?.trim())).toEqual([
		'Overview',
		'Rule Traffic',
		'Proxy Hosts',
		'Live Connections',
		'Global Settings',
		'Configuration File'
	]);
	expect(target.querySelector('nav a[aria-current="page"]')?.getAttribute('href')).toBe('#/');
});

test('polls the runtime status while mounted and stops on unmount', async () => {
	render();
	expect(requests).toContain('/apis/configs/status');
	await settle();
	expect(target.textContent).toContain('Running');
	expect(target.textContent).toContain('127.0.0.1:1088');
	expect(status.active).toBe(true);
	teardown();
	expect(status.active).toBe(false);
});

test('the overview shows KPIs and one card per configured rule from status, rules and stats', async () => {
	render();
	await settle();
	expect(requests).toContain('/apis/configs/rules');
	expect(requests).toContain('/apis/stats');
	const kpi = (name: string) => target.querySelector(`[data-kpi="${name}"]`)?.textContent?.trim();
	expect(kpi('rules')).toBe('2 Running');
	expect(kpi('rule-states')?.replace(/\s+/g, ' ')).toBe('4 configured · 1 Retrying · 1 Disabled');
	expect(kpi('active')).toBe(String(snapshotTotals.active));
	const total = sumStats(list(snapshotFixture.rules).map((rule) => rule.stats)).total;
	expect(kpi('connections-total')?.replace(/\s+/g, ' ')).toBe(
		`${formatCount(total)} total · since ${formatShortTime(snapshotFixture.since)}`
	);
	// The direction arrows carry their names for assistive technology.
	expect(kpi('rate')?.replace(/\s+/g, ' ')).toBe(
		`Upload ${snapshotTotals.rateUp} Download ${snapshotTotals.rateDown}`
	);
	expect(kpi('total')?.replace(/\s+/g, ' ')).toBe(
		`Upload ${snapshotTotals.up} Download ${snapshotTotals.down}`
	);

	const cards = Array.from(target.querySelectorAll('article'));
	expect(cards.map((card) => card.querySelector('a')?.textContent?.trim())).toEqual([
		'office',
		'mirror',
		'db-tunnel',
		'lab'
	]);
	const chips = cards.map((card) => card.querySelector('[data-state]')?.getAttribute('data-state'));
	expect(chips).toEqual(['running', 'running', 'retrying', 'disabled']);
	const office = cards[0].textContent!.replace(/\s+/g, ' ');
	expect(office).toContain('127.0.0.1:18097');
	expect(office).toContain('2 hops');
	expect(office).toContain('12.0 KB/s');
	expect(office).toContain('1.0 MB/s');
	expect(cards[1].textContent).toContain('10.0.0.5:5432');
	expect(cards[2].textContent).toContain('Retrying (3)');
	expect(cards[2].textContent).toContain('connection refused');
	expect(cards[2].textContent).toContain('remote');
	expect(cards[0].querySelector('a[href="#/rules/office"]')).not.toBeNull();
	expect(cards[0].querySelector('a[href="#/stats?rule=office"]')).not.toBeNull();
	expect(stats.active).toBe(true);
});

test('runtime and rule errors show proxy URLs with their credentials masked, nowhere in the DOM', async () => {
	stubApi({
		'/apis/configs/rules': () =>
			new Response(`rules[0].forward.way[0]: invalid proxy URL "${LEAKY_URL}": bad`, {
				status: 400
			})
	});
	render();
	await settle();
	expect(statusFixture.error).toContain(SECRET);
	expect(statusFixture.rules?.[2].error).toContain(SECRET);
	const runtime = target.querySelector('aside [data-state]')!.parentElement!;
	expect(runtime.textContent).toContain(
		'invalid proxy URL "socks5://xxxxx@host:bad": parse "socks5://xxxxx@host:bad": invalid port ":bad" after host (e.g. socks5://host:1080)'
	);
	const banner = target.querySelector('main [role="alert"]')!;
	expect(banner.textContent).toContain('invalid proxy URL "socks5://xxxxx@host:bad": bad');
	expect(target.innerHTML).not.toContain(SECRET);
	expect(target.innerHTML).not.toContain('review-user');
	expect(target.innerHTML).not.toContain(LEAKY_URL);
	expect(target.innerHTML).not.toContain(LEAKY_SSH_URL);
	for (const element of target.querySelectorAll('[title]')) {
		expect(element.getAttribute('title')).not.toContain(SECRET);
	}
});

test('a rule card masks the credentials in its runtime error', async () => {
	render();
	await settle();
	const card = Array.from(target.querySelectorAll('article'))[2];
	expect(card.textContent).toContain(
		'listen through "ssh://xxxxx@edge.example:22": ssh: handshake failed'
	);
	expect(card.innerHTML).not.toContain(SECRET);
	expect(card.innerHTML).not.toContain('ops:');
});

test('a failed rule list shows an error banner whose retry reloads it', async () => {
	let failures = 1;
	stubApi({
		'/apis/configs/rules': () =>
			failures-- > 0 ? new Response('boom', { status: 500 }) : json(rulesFixture)
	});
	render();
	await settle();
	const banner = target.querySelector('main [role="alert"]');
	expect(banner?.textContent).toContain('boom');
	click(banner!.querySelector('button')!);
	await settle();
	expect(target.querySelector('main [role="alert"]')).toBeNull();
	expect(target.querySelectorAll('article')).toHaveLength(4);
});

test('settings does not poll stats; a retired address such as #/web-ui is an unknown one', async () => {
	history.replaceState(null, '', '/#/settings');
	render();
	await settle();
	expect(location.hash).toBe('#/settings');
	expect(target.querySelector('h1')?.textContent).toBe('Global Settings');
	expect(document.title).toBe('Global Settings \u00b7 JumpWay');
	expect(target.querySelector('nav a[aria-current="page"]')?.getAttribute('href')).toBe(
		'#/settings'
	);
	expect(requests).not.toContain('/apis/stats');
	expect(stats.active).toBe(false);
	teardown();
	history.replaceState(null, '', '/#/web-ui');
	render();
	await settle();
	expect(location.hash).toBe('#/');
	expect(target.querySelector('h1')?.textContent).toBe('Overview');
});

test('sidebar links navigate in place and stats polling follows the overview', async () => {
	render();
	await settle();
	expect(stats.active).toBe(true);
	click(target.querySelector('nav a[href="#/yaml"]')!);
	await settle();
	expect(location.hash).toBe('#/yaml');
	expect(target.querySelector('h1')?.textContent).toBe('Configuration File');
	expect(stats.active).toBe(false);
	click(target.querySelector('nav a[href="#/"]')!);
	await settle();
	expect(target.querySelector('h1')?.textContent).toBe('Overview');
	expect(stats.active).toBe(true);
});

test('?lang=zh renders Chinese and the language switch persists the choice', async () => {
	history.replaceState(null, '', '/?lang=zh#/');
	render();
	expect(document.documentElement.lang).toBe('zh-CN');
	expect(target.querySelector('h1')?.textContent).toBe('\u6982\u89c8');
	expect(Array.from(target.querySelectorAll('nav a')).map((a) => a.textContent?.trim())).toEqual([
		'\u6982\u89c8',
		'\u89c4\u5219\u6d41\u91cf',
		'\u8df3\u677f\u4e3b\u673a',
		'\u6d3b\u52a8\u8fde\u63a5',
		'\u5168\u5c40\u8bbe\u7f6e',
		'\u914d\u7f6e\u6587\u4ef6'
	]);
	const english = target.querySelector<HTMLButtonElement>('[aria-label="English"]')!;
	expect(english.getAttribute('aria-pressed')).toBe('false');
	click(english);
	flushSync();
	expect(english.getAttribute('aria-pressed')).toBe('true');
	expect(target.querySelector('h1')?.textContent).toBe('Overview');
	expect(localStorage.getItem('jumpway.lang')).toBe('en');
	expect(location.search).toBe('?lang=en');
});

test('the theme switch stamps data-theme and persists', () => {
	render();
	click(target.querySelector('[aria-label="Dark"]')!);
	flushSync();
	expect(document.documentElement.dataset.theme).toBe('dark');
	expect(localStorage.getItem('jumpway.theme')).toBe('dark');
	expect(target.querySelector('[aria-label="Dark"]')?.getAttribute('aria-pressed')).toBe('true');
	expect(target.querySelector('a[href="#/"] img')?.getAttribute('src')).toMatch(/icon_white/);
});

const NAV_LABELS = [
	'Overview',
	'Rule Traffic',
	'Proxy Hosts',
	'Live Connections',
	'Global Settings',
	'Configuration File'
];
const navLinks = () => Array.from(target.querySelectorAll('aside nav a'));
const sidebarToggle = () =>
	target.querySelector<HTMLButtonElement>('aside button[aria-controls="sidebar"]')!;

test('the desktop sidebar collapses to an icon rail that keeps every control reachable, and the choice survives a reload', async () => {
	render();
	await settle();
	expect(target.querySelector('aside')?.id).toBe('sidebar');
	const toggle = sidebarToggle();
	expect(toggle.getAttribute('aria-label')).toBe('Collapse sidebar');
	expect(toggle.getAttribute('aria-expanded')).toBe('true');
	expect(navLinks().map((a) => a.textContent?.trim())).toEqual(NAV_LABELS);

	toggle.focus();
	click(toggle);
	flushSync();
	expect(sidebarToggle()).toBe(toggle);
	expect(document.activeElement).toBe(toggle);
	expect(toggle.getAttribute('aria-label')).toBe('Expand sidebar');
	expect(toggle.getAttribute('aria-expanded')).toBe('false');
	expect(localStorage.getItem('jumpway.sidebarCollapsed')).toBe('true');
	// Icon links: the name moves into aria-label, nothing visible is left to clip.
	expect(navLinks().map((a) => a.getAttribute('aria-label'))).toEqual(NAV_LABELS);
	expect(navLinks().map((a) => a.textContent?.trim())).toEqual(['', '', '', '', '', '']);
	expect(navLinks().every((a) => a.querySelector('svg') !== null)).toBe(true);
	expect(target.querySelector('aside nav a[aria-current="page"]')?.getAttribute('href')).toBe('#/');
	// The brand keeps its bitmap and its name, not a cropped word.
	const brand = target.querySelector('aside a[href="#/"]')!;
	expect(brand.querySelector('img')).not.toBeNull();
	expect(brand.textContent?.trim()).toBe('');
	expect(brand.getAttribute('aria-label')).toBe('JumpWay');
	// The runtime shrinks to a dot whose name, address and error stay readable.
	const runtime = target.querySelector<HTMLButtonElement>('aside [data-state]')!;
	expect(runtime.tagName).toBe('BUTTON');
	expect(runtime.getAttribute('data-state')).toBe('running');
	expect(runtime.getAttribute('aria-label')).toBe('Runtime: Running');
	const details = document.getElementById(runtime.getAttribute('aria-describedby')!)!;
	expect(details.textContent).toContain('127.0.0.1:1088');
	expect(details.textContent).toContain('invalid proxy URL "socks5://xxxxx@host:bad"');
	expect(target.innerHTML).not.toContain(SECRET);
	// Language and theme live behind one Preferences button; the resources stay icon links.
	const preferences = target.querySelector<HTMLButtonElement>(
		'aside button[aria-label="Preferences"]'
	)!;
	const menu = document.getElementById(preferences.getAttribute('popovertarget')!)!;
	expect(menu.getAttribute('popover')).toBe('auto');
	expect(preferences.getAttribute('aria-controls')).toBe(menu.id);
	expect(preferences.getAttribute('aria-expanded')).toBe('false');
	expect(
		menu.querySelector('[role="group"][aria-label="Language"] [aria-label="中文"]')
	).not.toBeNull();
	expect(
		menu.querySelector('[role="group"][aria-label="Theme"] [aria-label="Dark"]')
	).not.toBeNull();
	expect(
		Array.from(target.querySelectorAll('aside [aria-label="Resources"] a')).map((a) =>
			a.getAttribute('aria-label')
		)
	).toEqual(['Prometheus metrics', 'API docs', 'pprof', 'GitHub']);
	click(menu.querySelector('[aria-label="Dark"]')!);
	flushSync();
	expect(document.documentElement.dataset.theme).toBe('dark');
	click(menu.querySelector('[aria-label="中文"]')!);
	flushSync();
	expect(navLinks().map((a) => a.getAttribute('aria-label'))).toEqual([
		'\u6982\u89c8',
		'\u89c4\u5219\u6d41\u91cf',
		'\u8df3\u677f\u4e3b\u673a',
		'\u6d3b\u52a8\u8fde\u63a5',
		'\u5168\u5c40\u8bbe\u7f6e',
		'\u914d\u7f6e\u6587\u4ef6'
	]);
	expect(toggle.getAttribute('aria-label')).toBe('\u5c55\u5f00\u4fa7\u8fb9\u680f');
	expect(runtime.getAttribute('aria-label')).toBe('\u8fd0\u884c\u72b6\u6001: \u8fd0\u884c\u4e2d');
	click(menu.querySelector('[aria-label="English"]')!);
	flushSync();
	expect(toggle.getAttribute('aria-label')).toBe('Expand sidebar');

	// The rail is remembered across a reload; the compact runtime expands it again.
	teardown();
	render();
	await settle();
	expect(sidebarToggle().getAttribute('aria-expanded')).toBe('false');
	expect(navLinks().map((a) => a.textContent?.trim())).toEqual(['', '', '', '', '', '']);
	click(target.querySelector('aside [data-state]')!);
	flushSync();
	expect(sidebarToggle().getAttribute('aria-expanded')).toBe('true');
	expect(navLinks().map((a) => a.textContent?.trim())).toEqual(NAV_LABELS);
	expect(navLinks().map((a) => a.getAttribute('aria-label'))).toEqual(Array(6).fill(null));
	expect(target.querySelector('aside [data-state]')?.tagName).toBe('P');
	expect(target.querySelector('aside button[aria-label="Preferences"]')).toBeNull();
	expect(localStorage.getItem('jumpway.sidebarCollapsed')).toBe('false');
});

test('a denied storage neither blocks collapsing nor breaks the start-up read; the drawer ignores the rail choice', async () => {
	vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
		throw new Error('denied');
	});
	vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
		throw new Error('denied');
	});
	render();
	expect(sidebarToggle().getAttribute('aria-expanded')).toBe('true');
	click(sidebarToggle());
	flushSync();
	expect(sidebarToggle().getAttribute('aria-expanded')).toBe('false');
	expect(navLinks().map((a) => a.textContent?.trim())).toEqual(['', '', '', '', '', '']);
	teardown();
	vi.restoreAllMocks();

	localStorage.setItem('jumpway.sidebarCollapsed', 'true');
	stubMobile();
	render();
	expect(target.querySelector('aside')).toBeNull();
	click(target.querySelector('header button[aria-expanded]')!);
	flushSync();
	const drawer = target.querySelector<HTMLDialogElement>('dialog#navigation-drawer')!;
	expect(Array.from(drawer.querySelectorAll('nav a')).map((a) => a.textContent?.trim())).toEqual(
		NAV_LABELS
	);
	expect(drawer.querySelector('[aria-label="English"]')).not.toBeNull();
	expect(drawer.querySelector('[aria-label="Dark"]')).not.toBeNull();
	expect(drawer.querySelector('[aria-label="Preferences"]')).toBeNull();
	expect(drawer.querySelector('[aria-controls="sidebar"]')).toBeNull();
});

test('an unreachable backend shows the recovery banner until a retry succeeds', async () => {
	let down = true;
	stubApi({
		'/apis/configs/status': () =>
			down ? Promise.reject(new TypeError('Failed to fetch')) : json(statusFixture)
	});
	render();
	await settle();
	const banner = target.querySelector('main [role="alert"]')!;
	expect(banner.textContent).toContain('Cannot reach JumpWay.');
	expect(banner.textContent).toContain('Edit Config');
	expect(target.querySelector('[data-state="unknown"]')).not.toBeNull();
	down = false;
	click(banner.querySelector('button')!);
	await settle();
	expect(target.querySelector('main [role="alert"]')).toBeNull();
	expect(target.textContent).toContain('Running');
});

test('confirm() opens the dialog, focuses the safe answer for danger and resolves once', async () => {
	render();
	const button = target.querySelector<HTMLButtonElement>('nav a')!;
	button.focus();
	const answer = confirm({ message: 'Delete rule "office"?', danger: true });
	await settle();
	const dialog = target.querySelector<HTMLDialogElement>('dialog[open]')!;
	expect(dialog.textContent).toContain('Delete rule "office"?');
	const [cancel, ok] = Array.from(dialog.querySelectorAll('button'));
	expect(document.activeElement).toBe(cancel);
	expect(ok.className).toContain('btn-danger');
	// A second question while one is open is refused instead of stacking.
	await expect(confirm({ message: 'again' })).resolves.toBe(false);
	click(ok);
	await expect(answer).resolves.toBe(true);
	expect(dialog.open).toBe(false);
	expect(document.activeElement).toBe(button);

	const dismissed = confirm({ message: 'Reset statistics for all rules?' });
	await settle();
	expect(document.activeElement?.textContent?.trim()).toBe('Confirm');
	// Escape: the browser removes `open` first, then fires close.
	dialog.open = false;
	dialog.dispatchEvent(new Event('close'));
	await expect(dismissed).resolves.toBe(false);

	const orphan = confirm({ message: 'unmounting' });
	await settle();
	teardown();
	await expect(orphan).resolves.toBe(false);
});

test('the close event queued by the previous answer does not settle the next question', async () => {
	render();
	const opener = target.querySelector<HTMLElement>('nav a')!;
	opener.focus();
	const first = confirm({ message: 'first question' });
	await settle();
	const dialog = prompt()!;
	click(dialog.querySelectorAll('button')[1]);
	await expect(first).resolves.toBe(true);
	expect(dialog.open).toBe(false);

	let answer: boolean | null = null;
	const second = confirm({ message: 'second question' }).then((value) => (answer = value));
	await settle();
	expect(dialog.open).toBe(true);
	expect(dialog.textContent).toContain('second question');
	// dialog.close() from the first answer delivers its close event only now, after the reopen.
	dialog.dispatchEvent(new Event('close'));
	await settle();
	expect(dialog.open).toBe(true);
	expect(dialog.textContent).toContain('second question');
	expect(answer).toBeNull();

	click(dialog.querySelectorAll('button')[0]);
	await second;
	expect(answer).toBe(false);
	expect(dialog.open).toBe(false);
	expect(document.activeElement).toBe(opener);

	// A genuine Escape after a reopen still counts as "no".
	const third = confirm({ message: 'third question' });
	await settle();
	expect(dialog.open).toBe(true);
	dialog.open = false;
	dialog.dispatchEvent(new Event('close'));
	await expect(third).resolves.toBe(false);
	expect(document.activeElement).toBe(opener);
});

test('toasts render with live-region roles and can be dismissed', () => {
	render();
	toasts.success('Saved and applied.');
	toasts.error('boom');
	flushSync();
	const host = target.querySelector('[aria-label="Notifications"]')!;
	expect(host.querySelector('[role="status"]')?.textContent).toContain('Saved and applied.');
	const alert = host.querySelector('[role="alert"]')!;
	expect(alert.textContent).toContain('boom');
	click(alert.querySelector('button[aria-label="Dismiss"]')!);
	flushSync();
	expect(host.querySelector('[role="alert"]')).toBeNull();
	expect(toasts.list).toHaveLength(1);
});

test('the tooltip host follows the hovered control and stays inside the viewport', async () => {
	render();
	const owner = target.querySelector<HTMLButtonElement>('[aria-label="Dark"]')!;
	vi.spyOn(owner, 'getBoundingClientRect').mockReturnValue({
		left: 1000,
		right: 1032,
		top: 500,
		bottom: 532,
		width: 32,
		height: 32
	} as DOMRect);
	Object.defineProperty(document.documentElement, 'clientWidth', {
		configurable: true,
		value: 1024
	});
	Object.defineProperty(document.documentElement, 'clientHeight', {
		configurable: true,
		value: 768
	});
	const width = vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(120);
	const height = vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(24);
	owner.dispatchEvent(new Event('pointerenter'));
	flushSync();
	const tip = target.parentElement!.querySelector<HTMLElement>('#tooltip')!;
	expect(tip.textContent?.trim()).toBe('Dark');
	expect(owner.getAttribute('aria-describedby')).toBe('tooltip');
	expect(tip.style.left).toBe('896px');
	expect(tip.style.top).toBe('538px');
	owner.dispatchEvent(new Event('pointerleave'));
	await settle();
	expect(document.getElementById('tooltip')).toBeNull();
	width.mockRestore();
	height.mockRestore();
	delete (document.documentElement as { clientWidth?: unknown }).clientWidth;
	delete (document.documentElement as { clientHeight?: unknown }).clientHeight;
});

test('the tooltip is a manual popover so it paints above a modal drawer', async () => {
	// jsdom has no Popover API; the host must promote itself whenever the browser offers it.
	const showPopover = vi.fn();
	Object.defineProperty(HTMLElement.prototype, 'showPopover', {
		configurable: true,
		value: showPopover
	});
	try {
		render();
		const owner = target.querySelector<HTMLButtonElement>('[aria-label="Dark"]')!;
		owner.dispatchEvent(new Event('pointerenter'));
		flushSync();
		const tip = document.getElementById('tooltip')!;
		expect(tip.getAttribute('popover')).toBe('manual');
		expect(showPopover).toHaveBeenCalledTimes(1);
		expect(showPopover.mock.instances[0]).toBe(tip);
		// Re-anchoring keeps the same open popover; a fresh element opens again.
		const other = target.querySelector<HTMLButtonElement>('[aria-label="Light"]')!;
		other.dispatchEvent(new Event('pointerenter'));
		flushSync();
		expect(document.getElementById('tooltip')).toBe(tip);
		expect(showPopover).toHaveBeenCalledTimes(1);
		other.dispatchEvent(new Event('pointerleave'));
		await settle();
		expect(document.getElementById('tooltip')).toBeNull();
		owner.dispatchEvent(new Event('pointerenter'));
		flushSync();
		expect(showPopover).toHaveBeenCalledTimes(2);
	} finally {
		delete (HTMLElement.prototype as { showPopover?: unknown }).showPopover;
	}
});

test('below the desktop breakpoint the drawer opens from the header, navigates, and closes on Escape', async () => {
	stubMobile();
	render();
	expect(target.querySelector('aside')).toBeNull();
	const menu = target.querySelector<HTMLButtonElement>('header button[aria-expanded]')!;
	const dialog = target.querySelector<HTMLDialogElement>('dialog#navigation-drawer')!;
	expect(dialog.open).toBe(false);
	menu.focus();
	click(menu);
	flushSync();
	expect(dialog.open).toBe(true);
	expect(menu.getAttribute('aria-expanded')).toBe('true');
	click(dialog.querySelector('nav a[href="#/hosts"]')!);
	await settle();
	expect(location.hash).toBe('#/hosts');
	expect(target.querySelector('h1')?.textContent).toBe('Proxy Hosts');
	expect(dialog.open).toBe(false);
	expect(document.activeElement).toBe(menu);

	click(menu);
	flushSync();
	expect(dialog.open).toBe(true);
	// The browser closes a modal dialog on Escape and fires close.
	dialog.open = false;
	dialog.dispatchEvent(new Event('close'));
	flushSync();
	expect(menu.getAttribute('aria-expanded')).toBe('false');
	expect(document.activeElement).toBe(menu);
});

test('a dirty navigation from the drawer is owned once: cancel keeps it open, confirm closes it and returns focus', async () => {
	stubMobile();
	render();
	await settle();
	let asked = 0;
	const unregister = router.registerLeaveGuard(() => {
		asked++;
		return true;
	});
	const navigate = vi.spyOn(router, 'navigate');
	const menu = target.querySelector<HTMLButtonElement>('header button[aria-expanded]')!;
	const drawer = target.querySelector<HTMLDialogElement>('dialog#navigation-drawer')!;
	menu.focus();
	click(menu);
	flushSync();
	expect(drawer.open).toBe(true);

	click(drawer.querySelector('nav a[href="#/hosts"]')!);
	await settle();
	expect(navigate).toHaveBeenCalledTimes(1);
	expect(asked).toBe(1);
	let question = prompt()!;
	expect(question.textContent).toContain('Discard unsaved changes?');
	click(question.querySelectorAll('button')[0]);
	await settle();
	expect(location.hash).toBe('#/');
	expect(drawer.open).toBe(true);
	expect(prompt()).toBeNull();

	click(drawer.querySelector('nav a[href="#/hosts"]')!);
	await settle();
	expect(navigate).toHaveBeenCalledTimes(2);
	expect(asked).toBe(2);
	question = prompt()!;
	click(question.querySelectorAll('button')[1]);
	await settle();
	expect(location.hash).toBe('#/hosts');
	expect(target.querySelector('h1')?.textContent).toBe('Proxy Hosts');
	expect(drawer.open).toBe(false);
	expect(document.activeElement).toBe(menu);
	expect(navigate).toHaveBeenCalledTimes(2);
	unregister();
});
