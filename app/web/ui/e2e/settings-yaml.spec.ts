import type { Page } from '@playwright/test';
import { noProxyFixture, rawFixture } from './fixtures/api.ts';
import { expect, test } from './fixtures/test.ts';
import type { MockApi } from './mockApi.ts';

// Settings (web UI address + no-proxy, saved independently) and Advanced YAML in real Chrome
// against the stateful mock. The preview serves the page from 127.0.0.1:4173, so saving that
// address is not a move while any other port is.

const heading = (page: Page) => page.getByRole('heading', { level: 1 });
const addressForm = (page: Page) => page.getByRole('form', { name: /Web UI|网页配置/ });
const bypassForm = (page: Page) => page.getByRole('form', { name: /No proxy|不走代理/ });
const yamlForm = (page: Page) => page.getByRole('form', { name: 'Advanced YAML' });
const unsaved = (root: ReturnType<Page['getByRole']>) =>
	root.getByRole('status').filter({ hasText: /Unsaved changes|未保存的修改/ });
const notice = (page: Page) => page.getByRole('main').getByRole('alert');
const toast = (page: Page, text: string) => page.getByRole('status').filter({ hasText: text });
const warning = (page: Page, text: string) =>
	page.locator('[aria-label="Notifications"]').getByRole('alert').filter({ hasText: text });
const confirmDialog = (page: Page) =>
	page.locator('dialog[open][aria-describedby="confirm-message"]');
const writes = (api: MockApi) => api.writes.map((write) => `${write.method} ${write.path}`);
// Chrome logs its own console line for a 400 the test expects; refused or reset requests are
// reported too.
const expectedFailure = (entry: string) =>
	entry.startsWith('http 400') ||
	entry.includes('status of 400') ||
	entry.startsWith('requestfailed:') ||
	entry.includes('ERR_CONNECTION_REFUSED') ||
	entry.includes('ERR_CONNECTION_RESET');

