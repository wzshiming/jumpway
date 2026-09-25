import { flushSync, mount, unmount } from 'svelte';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
	protocolRulesFixture,
	rulesFixture,
	snapshotFixture,
	statusFixture,
	virtualRulesFixture,
	webUIFixture
} from '../../e2e/fixtures/api';
import App from '../App.svelte';
import { SAVED_PREFIX } from '../lib/api';
import { toasts } from '../lib/toast.svelte';
import type { Address, Protocol, Rule } from '../lib/types';
import { fieldsOf, layoutFor } from '../lib/urlBuilder';

// The whole app mounted in jsdom against a method-aware in-memory /apis stub.

// What a legacy proxy rule serves; the editor always spells it out.
const LEGACY: Protocol[] = [
	{ type: 'http' },
	{ type: 'socks5' },
	{ type: 'socks4' },
	{ type: 'ssh' }
];

interface Call {
	method: string;
	url: string;
	body: unknown;
}

let target: HTMLElement;
let app: ReturnType<typeof mount> | null = null;
let rules: Rule[];
// The configured web UI address; the status fixture reports the bound one.
let webUI: Address;
let calls: Call[];
// `${method} ${url}` → 400 text. A "saved, but " text still applies the mutation first.
let fail: Map<string, string>;
// `${method} ${url}` whose next answer waits for the promise; its body is fixed when the request arrives.
let delay: Map<string, Promise<void>>;

const json = (body: unknown) =>
	new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});
const text = (status: number, body: string) => new Response(body, { status });

function stubApi() {
	rules = structuredClone(rulesFixture);
	webUI = structuredClone(webUIFixture);
	calls = [];
	fail = new Map();
	delay = new Map();
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string, init?: RequestInit) => {
			const method = init?.method ?? 'GET';
			const body: unknown = init?.body ? JSON.parse(String(init.body)) : undefined;
			calls.push({ method, url, body });
			const held = delay.get(`${method} ${url}`);
			delay.delete(`${method} ${url}`);
			if (held) await held;
			const injected = fail.get(`${method} ${url}`);
			if (injected !== undefined && !injected.startsWith(SAVED_PREFIX)) return text(400, injected);
			const answer = () => (injected === undefined ? json(null) : text(400, injected));
			if (url === '/apis/configs/status') return json(statusFixture);
			if (url === '/apis/configs/web-ui') return json(webUI);
			if (url === '/apis/stats') return json(snapshotFixture);
			const rule = body as Rule;
			if (url === '/apis/configs/rules') {
				if (method === 'GET') return json(rules);
				if (rules.some((entry) => entry.name === rule.name))
					return text(400, `rule ${JSON.stringify(rule.name)} already exists`);
				rules.push(rule);
				return answer();
			}
			const match = /^\/apis\/configs\/rules\/([^/]+)$/.exec(url);
			if (!match) return text(404, 'not found');
			const name = decodeURIComponent(match[1]);
			const index = rules.findIndex((entry) => entry.name === name);
			if (index < 0) return text(400, `rule ${JSON.stringify(name)} not found`);
			if (method === 'GET') return json(rules[index]);
			if (method === 'PUT') rules[index] = rule;
			else rules.splice(index, 1);
			return answer();
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
	const element = target.querySelector<HTMLInputElement>('#' + id);
	if (!element) throw new Error('missing field ' + id);
	return element;
};

const type = (id: string, value: string) => {
	const element = field(id);
	element.value = value;
	element.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
};

const choose = (element: HTMLInputElement) => {
	element.checked = true;
	element.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
};

const button = (name: string, root: ParentNode = target) =>
	Array.from(root.querySelectorAll('button')).find(
		(element) => element.textContent?.trim() === name || element.getAttribute('aria-label') === name
	);

const form = () => target.querySelector<HTMLFormElement>('main form');
const confirmDialog = () =>
	target.querySelector<HTMLDialogElement>('dialog[open][aria-describedby="confirm-message"]');
const badge = () => target.querySelector('main [role="status"]')?.textContent?.trim() ?? null;
const inputs = () =>
	Object.fromEntries(
		Array.from(form()!.querySelectorAll<HTMLInputElement>('input[id]')).map((element) => [
			element.id,
			element.type === 'checkbox' || element.type === 'radio' ? element.checked : element.value
		])
	);
const chain = () =>
	Array.from(target.querySelectorAll('main ol[aria-label="Chain"] > li')).map((stage) =>
		Array.from(stage.querySelectorAll('p, li'))
			.map((node) => node.textContent!.replace(/\s+/g, ' ').trim())
			.join(' ')
	);

async function render(hash: string) {
	history.replaceState(null, '', '/' + hash);
	target = document.body.appendChild(document.createElement('div'));
	app = mount(App, { target });
	flushSync();
	await settle();
}

async function save() {
	document.dispatchEvent(
		new KeyboardEvent('keydown', { key: 's', ctrlKey: true, bubbles: true, cancelable: true })
	);
	await settle();
}

