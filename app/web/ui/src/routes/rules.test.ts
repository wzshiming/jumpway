import { flushSync, mount, unmount } from 'svelte';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import {
	rulesFixture,
	snapshotFixture,
	statusFixture,
	virtualRulesFixture
} from '../../e2e/fixtures/api';
import App from '../App.svelte';
import { SAVED_PREFIX } from '../lib/api';
import { toasts } from '../lib/toast.svelte';
import type { Rule } from '../lib/types';
import { fieldsOf, layoutFor } from '../lib/urlBuilder';

// The whole app mounted in jsdom against a method-aware in-memory /apis stub.

interface Call {
	method: string;
	url: string;
	body: unknown;
}

let target: HTMLElement;
let app: ReturnType<typeof mount> | null = null;
let rules: Rule[];
let calls: Call[];
// `${method} ${url}` → 400 text. A "saved, but " text still applies the mutation first.
let fail: Map<string, string>;

const json = (body: unknown) =>
	new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});
const text = (status: number, body: string) => new Response(body, { status });

function stubApi() {
	rules = structuredClone(rulesFixture);
	calls = [];
	fail = new Map();
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string, init?: RequestInit) => {
			const method = init?.method ?? 'GET';
			const body: unknown = init?.body ? JSON.parse(String(init.body)) : undefined;
			calls.push({ method, url, body });
			const injected = fail.get(`${method} ${url}`);
			if (injected !== undefined && !injected.startsWith(SAVED_PREFIX)) return text(400, injected);
			const answer = () => (injected === undefined ? json(null) : text(400, injected));
			if (url === '/apis/configs/status') return json(statusFixture);
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
	expect(target.querySelector('main header a[href="#/new"]')?.textContent?.trim()).toBe('New rule');
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
				listen: { host: '127.0.0.1', port: 18097, username: 'demo', password: 'placeholder' },
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

test('the editor footer is the shared save area: an iconed Save & Apply submit, Cancel and Delete as plain buttons, one dirty badge', async () => {
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

	click(target.querySelector('main header a[href="#/new"]'));
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
	fail.set('PUT /apis/configs/rules/mirror', 'rules[1].listen.port 18097 is already in use');
	type('listen-port', '18097');
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
		'rules[1].listen address 127.0.0.1:18097 is already used by rule "office"'
	);
	type('rule-name', 'mirror-2');
	type('listen-port', '18097');
	await save();
	expect(location.hash).toBe('#/rules/mirror');
	expect(badge()).toBe('Unsaved changes');
	const banner = target.querySelector('main [role="alert"]')!;
	expect(banner.textContent).toContain('already used by rule "office"');
	expect(inputs()).toMatchObject({ 'rule-name': 'mirror-2', 'listen-port': '18097' });

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
			listen: { host: '', port: 0, virtual: 'exit', username: 'demo', password: 'placeholder' },
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

// Shadowsocks: the cipher select shares the credentials block and the password.
const select = (id: string, value: string) => {
	const element = field(id);
	element.value = value;
	element.dispatchEvent(new Event('change', { bubbles: true }));
	flushSync();
};
const cipher = () => target.querySelector<HTMLSelectElement>('#listen-cipher');
const mode = (value: 'proxy' | 'forward') =>
	target.querySelector<HTMLInputElement>(`input[name="mode"][value="${value}"]`)!;
const CIPHERS = fieldsOf(layoutFor('shadowsocks')!).find(
	(input) => input.name === 'encrypto'
)!.items!;

test("the Shadowsocks cipher is a labelled select next to the credentials: Off by default with the builder's ciphers, sent beside the password, dropped again by Off", async () => {
	await render('#/rules/lab');
	expect(cipher()?.tagName).toBe('SELECT');
	expect(cipher()?.className).toBe('input');
	expect(form()!.querySelector('label[for="listen-cipher"]')?.textContent).toBe(
		'Shadowsocks cipher'
	);
	expect(cipher()?.value).toBe('');
	expect(options('listen-cipher')).toEqual(['', ...CIPHERS]);
	expect(cipher()?.options[0].textContent).toBe('Off');
	expect(target.querySelector('#listen-cipher-hint')?.textContent).toBe(
		'Serves Shadowsocks on the same port, sharing the password.'
	);
	expect(cipher()?.getAttribute('aria-describedby')).toBe('listen-cipher-hint');
	expect(badge()).toBeNull();

	select('listen-cipher', 'aes-256-gcm');
	expect(badge()).toBe('Unsaved changes');
	type('listen-username', '');
	await save();
	expect(writes()).toEqual([
		{
			method: 'PUT',
			url: '/apis/configs/rules/lab',
			body: {
				name: 'lab',
				disabled: true,
				listen: { host: '127.0.0.1', port: 18100, password: 'placeholder', cipher: 'aes-256-gcm' },
				forward: {}
			}
		}
	]);
	expect(badge()).toBeNull();
	expect(cipher()?.value).toBe('aes-256-gcm');

	select('listen-cipher', '');
	expect(badge()).toBe('Unsaved changes');
	await save();
	expect((writes()[1].body as Rule).listen).toEqual({
		host: '127.0.0.1',
		port: 18100,
		password: 'placeholder'
	});
});

test('a saved cipher reopens selected; forward mode hides and omits it while the draft survives the round trip', async () => {
	rules.find((rule) => rule.name === 'lab')!.listen.cipher = 'aes-256-gcm';
	await render('#/rules/lab');
	expect(cipher()?.value).toBe('aes-256-gcm');
	expect(cipher()?.options[cipher()!.selectedIndex].textContent).toBe('aes-256-gcm');

	choose(mode('forward'));
	expect(cipher()).toBeNull();
	type('target-port', '5432');
	await save();
	expect((writes()[0].body as Rule).listen).toEqual({ host: '127.0.0.1', port: 18100 });
	choose(mode('proxy'));
	expect(cipher()?.value).toBe('aes-256-gcm');
	expect(field('listen-password').value).toBe('placeholder');
	await save();
	expect((writes()[1].body as Rule).listen).toEqual({
		host: '127.0.0.1',
		port: 18100,
		username: 'demo',
		password: 'placeholder',
		cipher: 'aes-256-gcm'
	});
});

test('a cipher alias the backend accepts is shown as its own option and saved back unchanged', async () => {
	rules.find((rule) => rule.name === 'lab')!.listen.cipher = 'AES_256_GCM';
	await render('#/rules/lab');
	expect(cipher()?.value).toBe('AES_256_GCM');
	expect(cipher()?.selectedOptions[0]?.textContent).toBe('AES_256_GCM');
	expect(options('listen-cipher')).toEqual(['', 'AES_256_GCM', ...CIPHERS]);
	expect(badge()).toBeNull();
	type('listen-port', '18110');
	await save();
	expect((writes()[0].body as Rule).listen).toMatchObject({ port: 18110, cipher: 'AES_256_GCM' });
	select('listen-cipher', 'aes-128-gcm');
	expect(options('listen-cipher')).toEqual(['', ...CIPHERS]);
});