test('save areas share their button style and only the writing form shows a spinner', async ({
	page,
	api
}) => {
	for (const width of [1280, 375]) {
		await page.setViewportSize({ width, height: 812 });
		let reference: unknown;
		for (const [route, count, sticky] of [
			['rules/office', 1, true],
			['yaml', 1, true],
			['settings', 2, false]
		] as const) {
			await page.goto(`/?width=${width}#/${route}`);
			const bars = page.locator('main [data-form-actions]');
			await expect(bars).toHaveCount(count);
			for (const bar of await bars.all()) {
				const save = bar.getByRole('button', { name: 'Save & Apply', exact: true });
				await save.scrollIntoViewIfNeeded();
				await expect(save).toBeVisible();
				await expect(save).toHaveAttribute('type', 'submit');
				await expect(save.locator('svg')).toHaveCount(1);
				const appearance = await save.evaluate((button) => {
					const style = getComputedStyle(button);
					return {
						height: button.getBoundingClientRect().height,
						background: style.backgroundColor,
						color: style.color,
						font: style.font,
						padding: style.padding,
						border: style.border,
						gap: style.gap
					};
				});
				if (!reference) reference = appearance;
				expect(appearance).toEqual(reference);
				expect(await bar.evaluate((element) => getComputedStyle(element).position)).toBe(
					sticky ? 'sticky' : 'static'
				);
				expect(
					await bar
						.locator('button:not([type="submit"])')
						.evaluateAll((buttons) =>
							buttons.every((button) => button.getAttribute('type') === 'button')
						)
				).toBe(true);
			}
			expect(
				await page.locator('main').evaluate((element) => element.scrollWidth <= element.clientWidth)
			).toBe(true);
		}
	}

	await bypassForm(page).getByLabel('Hosts / CIDRs').fill('example.test');
	let release!: () => void;
	api.delay.set(
		'PUT /apis/configs/no-proxy',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	const save = bypassForm(page).getByRole('button', { name: 'Save & Apply', exact: true });
	const before = (await save.boundingBox())!.width;
	await save.click();
	await expect(save).toBeDisabled();
	await expect(save).toHaveAttribute('aria-busy', 'true');
	await expect(save.locator('[data-spinner]')).toHaveCount(1);
	await expect(
		addressForm(page).getByRole('button', { name: 'Save & Apply', exact: true })
	).toBeDisabled();
	await expect(addressForm(page).locator('[data-spinner]')).toHaveCount(0);
	expect((await save.boundingBox())!.width).toBe(before);
	release();
	await expect(save).toBeEnabled();
	await expect(save.locator('[data-spinner]')).toHaveCount(0);
	expect(writes(api)).toEqual(['PUT /apis/configs/no-proxy']);
});

test('sticky save areas end at the scroller edge and hide whatever scrolls beneath them', async ({
	page
}) => {
	// The page-enter animation still moves the form for 180 ms after load; reduced motion removes it.
	await page.emulateMedia({ reducedMotion: 'reduce' });
	const scrollTo = async (fraction: number) => {
		await page.evaluate((part) => {
			const main = document.getElementById('main')!;
			main.scrollTop = Math.round((main.scrollHeight - main.clientHeight) * part);
		}, fraction);
		await page.evaluate(() => new Promise(requestAnimationFrame));
	};
	await page.goto('/');
	for (const theme of ['light', 'dark'] as const) {
		await page.evaluate((value) => localStorage.setItem('jumpway.theme', value), theme);
		for (const viewport of [
			{ width: 1280, height: 800 },
			{ width: 375, height: 812 }
		]) {
			await page.setViewportSize(viewport);
			for (const route of ['rules/office', 'yaml'] as const) {
				const label = `${route} ${viewport.width}x${viewport.height} ${theme}`;
				await page.goto(`/?t=${theme}&w=${viewport.width}&r=${route}#/${route}`);
				await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
				const bar = page.locator('main [data-form-actions]');
				await expect(bar.getByRole('button', { name: 'Save & Apply', exact: true })).toBeVisible();
				// A YAML editor dragged taller than the viewport makes that page scroll as well.
				if (route === 'yaml')
					await page.locator('#yaml-source').evaluate((textarea) => {
						textarea.style.height = '1600px';
					});
				await scrollTo(0.5);
				const geometry = await page.evaluate(() => {
					const main = document.getElementById('main')!;
					const bar = document.querySelector('main [data-form-actions]')!;
					const m = main.getBoundingClientRect();
					const b = bar.getBoundingClientRect();
					const xs = [b.left + 2, (b.left + b.right) / 2, b.right - 2];
					const uncovered: string[] = [];
					for (let y = Math.ceil(b.top) + 1; y < Math.floor(m.bottom); y += 2) {
						for (const x of xs) {
							const hit = document.elementFromPoint(x, y);
							if (!hit?.closest('[data-form-actions]'))
								uncovered.push(`${Math.round(x)},${y}:${hit?.tagName.toLowerCase()}`);
						}
					}
					// Whole pixels inside the bar's edges, so no row straddles its border and the content above.
					const top = Math.ceil(b.top) + 1;
					const left = Math.ceil(b.left) + 1;
					return {
						scrollTop: main.scrollTop,
						scrollable: main.scrollHeight > main.clientHeight,
						barBottom: b.bottom,
						mainBottom: m.bottom,
						background: getComputedStyle(bar).backgroundColor,
						clip: {
							x: left,
							y: top,
							width: Math.floor(b.right) - left - 1,
							height: Math.floor(m.bottom) - top
						},
						uncovered: uncovered.slice(0, 6),
						uncoveredCount: uncovered.length
					};
				});
				expect(geometry.scrollable, label).toBe(true);
				expect(geometry.scrollTop, label).toBeGreaterThan(0);
				expect(geometry.barBottom, label).toBeCloseTo(geometry.mainBottom, 0);
				expect(geometry.uncovered, `${label}: ${geometry.uncoveredCount} points`).toEqual([]);
				expect(geometry.background, label).toMatch(/^rgb\(/);

				// Pixels from the bar's top down to the scroller's edge must not change when everything
				// underneath is painted magenta; the strip right above the bar proves the paint landed.
				const control = { ...geometry.clip, y: geometry.clip.y - 14, height: 10 };
				const plain = await page.screenshot({ clip: geometry.clip, animations: 'disabled' });
				const plainControl = await page.screenshot({ clip: control, animations: 'disabled' });
				const sentinel = await page.addStyleTag({
					content:
						'main form > :not([data-form-actions]), main form > :not([data-form-actions]) * { background: #ff00ff !important; color: #ff00ff !important; border-color: #ff00ff !important; }'
				});
				const painted = await page.screenshot({ clip: geometry.clip, animations: 'disabled' });
				const paintedControl = await page.screenshot({ clip: control, animations: 'disabled' });
				await sentinel.evaluate((node) => node.parentNode?.removeChild(node));
				expect(plainControl.equals(paintedControl), `${label}: sentinel not painted`).toBe(false);
				expect(plain.equals(painted), `${label}: content shows through the save area`).toBe(true);
				const shot = test
					.info()
					.outputPath(`${route.replace(/\W+/g, '-')}-${viewport.width}-${theme}.png`);
				await page.screenshot({ path: shot, animations: 'disabled' });

				// At the end of the page the bar sits in normal flow: nothing under it, padding kept.
				await scrollTo(1);
				const tail = await page.evaluate(() => {
					const main = document.getElementById('main')!;
					const bar = document.querySelector('main [data-form-actions]')!;
					const b = bar.getBoundingClientRect();
					const previous = bar.previousElementSibling!.getBoundingClientRect();
					return {
						gap: main.getBoundingClientRect().bottom - b.bottom,
						overlap: previous.bottom - b.top
					};
				});
				expect(tail.overlap, label).toBeLessThan(1);
				expect(tail.gap, label).toBeGreaterThanOrEqual(16);
			}
		}
	}
});

test('#/settings: each form keeps its own draft; no-proxy lines are cleaned; a bad port never reaches the API', async ({
	page,
	api
}) => {
	await page.goto('/#/settings');
	await expect(heading(page)).toHaveText('Global Settings');
	await expect(page).toHaveTitle('Global Settings · JumpWay');
	const host = addressForm(page).getByLabel('Host');
	const port = addressForm(page).getByLabel('Port');
	const list = bypassForm(page).getByLabel('Hosts / CIDRs');
	await expect(host).toHaveValue('127.0.0.1');
	await expect(port).toHaveValue('1088');
	await expect(list).toHaveValue(noProxyFixture.list!.join('\n'));
	await expect(bypassForm(page).getByLabel('From environment variables')).toHaveValue(
		'NO_PROXY\nno_proxy'
	);
	await expect(bypassForm(page).getByLabel('From files or URLs')).toHaveValue('');
	await expect(unsaved(addressForm(page))).toHaveCount(0);
	await expect(unsaved(bypassForm(page))).toHaveCount(0);

	await port.fill('70000');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(port).toBeFocused();
	await expect(
		addressForm(page).getByText('Port must be a whole number between 0 and 65535.')
	).toBeVisible();
	expect(api.writes).toEqual([]);

	// The preview's own port: a save that is not a move.
	await port.fill('4173');
	await expect(unsaved(addressForm(page))).toBeVisible();
	await expect(unsaved(bypassForm(page))).toHaveCount(0);
	await list.fill(' example.test \n\n10.0.0.0/8\n');
	await expect(unsaved(bypassForm(page))).toBeVisible();

	await bypassForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(unsaved(bypassForm(page))).toHaveCount(0);
	expect(api.writes).toEqual([
		{
			method: 'PUT',
			path: '/apis/configs/no-proxy',
			body: {
				list: ['example.test', '10.0.0.0/8'],
				from_env: ['NO_PROXY', 'no_proxy'],
				from_file: []
			}
		}
	]);
	await expect(toast(page, 'Saved and applied.')).toBeVisible();
	await expect(unsaved(addressForm(page))).toBeVisible();
	await expect(port).toHaveValue('4173');
	await expect(list).toHaveValue(' example.test \n\n10.0.0.0/8\n');

	await port.focus();
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(addressForm(page))).toHaveCount(0);
	expect(writes(api)).toEqual(['PUT /apis/configs/no-proxy', 'PUT /apis/configs/web-ui']);
	expect(api.writes[1].body).toEqual({ host: '127.0.0.1', port: 4173 });
	await expect(notice(page)).toHaveCount(0);
	expect(api.requests.filter((entry) => entry === 'GET /apis/configs/web-ui')).toHaveLength(1);
});

test('a new port shows a persistent moved link with ?lang= and the hash; the closed listener does not replace it', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	api.stopOnMove = true;
	await page.goto('/?lang=zh#/settings');
	await expect(heading(page)).toHaveText('全局设置');
	await addressForm(page).getByLabel('端口').fill('1099');
	await addressForm(page).getByRole('button', { name: '保存并应用' }).click();
	await expect(notice(page)).toContainText('网页配置已移至');
	const link = notice(page).getByRole('link', { name: '127.0.0.1:1099' });
	await expect(link).toHaveAttribute('href', 'http://127.0.0.1:1099/?lang=zh#/settings');
	await expect(unsaved(addressForm(page))).toHaveCount(0);
	await expect(toast(page, '已保存并应用。')).toBeVisible();
	expect(api.writes).toEqual([
		{ method: 'PUT', path: '/apis/configs/web-ui', body: { host: '127.0.0.1', port: 1099 } }
	]);
	// The status refresh after the write was refused, yet no generic banner replaces the notice.
	expect(
		api.requests.filter((entry) => entry === 'GET /apis/configs/status').length
	).toBeGreaterThanOrEqual(2);
	await expect(page.getByRole('main')).not.toContainText('无法连接到 JumpWay。');
	await expect(page.locator('aside [data-state="unknown"]')).toBeVisible();

	await page
		.getByRole('navigation', { name: '导航' })
		.getByRole('link', { name: '配置文件' })
		.click();
	await expect(heading(page)).toHaveText('配置文件');
	await expect(link).toHaveAttribute('href', 'http://127.0.0.1:1099/?lang=zh#/settings');
});

