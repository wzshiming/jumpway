import { flushSync, mount, unmount } from 'svelte';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
	noProxyFixture,
	rawFixture,
	snapshotFixture,
	statusFixture,
	webUIFixture
} from '../../e2e/fixtures/api';
import App from '../App.svelte';
import { SAVED_PREFIX } from '../lib/api';
import { moved } from '../lib/moved.svelte';
import { status } from '../lib/status.svelte';
import { toasts } from '../lib/toast.svelte';
import type { Address, NoProxy, RawConfig } from '../lib/types';

// Settings (web UI address + no-proxy) and Advanced YAML mounted in jsdom against a
// method-aware /apis stub. Like a real deployment, the status reports this page's own origin
// (jsdom serves it from localhost:3000) until a write moves the listener.

interface Call {
	method: string;
	url: string;
	body: unknown;
}

let target: HTMLElement;
let app: ReturnType<typeof mount> | null = null;
let calls: Call[];
let webUI: Address;
let noProxy: NoProxy;
let raw: RawConfig;
let statusAddress: string;
let statusRunning: boolean;
// What GET /raw answers after a PUT; null echoes the written text.
let rawAfterSave: string | null;
// `${method} ${url}` → 400 text. A "saved, but " text still applies the write first.
let fail: Map<string, string>;
// `${method} ${url}` that fail at the network level.
let down: Set<string>;
// `${method} ${url}` whose write is applied but whose answer never arrives.
let lost: Set<string>;
// `${method} ${url}` whose next answer waits for the promise; its body is fixed when the request arrives.
let delay: Map<string, Promise<void>>;
// A PUT that changes the web UI address closes the old listener: status stops answering.
let stopOnMove: boolean;
// The new web UI listener fails to bind: like the tray, the status then names the configured
// address (port 0 included) with running false, and the PUT reports "saved, but".
let bindFails: boolean;

const json = (body: unknown) =>
	new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});
const text = (status: number, body: string) => new Response(body, { status });

function respond(method: string, url: string, body: unknown): Response {
	const entry = `${method} ${url}`;
	const injected = fail.get(entry);
	if (injected !== undefined && !injected.startsWith(SAVED_PREFIX)) return text(400, injected);
	const answer = () => {
		if (lost.has(entry)) throw new TypeError('Failed to fetch');
		return injected === undefined ? json(null) : text(400, injected);
	};
	switch (url) {
		case '/apis/configs/status':
			return json({ ...statusFixture, address: statusAddress, running: statusRunning });
		case '/apis/stats':
			return json(snapshotFixture);
		case '/apis/configs/rules':
			return json([]);
		case '/apis/configs/web-ui':
			if (method === 'GET') return json(webUI);
			webUI = body as Address;
			if (bindFails) {
				statusAddress = (webUI.host || '127.0.0.1') + ':' + webUI.port;
				statusRunning = false;
				return text(
					400,
					SAVED_PREFIX + 'web_ui: listen tcp ' + statusAddress + ': bind: address already in use'
				);
			}
			statusAddress = webUI.port
				? (webUI.host || '127.0.0.1') + ':' + webUI.port
				: '127.0.0.1:43210';
			if (stopOnMove) down.add('GET /apis/configs/status');
			return answer();
		case '/apis/configs/no-proxy':
			if (method === 'GET') return json(noProxy);
			noProxy = body as NoProxy;
			return answer();
		case '/apis/configs/raw':
			if (method === 'GET') return json(raw);
			raw = { yaml: rawAfterSave ?? (body as RawConfig).yaml };
			return answer();
	}
	return text(404, 'not found');
}

function stubApi() {
	calls = [];
	webUI = structuredClone(webUIFixture);
	noProxy = structuredClone(noProxyFixture);
	raw = structuredClone(rawFixture);
	statusAddress = location.host;
	statusRunning = true;
	rawAfterSave = null;
	fail = new Map();
	down = new Set();
	lost = new Set();
	delay = new Map();
	stopOnMove = false;
	bindFails = false;
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string, init?: RequestInit) => {
			const method = init?.method ?? 'GET';
			const body: unknown = init?.body ? JSON.parse(String(init.body)) : undefined;
			calls.push({ method, url, body });
			if (down.has(`${method} ${url}`)) throw new TypeError('Failed to fetch');
			const held = delay.get(`${method} ${url}`);
			delay.delete(`${method} ${url}`);
			// Fixed now: a slow answer still describes the state at the time of the request.
			const response = respond(method, url, body);
			if (held) await held;
			return response;
		})
	);
}