beforeEach(() => {
	vi.useFakeTimers();
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

test('the overview lists every rule with its management facts: state, mode, listen, target and chain size; disabled ones too', async () => {
	await render('#/');
	expect(target.querySelector('h1')?.textContent).toBe('Overview');
	const cards = Array.from(target.querySelectorAll('main article'));
	expect(cards.map((card) => card.querySelector('header a')?.textContent?.trim())).toEqual([
		'office',
		'mirror',
		'db-tunnel',
		'lab'
	]);
	const facts = (card: Element) => ({
		state: card.querySelector('[data-state]')?.textContent?.trim(),
		mode: card.querySelector('header p')?.textContent?.replace(/\s+/g, ' ').trim(),
		dl: Array.from(card.querySelectorAll('dl dd')).map((dd) => dd.textContent!.trim())
	});
	expect(facts(cards[0])).toEqual({
		state: 'Running',
		mode: 'Proxy',
		dl: ['127.0.0.1:18097', '\u2014', '2 hops']
	});
	expect(facts(cards[2])).toEqual({
		state: 'Retrying (3)',
		mode: 'Port forward \u00b7 remote',
		dl: ['0.0.0.0:18099', '127.0.0.1:5432', 'direct']
	});
	expect(facts(cards[3])).toEqual({
		state: 'Disabled',
		mode: 'Proxy',
		dl: ['127.0.0.1:18100', '\u2014', 'direct']
	});
	expect(cards[0].querySelector('a[href="#/rules/office"]')).not.toBeNull();
	expect(cards[0].querySelector('a[href="#/stats?rule=office"]')).not.toBeNull();
	expect(
		target.querySelector('main section[aria-label="Rules"] a[href="#/new"]')?.textContent?.trim()
	).toBe('New rule');
});

test('an unknown rule name renders its empty state with a way back to the overview', async () => {
	await render('#/rules/ghost');
	const banner = target.querySelector('main [role="alert"]')!;
	expect(banner.textContent).toContain('Rule "ghost" was not found.');
	expect(banner.querySelector('a[href="#/"]')?.textContent?.trim()).toBe('Overview');
	expect(form()).toBeNull();
});

test('editing a loaded rule marks it dirty; Ctrl+S sends the serialized rule and clears the badge', async () => {
	await render('#/rules/office');
	expect(target.querySelector('h1')?.textContent).toBe('office');
	expect(inputs()).toMatchObject({
		'rule-name': 'office',
		'listen-host': '127.0.0.1',
		'listen-port': '18097',
		'listen-username': '',
		'listen-password': ''
	});
	expect(chain()).toEqual([
		'Clients',
		'This machine 127.0.0.1:18097',
		'Hop 2 \u00b7 entry node \u00b7 dialed from this machine ssh://bastion.example:22 ssh://bastion-2.example:22',
		'Hop 1 \u00b7 exit node socks5://hop-a.example:1080',
		'Target target'
	]);
	expect(badge()).toBeNull();
	expect(button('Delete')).not.toBeUndefined();

	type('listen-port', '18197');
	expect(badge()).toBe('Unsaved changes');
	type('listen-port', '18097');
	expect(badge()).toBeNull();
	type('listen-username', 'demo');
	type('listen-password', 'placeholder');
	expect(badge()).toBe('Unsaved changes');

	await save();
	expect(writes()).toEqual([
		{
			method: 'PUT',
			url: '/apis/configs/rules/office',
			body: {
				name: 'office',
				listen: {
					host: '127.0.0.1',
					port: 18097,
					username: 'demo',
					password: 'placeholder',
					protocols: LEGACY
				},
				forward: {
					way: [
						{ lb: ['socks5://demo:placeholder@hop-a.example:1080'] },
						{ lb: ['ssh://ops@bastion.example:22', 'ssh://ops@bastion-2.example:22'] }
					]
				}
			}
		}
	]);
	expect(badge()).toBeNull();
	expect(location.hash).toBe('#/rules/office');
	expect(inputs()).toMatchObject({ 'listen-username': 'demo', 'listen-password': 'placeholder' });
	expect(toasts.list.map((toast) => toast.message)).toEqual(['Saved and applied.']);
	expect(requested('GET /apis/configs/status')).toBe(2);
});

test('the editor footer is the shared save area: an iconed Save & Apply submit, Cancel, Duplicate and Delete as plain buttons, one dirty badge', async () => {
	await render('#/rules/office');
	const bar = target.querySelector<HTMLElement>('main form [data-form-actions]');
	if (!bar) throw new Error('missing form actions');
	expect(bar.className).toContain('sticky');
	const primary = bar.querySelector<HTMLButtonElement>('button[type="submit"]')!;
	expect(primary.textContent?.trim()).toBe('Save & Apply');
	expect(primary.className).toContain('btn-primary');
	expect(primary.querySelector('svg')).not.toBeNull();
	expect(
		Array.from(bar.querySelectorAll<HTMLButtonElement>('button:not([type="submit"])')).map(
			(element) => [element.textContent?.trim(), element.type, element.className]
		)
	).toEqual([
		['Cancel', 'button', 'btn btn-secondary'],
		['Duplicate rule', 'button', 'btn btn-secondary'],
		['Delete', 'button', 'btn btn-danger']
	]);
	expect(bar.querySelector('[role="status"]')).toBeNull();
	type('listen-port', '18197');
	expect(bar.querySelector('[role="status"]')?.textContent?.trim()).toBe('Unsaved changes');
	expect(target.querySelectorAll('main [role="status"]')).toHaveLength(1);
});

test('Cancel on a clean editor returns to the overview at once; #/new keeps Cancel too', async () => {
	await render('#/rules/office');
	click(button('Cancel', form()!));
	await settle();
	expect(confirmDialog()).toBeNull();
	expect(location.hash).toBe('#/');
	expect(target.querySelector('h1')?.textContent).toBe('Overview');
	expect(writes()).toEqual([]);

	click(target.querySelector('main section[aria-label="Rules"] a[href="#/new"]'));
	await settle();
	expect(target.querySelector('h1')?.textContent).toBe('New rule');
	expect(button('Cancel', form()!)?.type).toBe('button');
	expect(button('Delete')).toBeUndefined();
});

test('Cancel on a dirty draft asks: declining keeps every input, discarding returns home without a request', async () => {
	await render('#/new');
	type('rule-name', 'draft');
	type('listen-port', '18104');
	click(button('Cancel', form()!));
	await settle();
	const question = confirmDialog()!;
	expect(question.textContent).toContain('Discard unsaved changes?');
	click(question.querySelectorAll('button')[0]);
	await settle();
	expect(confirmDialog()).toBeNull();
	expect(location.hash).toBe('#/new');
	expect(inputs()).toMatchObject({ 'rule-name': 'draft', 'listen-port': '18104' });
	expect(badge()).toBe('Unsaved changes');

	click(button('Cancel', form()!));
	await settle();
	click(confirmDialog()!.querySelectorAll('button')[1]);
	await settle();
	expect(location.hash).toBe('#/');
	expect(target.querySelector('h1')?.textContent).toBe('Overview');
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	expect(writes()).toEqual([]);
	expect(toasts.list).toEqual([]);
});

test('after a rejected save the form is still dirty, so Cancel asks before leaving and sends nothing more', async () => {
	await render('#/rules/mirror');
	fail.set(
		'PUT /apis/configs/rules/mirror',
		'reload failed: listen tcp 127.0.0.1:18197: bind: address already in use'
	);
	type('listen-port', '18197');
	await save();
	expect(target.querySelector('main [role="alert"]')?.textContent).toContain('already in use');
	click(button('Cancel', form()!));
	await settle();
	expect(confirmDialog()?.textContent).toContain('Discard unsaved changes?');
	click(confirmDialog()!.querySelectorAll('button')[1]);
	await settle();
	expect(location.hash).toBe('#/');
	expect(writes().map((call) => `${call.method} ${call.url}`)).toEqual([
		'PUT /apis/configs/rules/mirror'
	]);
	expect(rules.find((rule) => rule.name === 'mirror')?.listen.port).toBe(18098);
});

test('#/new validates before any request and focuses the first invalid field', async () => {
	await render('#/new');
	expect(target.querySelector('h1')?.textContent).toBe('New rule');
	expect(button('Delete')).toBeUndefined();
	expect(inputs()).toMatchObject({
		'rule-name': '',
		'listen-host': '127.0.0.1',
		'listen-port': '0'
	});

	type('listen-username', 'keep-me');
	type('listen-port', '70000');
	choose(target.querySelector<HTMLInputElement>('input[name="mode"][value="forward"]')!);
	expect(target.querySelector('#listen-username')).toBeNull();
	expect(target.querySelector('main')?.textContent).toContain(
		'Credentials apply to proxy rules only'
	);
	expect(target.querySelector('#target-port')).not.toBeNull();
	await save();
	expect(writes()).toEqual([]);
	expect(field('rule-name').getAttribute('aria-invalid')).toBe('true');
	expect(document.activeElement).toBe(field('rule-name'));
	expect(target.querySelector('#rule-name-error')?.textContent).toBe('Rule name is required.');
	expect(target.querySelector('#listen-port-error')?.textContent).toBe(
		'Port must be a whole number between 0 and 65535.'
	);
	expect(target.querySelector('#target-port-error')?.textContent).toBe(
		'Port must be a whole number between 1 and 65535.'
	);

	type('rule-name', ' tunnel ');
	expect(target.querySelector('#rule-name-error')).toBeNull();
	expect(field('rule-name').hasAttribute('aria-invalid')).toBe(false);
	// The credentials draft survives a round trip through forward mode.
	choose(target.querySelector<HTMLInputElement>('input[name="mode"][value="proxy"]')!);
	expect(field('listen-username').value).toBe('keep-me');
});

test('#/new POSTs a forward rule once valid and the page moves to the saved route', async () => {
	await render('#/new');
	type('rule-name', ' tunnel ');
	type('listen-port', '18101');
	choose(target.querySelector<HTMLInputElement>('input[name="mode"][value="forward"]')!);
	type('target-host', '10.0.0.9');
	type('target-port', '5432');
	const enabled = form()!.querySelector<HTMLInputElement>('input[type="checkbox"]')!;
	enabled.checked = false;
	enabled.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
	expect(target.querySelector('main [data-chain-summary]')?.textContent?.trim()).toBe(
		'this machine \u2192 10.0.0.9:5432'
	);

	click(button('Save & Apply'));
	await settle();
	expect(writes()).toEqual([
		{
			method: 'POST',
			url: '/apis/configs/rules',
			body: {
				name: 'tunnel',
				disabled: true,
				listen: { host: '127.0.0.1', port: 18101 },
				forward: { host: '10.0.0.9', port: 5432 }
			}
		}
	]);
	expect(location.hash).toBe('#/rules/tunnel');
	expect(requested('GET /apis/configs/rules/tunnel')).toBe(1);
	expect(target.querySelector('h1')?.textContent).toBe('tunnel');
	expect(inputs()).toMatchObject({ 'rule-name': 'tunnel', 'target-port': '5432' });
	expect(badge()).toBeNull();
});

test('a rename PUTs to the old name and moves to the new route; a 400 keeps the form dirty', async () => {
	await render('#/rules/mirror');
	fail.set(
		'PUT /apis/configs/rules/mirror',
		'reload failed: listen tcp 127.0.0.1:18197: bind: address already in use'
	);
	type('rule-name', 'mirror-2');
	type('listen-port', '18197');
	await save();
	expect(location.hash).toBe('#/rules/mirror');
	expect(badge()).toBe('Unsaved changes');
	const banner = target.querySelector('main [role="alert"]')!;
	expect(banner.textContent).toContain('address already in use');
	expect(inputs()).toMatchObject({ 'rule-name': 'mirror-2', 'listen-port': '18197' });

	fail.clear();
	type('listen-port', '18098');
	await save();
	expect(writes().map((call) => `${call.method} ${call.url}`)).toEqual([
		'PUT /apis/configs/rules/mirror',
		'PUT /apis/configs/rules/mirror'
	]);
	expect((writes()[1].body as Rule).name).toBe('mirror-2');
	expect(location.hash).toBe('#/rules/mirror-2');
	expect(target.querySelector('h1')?.textContent).toBe('mirror-2');
	expect(target.querySelector('main [role="alert"]')).toBeNull();
});

test('"saved, but" is persisted: the form is clean, a warning stays, and the retry is a PUT', async () => {
	await render('#/new');
	fail.set(
		'POST /apis/configs/rules',
		SAVED_PREFIX + 'reload failed: listen tcp 127.0.0.1:18102: bind: address already in use'
	);
	type('rule-name', 'clash');
	type('listen-port', '18102');
	await save();
	expect(requested('POST /apis/configs/rules')).toBe(1);
	expect(toasts.list.map((toast) => [toast.kind, toast.message])).toEqual([
		[
			'warning',
			'saved, but reload failed: listen tcp 127.0.0.1:18102: bind: address already in use'
		]
	]);
	expect(location.hash).toBe('#/rules/clash');
	expect(badge()).toBeNull();
	// Still there after the page reloaded the saved rule.
	expect(toasts.list).toHaveLength(1);

	type('listen-port', '18103');
	await save();
	expect(requested('POST /apis/configs/rules')).toBe(1);
	expect(requested('PUT /apis/configs/rules/clash')).toBe(1);
	expect(rules.find((rule) => rule.name === 'clash')?.listen.port).toBe(18103);
});

test('a dirty editor asks before in-app navigation and arms beforeunload only while dirty', async () => {
	await render('#/rules/lab');
	const unload = () => {
		const event = new Event('beforeunload', { cancelable: true });
		window.dispatchEvent(event);
		return event.defaultPrevented;
	};
	expect(unload()).toBe(false);
	type('rule-name', 'lab-2');
	expect(unload()).toBe(true);

	click(target.querySelector('nav a[href="#/hosts"]'));
	await settle();
	const question = confirmDialog()!;
	expect(question.textContent).toContain('Discard unsaved changes?');
	click(question.querySelectorAll('button')[0]);
	await settle();
	expect(location.hash).toBe('#/rules/lab');
	expect(inputs()).toMatchObject({ 'rule-name': 'lab-2' });

	click(target.querySelector('nav a[href="#/hosts"]'));
	await settle();
	click(confirmDialog()!.querySelectorAll('button')[1]);
	await settle();
	expect(location.hash).toBe('#/hosts');
	expect(unload()).toBe(false);
	expect(writes()).toEqual([]);
});

test('deleting from the editor asks once, marks the form clean and returns to the overview', async () => {
	await render('#/rules/lab');
	type('rule-name', 'lab-2');
	click(button('Delete'));
	await settle();
	const question = confirmDialog()!;
	expect(question.textContent).toContain('Delete rule "lab"?');
	click(button('Delete', question));
	await settle();
	expect(writes()).toEqual([{ method: 'DELETE', url: '/apis/configs/rules/lab', body: undefined }]);
	expect(confirmDialog()).toBeNull();
	expect(location.hash).toBe('#/');
	expect(target.querySelector('h1')?.textContent).toBe('Overview');
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel']);
	expect(toasts.list.map((toast) => toast.message)).toEqual(['Rule deleted.']);
});

const RELOAD_FAILED =
	SAVED_PREFIX + 'reload failed: listen tcp 127.0.0.1:18097: bind: address already in use';
const ruleNames = () =>
	Array.from(target.querySelectorAll('main article header a')).map((link) =>
		link.textContent?.trim()
	);

test('editor delete: an ordinary 400 keeps the dirty form; "saved, but" cleans it and returns to the overview', async () => {
	await render('#/rules/lab');
	const unload = () => {
		const event = new Event('beforeunload', { cancelable: true });
		window.dispatchEvent(event);
		return event.defaultPrevented;
	};
	type('rule-name', 'lab-2');
	fail.set('DELETE /apis/configs/rules/lab', 'rule "lab" is in use');
	click(button('Delete'));
	await settle();
	click(button('Delete', confirmDialog()!));
	await settle();
	expect(location.hash).toBe('#/rules/lab');
	expect(badge()).toBe('Unsaved changes');
	expect(target.querySelector('main [role="alert"]')?.textContent).toContain(
		'rule "lab" is in use'
	);
	expect(inputs()).toMatchObject({ 'rule-name': 'lab-2' });
	expect(unload()).toBe(true);
	expect(toasts.list).toEqual([]);
	expect(requested('GET /apis/configs/status')).toBe(1);

	fail.set('DELETE /apis/configs/rules/lab', RELOAD_FAILED);
	click(button('Delete'));
	await settle();
	click(button('Delete', confirmDialog()!));
	await settle();
	expect(writes().map((call) => `${call.method} ${call.url}`)).toEqual([
		'DELETE /apis/configs/rules/lab',
		'DELETE /apis/configs/rules/lab'
	]);
	expect(confirmDialog()).toBeNull();
	expect(location.hash).toBe('#/');
	expect(target.querySelector('h1')?.textContent).toBe('Overview');
	expect(ruleNames()).toEqual(['office', 'mirror', 'db-tunnel']);
	expect(toasts.list.map((toast) => [toast.kind, toast.message])).toEqual([
		['warning', RELOAD_FAILED]
	]);
	expect(unload()).toBe(false);
	expect(requested('GET /apis/configs/status')).toBe(2);
});

test('hops and URLs can be added, reordered and removed while the chain follows; keyed rows keep focus', async () => {
	await render('#/rules/db-tunnel');
	const exit = () => target.querySelector('#exit-hint')!.parentElement!;
	expect(exit().querySelectorAll('li[id^="exit-hop-"]')).toHaveLength(0);
	expect(target.querySelector('main [data-chain-summary]')?.textContent?.trim()).toBe(
		'this machine \u2192 127.0.0.1:5432'
	);

	click(button('Add hop', exit()));
	await settle();
	let urlInputs = exit().querySelectorAll<HTMLInputElement>('input[id^="exit-url-"]');
	expect(urlInputs).toHaveLength(1);
	expect(document.activeElement).toBe(urlInputs[0]);
	const first = urlInputs[0];
	type(first.id, 'socks5://exit.example:1080');
	// Typing never remounts the row: same element, focus untouched.
	expect(target.querySelector('#' + first.id)).toBe(first);

	click(button('Add hop', exit()));
	await settle();
	urlInputs = exit().querySelectorAll<HTMLInputElement>('input[id^="exit-url-"]');
	type(urlInputs[1].id, 'ssh://ops@entry.example:22');
	click(button('Add URL', exit().querySelectorAll('li[id^="exit-hop-"]')[1]));
	await settle();
	urlInputs = exit().querySelectorAll<HTMLInputElement>('input[id^="exit-url-"]');
	expect(urlInputs).toHaveLength(3);
	expect(document.activeElement).toBe(urlInputs[2]);
	type(urlInputs[2].id, 'ssh://ops@entry-2.example:22');

	const titles = () =>
		Array.from(exit().querySelectorAll('li[id^="exit-hop-"] h4')).map((h) => h.textContent!.trim());
	expect(titles()).toEqual(['Hop 1 \u00b7 exit node', 'Hop 2 \u00b7 dialed from this machine']);
	expect(target.querySelector('main [data-chain-summary]')?.textContent?.trim()).toBe(
		'this machine \u2192 Hop 2 \u2192 Hop 1 \u2192 127.0.0.1:5432'
	);
	expect(chain().slice(1)).toEqual([
		'Hop 1 \u00b7 binds the port \u00b7 dialed from this machine ssh://edge.example:22',
		'This machine',
		'Hop 2 \u00b7 entry node \u00b7 dialed from this machine ssh://entry.example:22 ssh://entry-2.example:22',
		'Hop 1 \u00b7 exit node socks5://exit.example:1080',
		'Target 127.0.0.1:5432'
	]);

	const hops = exit().querySelectorAll('li[id^="exit-hop-"]');
	click(button('Up', hops[1]));
	await settle();
	expect(
		Array.from(exit().querySelectorAll<HTMLInputElement>('input[id^="exit-url-"]')).map(
			(input) => input.value
		)
	).toEqual([
		'ssh://ops@entry.example:22',
		'ssh://ops@entry-2.example:22',
		'socks5://exit.example:1080'
	]);
	expect(document.activeElement?.getAttribute('aria-label')).toBe('Down');

	click(button('Remove', exit().querySelectorAll('li[id^="exit-hop-"]')[0]));
	await settle();
	expect(exit().querySelectorAll('input[id^="exit-url-"]')).toHaveLength(2);
	click(button('Delete hop', exit().querySelectorAll('li[id^="exit-hop-"]')[1]));
	await settle();
	expect(titles()).toEqual(['Hop 1 \u00b7 exit node \u00b7 dialed from this machine']);
	expect(document.activeElement?.id).toBe('exit-add-hop');

	await save();
	expect((writes()[0].body as Rule).forward.way).toEqual([
		{ lb: ['ssh://ops@entry-2.example:22'] }
	]);
});

test('the URL builder prefills from the row, previews live, and Cancel or Escape change nothing', async () => {
	await render('#/rules/office');
	const row = target.querySelector<HTMLInputElement>('input[id^="exit-url-"]')!;
	expect(row.value).toBe('socks5://demo:placeholder@hop-a.example:1080');
	const wand = button('Build\u2026', row.parentElement!)!;
	wand.focus();
	click(wand);
	await settle();
	const dialog = target.querySelector<HTMLDialogElement>(
		'dialog[open][aria-labelledby="url-builder-title"]'
	)!;
	expect(dialog).not.toBeNull();
	const value = (id: string) =>
		dialog.querySelector<HTMLInputElement | HTMLSelectElement>('#' + id)!.value;
	expect(value('url-builder-protocol')).toBe('socks5');
	expect(value('url-builder-username')).toBe('demo');
	expect(value('url-builder-password')).toBe('placeholder');
	expect(dialog.querySelector<HTMLInputElement>('#url-builder-password')?.type).toBe('password');
	expect(value('url-builder-host')).toBe('hop-a.example');
	expect(value('url-builder-port')).toBe('1080');
	expect(document.activeElement).toBe(dialog.querySelector('#url-builder-username'));
	const preview = () => dialog.querySelector('output')!.textContent!.trim();
	expect(preview()).toBe('socks5://demo:placeholder@hop-a.example:1080');

	type('url-builder-host', '');
	expect(preview()).toBe('socks5://demo:placeholder@:1080');
	expect(button('Use URL', dialog)?.disabled).toBe(true);
	expect(dialog.querySelector('[role="status"]')?.textContent?.trim()).toBe(
		'Host and a numeric port are required.'
	);
	type('url-builder-host', 'fd00::5');
	type('url-builder-password', 'p@ss');
	expect(preview()).toBe('socks5://demo:p%40ss@[fd00::5]:1080');
	expect(button('Use URL', dialog)?.disabled).toBe(false);

	click(button('Cancel', dialog));
	await settle();
	expect(dialog.open).toBe(false);
	expect(row.value).toBe('socks5://demo:placeholder@hop-a.example:1080');
	expect(badge()).toBeNull();
	expect(document.activeElement).toBe(wand);

	click(wand);
	await settle();
	expect(dialog.open).toBe(true);
	// Escape: the browser removes `open`, then fires close.
	dialog.open = false;
	dialog.dispatchEvent(new Event('close'));
	await settle();
	expect(row.value).toBe('socks5://demo:placeholder@hop-a.example:1080');
	expect(badge()).toBeNull();

	click(wand);
	await settle();
	const select = dialog.querySelector<HTMLSelectElement>('#url-builder-protocol')!;
	select.value = 'shadowsocks';
	select.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
	expect(value('url-builder-encrypto')).toBe('aes-256-gcm');
	expect(value('url-builder-port')).toBe('8379');
	expect(button('Use URL', dialog)?.disabled).toBe(true);
	type('url-builder-host', 'ss.example');
	type('url-builder-password', 'secret');
	expect(preview()).toBe('ss://aes-256-gcm:secret@ss.example:8379');
	dialog.querySelector('form')!.requestSubmit();
	await settle();
	expect(dialog.open).toBe(false);
	expect(row.value).toBe('ss://aes-256-gcm:secret@ss.example:8379');
	expect(badge()).toBe('Unsaved changes');
	expect(document.activeElement).toBe(row);
	expect(chain()).toContain('Hop 1 \u00b7 exit node ss://ss.example:8379');
});

test('the URL builder offers command and netcat hops with a single raw command field', async () => {
	await render('#/rules/office');
	const row = target.querySelector<HTMLInputElement>('input[id^="exit-url-"]')!;
	const wand = button('Build\u2026', row.parentElement!)!;
	click(wand);
	await settle();
	const dialog = target.querySelector<HTMLDialogElement>(
		'dialog[open][aria-labelledby="url-builder-title"]'
	)!;
	const select = () => dialog.querySelector<HTMLSelectElement>('#url-builder-protocol')!;
	const pick = (name: string) => {
		select().value = name;
		select().dispatchEvent(new Event('change', { bubbles: true }));
		flushSync();
	};
	const command = () => dialog.querySelector<HTMLInputElement>('#url-builder-command')!;
	const fieldIds = () =>
		Array.from(
			dialog.querySelectorAll<HTMLElement>('[data-builder-fields] :is(input, select)')
		).map((element) => element.id);
	const preview = () => dialog.querySelector('output')!.textContent!.trim();
	const hint = () => dialog.querySelector('[role="status"]')?.textContent?.trim();

	pick('command');
	expect(fieldIds()).toEqual(['url-builder-command']);
	expect(dialog.querySelector('label[for="url-builder-command"]')?.textContent).toBe('Command');
	expect(command().placeholder).toBe('nc %h %p');
	expect(command().value).toBe('');
	expect(preview()).toBe('cmd:');
	expect(button('Use URL', dialog)?.disabled).toBe(true);
	expect(hint()).toBe('A command is required.');
	type('url-builder-command', 'ssh -W %h:%p jump');
	expect(preview()).toBe('cmd:ssh -W %h:%p jump');
	expect(button('Use URL', dialog)?.disabled).toBe(false);
	expect(hint()).toBe('');
	dialog.querySelector('form')!.requestSubmit();
	await settle();
	expect(dialog.open).toBe(false);
	expect(row.value).toBe('cmd:ssh -W %h:%p jump');

	click(wand);
	await settle();
	expect(select().value).toBe('command');
	expect(command().value).toBe('ssh -W %h:%p jump');

	pick('netcat');
	expect(fieldIds()).toEqual(['url-builder-command']);
	expect(dialog.querySelector('label[for="url-builder-command"]')?.textContent).toBe(
		'Execution prefix'
	);
	expect(command().placeholder).toBe('ssh jump');
	expect(preview()).toBe('nc:');
	expect(button('Use URL', dialog)?.disabled).toBe(false);
	expect(hint()).toBe('');
	type('url-builder-command', 'ssh jump');
	expect(preview()).toBe('nc:ssh jump');
	dialog.querySelector('form')!.requestSubmit();
	await settle();
	expect(row.value).toBe('nc:ssh jump');

	click(wand);
	await settle();
	expect(select().value).toBe('netcat');
	expect(command().value).toBe('ssh jump');
	click(button('Cancel', dialog));
	await settle();
	expect(row.value).toBe('nc:ssh jump');

	await save();
	expect((writes()[0].body as Rule).forward.way).toEqual([
		{ lb: ['nc:ssh jump'] },
		{ lb: ['ssh://ops@bastion.example:22', 'ssh://ops@bastion-2.example:22'] }
	]);
});

// Virtual endpoints: Address | Virtual segments on the listen and target sides.
const kind = (side: 'listen' | 'target', value: 'address' | 'virtual') =>
	target.querySelector<HTMLInputElement>(`input[name="${side}-kind"][value="${value}"]`)!;
const peers = () =>
	Array.from(target.querySelectorAll('main [data-virtual-peers] a')).map((link) => [
		link.textContent?.trim(),
		link.getAttribute('href')
	]);
const peerText = () =>
	Array.from(target.querySelectorAll('main [data-virtual-peers]'))
		.map((node) => node.textContent!.replace(/\s+/g, ' ').trim())
		.join(' | ');
const options = (id: string) =>
	Array.from(target.querySelectorAll<HTMLOptionElement>(`#${id} option`)).map(
		(option) => option.value
	);

test('#/new: Virtual replaces host, port and listen-through hops by one channel; hidden fields are not validated; a self loop is refused; the pair reopens', async () => {
	await render('#/new');
	expect(kind('listen', 'address').checked).toBe(true);
	expect(target.querySelector('input[name="target-kind"]')).toBeNull();
	type('rule-name', 'pair');
	type('listen-port', 'junk');
	type('listen-username', 'demo');
	choose(kind('listen', 'virtual'));
	expect(target.querySelector('#listen-host')).toBeNull();
	expect(target.querySelector('#listen-port')).toBeNull();
	expect(target.querySelector('#listen-add-hop')).toBeNull();
	expect(target.querySelector('main')?.textContent).not.toContain('Listen through');
	expect(field('listen-username').value).toBe('demo');
	type('listen-virtual', 'exit');
	expect(chain()).toEqual(['Clients', 'This machine virtual://exit', 'Direct', 'Target target']);

	choose(target.querySelector<HTMLInputElement>('input[name="mode"][value="forward"]')!);
	expect(kind('target', 'address').checked).toBe(true);
	expect(target.querySelector('#target-port')).not.toBeNull();
	choose(kind('target', 'virtual'));
	expect(target.querySelector('#target-host')).toBeNull();
	expect(target.querySelector('#target-port')).toBeNull();
	expect(target.querySelector('#exit-add-hop')).toBeNull();
	expect(target.querySelector('main [data-chain-summary]')).toBeNull();
	expect(target.querySelector('#listen-username')).toBeNull();
	type('target-virtual', 'exit');
	expect(chain()).toEqual([
		'Clients',
		'This machine virtual://exit',
		'Direct',
		'Target virtual://exit'
	]);
	await save();
	expect(writes()).toEqual([]);
	expect(target.querySelector('#target-virtual-error')?.textContent).toBe(
		'A rule cannot forward to its own listen channel.'
	);
	expect(document.activeElement).toBe(field('target-virtual'));
	type('target-virtual', 'egress');
	expect(target.querySelector('#target-virtual-error')).toBeNull();

	await save();
	expect(writes()).toEqual([
		{
			method: 'POST',
			url: '/apis/configs/rules',
			body: {
				name: 'pair',
				listen: { host: '', port: 0, virtual: 'exit' },
				forward: { virtual: 'egress' }
			}
		}
	]);
	expect(location.hash).toBe('#/rules/pair');
	expect(badge()).toBeNull();
	expect(kind('listen', 'virtual').checked).toBe(true);
	expect(kind('target', 'virtual').checked).toBe(true);
	expect(inputs()).toMatchObject({ 'listen-virtual': 'exit', 'target-virtual': 'egress' });
	expect(inputs()).not.toHaveProperty('listen-port');
});

test('channels must be valid before a request; the credentials of a virtual proxy listener are sent', async () => {
	await render('#/new');
	type('rule-name', 'exit');
	choose(kind('listen', 'virtual'));
	type('listen-virtual', 'a b');
	type('listen-username', 'demo');
	type('listen-password', 'placeholder');
	await save();
	expect(writes()).toEqual([]);
	expect(target.querySelector('#listen-virtual-error')?.textContent).toBe(
		'Channel is required and must not contain spaces or "/".'
	);
	expect(document.activeElement).toBe(field('listen-virtual'));
	type('listen-virtual', ' exit ');
	expect(target.querySelector('#listen-virtual-error')).toBeNull();
	await save();
	expect(writes().map((call) => call.body)).toEqual([
		{
			name: 'exit',
			listen: {
				host: '',
				port: 0,
				virtual: 'exit',
				username: 'demo',
				password: 'placeholder',
				protocols: LEGACY
			},
			forward: {}
		}
	]);
});

test('switching kinds keeps every typed draft and hides inactive hops from the diagram; an address rule never sends virtual fields', async () => {
	await render('#/rules/db-tunnel');
	expect(chain().join(' ')).toContain('ssh://edge.example:22');
	choose(kind('listen', 'virtual'));
	type('listen-virtual', 'entry');
	expect(chain().join(' ')).not.toContain('edge.example');
	expect(chain()).toContain('This machine virtual://entry');
	choose(kind('listen', 'address'));
	expect(inputs()).toMatchObject({ 'listen-host': '0.0.0.0', 'listen-port': '18099' });
	expect(chain().join(' ')).toContain('ssh://edge.example:22');
	choose(kind('target', 'virtual'));
	type('target-virtual', 'exit');
	expect(chain().join(' ')).toContain('Target virtual://exit');
	choose(kind('target', 'address'));
	expect(inputs()).toMatchObject({ 'target-host': '127.0.0.1', 'target-port': '5432' });
	choose(kind('listen', 'virtual'));
	expect(field('listen-virtual').value).toBe('entry');
	choose(kind('listen', 'address'));
	type('target-port', '5433');
	await save();
	expect(writes().map((call) => call.body)).toEqual([
		{
			name: 'db-tunnel',
			listen: { host: '0.0.0.0', port: 18099, way: [{ lb: ['ssh://ops@edge.example:22'] }] },
			forward: { host: '127.0.0.1', port: 5433 }
		}
	]);
});

test('the editor links peers by channel: the listener under a virtual target or a warning, incoming rules under a virtual listen, nothing while the list is unknown', async () => {
	rules.push(...structuredClone(virtualRulesFixture));
	await render('#/rules/lan-entry');
	expect(kind('target', 'virtual').checked).toBe(true);
	expect(peers()).toEqual([['shared-exit', '#/rules/shared-exit']]);
	expect(peerText()).toBe('Exit rule shared-exit');
	expect(options('target-virtual-list')).toEqual(['exit']);
	type('target-virtual', 'missing');
	expect(peers()).toEqual([]);
	expect(peerText()).toBe('No enabled rule listens on this channel.');
	type('target-virtual', ' ');
	expect(peerText()).toBe('');
	// A draft that becomes a virtual listen never lists its own stale copy, even after a rename.
	choose(kind('listen', 'virtual'));
	expect(options('listen-virtual-list')).toEqual(['missing']);
	type('listen-virtual', 'exit');
	type('rule-name', 'lan');
	expect(peers()).toEqual([]);
	expect(peerText()).toBe('');

	unmount(app!);
	app = null;
	target.remove();
	await render('#/rules/shared-exit');
	expect(peers()).toEqual([['lan-entry', '#/rules/lan-entry']]);
	expect(peerText()).toBe('Incoming rules lan-entry');

	unmount(app!);
	app = null;
	target.remove();
	fail.set('GET /apis/configs/rules', 'boom');
	await render('#/rules/orphan');
	expect(form()).not.toBeNull();
	expect(field('target-virtual').value).toBe('missing');
	expect(peerText()).toBe('');
});

// Listen protocols: a checkbox per scheme, shared credentials, one custom row per checked scheme.
const select = (id: string, value: string) => {
	const element = field(id);
	element.value = value;
	element.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
};
const toggle = (id: string, checked: boolean) => {
	const element = field(id);
	element.checked = checked;
	element.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
};
const mode = (value: 'proxy' | 'forward') =>
	target.querySelector<HTMLInputElement>(`input[name="mode"][value="${value}"]`)!;
const CIPHERS = fieldsOf(layoutFor('shadowsocks')!).find(
	(input) => input.name === 'encrypto'
)!.items!;
const TYPES = ['http', 'socks5', 'socks4', 'ssh', 'ss'];
const checkedTypes = () =>
	TYPES.filter((type) => target.querySelector<HTMLInputElement>(`#protocol-${type}`)?.checked);
const group = (name: string) =>
	Array.from(form()!.querySelectorAll('fieldset')).find((element) => {
		const labelledBy = element.getAttribute('aria-labelledby');
		const label = labelledBy
			? document.getElementById(labelledBy)
			: element.querySelector(':scope > legend');
		return label?.textContent?.trim() === name;
	});
const bands = () =>
	Array.from(form()!.querySelectorAll('.band > .band-title')).map((title) =>
		title.textContent?.trim()
	);
const reopen = async (hash: string) => {
	unmount(app!);
	app = null;
	target.remove();
	await render(hash);
};

test('the editor stacks General, Chain, Listen and Exit; General holds name, switch, mode and the listen type; the target type stays in Exit', async () => {
	await render('#/rules/office');
	expect(bands()).toEqual(['General', 'Chain', 'Listen', 'Exit']);
	const general = group('General')!;
	expect(
		Array.from(general.querySelectorAll('input')).map(
			(input) => input.id || (input.name ? `${input.name}=${input.value}` : input.type)
		)
	).toEqual([
		'rule-name',
		'checkbox',
		'mode=proxy',
		'mode=forward',
		'listen-kind=address',
		'listen-kind=virtual'
	]);
	expect(general.querySelector('input[role="switch"]')).not.toBeNull();
	expect(
		Array.from(general.querySelectorAll('legend, label[for="rule-name"]')).map((node) =>
			node.textContent?.trim()
		)
	).toEqual(['General', 'Rule name', 'Mode', 'Type']);
	expect(group('Listen')!.querySelector('input[type="radio"]')).toBeNull();
	expect(group('Exit')!.querySelector('input[type="radio"]')).toBeNull();
	choose(mode('forward'));
	expect(group('Exit')!.querySelectorAll('input[name="target-kind"]')).toHaveLength(2);
	expect(general.querySelectorAll('input[name="listen-kind"]')).toHaveLength(2);
	choose(kind('listen', 'virtual'));
	expect(group('Listen')!.querySelector('#listen-virtual')).not.toBeNull();
	expect(group('Listen')!.querySelector('#listen-port')).toBeNull();
});

test('a legacy rule checks HTTP, SOCKS5, SOCKS4 and SSH; checking Shadowsocks adds its row with a default cipher; the body lists exactly the checked ones and reopens so', async () => {
	await render('#/rules/lab');
	expect(checkedTypes()).toEqual(['http', 'socks5', 'socks4', 'ssh']);
	expect(
		Array.from(group('Protocols')!.querySelectorAll('label')).map((label) =>
			label.textContent?.trim()
		)
	).toEqual(['HTTP', 'SOCKS5', 'SOCKS4', 'SSH', 'Shadowsocks']);
	expect(group('HTTP')).not.toBeUndefined();
	expect(group('SSH')).not.toBeUndefined();
	expect(group('Shadowsocks')).toBeUndefined();
	expect(field('protocol-http-custom').checked).toBe(false);
	expect(target.querySelector('#protocol-http-username')).toBeNull();
	expect(badge()).toBeNull();

	toggle('protocol-ss', true);
	expect(badge()).toBe('Unsaved changes');
	const cipher = () => target.querySelector<HTMLSelectElement>('#protocol-ss-cipher');
	expect(group('Shadowsocks')!.contains(cipher())).toBe(true);
	expect(cipher()?.className).toBe('input');
	expect(form()!.querySelector('label[for="protocol-ss-cipher"]')?.textContent).toBe('Cipher');
	expect(cipher()?.value).toBe('aes-256-gcm');
	expect(options('protocol-ss-cipher')).toEqual(CIPHERS);
	for (const type of ['http', 'socks5', 'socks4', 'ssh']) toggle(`protocol-${type}`, false);
	expect(checkedTypes()).toEqual(['ss']);
	expect(group('HTTP')).toBeUndefined();
	select('protocol-ss-cipher', 'chacha20-ietf-poly1305');
	await save();
	expect(writes()).toEqual([
		{
			method: 'PUT',
			url: '/apis/configs/rules/lab',
			body: {
				name: 'lab',
				disabled: true,
				listen: {
					host: '127.0.0.1',
					port: 18100,
					username: 'demo',
					password: 'placeholder',
					protocols: [{ type: 'ss', cipher: 'chacha20-ietf-poly1305' }]
				},
				forward: {}
			}
		}
	]);
	expect(badge()).toBeNull();

	await reopen('#/rules/lab');
	expect(checkedTypes()).toEqual(['ss']);
	expect(cipher()?.value).toBe('chacha20-ietf-poly1305');
	expect(field('protocol-ss-custom').checked).toBe(false);
	expect(badge()).toBeNull();
});

test('custom credentials live under the protocol legend: SOCKS4 takes a username only, empty fields inherit, and unchecking custom or the scheme keeps the typed draft but sends nothing', async () => {
	rules.push(...structuredClone(protocolRulesFixture));
	await render('#/rules/mixed');
	expect(checkedTypes()).toEqual(['http', 'socks5']);
	expect(inputs()).toMatchObject({
		'listen-username': 'demo',
		'listen-password': 'placeholder',
		'protocol-http-custom': false,
		'protocol-socks5-custom': true,
		'protocol-socks5-username': 'socks-user',
		'protocol-socks5-password': 'socks-pass'
	});
	expect(field('protocol-socks5-password').type).toBe('password');
	expect(group('SOCKS5')!.querySelector('label[for="protocol-socks5-password"]')?.textContent).toBe(
		'Password'
	);
	expect(form()!.querySelectorAll('label[for$="-password"]')).toHaveLength(2);
	expect(target.querySelector('#protocol-http-username')).toBeNull();

	toggle('protocol-socks4', true);
	toggle('protocol-socks4-custom', true);
	expect(group('SOCKS4')!.querySelector('#protocol-socks4-username')).not.toBeNull();
	expect(target.querySelector('#protocol-socks4-password')).toBeNull();
	type('protocol-socks4-username', 'legacy-user');
	toggle('protocol-http-custom', true);
	type('protocol-http-username', 'web');
	toggle('protocol-http-custom', false);
	expect(target.querySelector('#protocol-http-username')).toBeNull();
	toggle('protocol-socks5', false);
	expect(group('SOCKS5')).toBeUndefined();
	await save();
	expect((writes()[0].body as Rule).listen).toEqual({
		host: '127.0.0.1',
		port: 18201,
		username: 'demo',
		password: 'placeholder',
		protocols: [{ type: 'http' }, { type: 'socks4', username: 'legacy-user' }]
	});

	toggle('protocol-socks5', true);
	expect(field('protocol-socks5-username').value).toBe('socks-user');
	toggle('protocol-http-custom', true);
	expect(field('protocol-http-username').value).toBe('web');
	choose(mode('forward'));
	expect(target.querySelector('#protocol-http')).toBeNull();
	expect(target.querySelector('#listen-username')).toBeNull();
	type('target-port', '5432');
	await save();
	expect((writes()[1].body as Rule).listen).toEqual({ host: '127.0.0.1', port: 18201 });
	choose(mode('proxy'));
	expect(checkedTypes()).toEqual(['http', 'socks5', 'socks4']);
	expect(field('protocol-http-username').value).toBe('web');
	expect(field('protocol-socks5-password').value).toBe('socks-pass');
});

test('#/new refuses an empty protocol selection and a Shadowsocks row without an effective password, focusing and labelling the field to fix', async () => {
	await render('#/new');
	type('rule-name', 'p');
	for (const type of ['http', 'socks5', 'socks4', 'ssh']) toggle(`protocol-${type}`, false);
	await save();
	expect(writes()).toEqual([]);
	expect(field('protocol-http').getAttribute('aria-invalid')).toBe('true');
	expect(document.activeElement).toBe(field('protocol-http'));
	expect(target.querySelector('#protocols-error')?.textContent).toBe(
		'Select at least one protocol.'
	);
	expect(field('protocol-http').getAttribute('aria-describedby')).toBe('protocols-error');
	toggle('protocol-ss', true);
	expect(target.querySelector('#protocols-error')).toBeNull();
	expect(field('protocol-http').hasAttribute('aria-invalid')).toBe(false);

	await save();
	expect(writes()).toEqual([]);
	expect(field('listen-password').getAttribute('aria-invalid')).toBe('true');
	expect(document.activeElement).toBe(field('listen-password'));
	expect(target.querySelector('#listen-password-error')?.textContent).toBe(
		'Shadowsocks needs a password.'
	);
	// The error follows the effective field: a custom row answers for its own password.
	toggle('protocol-ss-custom', true);
	expect(target.querySelector('#listen-password-error')).toBeNull();
	expect(target.querySelector('#protocol-ss-password-error')?.textContent).toBe(
		'Shadowsocks needs a password.'
	);
	type('protocol-ss-password', 'own');
	expect(target.querySelector('#protocol-ss-password-error')).toBeNull();
	await save();
	expect(writes()).toEqual([
		{
			method: 'POST',
			url: '/apis/configs/rules',
			body: {
				name: 'p',
				listen: {
					host: '127.0.0.1',
					port: 0,
					protocols: [{ type: 'ss', password: 'own', cipher: 'aes-256-gcm' }]
				},
				forward: {}
			}
		}
	]);
	expect(location.hash).toBe('#/rules/p');
	expect(field('protocol-ss-password').value).toBe('own');
});

test('a legacy flat cipher reopens as a checked Shadowsocks row, an alias as its own option; saving moves the cipher into the ss entry and forward mode hides but keeps it', async () => {
	rules.find((rule) => rule.name === 'lab')!.listen.cipher = 'AES_256_GCM';
	await render('#/rules/lab');
	expect(checkedTypes()).toEqual(['http', 'socks5', 'socks4', 'ssh', 'ss']);
	const cipher = () => target.querySelector<HTMLSelectElement>('#protocol-ss-cipher')!;
	expect(cipher().value).toBe('AES_256_GCM');
	expect(cipher().selectedOptions[0]?.textContent).toBe('AES_256_GCM');
	expect(options('protocol-ss-cipher')).toEqual(['AES_256_GCM', ...CIPHERS]);
	expect(badge()).toBeNull();
	type('listen-port', '18110');
	await save();
	expect((writes()[0].body as Rule).listen).toEqual({
		host: '127.0.0.1',
		port: 18110,
		username: 'demo',
		password: 'placeholder',
		protocols: [...LEGACY, { type: 'ss', cipher: 'AES_256_GCM' }]
	});
	select('protocol-ss-cipher', 'aes-128-gcm');
	expect(options('protocol-ss-cipher')).toEqual(CIPHERS);

	choose(mode('forward'));
	expect(target.querySelector('#protocol-ss-cipher')).toBeNull();
	type('target-port', '5432');
	await save();
	expect((writes()[1].body as Rule).listen).toEqual({ host: '127.0.0.1', port: 18110 });
	choose(mode('proxy'));
	expect(cipher().value).toBe('aes-128-gcm');
	expect(field('listen-password').value).toBe('placeholder');
	await save();
	expect((writes()[2].body as Rule).listen.protocols).toEqual([
		...LEGACY,
		{ type: 'ss', cipher: 'aes-128-gcm' }
	]);
});

// Duplicating: #/new?rule=<name> opens a dirty copy; the stored rules pre-check names and addresses.
const errorText = (id: string) => target.querySelector(`#${id}-error`)?.textContent ?? null;

test('#/new?rule= copies the source under a free "-copy" name, dirty from the start, and POSTs it once its port is free', async () => {
	rules.push({ name: 'office-copy', listen: { host: '127.0.0.1', port: 18150 }, forward: {} });
	await render('#/new?rule=office');
	expect(target.querySelector('h1')?.textContent).toBe('New rule');
	expect(requested('GET /apis/configs/rules/office')).toBe(1);
	expect(inputs()).toMatchObject({
		'rule-name': 'office-copy-2',
		'listen-host': '127.0.0.1',
		'listen-port': '18097'
	});
	expect(chain().join(' ')).toContain('socks5://hop-a.example:1080');
	expect(badge()).toBe('Unsaved changes');
	expect(button('Delete')).toBeUndefined();
	expect(button('Duplicate rule')).toBeUndefined();

	// The source still holds the port: no request leaves until it changes.
	await save();
	expect(writes()).toEqual([]);
	expect(errorText('listen-port')).toBe('Already used by rule "office".');
	expect(document.activeElement).toBe(field('listen-port'));
	type('listen-port', '18197');
	expect(errorText('listen-port')).toBeNull();
	await save();
	expect(writes()).toEqual([
		{
			method: 'POST',
			url: '/apis/configs/rules',
			body: {
				name: 'office-copy-2',
				listen: { host: '127.0.0.1', port: 18197, protocols: LEGACY },
				forward: {
					way: [
						{ lb: ['socks5://demo:placeholder@hop-a.example:1080'] },
						{ lb: ['ssh://ops@bastion.example:22', 'ssh://ops@bastion-2.example:22'] }
					]
				}
			}
		}
	]);
	expect(location.hash).toBe('#/rules/office-copy-2');
	expect(target.querySelector('h1')?.textContent).toBe('office-copy-2');
	expect(badge()).toBeNull();
	expect(rules.find((rule) => rule.name === 'office')?.listen.port).toBe(18097);
});

test('a copy whose source is gone shows the not-found state with a way back; the name falls back to "-copy" when the list fails', async () => {
	await render('#/new?rule=ghost');
	expect(target.querySelector('h1')?.textContent).toBe('New rule');
	const banner = target.querySelector('main [role="alert"]')!;
	expect(banner.textContent).toContain('Rule "ghost" was not found.');
	expect(banner.querySelector('a[href="#/"]')?.textContent?.trim()).toBe('Overview');
	expect(form()).toBeNull();
	expect(badge()).toBeNull();

	unmount(app!);
	app = null;
	target.remove();
	fail.set('GET /apis/configs/rules', 'boom');
	await render('#/new?rule=lab');
	expect(inputs()).toMatchObject({ 'rule-name': 'lab-copy', 'listen-username': 'demo' });
	expect(badge()).toBe('Unsaved changes');
});

test('the editor duplicates through the router: a clean rule opens its copy at once, a dirty one asks first', async () => {
	await render('#/rules/mirror');
	click(button('Duplicate rule'));
	await settle();
	expect(location.hash).toBe('#/new?rule=mirror');
	expect(target.querySelector('h1')?.textContent).toBe('New rule');
	expect(inputs()).toMatchObject({ 'rule-name': 'mirror-copy', 'target-port': '5432' });
	expect(badge()).toBe('Unsaved changes');

	// Leaving the copy is guarded like any dirty draft.
	click(button('Cancel', form()!));
	await settle();
	expect(confirmDialog()?.textContent).toContain('Discard unsaved changes?');
	click(confirmDialog()!.querySelectorAll('button')[1]);
	await settle();
	expect(location.hash).toBe('#/');

	click(target.querySelector('main article a[href="#/rules/lab"]'));
	await settle();
	type('rule-name', 'lab-2');
	click(button('Duplicate rule'));
	await settle();
	expect(confirmDialog()?.textContent).toContain('Discard unsaved changes?');
	click(confirmDialog()!.querySelectorAll('button')[0]);
	await settle();
	expect(location.hash).toBe('#/rules/lab');
	expect(inputs()).toMatchObject({ 'rule-name': 'lab-2' });
	expect(writes()).toEqual([]);
});

test('a name that repeats another rule or contains "/" is refused inline before any request', async () => {
	await render('#/rules/mirror');
	type('rule-name', ' office ');
	await save();
	expect(writes()).toEqual([]);
	expect(errorText('rule-name')).toBe('A rule with this name already exists.');
	expect(field('rule-name').getAttribute('aria-invalid')).toBe('true');
	expect(document.activeElement).toBe(field('rule-name'));
	expect(badge()).toBe('Unsaved changes');
	type('rule-name', 'office/2');
	expect(errorText('rule-name')).toBeNull();
	await save();
	expect(writes()).toEqual([]);
	expect(errorText('rule-name')).toBe('Names must not contain "/".');
	// Its own saved name is not a clash, so the rename below goes through.
	type('rule-name', 'mirror');
	type('listen-port', '18097');
	await save();
	expect(writes()).toEqual([]);
	expect(errorText('listen-port')).toBe('Already used by rule "office".');
	expect(document.activeElement).toBe(field('listen-port'));
	type('listen-port', '1088');
	await save();
	expect(writes()).toEqual([]);
	expect(errorText('listen-port')).toBe('Already used by the web UI.');
	type('listen-port', '18098');
	type('rule-name', 'mirror-2');
	await save();
	expect(writes().map((call) => `${call.method} ${call.url}`)).toEqual([
		'PUT /apis/configs/rules/mirror'
	]);
	expect(location.hash).toBe('#/rules/mirror-2');
});

test('the web UI clash reads the configured address, not the bound one; when it cannot be read the server decides', async () => {
	// Configured on the wildcard host, bound (per status) on 127.0.0.1:1088: the server accepts this.
	webUI = { host: '0.0.0.0', port: 1088 };
	await render('#/rules/mirror');
	expect(requested('GET /apis/configs/web-ui')).toBe(1);
	type('listen-port', '1088');
	await save();
	expect(errorText('listen-port')).toBeNull();
	expect(writes().map((call) => `${call.method} ${call.url}`)).toEqual([
		'PUT /apis/configs/rules/mirror'
	]);

	unmount(app!);
	app = null;
	target.remove();
	toasts.clear();
	stubApi();
	fail.set('GET /apis/configs/web-ui', 'boom');
	await render('#/rules/office');
	expect(toasts.list).toEqual([]);
	type('listen-port', '1088');
	await save();
	expect(errorText('listen-port')).toBeNull();
	expect(writes().map((call) => `${call.method} ${call.url}`)).toEqual([
		'PUT /apis/configs/rules/office'
	]);
});

test('a hop URL without a scheme or left blank is marked inline and focused; fixing it clears the mark', async () => {
	await render('#/rules/office');
	const urls = () =>
		Array.from(target.querySelectorAll<HTMLInputElement>('input[id^="exit-url-"]'));
	type(urls()[0].id, '127.0.0.1:1080');
	click(button('Add hop', target.querySelector('#exit-hint')!.parentElement!));
	await settle();
	expect(urls()).toHaveLength(4);
	await save();
	expect(writes()).toEqual([]);
	expect(urls()[0].getAttribute('aria-invalid')).toBe('true');
	expect(urls()[0].getAttribute('aria-describedby')).toBe(urls()[0].id + '-error');
	expect(errorText(urls()[0].id)).toBe('Include the scheme (e.g. socks5://host:1080).');
	expect(document.activeElement).toBe(urls()[0]);
	expect(urls()[1].hasAttribute('aria-invalid')).toBe(false);
	expect(urls()[3].getAttribute('aria-invalid')).toBe('true');
	expect(errorText(urls()[3].id)).toBe('Enter a proxy URL.');
	type(urls()[0].id, 'socks5://127.0.0.1:1080');
	expect(urls()[0].hasAttribute('aria-invalid')).toBe(false);
	expect(errorText(urls()[0].id)).toBeNull();
	expect(errorText(urls()[3].id)).toBe('Enter a proxy URL.');
	type(urls()[3].id, 'ssh://ops@entry.example:22');
	expect(errorText(urls()[3].id)).toBeNull();
	await save();
	expect((writes()[0].body as Rule).forward.way).toEqual([
		{ lb: ['socks5://127.0.0.1:1080'] },
		{ lb: ['ssh://ops@bastion.example:22', 'ssh://ops@bastion-2.example:22'] },
		{ lb: ['ssh://ops@entry.example:22'] }
	]);
});

test('credentials the server would refuse are marked on the field to fix: a ":" in a username, a password no username covers', async () => {
	await render('#/rules/lab');
	type('listen-username', 'us:er');
	toggle('protocol-http-custom', true);
	type('protocol-http-password', 'web-pass');
	await save();
	expect(writes()).toEqual([]);
	expect(errorText('listen-username')).toBe('Usernames must not contain ":".');
	expect(document.activeElement).toBe(field('listen-username'));
	// The HTTP row inherits the shared username, so its own password is covered.
	expect(errorText('protocol-http-password')).toBeNull();
	type('listen-username', '');
	expect(errorText('listen-username')).toBeNull();
	await save();
	expect(writes()).toEqual([]);
	expect(errorText('protocol-http-password')).toBe('Set a username for this password.');
	expect(errorText('listen-password')).toBe('Set a username for this password.');
	expect(document.activeElement).toBe(field('listen-password'));
	// Errors stay on the fields they were found on until the next save reads the whole form.
	type('protocol-http-username', 'web');
	expect(errorText('protocol-http-password')).toBe('Set a username for this password.');
	// Shadowsocks reading the shared password is what lets it stand without a username.
	toggle('protocol-ss', true);
	await save();
	expect(errorText('protocol-http-password')).toBeNull();
	expect(errorText('listen-password')).toBeNull();
	expect((writes()[0].body as Rule).listen).toEqual({
		host: '127.0.0.1',
		port: 18100,
		password: 'placeholder',
		protocols: [
			{ type: 'http', username: 'web', password: 'web-pass' },
			{ type: 'socks5' },
			{ type: 'socks4' },
			{ type: 'ssh' },
			{ type: 'ss', cipher: 'aes-256-gcm' }
		]
	});
});

test('editing a rule shows field skeletons behind a busy wrapper until the rule answers', async () => {
	let release!: () => void;
	delay.set(
		'GET /apis/configs/rules/office',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	await render('#/rules/office');
	expect(form()).toBeNull();
	const wrapper = target.querySelector<HTMLElement>('main [aria-busy="true"]');
	expect(wrapper).not.toBeNull();
	expect(
		Array.from(wrapper!.querySelectorAll('.sr-only')).some(
			(element) => element.textContent?.trim() === 'Loading...'
		)
	).toBe(true);
	const bars = Array.from(wrapper!.querySelectorAll('[data-skeleton]'));
	expect(bars.length).toBeGreaterThanOrEqual(4);
	expect(bars.every((bar) => bar.getAttribute('aria-hidden') === 'true')).toBe(true);
	expect(wrapper!.querySelectorAll('.band').length).toBeGreaterThanOrEqual(2);
	release();
	await settle();
	expect(target.querySelectorAll('main [data-skeleton]')).toHaveLength(0);
	expect(target.querySelector('main [aria-busy="true"]')).toBeNull();
	expect(field('rule-name').value).toBe('office');
});