test('port 0 links to the address the status reports', async ({ page, api }) => {
	await page.goto('/#/settings');
	await addressForm(page).getByLabel('Port').fill('0');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(notice(page)).toContainText('The web UI has moved to');
	await expect(notice(page).getByRole('link', { name: '127.0.0.1:43210' })).toHaveAttribute(
		'href',
		'http://127.0.0.1:43210/#/settings'
	);
	expect(api.status.address).toBe('127.0.0.1:43210');
});

test('port 0 with the old listener gone says only that the address changed', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	api.stopOnMove = true;
	await page.goto('/#/settings');
	await addressForm(page).getByLabel('Port').fill('0');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(toast(page, 'Saved and applied.')).toBeVisible();
	await expect(notice(page)).toHaveText(
		'Saved. The listen address changed. Open the address shown in the tray status item.'
	);
	await expect(notice(page).getByRole('link')).toHaveCount(0);
	await expect(page.getByRole('main')).not.toContainText('Cannot reach JumpWay.');
});

test('a port whose bind fails is saved but not moved: the candidate stays unconfirmed until the status reports it running', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	api.bindFails = true;
	await page.goto('/#/settings');
	await addressForm(page).getByLabel('Port').fill('1099');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(
		warning(page, 'saved, but web_ui: listen tcp 127.0.0.1:1099: bind: address already in use')
	).toBeVisible();
	await expect(notice(page)).toContainText('Saved, but the web UI is not confirmed running.');
	await expect(notice(page)).toContainText('Not confirmed: 127.0.0.1:1099');
	await expect(notice(page)).not.toContainText('has moved to');
	await expect(notice(page).getByRole('link', { name: '127.0.0.1:1099' })).toHaveAttribute(
		'href',
		'http://127.0.0.1:1099/#/settings'
	);
	await expect(unsaved(addressForm(page))).toHaveCount(0);
	await expect(toast(page, 'Saved and applied.')).toHaveCount(0);
	await expect(page.getByRole('main')).not.toContainText('Cannot reach JumpWay.');
	expect(api.status).toMatchObject({ address: '127.0.0.1:1099', running: false });

	// Fixed from the tray: the next poll reports the web UI running there.
	api.status.running = true;
	await expect(notice(page)).toContainText('The web UI has moved to', { timeout: 15_000 });
	await expect(notice(page)).not.toContainText('Not confirmed');
	await expect(notice(page).getByRole('link', { name: '127.0.0.1:1099' })).toHaveAttribute(
		'href',
		'http://127.0.0.1:1099/#/settings'
	);
});