const settle = () => vi.advanceTimersByTimeAsync(0);
const writes = () => calls.filter((call) => call.method !== 'GET');
const requested = (entry: string) =>
	calls.filter((call) => `${call.method} ${call.url}` === entry).length;

const click = (element: Element | null | undefined) => {
	if (!element) throw new Error('missing element');
	element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
	flushSync();
};

const field = (id: string) => {
	const element = target.querySelector<HTMLInputElement | HTMLTextAreaElement>('#' + id);
	if (!element) throw new Error('missing field ' + id);
	return element;
};

const type = (id: string, value: string) => {
	const element = field(id);
	element.value = value;
	element.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
};

const button = (name: string, root: ParentNode = target) =>
	Array.from(root.querySelectorAll('button')).find(
		(element) => element.textContent?.trim() === name || element.getAttribute('aria-label') === name
	);

const addressForm = () => target.querySelector('form[aria-labelledby="web-ui-heading"]')!;
const bypassForm = () => target.querySelector('form[aria-labelledby="no-proxy-heading"]')!;
const badge = (root: ParentNode) =>
	root.querySelector('[role="status"]')?.textContent?.trim() ?? null;
const alerts = () =>
	Array.from(target.querySelectorAll('main [role="alert"]')).map((node) =>
		node.textContent!.replace(/\s+/g, ' ').trim()
	);
const notice = () => target.querySelector('main [role="alert"]');
const confirmDialog = () =>
	target.querySelector<HTMLDialogElement>('dialog[open][aria-describedby="confirm-message"]');
const yamlForm = () => target.querySelector('main form')!;

const MOVED = 'The web UI has moved to 127.0.0.1:1099';
const UNKNOWN =
	'Saved. The listen address changed. Open the address shown in the tray status item.';
const UNCONFIRMED =
	"Saved, but the web UI is not confirmed running. The Web UI may have stopped. Use the tray's Edit Config to check ~/.jumpway/config.yaml, then Reload Config to restart it.";
const UNCONFIRMED_AT = UNCONFIRMED + ' Not confirmed: 127.0.0.1:1099';

async function render(hash: string) {
	history.replaceState(null, '', '/' + hash);
	target = document.body.appendChild(document.createElement('div'));
	app = mount(App, { target });
	flushSync();
	await settle();
}

function teardown() {
	if (app) unmount(app);
	app = null;
	target?.remove();
}

async function shortcut() {
	document.dispatchEvent(
		new KeyboardEvent('keydown', { key: 's', ctrlKey: true, bubbles: true, cancelable: true })
	);
	await settle();
}

const unloadArmed = () => {
	const event = new Event('beforeunload', { cancelable: true });
	window.dispatchEvent(event);
	return event.defaultPrevented;
};

beforeEach(() => {
	vi.useFakeTimers();
	stubApi();
});

afterEach(() => {
	teardown();
	toasts.clear();
	moved.clear();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
	localStorage.clear();
	document.body.innerHTML = '';
	history.replaceState(null, '', '/');
});

