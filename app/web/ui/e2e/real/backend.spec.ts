import {
	expect,
	test as base,
	type APIRequestContext,
	type APIResponse,
	type Locator,
	type Page
} from '@playwright/test';
import { randomBytes } from 'node:crypto';
import { connect, type Socket } from 'node:net';
import type { RawConfig, Rule, RuleStats, Snapshot, Stats, Status } from '../../src/lib/types.ts';

// Opt-in acceptance against a RUNNING backend; nothing is mocked or intercepted here.
//   JUMPWAY_URL             web UI origin (unset: every test skips)
//   JUMPWAY_E2E_WRITE=1     allow writes; also needs a loopback URL or JUMPWAY_E2E_ISOLATED=1
//   JUMPWAY_E2E_ISOLATED=1  the backend is disposable: the YAML file may be rewritten and restored
//   JUMPWAY_TARGET_URL      an HTTP server whose GET /stream keeps sending data (traffic test)
// Writes only ever create uniquely named rules and remove them again; existing rules are never
// touched or reset.

const backend = process.env.JUMPWAY_URL ?? '';
const isolated = process.env.JUMPWAY_E2E_ISOLATED === '1';
const writable = process.env.JUMPWAY_E2E_WRITE === '1' && (isolated || isLoopback(backend));
const target = process.env.JUMPWAY_TARGET_URL ?? '';
const WRITE_HINT =
	'writes need JUMPWAY_E2E_WRITE=1 and a loopback JUMPWAY_URL (or JUMPWAY_E2E_ISOLATED=1)';

interface Failures {
	entries: string[];
}

// Every test also fails on console errors, page errors, failed requests and 4xx/5xx answers.
const test = base.extend<{ failures: Failures }>({
	failures: [
		async ({ page }, use) => {
			const failures: Failures = { entries: [] };
			page.on('console', (message) => {
				if (message.type() === 'error')
					failures.entries.push(`console: ${message.text()} ${message.location().url}`);
			});
			page.on('pageerror', (error) => failures.entries.push('pageerror: ' + error.message));
			page.on('requestfailed', (request) =>
				failures.entries.push('requestfailed: ' + request.url())
			);
			page.on('response', (response) => {
				if (response.status() >= 400)
					failures.entries.push(`http ${response.status()}: ${response.url()}`);
			});
			await use(failures);
			expect(failures.entries).toEqual([]);
		},
		{ auto: true }
	]
});

test.skip(!backend, 'JUMPWAY_URL is not set');

function isLoopback(origin: string): boolean {
	try {
		const host = new URL(origin).hostname.replace(/^\[|\]$/g, '');
		return host === 'localhost' || host === '::1' || /^127(\.\d{1,3}){3}$/.test(host);
	} catch {
		return false;
	}
}

const unique = (label: string) =>
	`e2e-real-${label}-${Date.now().toString(36)}-${randomBytes(3).toString('hex')}`;
const rulePath = (name: string) => '/apis/configs/rules/' + encodeURIComponent(name);
const heading = (page: Page) => page.getByRole('heading', { level: 1 });
const kpi = (page: Page, name: string) => page.locator(`[data-kpi="${name}"]`);
const toast = (page: Page, text: string) => page.getByRole('status').filter({ hasText: text });
const unsaved = (page: Page) => page.getByRole('main').getByText('Unsaved changes');
const tooltip = (page: Page) => page.locator('#tooltip');
const cardOf = (page: Page, name: string) =>
	page.getByRole('article').filter({ has: page.getByRole('link', { name, exact: true }) });
const statsRowOf = (page: Page, name: string) =>
	page.locator('article[data-rule]').filter({ has: page.getByRole('link', { name, exact: true }) });

// Tooltips hide on scroll; scrolling the target into view first keeps the hover's tooltip.
async function hover(page: Page, locator: Locator) {
	await locator.scrollIntoViewIfNeeded();
	await page.waitForTimeout(100);
	await locator.hover();
}

async function ok(response: APIResponse): Promise<APIResponse> {
	const detail = `${response.url()} → ${response.status()} ${await response.text()}`;
	expect(response.ok(), detail).toBe(true);
	return response;
}

const body = async <T>(response: APIResponse): Promise<T> =>
	(await (await ok(response)).json()) as T;