test('no-proxy: a 400 keeps the draft; "saved, but" persists with a warning; a refused write is never reported saved', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	await page.goto('/#/settings');
	const list = bypassForm(page).getByLabel('Hosts / CIDRs');
	await expect(list).toHaveValue(noProxyFixture.list!.join('\n'));
	api.fail.set('PUT /apis/configs/no-proxy', 'no_proxy.list[0] "bad host" is not a host or CIDR');
	await list.fill('bad host');
	await bypassForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(bypassForm(page).getByRole('alert')).toContainText(
		'"bad host" is not a host or CIDR'
	);
	await expect(unsaved(bypassForm(page))).toBeVisible();
	await expect(list).toHaveValue('bad host');
	await expect(toast(page, 'Saved and applied.')).toHaveCount(0);

	api.fail.set('PUT /apis/configs/no-proxy', 'saved, but reload failed: dns lookup timed out');
	await bypassForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(unsaved(bypassForm(page))).toHaveCount(0);
	await expect(bypassForm(page).getByRole('alert')).toHaveCount(0);
	await expect(warning(page, 'saved, but reload failed: dns lookup timed out')).toBeVisible();
	expect(api.noProxy.list).toEqual(['bad host']);

	api.fail.clear();
	api.down.add('/apis/configs/no-proxy');
	await bypassForm(page).getByLabel('From files or URLs').fill('/etc/no_proxy.txt');
	await bypassForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(bypassForm(page).getByRole('alert')).toContainText(
		'The request did not complete. The change may or may not have been applied.'
	);
	await expect(unsaved(bypassForm(page))).toBeVisible();
	await expect(toast(page, 'Saved and applied.')).toHaveCount(0);
	// The refused attempt reached the mock but never became a write.
	expect(api.requests.filter((entry) => entry === 'PUT /apis/configs/no-proxy')).toHaveLength(3);
	expect(writes(api)).toHaveLength(2);
});

