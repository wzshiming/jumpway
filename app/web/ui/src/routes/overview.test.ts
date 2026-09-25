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
import { router } from '../lib/router.svelte';
import { toasts } from '../lib/toast.svelte';
import type { Rule } from '../lib/types';

// The overview's rule cards switch a rule on and off against a method-aware in-memory /apis stub.

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
// `${method} ${url}` whose answer waits for the promise; like a slow server, the answer (and a
// mutation) is fixed when the request arrives.
let delay: Map<string, Promise<void>>;
// URLs whose PUT is applied but whose answer is lost on the way back.
let lost: Set<string>;

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
	delay = new Map();
	lost = new Set();
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string, init?: RequestInit) => {
			const method = init?.method ?? 'GET';
			const body: unknown = init?.body ? JSON.parse(String(init.body)) : undefined;
			calls.push({ method, url, body });
			const respond = (): Response => {
				const injected = fail.get(`${method} ${url}`);
				if (injected !== undefined && !injected.startsWith(SAVED_PREFIX))
					return text(400, injected);
				const answer = () => {
					if (lost.has(url)) throw new TypeError('Failed to fetch');
					return injected === undefined ? json(null) : text(400, injected);
				};
				if (url === '/apis/configs/status') return json(statusFixture);
				if (url === '/apis/stats') return json(snapshotFixture);
				if (url === '/apis/configs/rules') return json(rules);
				const match = /^\/apis\/configs\/rules\/([^/]+)$/.exec(url);
				if (!match) return text(404, 'not found');
				const name = decodeURIComponent(match[1]);
				const index = rules.findIndex((entry) => entry.name === name);
				if (index < 0) return text(400, `rule ${JSON.stringify(name)} not found`);
				if (method === 'GET') return json(rules[index]);
				if (method === 'PUT') rules[index] = body as Rule;
				else if (method === 'DELETE') rules.splice(index, 1);
				return answer();
			};
			const held = delay.get(`${method} ${url}`);
			delay.delete(`${method} ${url}`);
			const response = respond();
			if (held) await held;
			return response;
		})
	);
}

const settle = () => vi.advanceTimersByTimeAsync(0);
const requested = (entry: string) =>
	calls.filter((call) => `${call.method} ${call.url}` === entry).length;
const writes = () => calls.filter((call) => call.method !== 'GET');

const click = (element: Element | null | undefined) => {
	if (!element) throw new Error('missing element');
	element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
	flushSync();
};

const cards = () => Array.from(target.querySelectorAll<HTMLElement>('main article'));
const card = (name: string) => {
	const found = cards().find((element) => element.querySelector('a')?.textContent?.trim() === name);
	if (!found) throw new Error('missing card ' + name);
	return found;
};
const names = () => cards().map((element) => element.querySelector('a')?.textContent?.trim());
const toggle = (name: string) => card(name).querySelector<HTMLInputElement>('[role="switch"]')!;
const deleteButton = (name: string) =>
	card(name).querySelector<HTMLButtonElement>('footer button[aria-label="Delete"]')!;
const confirmDialog = () =>
	target.querySelector<HTMLDialogElement>('dialog[open][aria-describedby="confirm-message"]');
const answer = (index: 0 | 1) => click(confirmDialog()!.querySelectorAll('button')[index]);
const kpi = (name: string) => target.querySelector(`[data-kpi="${name}"]`)?.textContent?.trim();
const chip = (name: string) => card(name).querySelector('[data-state]')?.getAttribute('data-state');
const toastTexts = (role: string) =>
	Array.from(target.querySelectorAll(`[aria-label="Notifications"] [role="${role}"]`)).map(
		(element) => element.querySelector('p')?.textContent?.trim()
	);