const statusOf = async (request: APIRequestContext) =>
	body<Status>(await request.get('/apis/configs/status'));
const rulesOf = async (request: APIRequestContext) =>
	(await body<Rule[] | null>(await request.get('/apis/configs/rules'))) ?? [];
const rawOf = async (request: APIRequestContext) =>
	(await body<RawConfig>(await request.get('/apis/configs/raw'))).yaml;
async function statsOf(request: APIRequestContext, name: string): Promise<RuleStats | undefined> {
	const snapshot = await body<Snapshot>(await request.get('/apis/stats'));
	return (snapshot.rules ?? []).find((rule) => rule.name === name);
}

// Port 0 rules get an ephemeral port; status reports it once the runtime listens.
async function boundAddress(request: APIRequestContext, name: string): Promise<string> {
	let address = '';
	await expect
		.poll(
			async () => {
				const rule = (await statusOf(request)).rules?.find((entry) => entry.name === name);
				address = rule?.address ?? '';
				return rule?.running === true && /^127\.0\.0\.1:[1-9]\d*$/.test(address);
			},
			{ timeout: 10_000, message: `${name} never reported a bound loopback address` }
		)
		.toBe(true);
	return address;
}

// The Go handler answers a missing rule with 400 and exactly this text.
const notFound = (status: number, answer: string, name: string) =>
	status === 404 || (status === 400 && answer.trim() === `rule ${JSON.stringify(name)} not found`);

// Deletes the test's own rules. Every name is attempted, and a rule only counts as gone when a GET
// afterwards says so; problems are returned, not thrown, so they can be reported next to the
// failure that interrupted the test.
async function removeRules(request: APIRequestContext, names: string[]): Promise<string[]> {
	const problems: string[] = [];
	for (const name of names) {
		try {
			const deleted = await request.delete(rulePath(name));
			const answer = await deleted.text();
			if (!deleted.ok() && !notFound(deleted.status(), answer, name))
				problems.push(`DELETE ${name} → ${deleted.status()} ${answer}`);
			const got = await request.get(rulePath(name));
			const rule = await got.text();
			if (!notFound(got.status(), rule, name))
				problems.push(`GET ${name} → ${got.status()} ${rule} (still present)`);
		} catch (error) {
			problems.push(`${name}: ${error instanceof Error ? error.message : String(error)}`);
		}
	}
	return problems;
}

async function shoot(page: Page, name: string) {
	const path = test.info().outputPath(name + '.png');
	await page.screenshot({ path, fullPage: true, animations: 'disabled' });
	await test.info().attach(name, { path, contentType: 'image/png' });
}

interface Tunnel {
	socket: Socket;
	received: () => number;
	closed: Promise<void>;
}

// HTTP CONNECT through the rule's proxy port, then GET path; resolves once the target's response
// headers arrived with a 2xx status, after which received() counts body bytes only (chunk framing
// included). Any fault before that — refused CONNECT, non-2xx answer, error, EOF or silence —
// rejects and destroys the socket; afterwards the socket's own errors are swallowed and `closed`
// settles on any close.
function openTunnel(
	proxyPort: number,
	targetAddress: string,
	path: string,
	handshakeMs = 10_000
): Promise<Tunnel> {
	return new Promise((resolve, reject) => {
		const socket = connect({ host: '127.0.0.1', port: proxyPort });
		const closed = new Promise<void>((done) => socket.once('close', () => done()));
		let phase: 'connect' | 'response' | 'body' = 'connect';
		let head = '';
		let received = 0;
		let settled = false;
		const onEnd = () => fail(new Error(`EOF while waiting for the ${phase} answer`));
		const onClose = () => fail(new Error(`closed while waiting for the ${phase} answer`));
		const timer = setTimeout(
			() => fail(new Error(`no ${phase} answer within ${handshakeMs} ms`)),
			handshakeMs
		);
		function settle() {
			settled = true;
			clearTimeout(timer);
			socket.off('end', onEnd);
			socket.off('close', onClose);
		}
		function fail(error: Error) {
			if (settled) return;
			settle();
			socket.destroy();
			reject(error);
		}
		socket.on('error', fail);
		socket.on('end', onEnd);
		socket.on('close', onClose);
		socket.once('connect', () => {
			socket.write(`CONNECT ${targetAddress} HTTP/1.1\r\nHost: ${targetAddress}\r\n\r\n`);
		});
		socket.on('data', (chunk: Buffer) => {
			if (phase === 'body') {
				received += chunk.length;
				return;
			}
			head += chunk.toString('latin1');
			while (phase !== 'body') {
				const end = head.indexOf('\r\n\r\n');
				if (end < 0) return;
				const statusLine = head.slice(0, head.indexOf('\r\n'));
				head = head.slice(end + 4);
				if (phase === 'connect') {
					if (!/^HTTP\/1\.[01] 200\b/.test(statusLine)) {
						fail(new Error('CONNECT refused: ' + statusLine));
						return;
					}
					phase = 'response';
					socket.write(`GET ${path} HTTP/1.1\r\nHost: ${targetAddress}\r\n\r\n`);
					continue;
				}
				if (!/^HTTP\/1\.[01] 2\d\d\b/.test(statusLine)) {
					fail(new Error(`GET ${path} answered: ${statusLine}`));
					return;
				}
				phase = 'body';
				received = head.length;
				head = '';
				settle();
				resolve({ socket, received: () => received, closed });
			}
		});
	});
}