test('a refused address write keeps the draft and offers the candidate link; the retry lands', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	await page.goto('/#/settings');
	const port = addressForm(page).getByLabel('Port');
	await expect(port).toHaveValue('1088');
	api.down.add('/apis/configs/web-ui');
	await port.fill('1099');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(notice(page)).toContainText(
		'The request did not complete. The change may or may not have been applied.'
	);
	await expect(notice(page)).toContainText('If it was applied, the web UI is now at');
	await expect(notice(page).getByRole('link', { name: '127.0.0.1:1099' })).toHaveAttribute(
		'href',
		'http://127.0.0.1:1099/#/settings'
	);
	await expect(unsaved(addressForm(page))).toBeVisible();
	await expect(port).toHaveValue('1099');
	await expect(toast(page, 'Saved and applied.')).toHaveCount(0);

	api.down.clear();
	await page.keyboard.press('ControlOrMeta+s');
	await expect(notice(page)).toContainText('The web UI has moved to');
	await expect(unsaved(addressForm(page))).toHaveCount(0);
	await expect(toast(page, 'Saved and applied.')).toBeVisible();
	expect(api.requests.filter((entry) => entry === 'PUT /apis/configs/web-ui')).toHaveLength(2);
	expect(writes(api)).toEqual(['PUT /apis/configs/web-ui']);
});