test('#/settings loads both resources; each form tracks and saves its own draft', async () => {
	await render('#/settings');
	expect(target.querySelector('h1')?.textContent).toBe('Global Settings');
	expect(field('web-ui-host').value).toBe('127.0.0.1');
	expect(field('web-ui-port').value).toBe('1088');
	expect(field('no-proxy-list').value).toBe('localhost\n127.0.0.1\n10.0.0.0/8');
	expect(field('no-proxy-env').value).toBe('NO_PROXY\nno_proxy');
	expect(field('no-proxy-files').value).toBe('');
	expect(badge(addressForm())).toBeNull();
	expect(badge(bypassForm())).toBeNull();
	expect(unloadArmed()).toBe(false);

	// The address stays where this page is served from, so saving it must not announce a move.
	type('web-ui-host', '');
	type('web-ui-port', location.port);
	expect(badge(addressForm())).toBe('Unsaved changes');
	expect(badge(bypassForm())).toBeNull();
	type('no-proxy-list', ' example.test \r\n\n10.0.0.0/8\nlocalhost\n');
	type('no-proxy-files', '\n  \n');
	expect(badge(bypassForm())).toBe('Unsaved changes');
	expect(unloadArmed()).toBe(true);

	click(button('Save & Apply', bypassForm()));
	await settle();
	expect(writes()).toEqual([
		{
			method: 'PUT',
			url: '/apis/configs/no-proxy',
			body: {
				list: ['example.test', '10.0.0.0/8', 'localhost'],
				from_env: ['NO_PROXY', 'no_proxy'],
				from_file: []
			}
		}
	]);
	expect(badge(bypassForm())).toBeNull();
	expect(badge(addressForm())).toBe('Unsaved changes');
	expect(field('web-ui-host').value).toBe('');
	expect(field('web-ui-port').value).toBe(location.port);
	// The DOM keeps the raw draft (CRLF normalised); only the request body is cleaned.
	expect(field('no-proxy-list').value).toBe(' example.test \n\n10.0.0.0/8\nlocalhost\n');
	expect(toasts.list.map((toast) => toast.message)).toEqual(['Saved and applied.']);
	expect(requested('GET /apis/configs/status')).toBe(2);

	// Ctrl+S saves the form that has the focus.
	field('web-ui-host').focus();
	await shortcut();
	expect(writes().map((call) => `${call.method} ${call.url}`)).toEqual([
		'PUT /apis/configs/no-proxy',
		'PUT /apis/configs/web-ui'
	]);
	expect(writes()[1].body).toEqual({ host: '', port: Number(location.port) });
	expect(badge(addressForm())).toBeNull();
	expect(alerts()).toEqual([]);
	expect(unloadArmed()).toBe(false);
	expect(requested('GET /apis/configs/web-ui')).toBe(1);
	expect(requested('GET /apis/configs/no-proxy')).toBe(1);

	// Nothing focused and nothing dirty: the shortcut writes nothing.
	(document.activeElement as HTMLElement | null)?.blur();
	await shortcut();
	expect(writes()).toHaveLength(2);
});