async function render(path = '/') {
	history.replaceState(null, '', path);
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

beforeEach(() => {
	vi.useFakeTimers();
	stubApi();
});

afterEach(() => {
	teardown();
	toasts.clear();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
	localStorage.clear();
	document.documentElement.lang = 'en';
	document.body.innerHTML = '';
});

test('every card has an accessible switch that mirrors the configured state, labelled in en and zh', async () => {
	await render();
	const switches = cards().map((element) =>
		element.querySelector<HTMLInputElement>('[role="switch"]')!
	);
	expect(switches.map((element) => element.checked)).toEqual([true, true, true, false]);
	expect(switches.map((element) => element.getAttribute('aria-label'))).toEqual([
		'Enabled',
		'Enabled',
		'Enabled',
		'Enabled'
	]);
	expect(switches.every((element) => element.type === 'checkbox' && !element.disabled)).toBe(true);
	// The runtime chip stays a separate fact: "enabled" is configuration, "running" is what happens.
	expect(
		cards().map((element) => element.querySelector('[data-state]')?.getAttribute('data-state'))
	).toEqual(['running', 'running', 'retrying', 'disabled']);
	teardown();
	await render('/?lang=zh#/');
	expect(toggle('office').getAttribute('aria-label')).toBe('启用');
});

const pageHeader = () => target.querySelector('main h1')!.closest('header')!;
const breadcrumb = () => target.querySelector('main nav[aria-label="Breadcrumb"] a');
const grid = () => target.querySelector<HTMLElement>('main section[aria-label="Rules"] > div')!;

test('the overview manages the rules: a New rule card closes the grid and Edit, Duplicate, Rule Traffic, Delete sit on every card', async () => {
	await render();
	expect(pageHeader().querySelector('a[href="#/new"]')).toBeNull();
	const create = grid().lastElementChild!;
	expect([create.tagName, create.getAttribute('href'), create.textContent?.trim()]).toEqual([
		'A',
		'#/new',
		'New rule'
	]);
	expect(create.previousElementSibling?.tagName).toBe('ARTICLE');
	expect(target.querySelectorAll('main a[href="#/new"]')).toHaveLength(1);
	for (const element of cards()) {
		const controls = Array.from(element.querySelectorAll('footer a, footer button')).map(
			(control) => [control.tagName, control.getAttribute('aria-label')]
		);
		expect(controls).toEqual([
			['A', 'Edit'],
			['A', 'Duplicate rule'],
			['A', 'Rule Traffic'],
			['BUTTON', 'Delete']
		]);
	}
	expect(target.querySelector('nav a[href="#/rules"]')).toBeNull();
	// The editor belongs to the overview's entry, which is where rules are managed.
	click(card('office').querySelector('a[href="#/rules/office"]'));
	await settle();
	expect(target.querySelector('nav a[aria-current="page"]')?.getAttribute('href')).toBe('#/');
	expect(breadcrumb()?.getAttribute('href')).toBe('#/');
	expect(breadcrumb()?.textContent?.trim()).toBe('Overview');
});

test('an empty list shows only the New rule card', async () => {
	rules = [];
	await render('/#/');
	expect(target.querySelector('main h1')?.textContent).toBe('Overview');
	expect(cards()).toEqual([]);
	expect(target.querySelector('main section[aria-label="Rules"]')?.textContent).not.toContain(
		'No rules yet.'
	);
	expect(
		Array.from(grid().children).map((element) => [element.tagName, element.getAttribute('href')])
	).toEqual([['A', '#/new']]);
	expect(pageHeader().querySelector('a[href="#/new"]')).toBeNull();
	teardown();
	rules = structuredClone(rulesFixture);
	await render('/?lang=zh#/');
	const trailing = card('office').parentElement!.lastElementChild;
	expect(trailing?.getAttribute('href')).toBe('#/new');
	expect(trailing?.textContent?.trim()).toBe('新建规则');
	expect(card('office').querySelector('footer button')?.getAttribute('aria-label')).toBe('删除');
});

test('switching a rule off re-reads it, PUTs it back with only `disabled` added, and flips only once answered', async () => {
	await render();
	// The overview's copy is stale: the exit chain grew behind its back.
	const latestWay = [...(rules[0].forward.way as unknown[]), 'ssh://ops@third.example:22'];
	rules[0].forward.way = latestWay as Rule['forward']['way'];
	const statusReads = requested('GET /apis/configs/status');
	let release!: () => void;
	delay.set(
		'PUT /apis/configs/rules/office',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);

	toggle('office').focus();
	click(toggle('office'));
	await settle();
	expect(requested('GET /apis/configs/rules/office')).toBe(1);
	expect(requested('PUT /apis/configs/rules/office')).toBe(1);
	// Pending: still shown as enabled, the control is disabled, and a second click is a no-op.
	expect(toggle('office').checked).toBe(true);
	expect(toggle('office').disabled).toBe(true);
	expect(chip('office')).toBe('running');
	click(toggle('office'));
	await settle();
	expect(requested('PUT /apis/configs/rules/office')).toBe(1);
	// Other cards keep working meanwhile.
	expect(toggle('mirror').disabled).toBe(false);
	// Browsers drop focus from a disabled control (Chrome on the next key press): it comes back.
	const title = card('office').querySelector('a')!;
	title.focus();
	title.blur();
	expect(document.activeElement).toBe(document.body);

	release();
	await settle();
	expect(toggle('office').checked).toBe(false);
	expect(toggle('office').disabled).toBe(false);
	expect(document.activeElement).toBe(toggle('office'));
	expect(chip('office')).toBe('disabled');
	expect(writes()).toEqual([
		{
			method: 'PUT',
			url: '/apis/configs/rules/office',
			body: { ...rulesFixture[0], forward: { way: latestWay }, disabled: true }
		}
	]);
	expect(requested('GET /apis/configs/status')).toBe(statusReads + 1);
	expect(toastTexts('status')).toEqual(['Saved and applied.']);
	expect(toastTexts('alert')).toEqual([]);
	expect(target.innerHTML).not.toContain('third.example');
});

test('switching a disabled rule on drops `disabled` and keeps its credentials and every other field', async () => {
	await render();
	expect(chip('lab')).toBe('disabled');
	click(toggle('lab'));
	await settle();
	const [write] = writes();
	expect(write.url).toBe('/apis/configs/rules/lab');
	const { disabled: _, ...expected } = rulesFixture[3];
	expect(write.body).toEqual(expected);
	expect('disabled' in (write.body as object)).toBe(false);
	expect((write.body as Rule).listen.password).toBe('placeholder');
	expect(toggle('lab').checked).toBe(true);
	expect(chip('lab')).toBe('stopped');
	expect(target.innerHTML).not.toContain('placeholder');
});

test('an ordinary 400 keeps the switch where it was and reports the message', async () => {
	await render();
	fail.set('PUT /apis/configs/rules/mirror', 'rules[1].listen.port 18098 is in use');
	click(toggle('mirror'));
	await settle();
	expect(requested('PUT /apis/configs/rules/mirror')).toBe(1);
	expect(toggle('mirror').checked).toBe(true);
	expect(toggle('mirror').disabled).toBe(false);
	expect(chip('mirror')).toBe('running');
	expect(rules[1].disabled).toBeUndefined();
	expect(toastTexts('alert')).toEqual(['rules[1].listen.port 18098 is in use']);
	expect(toastTexts('status')).toEqual([]);
	// The card is usable again: the next attempt goes through.
	fail.clear();
	click(toggle('mirror'));
	await settle();
	expect(requested('PUT /apis/configs/rules/mirror')).toBe(2);
	expect(toggle('mirror').checked).toBe(false);
});

test('"saved, but" shows the persisted value with a warning instead of a success', async () => {
	await render();
	const message =
		SAVED_PREFIX + 'reload failed: listen tcp 127.0.0.1:18099: bind: address already in use';
	fail.set('PUT /apis/configs/rules/db-tunnel', message);
	const statusReads = requested('GET /apis/configs/status');
	click(toggle('db-tunnel'));
	await settle();
	expect(toggle('db-tunnel').checked).toBe(false);
	expect(chip('db-tunnel')).toBe('disabled');
	expect(rules[2].disabled).toBe(true);
	expect(toastTexts('alert')).toEqual([message]);
	expect(toastTexts('status')).toEqual([]);
	await settle();
	expect(requested('GET /apis/configs/status')).toBe(statusReads + 1);
});

test('a lost answer is not claimed either way: the rule is re-read and the switch follows what is stored', async () => {
	await render();
	lost.add('/apis/configs/rules/office');
	click(toggle('office'));
	await settle();
	expect(requested('PUT /apis/configs/rules/office')).toBe(1);
	expect(toastTexts('alert')).toEqual([
		'The request did not complete. The change may or may not have been applied.'
	]);
	expect(toastTexts('status')).toEqual([]);
	// Re-read after the failure: the write had been applied, so the switch shows off.
	expect(requested('GET /apis/configs/rules/office')).toBe(2);
	expect(toggle('office').checked).toBe(false);
	expect(chip('office')).toBe('disabled');
	expect(toggle('office').disabled).toBe(false);
});

test('a re-read that fails too leaves the switch untouched', async () => {
	await render();
	let puts = 0;
	vi.mocked(fetch).mockImplementation(async (url: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';
		calls.push({ method, url: String(url), body: undefined });
		if (url === '/apis/configs/rules/office' && method === 'GET' && puts > 0)
			throw new TypeError('Failed to fetch');
		if (method === 'PUT') {
			puts++;
			throw new TypeError('Failed to fetch');
		}
		if (url === '/apis/configs/rules/office') return json(rules[0]);
		if (url === '/apis/configs/status') return json(statusFixture);
		return json(snapshotFixture);
	});
	click(toggle('office'));
	await settle();
	expect(puts).toBe(1);
	expect(toggle('office').checked).toBe(true);
	expect(chip('office')).toBe('running');
	expect(toggle('office').disabled).toBe(false);
	expect(toastTexts('alert')).toEqual([
		'The request did not complete. The change may or may not have been applied.'
	]);
});

test('unmounting during a pending write neither touches the DOM nor polls again', async () => {
	await render();
	let release!: () => void;
	delay.set(
		'PUT /apis/configs/rules/office',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	click(toggle('office'));
	await settle();
	expect(requested('PUT /apis/configs/rules/office')).toBe(1);
	teardown();
	const seen = calls.length;
	release();
	await settle();
	await settle();
	expect(calls.length).toBe(seen);
	expect(rules[0].disabled).toBe(true);
});

test('leaving the page while a switch is still re-reading abandons it: what it read is never written', async () => {
	await render();
	let release!: () => void;
	delay.set(
		'GET /apis/configs/rules/office',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	click(toggle('office'));
	await settle();
	expect(requested('GET /apis/configs/rules/office')).toBe(1);
	expect(writes()).toEqual([]);

	// Meanwhile the user opens the editor (the overview unmounts) and the rule is saved with a new port.
	click(card('office').querySelector('a[href="#/rules/office"]'));
	await settle();
	expect(location.hash).toBe('#/rules/office');
	expect(target.querySelector('main h1')?.textContent?.trim()).toBe('office');
	rules[0] = { ...rules[0], listen: { ...rules[0].listen, port: 18123 } };
	const stored = structuredClone(rules[0]);
	const statusReads = requested('GET /apis/configs/status');

	// The stale answer (port 18097) arrives now: too late to become a write.
	release();
	await settle();
	await settle();
	expect(writes()).toEqual([]);
	expect(rules[0]).toEqual(stored);
	expect(requested('GET /apis/configs/status')).toBe(statusReads);
	expect(toastTexts('status')).toEqual([]);
	expect(toastTexts('alert')).toEqual([]);
});

test('Delete asks by name; Cancel sends nothing, confirming removes the card, re-reads the list and status, and counts it out of the KPI', async () => {
	await render();
	expect(kpi('rules')).toBe('2/4');
	click(deleteButton('office'));
	await settle();
	expect(confirmDialog()?.textContent).toContain('Delete rule "office"?');
	answer(0);
	await settle();
	expect(confirmDialog()).toBeNull();
	expect(writes()).toEqual([]);
	expect(names()).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	expect(deleteButton('office').disabled).toBe(false);

	click(deleteButton('office'));
	await settle();
	answer(1);
	await settle();
	expect(writes()).toEqual([
		{ method: 'DELETE', url: '/apis/configs/rules/office', body: undefined }
	]);
	expect(names()).toEqual(['mirror', 'db-tunnel', 'lab']);
	expect(kpi('rules')).toBe('2/3');
	expect(toastTexts('status')).toEqual(['Rule deleted.']);
	expect(toastTexts('alert')).toEqual([]);
	expect(requested('GET /apis/configs/status')).toBe(2);
	expect(requested('GET /apis/configs/rules')).toBe(2);
	// The remaining cards are untouched: same switches, same states.
	expect(cards().map((element) => toggle(names()[cards().indexOf(element)]!).checked)).toEqual([
		true,
		true,
		false
	]);
	expect(
		cards().every(
			(element) => !element.querySelector<HTMLInputElement>('[role="switch"]')!.disabled
		)
	).toBe(true);
});

test('an ordinary 400 on delete keeps the card and reports the message; nothing is re-read', async () => {
	await render();
	fail.set('DELETE /apis/configs/rules/mirror', 'rule "mirror" is in use');
	click(deleteButton('mirror'));
	await settle();
	answer(1);
	await settle();
	expect(requested('DELETE /apis/configs/rules/mirror')).toBe(1);
	expect(names()).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	expect(toastTexts('alert')).toEqual(['rule "mirror" is in use']);
	expect(toastTexts('status')).toEqual([]);
	expect(requested('GET /apis/configs/rules')).toBe(1);
	expect(requested('GET /apis/configs/status')).toBe(1);
	expect(deleteButton('mirror').disabled).toBe(false);
	expect(toggle('mirror').disabled).toBe(false);
});

test('a "saved, but" delete is a delete: the card goes with a warning instead of a success', async () => {
	await render();
	const message =
		SAVED_PREFIX + 'reload failed: listen tcp 127.0.0.1:18097: bind: address already in use';
	fail.set('DELETE /apis/configs/rules/mirror', message);
	click(deleteButton('mirror'));
	await settle();
	answer(1);
	await settle();
	expect(names()).toEqual(['office', 'db-tunnel', 'lab']);
	expect(rules.map((rule) => rule.name)).toEqual(['office', 'db-tunnel', 'lab']);
	expect(toastTexts('alert')).toEqual([message]);
	expect(toastTexts('status')).toEqual([]);
	expect(requested('GET /apis/configs/status')).toBe(2);
	expect(requested('GET /apis/configs/rules')).toBe(2);
});

test('a lost delete answer is not claimed: the list is re-read and shows what is stored', async () => {
	await render();
	lost.add('/apis/configs/rules/db-tunnel');
	click(deleteButton('db-tunnel'));
	await settle();
	answer(1);
	await settle();
	expect(requested('DELETE /apis/configs/rules/db-tunnel')).toBe(1);
	expect(toastTexts('alert')).toEqual([
		'The request did not complete. The change may or may not have been applied.'
	]);
	expect(toastTexts('status')).toEqual([]);
	// The re-read is the only source of truth: the server had applied it.
	expect(requested('GET /apis/configs/rules')).toBe(2);
	expect(names()).toEqual(['office', 'mirror', 'lab']);
});

test('a card with a write in flight cannot be deleted, and a card being deleted cannot be switched; others stay usable', async () => {
	await render();
	let release!: () => void;
	delay.set(
		'PUT /apis/configs/rules/office',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	click(toggle('office'));
	await settle();
	expect(toggle('office').disabled).toBe(true);
	expect(deleteButton('office').disabled).toBe(true);
	click(deleteButton('office'));
	await settle();
	expect(confirmDialog()).toBeNull();
	expect(deleteButton('mirror').disabled).toBe(false);
	release();
	await settle();
	expect(deleteButton('office').disabled).toBe(false);

	click(deleteButton('mirror'));
	await settle();
	expect(confirmDialog()).not.toBeNull();
	expect(toggle('mirror').disabled).toBe(true);
	expect(deleteButton('mirror').disabled).toBe(true);
	expect(toggle('db-tunnel').disabled).toBe(false);
	click(toggle('mirror'));
	await settle();
	answer(0);
	await settle();
	expect(toggle('mirror').disabled).toBe(false);
	expect(deleteButton('mirror').disabled).toBe(false);
	expect(writes()).toEqual([
		{
			method: 'PUT',
			url: '/apis/configs/rules/office',
			body: { ...rulesFixture[0], disabled: true }
		}
	]);
});

test('leaving the overview while the delete question is open: a late "Delete" answer deletes nothing and says nothing', async () => {
	await render();
	click(deleteButton('lab'));
	await settle();
	const question = confirmDialog()!;
	expect(question.textContent).toContain('Delete rule "lab"?');
	// A browser Back is not blocked by a page-level question; the overview unmounts under it.
	await router.navigate('#/hosts');
	await settle();
	expect(location.hash).toBe('#/hosts');
	expect(target.querySelector('main h1')?.textContent).toBe('Proxy Hosts');
	expect(question.open).toBe(true);
	click(question.querySelectorAll('button')[1]);
	await settle();
	await settle();
	expect(writes()).toEqual([]);
	expect(rules.map((rule) => rule.name)).toContain('lab');
	expect(toastTexts('status')).toEqual([]);
	expect(toastTexts('alert')).toEqual([]);
	expect(requested('GET /apis/configs/status')).toBe(1);
});

test('a list answer that predates a delete cannot bring the deleted card back', async () => {
	await render();
	// The re-read after the first delete is slow; its payload is fixed when it is requested.
	let release!: () => void;
	delay.set(
		'GET /apis/configs/rules',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	click(deleteButton('office'));
	await settle();
	answer(1);
	await settle();
	expect(names()).toEqual(['mirror', 'db-tunnel', 'lab']);
	expect(requested('GET /apis/configs/rules')).toBe(2);

	click(deleteButton('mirror'));
	await settle();
	answer(1);
	await settle();
	expect(requested('DELETE /apis/configs/rules/mirror')).toBe(1);
	expect(requested('GET /apis/configs/rules')).toBe(3);
	expect(names()).toEqual(['db-tunnel', 'lab']);
	// The slow answer still lists mirror; it is superseded, not applied.
	release();
	await settle();
	await settle();
	expect(names()).toEqual(['db-tunnel', 'lab']);
});

test.each([
	{ name: 'mirror', enabled: false, state: 'disabled' },
	{ name: 'lab', enabled: true, state: 'stopped' }
])(
	'a list answer that predates an accepted write cannot undo it: $name switched while the re-read after a delete is still open',
	async ({ name, enabled, state }) => {
		await render();
		const section = target.querySelector('main section[aria-label="Rules"]')!;
		// The re-read after the delete is slow; its payload is fixed when it is requested.
		let release!: () => void;
		delay.set(
			'GET /apis/configs/rules',
			new Promise<void>((resolve) => {
				release = resolve;
			})
		);
		click(deleteButton('office'));
		await settle();
		answer(1);
		await settle();
		expect(names()).toEqual(['mirror', 'db-tunnel', 'lab']);
		expect(requested('GET /apis/configs/rules')).toBe(2);
		expect(section.getAttribute('aria-busy')).toBe('true');

		// Meanwhile the switch is answered: the backend now stores the flipped value.
		click(toggle(name));
		await settle();
		expect(requested(`PUT /apis/configs/rules/${name}`)).toBe(1);
		expect(rules.find((rule) => rule.name === name)?.disabled).toBe(enabled ? undefined : true);
		expect(toggle(name).checked).toBe(enabled);
		expect(chip(name)).toBe(state);
		expect(toggle(name).disabled).toBe(false);
		expect(deleteButton(name).disabled).toBe(false);

		// The slow answer still carries the value from before the write: superseded, not applied.
		release();
		await settle();
		await settle();
		expect(toggle(name).checked).toBe(enabled);
		expect(chip(name)).toBe(state);
		expect(names()).toEqual(['mirror', 'db-tunnel', 'lab']);
		expect(kpi('rules')).toBe('2/3');
		expect(section.getAttribute('aria-busy')).toBe('false');
		expect(toggle(name).disabled).toBe(false);
		expect(deleteButton(name).disabled).toBe(false);
		expect(toastTexts('status')).toEqual(['Rule deleted.', 'Saved and applied.']);
		expect(toastTexts('alert')).toEqual([]);
		// What the overview shows is what the backend stores.
		expect(requested('GET /apis/configs/rules')).toBe(3);
		expect(cards().map((element) => element.querySelector('a')?.textContent?.trim())).toEqual(
			rules.map((rule) => rule.name)
		);
		expect(
			cards().map((element) => element.querySelector<HTMLInputElement>('[role="switch"]')!.checked)
		).toEqual(rules.map((rule) => !rule.disabled));
	}
);

const compact = (element: Element | null | undefined) =>
	element?.textContent?.replace(/\s+/g, ' ').trim() ?? null;
const peersOf = (name: string) =>
	Array.from(card(name).querySelectorAll('[data-virtual-peers] a')).map((link) => [
		link.textContent?.trim(),
		link.getAttribute('href')
	]);

test('virtual cards label channels as virtual:// and link peers by channel; a dangling target warns; disabling the exit makes its entries dangling', async () => {
	rules.push(...structuredClone(virtualRulesFixture));
	await render();
	expect(names()).toEqual([
		'office',
		'mirror',
		'db-tunnel',
		'lab',
		'shared-exit',
		'lan-entry',
		'orphan'
	]);
	const dds = (name: string) => Array.from(card(name).querySelectorAll('dl dd'));
	expect(compact(card('shared-exit').querySelector('header p'))).toBe('Proxy');
	expect(dds('shared-exit')[0].textContent).toContain('virtual://exit');
	expect(compact(dds('shared-exit')[0].querySelector('[data-virtual-peers]'))).toBe(
		'Incoming rules lan-entry'
	);
	expect(peersOf('shared-exit')).toEqual([['lan-entry', '#/rules/lan-entry']]);
	expect(compact(card('lan-entry').querySelector('header p'))).toBe('Port forward');
	expect(dds('lan-entry')[1].textContent).toContain('virtual://exit');
	expect(peersOf('lan-entry')).toEqual([['shared-exit', '#/rules/shared-exit']]);
	expect(peersOf('orphan')).toEqual([]);
	expect(compact(card('orphan').querySelector('[data-virtual-peers]'))).toBe(
		'No enabled rule listens on this channel.'
	);
	for (const name of ['office', 'mirror', 'db-tunnel', 'lab']) {
		expect(card(name).querySelector('[data-virtual-peers]')).toBeNull();
	}

	click(toggle('shared-exit'));
	await settle();
	expect(rules.find((rule) => rule.name === 'shared-exit')?.disabled).toBe(true);
	expect(compact(card('lan-entry').querySelector('[data-virtual-peers]'))).toBe(
		'No enabled rule listens on this channel.'
	);
	// The incoming side lists enabled entries whatever the listener's own state.
	expect(peersOf('shared-exit')).toEqual([['lan-entry', '#/rules/lan-entry']]);
});