test('a status answer requested before the write cannot dismiss the candidate link, even when it arrives after the lost answer', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	let release!: () => void;
	api.delay.set('GET /apis/configs/status', new Promise<void>((resolve) => (release = resolve)));
	await page.goto('/#/settings');
	const port = addressForm(page).getByLabel('Port');
	await expect(port).toHaveValue('1088');
	await expect(page.locator('aside [data-state="unknown"]')).toBeVisible();
	expect(api.requests.filter((entry) => entry === 'GET /apis/configs/status')).toHaveLength(1);

	// The write lands and closes the old listener, but its answer is lost on the way back.
	api.stopOnMove = true;
	api.lost.add('/apis/configs/web-ui');
	await port.fill('1099');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	const link = notice(page).getByRole('link', { name: '127.0.0.1:1099' });
	await expect(notice(page)).toContainText('If it was applied, the web UI is now at');
	await expect(link).toHaveAttribute('href', 'http://127.0.0.1:1099/#/settings');
	await expect(unsaved(addressForm(page))).toBeVisible();
	expect(api.webUI).toEqual({ host: '127.0.0.1', port: 1099 });

	// The poll that started before the save answers now: it only describes the old listener.
	const answered = page.waitForResponse('**/apis/configs/status');
	release();
	await answered;
	await expect(page.locator('aside [data-state="running"]')).toBeVisible();
	await expect(notice(page)).toContainText('If it was applied, the web UI is now at');
	await expect(link).toHaveAttribute('href', 'http://127.0.0.1:1099/#/settings');
	await expect(toast(page, 'Saved and applied.')).toHaveCount(0);
});

test('port 0 whose bind fails is saved but unconfirmed and never linked as :0', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	api.bindFails = true;
	await page.goto('/#/settings');
	await addressForm(page).getByLabel('Port').fill('0');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(
		warning(page, 'saved, but web_ui: listen tcp 127.0.0.1:0: bind: address already in use')
	).toBeVisible();
	await expect(notice(page)).toContainText('Saved, but the web UI is not confirmed running.');
	await expect(notice(page)).toContainText('The Web UI may have stopped.');
	await expect(notice(page)).not.toContainText('applied');
	await expect(notice(page).getByRole('link')).toHaveCount(0);
	await expect(page.locator('main a[href*=":0/"]')).toHaveCount(0);
	await expect(unsaved(addressForm(page))).toHaveCount(0);
	expect(api.status).toMatchObject({ address: '127.0.0.1:0', running: false });
});

// The page as a LAN browser sees it: served from an unroutable TEST-NET origin, assets answered
// by the preview and /apis by the mock registered before this route. The listener is a wildcard
// bind, which the backend reports as 127.0.0.1.
const LAN = 'http://192.0.2.20:1088';
async function serveFromLAN(page: Page, api: MockApi) {
	const preview = test.info().project.use.baseURL!;
	await page.route(LAN + '/**', async (route) => {
		const url = new URL(route.request().url());
		if (url.pathname.startsWith('/apis/')) return route.fallback();
		const response = await route.fetch({ url: preview + url.pathname + url.search });
		await route.fulfill({ response });
	});
	api.webUI = { host: '0.0.0.0', port: 1088 };
	api.status.address = '127.0.0.1:1088';
	api.stopOnMove = true;
}

test('from a LAN browser, a YAML save that leaves the wildcard web_ui alone is not a move', async ({
	page,
	api
}) => {
	await serveFromLAN(page, api);
	await page.goto(LAN + '/#/yaml');
	const source = page.getByLabel('Configuration YAML', { exact: true });
	await expect(source).toHaveValue(rawFixture.yaml);
	await source.fill(rawFixture.yaml + '# from the lan\n');
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(toast(page, 'Saved and applied.')).toBeVisible();
	// The re-read only happens once the write is known not to have moved the page.
	await expect
		.poll(() => api.requests.filter((entry) => entry === 'GET /apis/configs/raw').length)
		.toBe(2);
	await expect(notice(page)).toHaveCount(0);
	await expect(unsaved(yamlForm(page))).toHaveCount(0);
	await expect(page.locator('aside [data-state="running"]')).toBeVisible();
});