test('every save area is the same control: iconed Save & Apply submit, page actions as plain buttons; a write in flight spins without changing the caption', async () => {
	const actions = (root: ParentNode) => {
		const bar = root.querySelector<HTMLElement>('[data-form-actions]');
		if (!bar) throw new Error('missing form actions');
		const primary = bar.querySelector<HTMLButtonElement>('button[type="submit"]')!;
		return {
			sticky: bar.className.includes('sticky'),
			caption: primary.textContent?.trim(),
			icon: primary.querySelector('svg') !== null,
			disabled: primary.disabled,
			busy: primary.getAttribute('aria-busy'),
			spinner: primary.querySelector('[data-spinner]') !== null,
			others: Array.from(
				bar.querySelectorAll<HTMLButtonElement>('button:not([type="submit"])')
			).map((element) => [element.textContent?.trim(), element.type])
		};
	};
	const idle = {
		sticky: false,
		caption: 'Save & Apply',
		icon: true,
		disabled: false,
		busy: null,
		spinner: false,
		others: []
	};
	await render('#/settings');
	expect(actions(addressForm())).toEqual(idle);
	expect(actions(bypassForm())).toEqual(idle);

	// While one form writes, its button shows the spinner in place of the icon and the other
	// form is merely disabled.
	type('web-ui-host', '');
	type('web-ui-port', location.port);
	let release!: () => void;
	delay.set(
		'PUT /apis/configs/web-ui',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(actions(addressForm())).toEqual({
		...idle,
		disabled: true,
		busy: 'true',
		spinner: true
	});
	expect(actions(bypassForm())).toEqual({ ...idle, disabled: true });
	release();
	await settle();
	expect(actions(addressForm())).toEqual(idle);
	expect(actions(bypassForm())).toEqual(idle);

	teardown();
	await render('#/yaml');
	expect(actions(yamlForm())).toEqual({
		...idle,
		sticky: true,
		others: [['Reload from disk', 'button']]
	});
});

test('port 70000 is rejected before any request; an unfocused Ctrl+S saves the only dirty form', async () => {
	await render('#/settings');
	type('web-ui-port', '70000');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(writes()).toEqual([]);
	expect(field('web-ui-port').getAttribute('aria-invalid')).toBe('true');
	expect(document.activeElement).toBe(field('web-ui-port'));
	expect(target.querySelector('#web-ui-port-error')?.textContent).toBe(
		'Port must be a whole number between 0 and 65535.'
	);
	type('web-ui-port', '1088');
	expect(target.querySelector('#web-ui-port-error')).toBeNull();
	expect(badge(addressForm())).toBeNull();

	type('no-proxy-env', 'NO_PROXY');
	(document.activeElement as HTMLElement | null)?.blur();
	expect(document.activeElement).toBe(document.body);
	await shortcut();
	expect(writes().map((call) => `${call.method} ${call.url}`)).toEqual([
		'PUT /apis/configs/no-proxy'
	]);
	expect((writes()[0].body as NoProxy).from_env).toEqual(['NO_PROXY']);
});

test('a changed port shows a persistent moved link keeping ?lang= and the hash; status failures do not replace it', async () => {
	await render('?lang=en#/settings');
	stopOnMove = true;
	type('web-ui-port', '1099');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(writes()).toEqual([
		{ method: 'PUT', url: '/apis/configs/web-ui', body: { host: '127.0.0.1', port: 1099 } }
	]);
	expect(badge(addressForm())).toBeNull();
	expect(toasts.list.map((toast) => toast.message)).toEqual(['Saved and applied.']);
	expect(alerts()).toEqual(['The web UI has moved to 127.0.0.1:1099']);
	const link = notice()!.querySelector('a')!;
	expect(link.getAttribute('href')).toBe('http://127.0.0.1:1099/?lang=en#/settings');
	expect(target.querySelector('main')?.textContent).not.toContain('Cannot reach JumpWay.');

	// The old listener is gone: polls keep failing (the first one backed off to 20 s), the notice
	// stays and no generic banner replaces it.
	await vi.advanceTimersByTimeAsync(20_000);
	expect(requested('GET /apis/configs/status')).toBeGreaterThanOrEqual(3);
	expect(alerts()).toEqual(['The web UI has moved to 127.0.0.1:1099']);
	expect(target.querySelector('[data-state="unknown"]')).not.toBeNull();

	click(target.querySelector('nav a[href="#/yaml"]'));
	await settle();
	expect(location.hash).toBe('#/yaml');
	expect(alerts()).toEqual(['The web UI has moved to 127.0.0.1:1099']);
});

test('port 0 takes the new address from the status reply, or reports it unknown when none arrives', async () => {
	await render('#/settings');
	type('web-ui-port', '0');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(writes()[0].body).toEqual({ host: '127.0.0.1', port: 0 });
	expect(alerts()).toEqual(['The web UI has moved to 127.0.0.1:43210']);
	expect(notice()!.querySelector('a')?.getAttribute('href')).toBe(
		'http://127.0.0.1:43210/#/settings'
	);
	teardown();

	stubApi();
	stopOnMove = true;
	await render('#/settings');
	type('web-ui-port', '0');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(alerts()).toEqual([UNKNOWN]);
	expect(notice()!.querySelector('a')).toBeNull();
	expect(target.querySelector('main')?.textContent).not.toContain('Cannot reach JumpWay.');
});

test('a port whose bind fails is saved but never moved: the candidate stays unconfirmed while the listener is gone or stopped, and a running answer confirms it', async () => {
	await render('#/settings');
	bindFails = true;
	down.add('GET /apis/configs/status');
	type('web-ui-port', '1099');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(writes()[0].body).toEqual({ host: '127.0.0.1', port: 1099 });
	expect(toasts.list.map((toast) => [toast.kind, toast.message])).toEqual([
		['warning', 'saved, but web_ui: listen tcp 127.0.0.1:1099: bind: address already in use']
	]);
	expect(badge(addressForm())).toBeNull();
	// The reload failed and nothing answers: the requested address is a candidate, not a move.
	expect(alerts()).toEqual([UNCONFIRMED_AT]);
	const href = () => notice()!.querySelector('a')?.getAttribute('href');
	expect(href()).toBe('http://127.0.0.1:1099/#/settings');

	// A listener answers here again but reports the web UI stopped: still unconfirmed. The poll
	// after the refused refresh is backed off to 20 s.
	down.clear();
	await vi.advanceTimersByTimeAsync(20_000);
	expect(status.data).toMatchObject({ address: '127.0.0.1:1099', running: false });
	expect(alerts()).toEqual([UNCONFIRMED_AT]);
	expect(href()).toBe('http://127.0.0.1:1099/#/settings');

	// Fixed from the tray: the web UI runs at the requested address.
	statusRunning = true;
	await vi.advanceTimersByTimeAsync(10_000);
	expect(alerts()).toEqual([MOVED]);
	expect(href()).toBe('http://127.0.0.1:1099/#/settings');
});

test('a status answer reporting the web UI stopped demotes a moved claim to unconfirmed; a running one restores it', async () => {
	await render('#/settings');
	stopOnMove = true;
	type('web-ui-port', '1099');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(alerts()).toEqual([MOVED]);

	down.clear();
	statusRunning = false;
	// The poll after the refused refresh is backed off to 20 s.
	await vi.advanceTimersByTimeAsync(20_000);
	expect(alerts()).toEqual([UNCONFIRMED_AT]);
	expect(notice()!.querySelector('a')?.getAttribute('href')).toBe(
		'http://127.0.0.1:1099/#/settings'
	);

	statusRunning = true;
	await vi.advanceTimersByTimeAsync(10_000);
	expect(alerts()).toEqual([MOVED]);
});

test('port 0 whose bind fails is saved but unconfirmed, never linked as :0; the notice clears once a listener answers here again', async () => {
	await render('#/settings');
	bindFails = true;
	type('web-ui-port', '0');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(writes()[0].body).toEqual({ host: '127.0.0.1', port: 0 });
	expect(toasts.list.map((toast) => [toast.kind, toast.message])).toEqual([
		['warning', 'saved, but web_ui: listen tcp 127.0.0.1:0: bind: address already in use']
	]);
	expect(badge(addressForm())).toBeNull();
	expect(alerts()).toEqual([UNCONFIRMED]);
	expect(notice()!.querySelector('a')).toBeNull();
	expect(target.querySelector('main a[href*=":0/"]')).toBeNull();

	// The polls keep answering the unbound address; nothing to link to yet.
	await vi.advanceTimersByTimeAsync(10_000);
	expect(alerts()).toEqual([UNCONFIRMED]);
	expect(target.querySelector('main a[href*=":0/"]')).toBeNull();

	// Fixed from the tray: the listener is back on this origin.
	statusAddress = location.host;
	statusRunning = true;
	await vi.advanceTimersByTimeAsync(10_000);
	expect(alerts()).toEqual([]);
});

test('a host the URL parser would repair into another site is never linked, whatever the backend does with it', async () => {
	await render('#/settings');
	type('web-ui-host', '//other.example');
	type('web-ui-port', '1099');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(writes()[0].body).toEqual({ host: '//other.example', port: 1099 });
	expect(alerts()).toEqual([UNKNOWN]);
	expect(target.querySelector('main a[href*="other.example"]')).toBeNull();
	expect(notice()!.querySelector('a')).toBeNull();
});

test('no-proxy: an ordinary 400 keeps the draft dirty; "saved, but" marks it clean with a warning', async () => {
	await render('#/settings');
	fail.set('PUT /apis/configs/no-proxy', 'no_proxy.list[0] "bad host" is not a host or CIDR');
	type('no-proxy-list', 'bad host');
	click(button('Save & Apply', bypassForm()));
	await settle();
	expect(requested('PUT /apis/configs/no-proxy')).toBe(1);
	expect(badge(bypassForm())).toBe('Unsaved changes');
	expect(bypassForm().querySelector('[role="alert"]')?.textContent).toContain(
		'"bad host" is not a host or CIDR'
	);
	expect(field('no-proxy-list').value).toBe('bad host');
	expect(toasts.list).toEqual([]);

	fail.set('PUT /apis/configs/no-proxy', SAVED_PREFIX + 'reload failed: dns lookup timed out');
	click(button('Save & Apply', bypassForm()));
	await settle();
	expect(requested('PUT /apis/configs/no-proxy')).toBe(2);
	expect(badge(bypassForm())).toBeNull();
	expect(bypassForm().querySelector('[role="alert"]')).toBeNull();
	expect(toasts.list.map((toast) => [toast.kind, toast.message])).toEqual([
		['warning', 'saved, but reload failed: dns lookup timed out']
	]);
	expect(noProxy.list).toEqual(['bad host']);
});

test('a network failure never claims saved: the address draft stays dirty and a candidate link says where the page may be', async () => {
	await render('#/settings');
	down.add('PUT /apis/configs/web-ui');
	type('web-ui-port', '1099');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(requested('PUT /apis/configs/web-ui')).toBe(1);
	expect(toasts.list).toEqual([]);
	expect(badge(addressForm())).toBe('Unsaved changes');
	expect(field('web-ui-port').value).toBe('1099');
	expect(alerts()).toEqual([
		'The request did not complete. The change may or may not have been applied. If it was applied, the web UI is now at 127.0.0.1:1099'
	]);
	expect(notice()!.querySelector('a')?.getAttribute('href')).toBe(
		'http://127.0.0.1:1099/#/settings'
	);

	// The old listener answers the next poll, so the write did not move anything.
	await vi.advanceTimersByTimeAsync(10_000);
	expect(alerts()).toEqual([]);
	expect(badge(addressForm())).toBe('Unsaved changes');

	down.clear();
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(requested('PUT /apis/configs/web-ui')).toBe(2);
	expect(badge(addressForm())).toBeNull();
	expect(alerts()).toEqual(['The web UI has moved to 127.0.0.1:1099']);
});

test('a status answer requested before the write cannot dismiss the candidate link, even when it arrives after the lost answer', async () => {
	let release!: () => void;
	delay.set('GET /apis/configs/status', new Promise<void>((resolve) => (release = resolve)));
	await render('#/settings');
	expect(requested('GET /apis/configs/status')).toBe(1);
	expect(status.loading).toBe(true);

	// The write lands and closes the old listener, but its answer is lost on the way back.
	stopOnMove = true;
	lost.add('PUT /apis/configs/web-ui');
	type('web-ui-port', '1099');
	click(button('Save & Apply', addressForm()));
	await settle();
	expect(webUI).toEqual({ host: '127.0.0.1', port: 1099 });
	expect(badge(addressForm())).toBe('Unsaved changes');
	const maybe =
		'The request did not complete. The change may or may not have been applied. If it was applied, the web UI is now at 127.0.0.1:1099';
	expect(alerts()).toEqual([maybe]);
	const href = () => notice()!.querySelector('a')?.getAttribute('href');
	expect(href()).toBe('http://127.0.0.1:1099/#/settings');

	// The poll that started before the save answers now: it only describes the old listener.
	release();
	await settle();
	expect(status.loading).toBe(false);
	expect(status.data?.address).toBe(location.host);
	expect(alerts()).toEqual([maybe]);
	expect(href()).toBe('http://127.0.0.1:1099/#/settings');

	// Later polls fail: the old listener is gone, and the link is the only way to the new one.
	await vi.advanceTimersByTimeAsync(10_000);
	expect(requested('GET /apis/configs/status')).toBe(2);
	expect(alerts()[0]).toBe(maybe);
	expect(href()).toBe('http://127.0.0.1:1099/#/settings');
});

test('a no-proxy write without an answer keeps the draft and says so in the form', async () => {
	await render('#/settings');
	down.add('PUT /apis/configs/no-proxy');
	type('no-proxy-files', '/etc/no_proxy.txt');
	click(button('Save & Apply', bypassForm()));
	await settle();
	expect(toasts.list).toEqual([]);
	expect(badge(bypassForm())).toBe('Unsaved changes');
	expect(bypassForm().querySelector('[role="alert"]')?.textContent).toContain(
		'The request did not complete. The change may or may not have been applied.'
	);
	expect(field('no-proxy-files').value).toBe('/etc/no_proxy.txt');
});

test('dirty settings ask before in-app navigation and keep the draft when cancelled', async () => {
	await render('#/settings');
	type('web-ui-host', '0.0.0.0');
	click(target.querySelector('nav a[href="#/yaml"]'));
	await settle();
	const question = confirmDialog()!;
	expect(question.textContent).toContain('Discard unsaved changes?');
	click(question.querySelectorAll('button')[0]);
	await settle();
	expect(location.hash).toBe('#/settings');
	expect(field('web-ui-host').value).toBe('0.0.0.0');
	expect(writes()).toEqual([]);
});

test('#/yaml shows the file verbatim; Ctrl+S PUTs the text, adopts the re-read and clears the badge', async () => {
	await render('#/yaml');
	expect(target.querySelector('h1')?.textContent).toBe('Configuration File');
	const source = field('yaml-source') as HTMLTextAreaElement;
	expect(source.value).toBe(rawFixture.yaml);
	expect(source.getAttribute('spellcheck')).toBe('false');
	expect(badge(yamlForm())).toBeNull();
	expect(unloadArmed()).toBe(false);

	const edited = rawFixture.yaml + '# trailing note\n';
	rawAfterSave = edited.replace('port:   18097', 'port: 18097');
	type('yaml-source', edited);
	expect(badge(yamlForm())).toBe('Unsaved changes');
	expect(unloadArmed()).toBe(true);
	source.focus();
	await shortcut();
	expect(writes()).toEqual([{ method: 'PUT', url: '/apis/configs/raw', body: { yaml: edited } }]);
	expect(requested('GET /apis/configs/raw')).toBe(2);
	expect(source.value).toBe(rawAfterSave);
	expect(badge(yamlForm())).toBeNull();
	expect(toasts.list.map((toast) => toast.message)).toEqual(['Saved and applied.']);
	expect(requested('GET /apis/configs/status')).toBe(2);
	expect(alerts()).toEqual([]);
});

test('YAML reload asks only when dirty; cancel keeps the draft without a request; confirm restores the file', async () => {
	await render('#/yaml');
	click(button('Reload from disk'));
	await settle();
	expect(confirmDialog()).toBeNull();
	expect(requested('GET /apis/configs/raw')).toBe(2);
	expect(toasts.list.map((toast) => toast.message)).toEqual(['Reloaded from disk.']);

	type('yaml-source', 'rules: []\n');
	click(button('Reload from disk'));
	await settle();
	const question = confirmDialog()!;
	expect(question.textContent).toContain('Reload from disk and discard unsaved changes?');
	click(question.querySelectorAll('button')[0]);
	await settle();
	expect(requested('GET /apis/configs/raw')).toBe(2);
	expect(field('yaml-source').value).toBe('rules: []\n');
	expect(badge(yamlForm())).toBe('Unsaved changes');

	click(button('Reload from disk'));
	await settle();
	click(confirmDialog()!.querySelectorAll('button')[1]);
	await settle();
	expect(requested('GET /apis/configs/raw')).toBe(3);
	expect(field('yaml-source').value).toBe(rawFixture.yaml);
	expect(badge(yamlForm())).toBeNull();
	expect(toasts.list.map((toast) => toast.message)).toEqual([
		'Reloaded from disk.',
		'Reloaded from disk.'
	]);
	expect(writes()).toEqual([]);
});

test('YAML: a 400 keeps the text dirty; "saved, but" cleans it with a warning; a failed re-read keeps the saved text clean', async () => {
	await render('#/yaml');
	fail.set('PUT /apis/configs/raw', 'yaml: line 3: mapping values are not allowed in this context');
	type('yaml-source', 'web_ui: [\n');
	click(button('Save & Apply'));
	await settle();
	expect(requested('PUT /apis/configs/raw')).toBe(1);
	expect(badge(yamlForm())).toBe('Unsaved changes');
	expect(alerts()[0]).toContain('yaml: line 3: mapping values are not allowed in this context');
	expect(field('yaml-source').value).toBe('web_ui: [\n');
	expect(toasts.list).toEqual([]);

	fail.set('PUT /apis/configs/raw', SAVED_PREFIX + 'reload failed: bind: address already in use');
	fail.set('GET /apis/configs/raw', 'store is locked');
	type('yaml-source', 'rules: []\n');
	click(button('Save & Apply'));
	await settle();
	expect(requested('PUT /apis/configs/raw')).toBe(2);
	expect(requested('GET /apis/configs/raw')).toBe(2);
	expect(badge(yamlForm())).toBeNull();
	expect(field('yaml-source').value).toBe('rules: []\n');
	expect(toasts.list.map((toast) => [toast.kind, toast.message])).toEqual([
		['warning', 'saved, but reload failed: bind: address already in use']
	]);
	expect(alerts()[0]).toContain('store is locked');
	expect(raw.yaml).toBe('rules: []\n');
});

test('a YAML write without an answer stays dirty and is not reported as saved', async () => {
	await render('#/yaml');
	down.add('PUT /apis/configs/raw');
	type('yaml-source', 'rules: []\n');
	click(button('Save & Apply'));
	await settle();
	expect(toasts.list).toEqual([]);
	expect(badge(yamlForm())).toBe('Unsaved changes');
	expect(alerts()[0]).toContain(
		'The request did not complete. The change may or may not have been applied.'
	);
	expect(field('yaml-source').value).toBe('rules: []\n');
});

test('a YAML save that moves web_ui links to the address the status reports and skips the re-read', async () => {
	await render('#/yaml');
	type('yaml-source', 'web_ui:\n  port: 1099\n');
	statusAddress = '0.0.0.0:1099';
	click(button('Save & Apply'));
	await settle();
	expect(requested('PUT /apis/configs/raw')).toBe(1);
	expect(requested('GET /apis/configs/raw')).toBe(1);
	expect(badge(yamlForm())).toBeNull();
	expect(alerts()).toEqual(['The web UI has moved to 0.0.0.0:1099']);
	expect(notice()!.querySelector('a')?.getAttribute('href')).toBe(
		`http://${location.hostname}:1099/#/yaml`
	);
});

const loadingAnnounced = (root: ParentNode) =>
	Array.from(root.querySelectorAll('.sr-only')).some(
		(element) => element.textContent?.trim() === 'Loading...'
	);
const skeletons = () => target.querySelectorAll('main [data-skeleton]');
const busyWrapper = () => target.querySelector<HTMLElement>('main [aria-busy="true"]');

test('the settings show field skeletons behind a busy wrapper until both reads answer', async () => {
	let release!: () => void;
	delay.set(
		'GET /apis/configs/no-proxy',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	await render('#/settings');
	expect(target.querySelector('main form')).toBeNull();
	const wrapper = busyWrapper();
	expect(wrapper).not.toBeNull();
	expect(loadingAnnounced(wrapper!)).toBe(true);
	const bars = Array.from(wrapper!.querySelectorAll('[data-skeleton]'));
	expect(bars.length).toBeGreaterThanOrEqual(4);
	expect(bars.every((bar) => bar.getAttribute('aria-hidden') === 'true')).toBe(true);
	expect(wrapper!.querySelectorAll('.band')).toHaveLength(2);
	release();
	await settle();
	expect(skeletons()).toHaveLength(0);
	expect(busyWrapper()).toBeNull();
	expect(loadingAnnounced(target)).toBe(false);
	expect(field('web-ui-host').value).toBe(webUIFixture.host);
	expect(target.querySelectorAll('main form')).toHaveLength(2);
});

test('the YAML page shows a label, a tall block and a button-sized block until the file answers', async () => {
	let release!: () => void;
	delay.set(
		'GET /apis/configs/raw',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	await render('#/yaml');
	expect(target.querySelector('main form')).toBeNull();
	const wrapper = busyWrapper();
	expect(wrapper).not.toBeNull();
	expect(loadingAnnounced(wrapper!)).toBe(true);
	const bars = Array.from(wrapper!.querySelectorAll<HTMLElement>('[data-skeleton]'));
	expect(bars).toHaveLength(3);
	expect(bars.map((bar) => bar.classList.contains('w-full'))).toEqual([false, true, false]);
	expect(bars[1].className).toContain('min-h-[60vh]');
	release();
	await settle();
	expect(skeletons()).toHaveLength(0);
	expect(busyWrapper()).toBeNull();
	expect(field('yaml-source').value).toBe(rawFixture.yaml);
});