async function settles(promise: Promise<void>, ms: number): Promise<boolean> {
	let timer: ReturnType<typeof setTimeout> | undefined;
	const expired = new Promise<boolean>((done) => {
		timer = setTimeout(() => done(false), ms);
	});
	try {
		return await Promise.race([promise.then(() => true), expired]);
	} finally {
		clearTimeout(timer);
	}
}

test('overview, sidebar status and resource links reflect the live backend', async ({
	page,
	request
}) => {
	const status = await statusOf(request);
	const rules = await rulesOf(request);
	await page.goto('/');
	await expect(heading(page)).toHaveText('Overview');
	await expect(page).toHaveTitle('Overview · JumpWay');
	const aside = page.locator('aside');
	await expect(aside.locator('[data-state]').first()).toHaveText('Running');
	await expect(aside).toContainText(status.address);
	const running = (status.rules ?? []).filter((rule) => rule.running).length;
	await expect(kpi(page, 'rules')).toHaveText(`${running} Running`);
	await expect(kpi(page, 'rule-states')).toContainText(`${rules.length} configured`);
	await expect(page.getByRole('article')).toHaveCount(rules.length);

	const links = await aside
		.locator('a[href^="/"]')
		.evaluateAll((anchors) => anchors.map((anchor) => anchor.getAttribute('href')!));
	expect(links).toEqual(expect.arrayContaining(['/metrics', '/swaggerui/', '/debug/pprof/']));
	for (const href of links) await ok(await request.get(href));
	const exposition = await (await request.get('/metrics')).text();
	expect(exposition).toMatch(/^# TYPE jumpway_\w+ /m);
});

test('#/new creates a rule the runtime binds; renaming follows the encoded route; overview and statistics list it', async ({
	page,
	request
}) => {
	test.skip(!writable, WRITE_HINT);
	const name = unique('rule');
	const renamed = name + ' renamed';
	try {
		await page.goto('/#/new');
		await expect(heading(page)).toHaveText('New rule');
		await expect(page.getByLabel('Host', { exact: true })).toHaveValue('127.0.0.1');
		await expect(page.getByLabel('Port', { exact: true })).toHaveValue('0');
		await page.getByLabel('Rule name').fill(name);
		await page.getByRole('button', { name: 'Save & Apply' }).click();
		await expect(page).toHaveURL(new RegExp('#/rules/' + encodeURIComponent(name) + '$'));
		await expect(toast(page, 'Saved and applied.')).toBeVisible();
		await expect(unsaved(page)).toHaveCount(0);
		expect(await body<Rule>(await request.get(rulePath(name)))).toEqual({
			name,
			listen: { host: '127.0.0.1', port: 0 },
			forward: {}
		});
		await boundAddress(request, name);

		// The overview switch stops the rule: only `disabled` is added, and the runtime leaves the
		// status entirely; switching back drops it again and binds a fresh ephemeral port.
		await page.goto('/');
		const created = cardOf(page, name);
		const toggle = created.getByRole('switch', { name: 'Enabled' });
		await expect(toggle).toBeChecked();
		await expect(created.locator('[data-state]')).toHaveText('Running');
		await toggle.click();
		await expect(toggle).not.toBeChecked();
		await expect(toggle).toBeEnabled();
		await expect(created.locator('[data-state]')).toHaveText('Disabled');
		await expect(toast(page, 'Saved and applied.').last()).toBeVisible();
		expect(await body<Rule>(await request.get(rulePath(name)))).toEqual({
			name,
			disabled: true,
			listen: { host: '127.0.0.1', port: 0 },
			forward: {}
		});
		expect((await statusOf(request)).rules?.find((rule) => rule.name === name)).toBeUndefined();
		await toggle.click();
		await expect(toggle).toBeChecked();
		await expect(toggle).toBeEnabled();
		expect(await body<Rule>(await request.get(rulePath(name)))).toEqual({
			name,
			listen: { host: '127.0.0.1', port: 0 },
			forward: {}
		});
		const rebound = await boundAddress(request, name);
		await expect(created.locator('[data-state]')).toHaveText('Running');
		await expect(created).toContainText(rebound);

		await page.goto('/#/rules/' + encodeURIComponent(name));
		await expect(heading(page)).toHaveText(name);
		await page.getByLabel('Rule name').fill(renamed);
		await page.keyboard.press('ControlOrMeta+s');
		await expect(page).toHaveURL(new RegExp('#/rules/' + encodeURIComponent(renamed) + '$'));
		await expect(heading(page)).toHaveText(renamed);
		// The create toast may still be on screen.
		await expect(toast(page, 'Saved and applied.').last()).toBeVisible();
		expect((await request.get(rulePath(name))).status()).toBe(400);
		expect((await body<Rule>(await request.get(rulePath(renamed)))).name).toBe(renamed);
		// The renamed rule is a new runtime, so it may sit on another ephemeral port.
		const address = await boundAddress(request, renamed);

		await page.goto('/');
		const card = cardOf(page, renamed);
		await expect(card.locator('[data-state]')).toHaveText('Running');
		await expect(card).toContainText(address);
		await expect(card).toContainText('Proxy');

		await page.goto('/#/stats');
		const row = statsRowOf(page, renamed);
		await expect(row.locator('[data-state]')).toHaveText('Running');
		await expect(row.locator('[data-address]')).toHaveText(address);
		// The collapsed item carries the counters; the expansion holds the chain.
		await expect(row.locator('[data-connections] dd')).toHaveText('0 / 0');
		await expect(row.locator('[data-metric="down"]')).toHaveText('0 B');
		await row.locator('button[aria-controls]').click();
		await expect(
			page
				.locator('#' + (await row.locator('button[aria-controls]').getAttribute('aria-controls')))
				.getByRole('list', { name: 'Chain' })
		).toBeVisible();
	} finally {
		expect.soft(await removeRules(request, [name, renamed]), 'cleanup').toEqual([]);
	}
});

test('the YAML page writes the file, re-reads it from disk, and the original is restored', async ({
	page,
	request
}) => {
	test.skip(!writable || !isolated, 'the YAML round-trip needs JUMPWAY_E2E_ISOLATED=1 as well');
	const original = await rawOf(request);
	const marker = (original.endsWith('\n') || !original ? '' : '\n') + `# ${unique('yaml')}\n`;
	const edited = original + marker;
	const yamlForm = page.getByRole('form', { name: 'Advanced YAML' });
	const source = page.getByLabel('Configuration YAML', { exact: true });
	try {
		await page.goto('/#/yaml');
		await expect(source).toHaveValue(original);
		await source.fill(edited);
		await expect(unsaved(page)).toBeVisible();
		await yamlForm.getByRole('button', { name: 'Save & Apply' }).click();
		await expect(toast(page, 'Saved and applied.')).toBeVisible();
		await expect(unsaved(page)).toHaveCount(0);
		expect(await rawOf(request)).toBe(edited);

		await yamlForm.getByRole('button', { name: 'Reload from disk' }).click();
		await expect(toast(page, 'Reloaded from disk.')).toBeVisible();
		await expect(source).toHaveValue(edited);
	} finally {
		await ok(await request.put('/apis/configs/raw', { data: { yaml: original } }));
	}
	expect(await rawOf(request)).toBe(original);
});

test('live traffic through a proxy rule reaches the API and the connections page; Disconnect closes the socket', async ({
	page,
	request
}) => {
	test.skip(!writable, WRITE_HINT);
	test.skip(!target, 'JUMPWAY_TARGET_URL is not set');
	test.setTimeout(60_000);
	const name = unique('traffic');
	const targetAddress = new URL(target).host;
	let tunnel: Tunnel | undefined;
	try {
		await ok(
			await request.post('/apis/configs/rules', {
				data: { name, listen: { host: '127.0.0.1', port: 0 }, forward: {} }
			})
		);
		const address = await boundAddress(request, name);
		tunnel = await openTunnel(Number(address.split(':').pop()), targetAddress, '/stream');
		const opened = tunnel;
		await expect
			.poll(() => opened.received(), { timeout: 10_000, message: 'no body bytes arrived' })
			.toBeGreaterThan(0);
		const firstSample = opened.received();
		await expect
			.poll(() => opened.received(), { timeout: 10_000, message: 'the body stopped growing' })
			.toBeGreaterThan(firstSample);

		// A current rate, not just a peak: the stream must still be flowing when the API is asked.
		const flowing = (counters: Stats | undefined) =>
			Boolean(counters && counters.active >= 1 && counters.down > 0 && counters.rate_down > 0);
		let stats: RuleStats | undefined;
		await expect
			.poll(
				async () => {
					stats = await statsOf(request, name);
					return flowing(stats?.stats);
				},
				{ timeout: 15_000, message: 'the tunnel never showed up in /apis/stats as flowing' }
			)
			.toBe(true);
		const downloaded = stats!.stats.down;
		await expect
			.poll(
				async () => {
					stats = await statsOf(request, name);
					return flowing(stats?.stats) ? stats!.stats.down : 0;
				},
				{ timeout: 10_000, message: 'downloaded bytes in /apis/stats stopped growing' }
			)
			.toBeGreaterThan(downloaded);
		const [connection] = stats!.connections ?? [];
		expect(connection.target).toBe(targetAddress);
		expect(connection.client).toMatch(/^127\.0\.0\.1:\d+$/);
		expect(connection.via).toBe('');
		expect(connection.path ?? []).toEqual([]);

		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/');
		const card = cardOf(page, name);
		await expect(card.locator('[data-state]')).toHaveText('Running');
		await expect(card).toContainText(address);
		await expect(kpi(page, 'active')).not.toHaveText('0');
		await shoot(page, 'real-overview-desktop');

		await page.setViewportSize({ width: 375, height: 812 });
		await page.goto('/#/connections?rule=' + encodeURIComponent(name));
		await expect(heading(page)).toHaveText('Live Connections');
		await expect(page.getByRole('combobox', { name: 'Rule' })).toHaveValue(name);
		// A card at this width; the same data attributes name the table row from 1100px.
		const rows = page.locator('[data-connection]');
		await expect(rows).toHaveCount(1);
		await expect(rows.locator('[data-rule-link]')).toHaveText(name);
		await expect(rows.locator('[data-client]')).toContainText('127.0.0.1');
		await expect(rows.locator('[data-target]')).toHaveText(targetAddress);
		await expect(rows.locator('[data-metric="down"]')).not.toHaveText(/^(0 B|—)$/);
		await shoot(page, 'real-connections-mobile');
		await hover(page, rows.getByRole('button', { name: 'Path' }));
		await expect(tooltip(page)).toHaveText('direct');

		await rows.getByRole('button', { name: 'Disconnect' }).click();
		await expect(toast(page, 'Connection closed.')).toBeVisible();
		expect(await settles(opened.closed, 10_000), 'tunnel socket closed').toBe(true);
		await expect(rows).toHaveCount(0);
		await expect
			.poll(async () => (await statsOf(request, name))?.stats.active, { timeout: 10_000 })
			.toBe(0);
	} finally {
		tunnel?.socket.destroy();
		expect.soft(await removeRules(request, [name]), 'cleanup').toEqual([]);
	}
});