test('from a LAN browser, a wildcard bind follows the name in the location bar to its new port', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	await serveFromLAN(page, api);
	await page.goto(LAN + '/#/settings');
	await expect(addressForm(page).getByLabel('Host')).toHaveValue('0.0.0.0');
	await addressForm(page).getByLabel('Port').fill('1099');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(notice(page)).toContainText('The web UI has moved to');
	await expect(notice(page).getByRole('link', { name: '192.0.2.20:1099' })).toHaveAttribute(
		'href',
		'http://192.0.2.20:1099/#/settings'
	);
	expect(api.writes.map((write) => write.body)).toEqual([{ host: '0.0.0.0', port: 1099 }]);
});

test('from a LAN browser, an empty host binds loopback on the same port: a move only the machine itself can follow', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	await serveFromLAN(page, api);
	await page.goto(LAN + '/#/settings');
	const host = addressForm(page).getByLabel('Host');
	await expect(host).toHaveValue('0.0.0.0');
	await host.fill('');
	await addressForm(page).getByRole('button', { name: 'Save & Apply' }).click();
	await expect(notice(page)).toContainText('The web UI has moved to');
	await expect(notice(page).getByRole('link', { name: '127.0.0.1:1088' })).toHaveAttribute(
		'href',
		'http://127.0.0.1:1088/#/settings'
	);
	await expect(unsaved(addressForm(page))).toHaveCount(0);
	expect(api.writes.map((write) => write.body)).toEqual([{ host: '', port: 1088 }]);
	await expect(page.locator('aside [data-state="unknown"]')).toBeVisible();
	await expect(page.getByRole('main')).not.toContainText('Cannot reach JumpWay.');
});

// Like a real deployment, the status reports the origin this page is served from; the fixture's
// 127.0.0.1:1088 would otherwise read as a move after every YAML save.
const servedFromHere = (api: MockApi) => {
	api.status.address = '127.0.0.1:4173';
};

test('#/yaml shows the file verbatim; Ctrl+S saves it and re-reads the stored text', async ({
	page,
	api
}) => {
	servedFromHere(api);
	await page.goto('/#/yaml');
	await expect(heading(page)).toHaveText('Configuration File');
	await expect(page).toHaveTitle('Configuration File · JumpWay');
	const source = page.getByLabel('Configuration YAML', { exact: true });
	await expect(source).toHaveValue(rawFixture.yaml);
	await expect(source).toHaveAttribute('spellcheck', 'false');
	await expect(unsaved(yamlForm(page))).toHaveCount(0);

	const edited = rawFixture.yaml + '# trailing note\n';
	await source.fill(edited);
	await expect(unsaved(yamlForm(page))).toBeVisible();
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(yamlForm(page))).toHaveCount(0);
	expect(api.writes).toEqual([
		{ method: 'PUT', path: '/apis/configs/raw', body: { yaml: edited } }
	]);
	expect(api.raw.yaml).toBe(edited);
	await expect(source).toHaveValue(edited);
	await expect(toast(page, 'Saved and applied.')).toBeVisible();
	expect(api.requests.filter((entry) => entry === 'GET /apis/configs/raw')).toHaveLength(2);
	await expect(notice(page)).toHaveCount(0);
});

test('YAML reload asks only when dirty; cancel keeps the draft without a request; confirm restores the file', async ({
	page,
	api
}) => {
	await page.goto('/#/yaml');
	const source = page.getByLabel('Configuration YAML', { exact: true });
	await expect(source).toHaveValue(rawFixture.yaml);
	const reads = () => api.requests.filter((entry) => entry === 'GET /apis/configs/raw').length;

	await page.getByRole('button', { name: 'Reload from disk' }).click();
	await expect(toast(page, 'Reloaded from disk.')).toBeVisible();
	await expect(confirmDialog(page)).toHaveCount(0);
	expect(reads()).toBe(2);

	await source.fill('rules: []\n');
	await page.getByRole('button', { name: 'Reload from disk' }).click();
	await expect(confirmDialog(page)).toContainText('Reload from disk and discard unsaved changes?');
	await confirmDialog(page).getByRole('button', { name: 'Cancel' }).click();
	await expect(confirmDialog(page)).toHaveCount(0);
	await expect(source).toHaveValue('rules: []\n');
	await expect(unsaved(yamlForm(page))).toBeVisible();
	expect(reads()).toBe(2);

	await page.getByRole('button', { name: 'Reload from disk' }).click();
	await confirmDialog(page).getByRole('button', { name: 'Reload from disk' }).click();
	await expect(source).toHaveValue(rawFixture.yaml);
	await expect(unsaved(yamlForm(page))).toHaveCount(0);
	expect(reads()).toBe(3);
	expect(api.writes).toEqual([]);
});

test('YAML: a 400 keeps the text dirty; "saved, but" cleans it with a warning; a failed re-read keeps the saved text', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expectedFailure;
	servedFromHere(api);
	await page.goto('/#/yaml');
	const source = page.getByLabel('Configuration YAML', { exact: true });
	await expect(source).toHaveValue(rawFixture.yaml);

	// The server validates; an empty document is its own 400.
	await source.fill('');
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(yamlForm(page).getByRole('alert')).toContainText('config is empty');
	await expect(unsaved(yamlForm(page))).toBeVisible();

	api.fail.set(
		'PUT /apis/configs/raw',
		'yaml: line 3: mapping values are not allowed in this context'
	);
	await source.fill('web_ui: [\n');
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(yamlForm(page).getByRole('alert')).toContainText(
		'yaml: line 3: mapping values are not allowed in this context'
	);
	await expect(unsaved(yamlForm(page))).toBeVisible();
	await expect(source).toHaveValue('web_ui: [\n');
	await expect(toast(page, 'Saved and applied.')).toHaveCount(0);

	api.fail.set('PUT /apis/configs/raw', 'saved, but reload failed: bind: address already in use');
	api.fail.set('GET /apis/configs/raw', 'store is locked');
	await source.fill('rules: []\n');
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(unsaved(yamlForm(page))).toHaveCount(0);
	await expect(
		warning(page, 'saved, but reload failed: bind: address already in use')
	).toBeVisible();
	await expect(yamlForm(page).getByRole('alert')).toContainText('store is locked');
	await expect(source).toHaveValue('rules: []\n');
	expect(api.raw.yaml).toBe('rules: []\n');
});

test.describe('screenshots', () => {
	const shoot = async (page: Page, name: string) => {
		const path = test.info().outputPath(name + '.png');
		await page.screenshot({ path, fullPage: true, animations: 'disabled' });
		test.info().attach(name, { path, contentType: 'image/png' });
		const overflow = await page.evaluate(() => ({
			scrollWidth: document.getElementById('main')!.scrollWidth,
			clientWidth: document.getElementById('main')!.clientWidth
		}));
		expect(overflow.scrollWidth, name).toBeLessThanOrEqual(overflow.clientWidth);
	};

	test('desktop settings in en and YAML in zh', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/#/settings');
		await expect(addressForm(page).getByLabel('Port')).toHaveValue('1088');
		await shoot(page, 'desktop-settings-en');
		await page.goto('/?lang=zh#/yaml');
		await expect(page.getByLabel('YAML 配置', { exact: true })).toHaveValue(rawFixture.yaml);
		await shoot(page, 'desktop-yaml-zh');
	});

	test('mobile settings in zh and YAML in en', async ({ page }) => {
		await page.setViewportSize({ width: 375, height: 812 });
		await page.goto('/?lang=zh#/settings');
		await expect(addressForm(page).getByLabel('端口')).toHaveValue('1088');
		await shoot(page, 'mobile-settings-zh');
		await page.goto('/#/yaml');
		await expect(page.getByLabel('Configuration YAML', { exact: true })).toHaveValue(
			rawFixture.yaml
		);
		await shoot(page, 'mobile-yaml-en');
	});
});
