import type { Locator, Page } from '@playwright/test';
import { protocolRulesFixture, rulesFixture } from './fixtures/api.ts';
import { expect, test } from './fixtures/test.ts';
import { count, type MockApi } from './mockApi.ts';

// Rule cards on the overview, editor, hop editor, URL builder and the dirty-leave guards, in real
// Chrome against the stateful mock (writes recorded, failures injectable).

// What a legacy proxy rule serves; the editor always spells it out.
const LEGACY = [{ type: 'http' }, { type: 'socks5' }, { type: 'socks4' }, { type: 'ssh' }];

const heading = (page: Page) => page.getByRole('heading', { level: 1 });
const cards = (page: Page) => page.getByRole('main').getByRole('article');
const cardNames = (page: Page) => cards(page).locator('header a');
const newRule = (page: Page) => page.getByRole('main').getByRole('link', { name: 'New rule' });
const form = (page: Page) => page.getByRole('main').locator('form');
const unsaved = (page: Page) => page.getByRole('main').getByText('Unsaved changes');
const confirmDialog = (page: Page) =>
	page.locator('dialog[open][aria-describedby="confirm-message"]');
const builder = (page: Page) => page.locator('dialog[open][aria-labelledby="url-builder-title"]');
const exitEditor = (page: Page) => page.locator('#exit-hint').locator('..');
const chainStages = async (page: Page) =>
	page
		.getByRole('list', { name: 'Chain' })
		.locator('> li')
		.evaluateAll((stages) =>
			stages.map((stage) =>
				Array.from(stage.querySelectorAll('p, li'))
					.map((node) => node.textContent!.replace(/\s+/g, ' ').trim())
					.join(' ')
			)
		);
const writes = (api: MockApi) => api.writes.map((write) => `${write.method} ${write.path}`);
// Chrome logs its own console line for a 400 the test expects.
const expected400 = (entry: string) =>
	entry.startsWith('http 400') || entry.includes('status of 400');

// Tooltips hide on scroll; scrolling the target into view first keeps the hover's tooltip.
async function hover(page: Page, locator: Locator) {
	await locator.scrollIntoViewIfNeeded();
	await page.waitForTimeout(100);
	await locator.hover();
}

// Rejected native traversals are decided by a blocking window.confirm inside popstate.
function dialogs(page: Page) {
	const seen: string[] = [];
	const control = { accept: false };
	page.on('dialog', (dialog) => {
		seen.push(`${dialog.type()} ${dialog.message()}`);
		void (control.accept ? dialog.accept() : dialog.dismiss());
	});
	return { seen, control };
}

test('the overview cards carry state, name, mode, address, target and chain size; delete asks first', async ({
	page,
	api
}) => {
	await page.goto('/');
	await expect(heading(page)).toHaveText('Overview');
	await expect(page).toHaveTitle('Overview · JumpWay');
	await expect(cards(page)).toHaveCount(4);
	await expect(cards(page).locator('[data-state]')).toHaveText([
		'Running',
		'Running',
		'Retrying (3)',
		'Disabled'
	]);
	const office = cards(page).nth(0);
	await expect(office.getByRole('link', { name: 'office' })).toHaveAttribute(
		'href',
		'#/rules/office'
	);
	await expect(office).toContainText('Proxy');
	await expect(office).toContainText('127.0.0.1:18097');
	await expect(office).toContainText('2 hops');
	const tunnel = cards(page).nth(2);
	await expect(tunnel).toContainText('Port forward · remote');
	await expect(tunnel).toContainText('0.0.0.0:18099');
	await expect(tunnel).toContainText('127.0.0.1:5432');
	await expect(tunnel).toContainText('direct');
	await expect(office.getByRole('link', { name: 'Edit' })).toHaveAttribute(
		'href',
		'#/rules/office'
	);
	await expect(office.getByRole('link', { name: 'Rule Traffic' })).toHaveAttribute(
		'href',
		'#/stats?rule=office'
	);
	await hover(page, office.getByRole('button', { name: 'Delete' }));
	await expect(page.locator('#tooltip')).toHaveText('Delete');

	const mirror = cards(page).nth(1);
	await mirror.getByRole('button', { name: 'Delete' }).click();
	await expect(confirmDialog(page)).toContainText('Delete rule "mirror"?');
	// While the question is open the card's own switch and Delete wait; the others do not.
	await expect(mirror.getByRole('switch', { name: 'Enabled' })).toBeDisabled();
	await expect(cards(page).nth(0).getByRole('switch', { name: 'Enabled' })).toBeEnabled();
	await confirmDialog(page).getByRole('button', { name: 'Cancel' }).click();
	await expect(confirmDialog(page)).toHaveCount(0);
	await expect(cards(page)).toHaveCount(4);
	await expect(mirror.getByRole('switch', { name: 'Enabled' })).toBeEnabled();
	expect(api.writes).toEqual([]);

	await mirror.getByRole('button', { name: 'Delete' }).click();
	await confirmDialog(page).getByRole('button', { name: 'Delete' }).click();
	await expect(cards(page)).toHaveCount(3);
	await expect(cardNames(page)).toHaveText(['office', 'db-tunnel', 'lab']);
	// mirror was running: the re-read status counts it out as well.
	await expect(page.locator('[data-kpi="rules"]')).toHaveText('1 Running');
	await expect(page.locator('[data-kpi="rule-states"]')).toHaveText(
		'3 configured · 1 Retrying · 1 Disabled'
	);
	expect(writes(api)).toEqual(['DELETE /apis/configs/rules/mirror']);
	await expect(page.getByRole('status').filter({ hasText: 'Rule deleted.' })).toBeVisible();
	await expect(page).toHaveURL(/#\/$/);
	expect(count(api, 'GET /apis/configs/status')).toBeGreaterThanOrEqual(2);
	expect(count(api, 'GET /apis/configs/rules')).toBeGreaterThanOrEqual(2);

	await expect(
		page.getByRole('main').locator('header').getByRole('link', { name: 'New rule' })
	).toHaveCount(0);
	const rulesRegion = page.getByRole('region', { name: 'Rules', exact: true });
	const grid = rulesRegion.locator('> div');
	expect(
		await grid.evaluate((element) =>
			Array.from(element.children, (child) => [child.tagName, child.getAttribute('href')])
		)
	).toEqual([
		['ARTICLE', null],
		['ARTICLE', null],
		['ARTICLE', null],
		['A', '#/new']
	]);
	// Leaving early aborts the delete's list re-read.
	await expect(rulesRegion).toHaveAttribute('aria-busy', 'false');
	await cards(page).last().getByRole('button', { name: 'Delete' }).focus();
	await page.keyboard.press('Tab');
	await expect(newRule(page)).toBeFocused();
	await expect(newRule(page)).toHaveCSS('outline-style', 'solid');
	const peers = page.waitForResponse('**/apis/configs/rules');
	await page.keyboard.press('Enter');
	await expect(page).toHaveURL(/#\/new$/);
	await expect(heading(page)).toHaveText('New rule');
	expect(await (await peers).finished()).toBeNull();
	await page.goBack();
	await expect(page).toHaveURL(/#\/$/);
	await expect(rulesRegion).toHaveAttribute('aria-busy', 'false');
	const reloadedPeers = page.waitForResponse('**/apis/configs/rules');
	await newRule(page).click();
	await expect(page).toHaveURL(/#\/new$/);
	await expect(heading(page)).toHaveText('New rule');
	expect(await (await reloadedPeers).finished()).toBeNull();
});

test('an empty overview and an unknown rule show their empty states', async ({
	page,
	api,
	failures
}) => {
	api.rules = [];
	await page.goto('/#/rules');
	await expect(page).toHaveURL(/#\/$/);
	await expect(newRule(page)).toHaveCount(1);
	await expect(newRule(page)).toBeVisible();
	await expect(cards(page)).toHaveCount(0);
	await expect(page.getByRole('main')).not.toContainText('No rules yet.');

	failures.allow = expected400;
	await page.goto('/#/rules/ghost');
	const banner = page.getByRole('main').getByRole('alert');
	await expect(banner).toContainText('Rule "ghost" was not found.');
	await expect(banner.getByRole('link', { name: 'Overview' })).toHaveAttribute('href', '#/');
	await expect(form(page)).toHaveCount(0);
});

test('create from the overview, rename in the editor, then manage the new card from the overview', async ({
	page,
	api
}) => {
	await page.goto('/');
	await newRule(page).click();
	await expect(page).toHaveURL(/#\/new$/);
	await expect(heading(page)).toHaveText('New rule');
	await expect(page.getByRole('button', { name: 'Delete' })).toHaveCount(0);
	await expect(page.getByLabel('Rule name')).toHaveValue('');
	await expect(page.getByLabel('Host', { exact: true })).toHaveValue('127.0.0.1');
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('0');
	await expect(unsaved(page)).toHaveCount(0);

	// Validation happens before any request.
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(page.getByLabel('Rule name')).toBeFocused();
	await expect(page.getByText('Rule name is required.')).toBeVisible();
	expect(api.writes).toEqual([]);

	await page.getByLabel('Rule name').fill('lab 2');
	await page.getByLabel('Port', { exact: true }).fill('18101');
	await page.getByLabel('Username').fill('demo');
	await page.getByLabel('Password', { exact: true }).fill('placeholder');
	await expect(unsaved(page)).toBeVisible();
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(page).toHaveURL(/#\/rules\/lab%202$/);
	await expect(heading(page)).toHaveText('lab 2');
	expect(api.writes).toEqual([
		{
			method: 'POST',
			path: '/apis/configs/rules',
			body: {
				name: 'lab 2',
				listen: {
					host: '127.0.0.1',
					port: 18101,
					username: 'demo',
					password: 'placeholder',
					protocols: LEGACY
				},
				forward: {}
			}
		}
	]);
	await expect(page.getByRole('status').filter({ hasText: 'Saved and applied.' })).toBeVisible();
	await expect(unsaved(page)).toHaveCount(0);
	await expect(page.getByLabel('Rule name')).toHaveValue('lab 2');
	await expect(page.getByRole('button', { name: 'Delete' })).toBeVisible();

	// Rename: PUT to the old name with the new body, then the route follows.
	await page.getByLabel('Rule name').fill('lab-2');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page).toHaveURL(/#\/rules\/lab-2$/);
	await expect(heading(page)).toHaveText('lab-2');
	expect(writes(api)).toEqual(['POST /apis/configs/rules', 'PUT /apis/configs/rules/lab%202']);
	expect(api.writes[1].body).toMatchObject({ name: 'lab-2' });
	expect(api.rules.map((rule) => rule.name)).toEqual([
		...rulesFixture.map((rule) => rule.name),
		'lab-2'
	]);

	// The breadcrumb leads home, where the new card is edited and deleted like any other.
	await page.getByRole('navigation', { name: 'Breadcrumb' }).getByRole('link').click();
	await expect(page).toHaveURL(/#\/$/);
	await expect(heading(page)).toHaveText('Overview');
	const created = cards(page).filter({
		has: page.getByRole('link', { name: 'lab-2', exact: true })
	});
	await expect(created).toHaveCount(1);
	await expect(created).toContainText('127.0.0.1:18101');
	await expect(created.getByRole('link', { name: 'Edit' })).toHaveAttribute(
		'href',
		'#/rules/lab-2'
	);
	await created.getByRole('button', { name: 'Delete' }).click();
	await expect(confirmDialog(page)).toContainText('Delete rule "lab-2"?');
	await confirmDialog(page).getByRole('button', { name: 'Cancel' }).click();
	await expect(created).toHaveCount(1);
	expect(api.writes).toHaveLength(2);
	await created.getByRole('button', { name: 'Delete' }).click();
	await confirmDialog(page).getByRole('button', { name: 'Delete' }).click();
	await expect(created).toHaveCount(0);
	await expect(cards(page)).toHaveCount(4);
	await expect(page).toHaveURL(/#\/$/);
	expect(writes(api).at(-1)).toBe('DELETE /apis/configs/rules/lab-2');
	await expect(page.getByRole('status').filter({ hasText: 'Rule deleted.' })).toBeVisible();
});

test('a dirty editor asks before leaving: custom dialog in-app, native confirm on Back', async ({
	page,
	api
}) => {
	const native = dialogs(page);
	await page.goto('/');
	await cards(page).nth(0).getByRole('link', { name: 'office' }).click();
	await expect(heading(page)).toHaveText('office');
	await page.getByLabel('Port', { exact: true }).fill('18197');
	await expect(unsaved(page)).toBeVisible();

	await page
		.getByRole('navigation', { name: 'Navigation' })
		.getByRole('link', { name: 'Proxy Hosts' })
		.click();
	await expect(confirmDialog(page)).toContainText('Discard unsaved changes?');
	await confirmDialog(page).getByRole('button', { name: 'Cancel' }).click();
	await expect(page).toHaveURL(/#\/rules\/office$/);
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('18197');
	expect(native.seen).toEqual([]);

	await page.goBack();
	await expect.poll(() => native.seen).toEqual(['confirm Discard unsaved changes?']);
	await expect(page).toHaveURL(/#\/rules\/office$/);
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('18197');

	native.control.accept = true;
	await page.goBack();
	await expect(page).toHaveURL(/#\/$/);
	await expect(heading(page)).toHaveText('Overview');
	expect(native.seen).toHaveLength(2);

	await page.goForward();
	await expect(heading(page)).toHaveText('office');
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('18097');
	await page.getByLabel('Port', { exact: true }).fill('18198');
	await page
		.getByRole('navigation', { name: 'Navigation' })
		.getByRole('link', { name: 'Proxy Hosts' })
		.click();
	await confirmDialog(page).getByRole('button', { name: 'Confirm' }).click();
	await expect(page).toHaveURL(/#\/hosts$/);
	expect(api.writes).toEqual([]);
	expect(native.seen).toHaveLength(2);
});

test('Cancel sits between Save & Apply and Duplicate/Delete: clean leaves at once, dirty asks, a save in flight holds it, Enter never triggers it', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expected400;
	const native = dialogs(page);
	const cancel = () => form(page).getByRole('button', { name: 'Cancel', exact: true });
	await page.goto('/#/rules/office');
	await expect(heading(page)).toHaveText('office');
	await expect(form(page).locator('[data-form-actions] button')).toHaveText([
		'Save & Apply',
		'Cancel',
		'Duplicate rule',
		'Delete'
	]);
	await expect(cancel()).toHaveAttribute('type', 'button');
	await expect(cancel()).toHaveCSS('height', '32px');
	await cancel().click();
	await expect(page).toHaveURL(/#\/$/);
	await expect(heading(page)).toHaveText('Overview');
	await expect(cards(page)).toHaveCount(4);
	expect(api.writes).toEqual([]);

	// Back lands on the editor: Cancel was an ordinary navigation, not a replaced entry.
	await page.goBack();
	await expect(heading(page)).toHaveText('office');
	await page.goForward();
	await expect(heading(page)).toHaveText('Overview');
	await newRule(page).click();
	await expect(heading(page)).toHaveText('New rule');
	await expect(form(page).locator('[data-form-actions] button')).toHaveText([
		'Save & Apply',
		'Cancel'
	]);
	const name = page.getByLabel('Rule name');
	await name.fill('draft');
	await expect(unsaved(page)).toBeVisible();
	// Enter inside a field submits through Save & Apply (the port is invalid, so nothing is sent).
	await page.getByLabel('Port', { exact: true }).fill('70000');
	await page.getByLabel('Port', { exact: true }).press('Enter');
	await expect(page.locator('#listen-port-error')).toBeVisible();
	await expect(page).toHaveURL(/#\/new$/);
	expect(api.writes).toEqual([]);

	await cancel().focus();
	await page.keyboard.press('Enter');
	await expect(confirmDialog(page)).toContainText('Discard unsaved changes?');
	await confirmDialog(page).getByRole('button', { name: 'Cancel' }).click();
	await expect(confirmDialog(page)).toHaveCount(0);
	await expect(page).toHaveURL(/#\/new$/);
	await expect(name).toHaveValue('draft');
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('70000');
	await expect(unsaved(page)).toBeVisible();
	expect(native.seen).toEqual([]);

	// While the POST is held every footer control waits, Cancel included.
	await page.getByLabel('Port', { exact: true }).fill('18104');
	let release!: () => void;
	api.delay.set(
		'POST /apis/configs/rules',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	await page.keyboard.press('ControlOrMeta+s');
	await expect(cancel()).toBeDisabled();
	await expect(page.getByRole('button', { name: 'Save & Apply' })).toBeDisabled();
	release();
	await expect(page).toHaveURL(/#\/rules\/draft$/);
	await expect(cancel()).toBeEnabled();
	expect(writes(api)).toEqual(['POST /apis/configs/rules']);

	// A rejected save leaves the form dirty: Cancel still asks, and discarding sends nothing more.
	api.fail.set(
		'PUT /apis/configs/rules/draft',
		'reload failed: listen tcp 127.0.0.1:18105: bind: address already in use'
	);
	await page.getByLabel('Port', { exact: true }).fill('18105');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page.getByRole('main').getByRole('alert')).toContainText('already in use');
	await cancel().click();
	await confirmDialog(page).getByRole('button', { name: 'Confirm' }).click();
	await expect(page).toHaveURL(/#\/$/);
	await expect(cardNames(page)).toHaveText(['office', 'mirror', 'db-tunnel', 'lab', 'draft']);
	await expect(cards(page).nth(4)).toContainText('127.0.0.1:18104');
	expect(writes(api)).toEqual(['POST /apis/configs/rules', 'PUT /apis/configs/rules/draft']);
	expect(native.seen).toEqual([]);
});

test('mode switches hide credentials and reveal the target; the body follows the mode', async ({
	page,
	api
}) => {
	await page.goto('/#/rules/lab');
	await expect(page.getByLabel('Username')).toHaveValue('demo');
	await expect(page.getByLabel('Password', { exact: true })).toHaveValue('placeholder');
	await expect(page.getByLabel('Password', { exact: true })).toHaveAttribute('type', 'password');
	await expect(page.getByRole('switch', { name: 'Enabled' })).not.toBeChecked();
	await expect(page.getByRole('radio', { name: 'Proxy' })).toBeChecked();

	await page.getByRole('radio', { name: 'Port forward' }).check();
	await expect(page.getByLabel('Username')).toHaveCount(0);
	await expect(page.getByText('Credentials apply to proxy rules only')).toBeVisible();
	await page.getByLabel('Target host').fill('db.internal');
	await page.getByLabel('Target port').fill('5432');
	await expect(page.locator('[data-chain-summary]')).toHaveText('this machine → db.internal:5432');
	await page.getByRole('switch', { name: 'Enabled' }).check();
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes).toEqual([
		{
			method: 'PUT',
			path: '/apis/configs/rules/lab',
			body: {
				name: 'lab',
				listen: { host: '127.0.0.1', port: 18100 },
				forward: { host: 'db.internal', port: 5432 }
			}
		}
	]);

	// Back to proxy mode: the credential draft is still there and the target is dropped.
	await page.getByRole('radio', { name: 'Proxy' }).check();
	await expect(page.getByLabel('Username')).toHaveValue('demo');
	await page.keyboard.press('ControlOrMeta+s');
	await expect.poll(() => api.writes.length).toBe(2);
	expect(api.writes[1].body).toEqual({
		name: 'lab',
		listen: {
			host: '127.0.0.1',
			port: 18100,
			username: 'demo',
			password: 'placeholder',
			protocols: LEGACY
		},
		forward: {}
	});
});

test('hops: add, add URLs, reorder and remove; ChainFlow lists exit hops last-dialed first', async ({
	page,
	api
}) => {
	await page.goto('/#/rules/db-tunnel');
	const editor = exitEditor(page);
	await expect(editor.locator('li[id^="exit-hop-"]')).toHaveCount(0);
	expect(await chainStages(page)).toEqual([
		'Clients 0.0.0.0:18099',
		'Hop 1 · binds the port · dialed from this machine ssh://edge.example:22',
		'This machine',
		'Direct',
		'Target 127.0.0.1:5432'
	]);

	await editor.getByRole('button', { name: 'Add hop' }).click();
	const urls = editor.locator('input[id^="exit-url-"]');
	await expect(urls.nth(0)).toBeFocused();
	await urls.nth(0).fill('socks5://exit.example:1080');
	await editor.getByRole('button', { name: 'Add hop' }).click();
	await urls.nth(1).fill('ssh://ops@entry.example:22');
	await editor
		.locator('li[id^="exit-hop-"]')
		.nth(1)
		.getByRole('button', { name: 'Add URL' })
		.click();
	await expect(urls.nth(2)).toBeFocused();
	await urls.nth(2).fill('ssh://ops@entry-2.example:22');
	await expect(editor.locator('li[id^="exit-hop-"] h4')).toHaveText([
		'Hop 1 · exit node',
		'Hop 2 · dialed from this machine'
	]);
	await expect(page.locator('[data-chain-summary]')).toHaveText(
		'this machine → Hop 2 → Hop 1 → 127.0.0.1:5432'
	);
	expect(await chainStages(page)).toEqual([
		'Clients 0.0.0.0:18099',
		'Hop 1 · binds the port · dialed from this machine ssh://edge.example:22',
		'This machine',
		'Hop 2 · entry node · dialed from this machine ssh://entry.example:22 ssh://entry-2.example:22',
		'Hop 1 · exit node socks5://exit.example:1080',
		'Target 127.0.0.1:5432'
	]);
	// The tooltip is the full URL with its userinfo masked; the visible text is scheme://host:port.
	await hover(page, page.getByRole('list', { name: 'Chain' }).getByText('ssh://entry.example:22'));
	await expect(page.locator('#tooltip')).toHaveText('ssh://xxxxx@entry.example:22');

	await editor.locator('li[id^="exit-hop-"]').nth(1).getByRole('button', { name: 'Up' }).click();
	await expect
		.poll(() =>
			urls.evaluateAll((inputs) => inputs.map((input) => (input as HTMLInputElement).value))
		)
		.toEqual([
			'ssh://ops@entry.example:22',
			'ssh://ops@entry-2.example:22',
			'socks5://exit.example:1080'
		]);
	await expect(
		editor.locator('li[id^="exit-hop-"]').nth(0).getByRole('button', { name: 'Down' })
	).toBeFocused();
	await editor
		.locator('li[id^="exit-hop-"]')
		.nth(0)
		.getByRole('button', { name: 'Remove' })
		.first()
		.click();
	await expect(urls).toHaveCount(2);
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes[0].body).toMatchObject({
		forward: {
			host: '127.0.0.1',
			port: 5432,
			way: [{ lb: ['ssh://ops@entry-2.example:22'] }, { lb: ['socks5://exit.example:1080'] }]
		}
	});
});

test('the URL builder: four goldens, prefill, and Cancel/Escape leave the row untouched', async ({
	page
}) => {
	await page.goto('/#/rules/office');
	const row = page.locator('input[id^="exit-url-"]').first();
	await expect(row).toHaveValue('socks5://demo:placeholder@hop-a.example:1080');
	const wand = row.locator('..').getByRole('button', { name: 'Build…' });
	await hover(page, wand);
	await expect(page.locator('#tooltip')).toHaveText('Build…');
	await wand.click();
	const dialog = builder(page);
	await expect(dialog).toBeVisible();
	expect(await dialog.evaluate((element) => element.matches(':modal'))).toBe(true);
	await expect(dialog.getByLabel('Protocol')).toHaveValue('socks5');
	await expect(dialog.getByLabel('Username')).toHaveValue('demo');
	await expect(dialog.getByLabel('Username')).toBeFocused();
	await expect(dialog.getByLabel('Password')).toHaveValue('placeholder');
	await expect(dialog.getByLabel('Password')).toHaveAttribute('type', 'password');
	await expect(dialog.getByLabel('Host')).toHaveValue('hop-a.example');
	await expect(dialog.getByLabel('Port')).toHaveValue('1080');
	const preview = dialog.locator('output');
	await expect(preview).toHaveText('socks5://demo:placeholder@hop-a.example:1080');

	// Golden 1: encoded credentials. Enter in a field submits only while the URL is valid.
	await dialog.getByLabel('Username').fill('u');
	await dialog.getByLabel('Password').fill('p@ss');
	await dialog.getByLabel('Host').fill('');
	await expect(preview).toHaveText('socks5://u:p%40ss@:1080');
	await expect(dialog.getByRole('button', { name: 'Use URL' })).toBeDisabled();
	await expect(dialog.locator('#url-builder-hint')).toHaveText(
		'Host and a numeric port are required.'
	);
	await dialog.getByLabel('Host').press('Enter');
	await expect(dialog).toBeVisible();
	await dialog.getByLabel('Host').fill('h');
	await expect(preview).toHaveText('socks5://u:p%40ss@h:1080');
	await dialog.getByLabel('Host').press('Enter');
	await expect(dialog).toHaveCount(0);
	await expect(row).toHaveValue('socks5://u:p%40ss@h:1080');
	await expect(row).toBeFocused();
	await expect(unsaved(page)).toBeVisible();

	// Golden 2: shadowsocks puts the cipher in the userinfo and defaults the port.
	await wand.click();
	await dialog.getByLabel('Protocol').selectOption('shadowsocks');
	await expect(dialog.getByLabel('Encryption method')).toHaveValue('aes-256-gcm');
	await expect(dialog.getByLabel('Port')).toHaveValue('8379');
	await expect(dialog.getByRole('button', { name: 'Use URL' })).toBeDisabled();
	await expect(dialog.locator('#url-builder-hint')).toHaveText(
		'Host, a numeric port, encryption method, and password are required.'
	);
	await dialog.getByLabel('Password').fill('secret');
	await dialog.getByLabel('Host').fill('host');
	await expect(preview).toHaveText('ss://aes-256-gcm:secret@host:8379');
	await dialog.getByRole('button', { name: 'Use URL' }).click();
	await expect(row).toHaveValue('ss://aes-256-gcm:secret@host:8379');

	// Prefill from an ss:// URL, then golden 3: ssh with an encoded identity file.
	await wand.click();
	await expect(dialog.getByLabel('Protocol')).toHaveValue('shadowsocks');
	await expect(dialog.getByLabel('Encryption method')).toHaveValue('aes-256-gcm');
	await expect(dialog.getByLabel('Password')).toHaveValue('secret');
	await dialog.getByLabel('Protocol').selectOption('ssh');
	await expect(dialog.getByLabel('Port')).toHaveValue('22');
	await dialog.getByLabel('Username').fill('user');
	await dialog.getByLabel('Host').fill('host');
	await dialog.getByLabel('Identity file').fill('~/.ssh/id_ed25519');
	await expect(preview).toHaveText('ssh://user@host:22?identity_file=~%2F.ssh%2Fid_ed25519');
	await dialog.getByRole('button', { name: 'Use URL' }).click();
	await expect(row).toHaveValue('ssh://user@host:22?identity_file=~%2F.ssh%2Fid_ed25519');

	// Golden 4: bare IPv6 gets brackets; Cancel and Escape change nothing.
	await wand.click();
	await expect(dialog.getByLabel('Identity file')).toHaveValue('~/.ssh/id_ed25519');
	await dialog.getByLabel('Protocol').selectOption('socks5');
	await dialog.getByLabel('Host').fill('::1');
	await expect(preview).toHaveText('socks5://[::1]:1080');
	await dialog.getByRole('button', { name: 'Cancel' }).click();
	await expect(dialog).toHaveCount(0);
	await expect(row).toHaveValue('ssh://user@host:22?identity_file=~%2F.ssh%2Fid_ed25519');
	await expect(wand).toBeFocused();
	await wand.click();
	await dialog.getByLabel('Host').fill('changed');
	await page.keyboard.press('Escape');
	await expect(dialog).toHaveCount(0);
	await expect(row).toHaveValue('ssh://user@host:22?identity_file=~%2F.ssh%2Fid_ed25519');
	await expect(wand).toBeFocused();
	await wand.click();
	await dialog.getByLabel('Protocol').selectOption('socks5');
	await dialog.getByLabel('Host').fill('::1');
	await dialog.getByRole('button', { name: 'Use URL' }).click();
	await expect(row).toHaveValue('socks5://[::1]:1080');
});

test('the URL builder: command and netcat hops take one raw command field, no host or port', async ({
	page,
	api
}) => {
	await page.goto('/#/rules/office');
	const row = page.locator('input[id^="exit-url-"]').first();
	const wand = row.locator('..').getByRole('button', { name: 'Build…' });
	await wand.click();
	const dialog = builder(page);
	const protocol = dialog.getByLabel('Protocol');
	const command = dialog.getByLabel('Command');
	const fields = dialog.locator('[data-builder-fields] :is(input, select)');
	const preview = dialog.locator('output');
	const hint = dialog.locator('#url-builder-hint');
	const useURL = dialog.getByRole('button', { name: 'Use URL' });

	await protocol.selectOption('command');
	await expect(fields).toHaveCount(1);
	await expect(dialog.getByLabel('Host')).toHaveCount(0);
	await expect(dialog.getByLabel('Port')).toHaveCount(0);
	await expect(command).toHaveValue('');
	await expect(command).toHaveAttribute('placeholder', 'nc %h %p');
	await expect(preview).toHaveText('cmd:');
	await expect(useURL).toBeDisabled();
	await expect(hint).toHaveText('A command is required.');
	await command.fill('   ');
	await expect(preview).toHaveText('cmd:');
	await expect(useURL).toBeDisabled();
	await command.fill('ssh -W %h:%p jump');
	await expect(preview).toHaveText('cmd:ssh -W %h:%p jump');
	await expect(hint).toHaveText('');
	await command.press('Enter');
	await expect(dialog).toHaveCount(0);
	await expect(row).toHaveValue('cmd:ssh -W %h:%p jump');
	await expect(unsaved(page)).toBeVisible();

	// Prefill from cmd:, then netcat: the command is optional and the prefix alone is valid.
	await wand.click();
	await expect(protocol).toHaveValue('command');
	await expect(command).toHaveValue('ssh -W %h:%p jump');
	await expect(command).toBeFocused();
	await protocol.selectOption('netcat');
	const prefix = dialog.getByLabel('Execution prefix');
	await expect(fields).toHaveCount(1);
	await expect(prefix).toHaveValue('');
	await expect(prefix).toHaveAttribute('placeholder', 'ssh jump');
	await expect(preview).toHaveText('nc:');
	await expect(useURL).toBeEnabled();
	await expect(hint).toHaveText('');
	await prefix.fill('ssh jump');
	await expect(preview).toHaveText('nc:ssh jump');
	await useURL.click();
	await expect(dialog).toHaveCount(0);
	await expect(row).toHaveValue('nc:ssh jump');

	await wand.click();
	await expect(protocol).toHaveValue('netcat');
	await expect(prefix).toHaveValue('ssh jump');
	await dialog.getByRole('button', { name: 'Cancel' }).click();
	await expect(dialog).toHaveCount(0);
	await expect(row).toHaveValue('nc:ssh jump');

	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes[0].body).toMatchObject({
		forward: {
			way: [
				{ lb: ['nc:ssh jump'] },
				{ lb: ['ssh://ops@bastion.example:22', 'ssh://ops@bastion-2.example:22'] }
			]
		}
	});
});

test('a 400 keeps the form dirty on its route and shows the redacted message', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expected400;
	await page.goto('/#/rules/mirror');
	api.fail.set(
		'PUT /apis/configs/rules/mirror',
		'rules[1].forward.way[0]: invalid proxy URL "socks5://demo:placeholder@bad:port": parse error'
	);
	await page.getByLabel('Port', { exact: true }).fill('18197');
	await page.keyboard.press('ControlOrMeta+s');
	const banner = page.getByRole('main').getByRole('alert');
	await expect(banner).toContainText('invalid proxy URL "socks5://xxxxx@bad:port"');
	expect(await page.content()).not.toContain('demo:placeholder@bad');
	await expect(unsaved(page)).toBeVisible();
	await expect(page).toHaveURL(/#\/rules\/mirror$/);
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('18197');
	expect(writes(api)).toEqual(['PUT /apis/configs/rules/mirror']);

	// The same body is retried once the backend accepts it; nothing was duplicated.
	api.fail.clear();
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	await expect(banner).toHaveCount(0);
	expect(writes(api)).toEqual(['PUT /apis/configs/rules/mirror', 'PUT /apis/configs/rules/mirror']);
});

test('"saved, but" leaves the form clean with a persistent warning and retries as a PUT', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expected400;
	await page.goto('/#/new');
	api.fail.set(
		'POST /apis/configs/rules',
		'saved, but reload failed: listen tcp 127.0.0.1:18102: bind: address already in use'
	);
	await page.getByLabel('Rule name').fill('clash');
	await page.getByLabel('Port', { exact: true }).fill('18102');
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(page).toHaveURL(/#\/rules\/clash$/);
	await expect(heading(page)).toHaveText('clash');
	const warning = page.getByRole('alert').filter({ hasText: 'saved, but reload failed' });
	await expect(warning).toBeVisible();
	await expect(unsaved(page)).toHaveCount(0);
	expect(writes(api)).toEqual(['POST /apis/configs/rules']);

	await page.getByLabel('Port', { exact: true }).fill('18103');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(writes(api)).toEqual(['POST /apis/configs/rules', 'PUT /apis/configs/rules/clash']);
	expect(api.rules.find((rule) => rule.name === 'clash')?.listen.port).toBe(18103);
	// The warning stays until dismissed.
	await expect(warning).toBeVisible();
	await warning.getByRole('button', { name: 'Dismiss' }).click();
	await expect(warning).toHaveCount(0);
});

test('a "saved, but" delete is a delete: the list refreshes, the dirty editor returns clean', async ({
	page,
	api,
	failures
}) => {
	failures.allow = expected400;
	const reloadFailed =
		'saved, but reload failed: listen tcp 127.0.0.1:18097: bind: address already in use';
	const warnings = page.getByRole('alert').filter({ hasText: reloadFailed });
	// Fires the app's beforeunload handler; true only while the editor is armed (dirty).
	const armed = () =>
		page.evaluate(() => !window.dispatchEvent(new Event('beforeunload', { cancelable: true })));

	await page.goto('/');
	await expect(cards(page)).toHaveCount(4);
	api.fail.set('DELETE /apis/configs/rules/mirror', reloadFailed);
	await cards(page).nth(1).getByRole('button', { name: 'Delete' }).click();
	await confirmDialog(page).getByRole('button', { name: 'Delete' }).click();
	await expect(cardNames(page)).toHaveText(['office', 'db-tunnel', 'lab']);
	await expect(warnings).toHaveCount(1);
	await expect(page.getByRole('status').filter({ hasText: 'Rule deleted.' })).toHaveCount(0);
	expect(writes(api)).toEqual(['DELETE /apis/configs/rules/mirror']);
	const statusReads = count(api, 'GET /apis/configs/status');
	expect(statusReads).toBeGreaterThanOrEqual(2);

	api.fail.set('DELETE /apis/configs/rules/lab', reloadFailed);
	await cards(page).nth(2).getByRole('link', { name: 'lab', exact: true }).click();
	await expect(heading(page)).toHaveText('lab');
	await page.getByLabel('Rule name').fill('lab-2');
	await expect(unsaved(page)).toBeVisible();
	expect(await armed()).toBe(true);
	await page.getByRole('button', { name: 'Delete', exact: true }).click();
	await confirmDialog(page).getByRole('button', { name: 'Delete' }).click();
	await expect(page).toHaveURL(/#\/$/);
	await expect(heading(page)).toHaveText('Overview');
	await expect(cardNames(page)).toHaveText(['office', 'db-tunnel']);
	await expect(warnings).toHaveCount(2);
	await expect(confirmDialog(page)).toHaveCount(0);
	expect(await armed()).toBe(false);
	expect(writes(api)).toEqual([
		'DELETE /apis/configs/rules/mirror',
		'DELETE /apis/configs/rules/lab'
	]);
	expect(count(api, 'GET /apis/configs/status')).toBeGreaterThan(statusReads);
});

test('the card footer keeps its four 32px actions and long rates inside the card at 375, 820 and 1280; the delete question fits a phone', async ({
	page,
	api
}) => {
	// Rates near the unit boundary are the widest formatBytes output ("1022.5 KB/s").
	for (const rule of api.snapshot.rules ?? []) {
		rule.stats.rate_up = 1047000;
		rule.stats.rate_down = 1047000;
	}
	for (const width of [375, 820, 1280]) {
		await page.setViewportSize({ width, height: 900 });
		await page.goto(`/?w=${width}#/`);
		await expect(cards(page)).toHaveCount(4);
		await expect(cards(page).first()).toContainText('1022.5 KB/s');
		const geometry = await page.evaluate(() => {
			const main = document.getElementById('main')!;
			const articles = Array.from(document.querySelectorAll('main article'));
			const trailing = document.querySelector('main article ~ a[href="#/new"]')!;
			const trailingBox = trailing.getBoundingClientRect();
			const previousBox = trailing.previousElementSibling!.getBoundingClientRect();
			return {
				overflow: main.scrollWidth - main.clientWidth,
				trailing: {
					border: getComputedStyle(trailing).borderTopStyle,
					sharesRow: Math.abs(trailingBox.top - previousBox.top) < 1,
					height: Math.round(trailingBox.height),
					previousHeight: Math.round(previousBox.height)
				},
				cards: articles.map((article) => {
					const box = article.getBoundingClientRect();
					const controls = Array.from(article.querySelectorAll('footer a, footer button')).map(
						(control) => control.getBoundingClientRect()
					);
					return {
						sizes: controls.map((rect) => `${Math.round(rect.width)}x${Math.round(rect.height)}`),
						inside: controls.every(
							(rect) => rect.left >= box.left - 1 && rect.right <= box.right + 1
						),
						wide: Array.from(article.querySelectorAll('*')).filter(
							(element) => element.getBoundingClientRect().right > box.right + 1
						).length
					};
				})
			};
		});
		expect(geometry.overflow, `${width}px`).toBeLessThanOrEqual(0);
		expect(geometry.trailing, `${width}px`).toEqual({
			border: 'dashed',
			sharesRow: width === 1280,
			height: width === 1280 ? geometry.trailing.previousHeight : 160,
			previousHeight: geometry.trailing.previousHeight
		});
		for (const card of geometry.cards) {
			expect(card.sizes, `${width}px`).toEqual(['32x32', '32x32', '32x32', '32x32']);
			expect(card.inside, `${width}px`).toBe(true);
			expect(card.wide, `${width}px`).toBe(0);
		}
	}

	await page.setViewportSize({ width: 375, height: 812 });
	await cards(page).nth(0).getByRole('button', { name: 'Delete' }).click();
	const dialog = confirmDialog(page);
	await expect(dialog).toContainText('Delete rule "office"?');
	const box = (await dialog.boundingBox())!;
	expect(box.x).toBeGreaterThanOrEqual(0);
	expect(box.x + box.width).toBeLessThanOrEqual(375);
	expect(box.y + box.height).toBeLessThanOrEqual(812);
	await expect(dialog.getByRole('button', { name: 'Delete' })).toBeInViewport();
	await dialog.getByRole('button', { name: 'Cancel' }).click();
	await expect(dialog).toHaveCount(0);
	expect(api.writes).toEqual([]);
});

test('virtual endpoints: a channel pairs an exit listener with an entry; cards and stats link the peers and warn once the exit is disabled or gone', async ({
	page,
	api
}) => {
	const general = page.getByRole('group', { name: 'General', exact: true });
	const listen = page.getByRole('group', { name: 'Listen', exact: true });
	const exit = page.getByRole('group', { name: 'Exit', exact: true });
	const peers = page.locator('[data-virtual-peers]');

	await page.goto('/#/new');
	await page.getByLabel('Rule name').fill('shared-exit');
	// Arrow keys move the segment like any radio group.
	await general.getByRole('radio', { name: 'Address' }).focus();
	await page.keyboard.press('ArrowRight');
	await expect(general.getByRole('radio', { name: 'Virtual' })).toBeChecked();
	await expect(page.getByLabel('Host', { exact: true })).toHaveCount(0);
	await expect(page.getByLabel('Port', { exact: true })).toHaveCount(0);
	await expect(page.getByText('Listen through')).toHaveCount(0);
	await listen.getByLabel('Channel').fill('exit');
	await page.getByLabel('Username').fill('demo');
	await page.getByLabel('Password', { exact: true }).fill('placeholder');
	expect(await chainStages(page)).toEqual([
		'Clients',
		'This machine virtual://exit',
		'Direct',
		'Target target'
	]);
	await expect(peers).toHaveCount(0);
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(page).toHaveURL(/#\/rules\/shared-exit$/);
	await expect(general.getByRole('radio', { name: 'Virtual' })).toBeChecked();
	await expect(listen.getByLabel('Channel')).toHaveValue('exit');
	await expect(unsaved(page)).toHaveCount(0);

	await page.goto('/#/new');
	await page.getByLabel('Rule name').fill('lan-entry');
	await page.getByLabel('Port', { exact: true }).fill('18101');
	await page.getByRole('radio', { name: 'Port forward' }).check();
	await exit.getByRole('radio', { name: 'Virtual' }).check();
	await expect(page.getByLabel('Target host')).toHaveCount(0);
	await expect(exitEditor(page)).toHaveCount(0);
	await exit.getByLabel('Channel').fill('missing');
	await expect(peers).toHaveText('No enabled rule listens on this channel.');
	await exit.getByLabel('Channel').fill('exit');
	await expect(peers).toContainText('Exit rule');
	await expect(peers.getByRole('link', { name: 'shared-exit' })).toHaveAttribute(
		'href',
		'#/rules/shared-exit'
	);
	expect(await chainStages(page)).toEqual([
		'Clients',
		'This machine 127.0.0.1:18101',
		'Direct',
		'Target virtual://exit'
	]);
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page).toHaveURL(/#\/rules\/lan-entry$/);
	await expect(page.getByLabel('Rule name')).toHaveValue('lan-entry');
	await expect(peers.getByRole('link', { name: 'shared-exit' })).toBeVisible();
	expect(api.writes).toEqual([
		{
			method: 'POST',
			path: '/apis/configs/rules',
			body: {
				name: 'shared-exit',
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
		},
		{
			method: 'POST',
			path: '/apis/configs/rules',
			body: {
				name: 'lan-entry',
				listen: { host: '127.0.0.1', port: 18101 },
				forward: { virtual: 'exit' }
			}
		}
	]);

	// The exit editor now lists its incoming entry.
	await page.goto('/#/rules/shared-exit');
	await expect(peers).toContainText('Incoming rules');
	await expect(peers.getByRole('link', { name: 'lan-entry' })).toHaveAttribute(
		'href',
		'#/rules/lan-entry'
	);

	const exitCard = cards(page).filter({
		has: page.locator('header a', { hasText: 'shared-exit' })
	});
	const entryCard = cards(page).filter({ has: page.locator('header a', { hasText: 'lan-entry' }) });
	for (const viewport of [
		{ width: 1280, height: 800 },
		{ width: 375, height: 812 }
	]) {
		await page.setViewportSize(viewport);
		await page.goto(`/?w=${viewport.width}#/`);
		await expect(cards(page)).toHaveCount(6);
		await expect(exitCard).toContainText('Proxy');
		await expect(exitCard).toContainText('virtual://exit');
		await expect(exitCard.locator('[data-state]')).toHaveText('Running');
		await expect(exitCard.locator('[data-virtual-peers]')).toContainText('Incoming rules');
		await expect(
			exitCard.locator('[data-virtual-peers]').getByRole('link', { name: 'lan-entry' })
		).toHaveAttribute('href', '#/rules/lan-entry');
		await expect(entryCard).toContainText('Port forward');
		await expect(entryCard).toContainText('127.0.0.1:18101');
		await expect(entryCard).toContainText('virtual://exit');
		await expect(
			entryCard.locator('[data-virtual-peers]').getByRole('link', { name: 'shared-exit' })
		).toHaveAttribute('href', '#/rules/shared-exit');
		await page.goto('/#/stats');
		await page
			.locator('main article[data-rule="lan-entry"] [data-virtual-peers]')
			.getByRole('link', { name: 'shared-exit' })
			.click({ timeout: 5000 });
		await expect(page).toHaveURL(/#\/rules\/shared-exit$/);
		await expect(page.getByLabel('Rule name')).toHaveValue('shared-exit');
		await expect(peers.getByRole('link', { name: 'lan-entry' })).toBeVisible();
		await page.goto('/#/stats');
		await page
			.locator('main article[data-rule="shared-exit"] [data-virtual-peers]')
			.getByRole('link', { name: 'lan-entry' })
			.click({ timeout: 5000 });
		await expect(page).toHaveURL(/#\/rules\/lan-entry$/);
		await expect(page.getByLabel('Rule name')).toHaveValue('lan-entry');
		await expect(peers.getByRole('link', { name: 'shared-exit' })).toBeVisible();
	}

	// Disabling the exit leaves the entry dangling; deleting it keeps it so.
	await page.goto('/#/');
	await exitCard.getByRole('switch', { name: 'Enabled' }).click();
	await expect(exitCard.locator('[data-state]')).toHaveText('Disabled');
	await expect(entryCard.locator('[data-virtual-peers]')).toHaveText(
		'No enabled rule listens on this channel.'
	);
	await exitCard.getByRole('button', { name: 'Delete' }).click();
	await confirmDialog(page).getByRole('button', { name: 'Delete' }).click();
	await expect(cards(page)).toHaveCount(5);
	await expect(entryCard.locator('[data-virtual-peers]')).toHaveText(
		'No enabled rule listens on this channel.'
	);

	await page.goto('/#/stats');
	const row = page.locator('main article[data-rule="lan-entry"]');
	await expect(row.locator('[data-mode]')).toHaveText('Port forward');
	await expect(row.locator('[data-address]')).toHaveText(
		/127\.0\.0\.1:18101\s*→\s*virtual:\/\/exit/
	);
	await expect(row.locator('[data-virtual-peers]')).toHaveText(
		'No enabled rule listens on this channel.'
	);

	await page.goto('/#/rules/lan-entry');
	await expect(exit.getByRole('radio', { name: 'Virtual' })).toBeChecked();
	await expect(exit.getByLabel('Channel')).toHaveValue('exit');
	await expect(peers).toHaveText('No enabled rule listens on this channel.');
});

// Full-page screenshot plus a check that nothing in #main runs past the viewport.
const shoot = async (page: Page, name: string) => {
	const path = test.info().outputPath(name + '.png');
	await page.screenshot({ path, fullPage: true, animations: 'disabled' });
	test.info().attach(name, { path, contentType: 'image/png' });
	const overflow = await page.evaluate(() => ({
		scrollWidth: document.getElementById('main')!.scrollWidth,
		clientWidth: document.getElementById('main')!.clientWidth,
		wide: Array.from(document.querySelectorAll('body *'))
			.filter(
				(element) =>
					element.getBoundingClientRect().right > document.documentElement.clientWidth + 1
			)
			.slice(0, 8)
			.map(
				(element) =>
					element.tagName +
					'.' +
					element.className.toString().slice(0, 60) +
					' right=' +
					Math.round(element.getBoundingClientRect().right)
			)
	}));
	expect(overflow.scrollWidth, `${name} ${overflow.wide.join(' | ')}`).toBeLessThanOrEqual(
		overflow.clientWidth
	);
};

test('Shadowsocks: checking it adds a row with a default cipher; the entry saves beside the shared password, reopens checked, hides in forward mode and fits a phone', async ({
	page,
	api
}) => {
	const ss = page.getByRole('group', { name: 'Shadowsocks', exact: true });
	const cipher = () => ss.getByLabel('Cipher');
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/#/new');
	await expect(heading(page)).toHaveText('New rule');
	await expect(page.getByRole('checkbox', { name: 'Shadowsocks' })).not.toBeChecked();
	await expect(ss).toHaveCount(0);
	await page.getByRole('checkbox', { name: 'Shadowsocks' }).check();
	await expect(cipher()).toHaveValue('aes-256-gcm');
	await expect(cipher().locator('option').first()).toHaveText('aes-128-gcm');
	await expect(cipher().locator('option', { hasText: 'aes-256-gcm' })).toHaveCount(1);
	await page.getByLabel('Rule name').fill('ss');
	await page.getByLabel('Port', { exact: true }).fill('18200');
	await page.getByLabel('Password', { exact: true }).fill('placeholder');
	await expect(unsaved(page)).toBeVisible();
	await shoot(page, 'desktop-editor-cipher');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page).toHaveURL(/#\/rules\/ss$/);
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes).toEqual([
		{
			method: 'POST',
			path: '/apis/configs/rules',
			body: {
				name: 'ss',
				listen: {
					host: '127.0.0.1',
					port: 18200,
					password: 'placeholder',
					protocols: [...LEGACY, { type: 'ss', cipher: 'aes-256-gcm' }]
				},
				forward: {}
			}
		}
	]);

	// A fresh load reads the saved entry back into the row.
	await page.reload();
	await expect(heading(page)).toHaveText('ss');
	await expect(page.getByRole('checkbox', { name: 'Shadowsocks' })).toBeChecked();
	await expect(cipher()).toHaveValue('aes-256-gcm');
	await expect(page.getByLabel('Username')).toHaveValue('');
	await page.getByRole('radio', { name: 'Port forward' }).check();
	await expect(ss).toHaveCount(0);
	await expect(page.getByRole('checkbox', { name: 'Shadowsocks' })).toHaveCount(0);
	await page.getByRole('radio', { name: 'Proxy' }).check();
	await expect(cipher()).toHaveValue('aes-256-gcm');

	await page.setViewportSize({ width: 390, height: 844 });
	await cipher().scrollIntoViewIfNeeded();
	const box = (await cipher().boundingBox())!;
	expect(box.x).toBeGreaterThanOrEqual(0);
	expect(box.x + box.width).toBeLessThanOrEqual(390);
	await shoot(page, 'mobile-editor-cipher');

	await page.goto('/?lang=zh#/rules/ss');
	await expect(ss.getByLabel('加密方式')).toHaveValue('aes-256-gcm');
	await expect(ss.getByRole('checkbox', { name: '单独凭据' })).not.toBeChecked();
});

test('listen protocols: one checked scheme is a single-protocol port, several share the credentials unless a row is custom; the top band and rows fit both geometries in en and zh', async ({
	page,
	api
}) => {
	api.rules.push(...structuredClone(protocolRulesFixture));
	const general = page.getByRole('group', { name: 'General', exact: true });
	const protocols = page.getByRole('group', { name: 'Protocols', exact: true });
	const row = (name: string) => page.getByRole('group', { name, exact: true });
	const checked = () =>
		protocols
			.getByRole('checkbox')
			.evaluateAll((boxes) =>
				boxes
					.filter((box) => (box as HTMLInputElement).checked)
					.map((box) => box.closest('label')!.textContent!.trim())
			);

	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/#/rules/lab');
	await expect(heading(page)).toHaveText('lab');
	// General carries name, switch, mode and listen type; the chain follows, then Listen and Exit.
	await expect(form(page).locator('.band > .band-title')).toHaveText([
		'General',
		'Chain',
		'Listen',
		'Exit'
	]);
	await expect(general.getByLabel('Rule name')).toHaveValue('lab');
	await expect(general.getByRole('switch', { name: 'Enabled' })).not.toBeChecked();
	await expect(general.getByRole('radio', { name: 'Proxy' })).toBeChecked();
	await expect(general.getByRole('radio', { name: 'Address' })).toBeChecked();
	await expect(protocols.locator('label')).toHaveText([
		'HTTP',
		'SOCKS5',
		'SOCKS4',
		'SSH',
		'Shadowsocks'
	]);
	await expect.poll(checked).toEqual(['HTTP', 'SOCKS5', 'SOCKS4', 'SSH']);
	await expect(row('HTTP').getByRole('checkbox', { name: 'Custom credentials' })).not.toBeChecked();
	await expect(row('HTTP').getByLabel('Username')).toHaveCount(0);

	// Keyboard: Space toggles a protocol like any checkbox; one left checked is a single-protocol port.
	await protocols.getByRole('checkbox', { name: 'HTTP' }).focus();
	await page.keyboard.press('Space');
	await protocols.getByRole('checkbox', { name: 'SOCKS4' }).uncheck();
	await protocols.getByRole('checkbox', { name: 'SSH' }).uncheck();
	await expect.poll(checked).toEqual(['SOCKS5']);
	await expect(row('HTTP')).toHaveCount(0);
	await expect(row('SOCKS5')).toHaveCount(1);
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes.at(-1)!.body).toMatchObject({
		listen: {
			username: 'demo',
			password: 'placeholder',
			protocols: [{ type: 'socks5' }]
		}
	});

	// Several schemes: SOCKS5 keeps the shared credentials, Shadowsocks gets its own password.
	await protocols.getByRole('checkbox', { name: 'HTTP' }).check();
	await protocols.getByRole('checkbox', { name: 'Shadowsocks' }).check();
	await row('Shadowsocks').getByRole('checkbox', { name: 'Custom credentials' }).check();
	await expect(row('Shadowsocks').getByLabel('Password')).toHaveAttribute('type', 'password');
	await row('Shadowsocks').getByLabel('Password').fill('ss-secret');
	await row('Shadowsocks').getByLabel('Cipher').selectOption('chacha20-ietf-poly1305');
	await expect(page.getByLabel('Password', { exact: true })).toHaveCount(2);
	await shoot(page, 'desktop-editor-protocols-en');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes.at(-1)!.body).toMatchObject({
		listen: {
			username: 'demo',
			password: 'placeholder',
			protocols: [
				{ type: 'http' },
				{ type: 'socks5' },
				{ type: 'ss', password: 'ss-secret', cipher: 'chacha20-ietf-poly1305' }
			]
		}
	});
	expect(await page.content()).not.toContain('placeholder="placeholder"');

	// Reopened from the mock, the same rows and fields come back.
	await page.reload();
	await expect(heading(page)).toHaveText('lab');
	await expect.poll(checked).toEqual(['HTTP', 'SOCKS5', 'Shadowsocks']);
	await expect(
		row('Shadowsocks').getByRole('checkbox', { name: 'Custom credentials' })
	).toBeChecked();
	await expect(row('Shadowsocks').getByLabel('Password')).toHaveValue('ss-secret');
	await expect(row('Shadowsocks').getByLabel('Cipher')).toHaveValue('chacha20-ietf-poly1305');
	await expect(
		row('SOCKS5').getByRole('checkbox', { name: 'Custom credentials' })
	).not.toBeChecked();

	// Unchecking a scheme or switching to forward keeps the draft; the body omits what is off.
	await protocols.getByRole('checkbox', { name: 'Shadowsocks' }).uncheck();
	await expect(row('Shadowsocks')).toHaveCount(0);
	await page.getByRole('radio', { name: 'Port forward' }).check();
	await expect(protocols).toHaveCount(0);
	await page.getByLabel('Target port').fill('5432');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes.at(-1)!.body).toEqual({
		name: 'lab',
		disabled: true,
		listen: { host: '127.0.0.1', port: 18100 },
		forward: { port: 5432 }
	});
	await page.getByRole('radio', { name: 'Proxy' }).check();
	await expect.poll(checked).toEqual(['HTTP', 'SOCKS5']);
	await protocols.getByRole('checkbox', { name: 'Shadowsocks' }).check();
	await expect(row('Shadowsocks').getByLabel('Password')).toHaveValue('ss-secret');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes.at(-1)!.body).toMatchObject({
		listen: {
			protocols: [
				{ type: 'http' },
				{ type: 'socks5' },
				{ type: 'ss', password: 'ss-secret', cipher: 'chacha20-ietf-poly1305' }
			]
		}
	});

	// An explicit stored list reopens as is: one scheme with its own credentials.
	await page.goto('/#/rules/ss-only');
	await expect(heading(page)).toHaveText('ss-only');
	await expect.poll(checked).toEqual(['Shadowsocks']);
	await expect(row('Shadowsocks').getByLabel('Password')).toHaveValue('placeholder');
	await expect(row('Shadowsocks').getByLabel('Cipher')).toHaveValue('chacha20-ietf-poly1305');

	// Mixed rule in Chinese on a phone with the SOCKS5 custom row open: labels, no overflow.
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto('/?lang=zh#/rules/mixed');
	await expect(heading(page)).toHaveText('mixed');
	const zhGeneral = page.getByRole('group', { name: '基本信息', exact: true });
	await expect(zhGeneral.getByRole('radio', { name: '代理' })).toBeChecked();
	await expect(zhGeneral.getByRole('radio', { name: '地址' })).toBeChecked();
	await expect(page.getByRole('group', { name: '协议', exact: true }).locator('label')).toHaveText([
		'HTTP',
		'SOCKS5',
		'SOCKS4',
		'SSH',
		'Shadowsocks'
	]);
	await expect(row('SOCKS5').getByRole('checkbox', { name: '单独凭据' })).toBeChecked();
	await expect(row('SOCKS5').getByLabel('用户名')).toHaveValue('socks-user');
	await expect(row('SOCKS5').getByLabel('密码')).toHaveValue('socks-pass');
	await expect(row('HTTP').getByRole('checkbox', { name: '单独凭据' })).not.toBeChecked();
	await row('SOCKS5').getByLabel('密码').scrollIntoViewIfNeeded();
	await shoot(page, 'mobile-editor-protocols-zh');
	await page.setViewportSize({ width: 1280, height: 800 });
	await row('SOCKS5').getByLabel('密码').scrollIntoViewIfNeeded();
	await shoot(page, 'desktop-editor-protocols-zh');
	expect(api.writes).toHaveLength(4);
});

test('protocol credentials align below their controls on desktop and stack on phones', async ({
	page,
	api
}) => {
	api.rules.push(...structuredClone(protocolRulesFixture));
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/#/rules/mixed');
	await expect(heading(page)).toHaveText('mixed');
	const protocols = page.getByRole('group', { name: 'Protocols', exact: true });
	for (const label of ['HTTP', 'SOCKS5', 'SOCKS4', 'SSH', 'Shadowsocks']) {
		await protocols.getByRole('checkbox', { name: label, exact: true }).check();
		await page
			.getByRole('group', { name: label, exact: true })
			.getByRole('checkbox', { name: 'Custom credentials' })
			.check();
	}
	for (const width of [1280, 390]) {
		await page.setViewportSize({ width, height: 844 });
		const positions = await page.evaluate(() =>
			['http', 'socks5', 'socks4', 'ssh', 'ss'].map((type) => {
				const rect = (field: string) => {
					const { x, y, width, height } = document
						.getElementById(`protocol-${type}-${field}`)!
						.getBoundingClientRect();
					return { x, y, width, height };
				};
				return {
					type,
					control: rect('custom'),
					first: rect(type === 'ss' ? 'cipher' : 'username'),
					password: type === 'socks4' ? null : rect('password')
				};
			})
		);
		for (const { type, control, first, password } of positions) {
			expect(first.y, `${type} fields below control at ${width}px`).toBeGreaterThan(
				control.y + control.height
			);
			if (!password) continue;
			expect(password.width).toBeCloseTo(first.width, 0);
			if (width === 1280) {
				expect(password.y, `${type} fields share a row`).toBeCloseTo(first.y, 0);
				expect(password.x).toBeGreaterThan(first.x + first.width);
			} else {
				expect(password.x, `${type} fields share a column`).toBeCloseTo(first.x, 0);
				expect(password.y).toBeGreaterThan(first.y + first.height);
			}
		}
		for (const label of ['HTTP', 'Shadowsocks']) {
			await page.getByRole('group', { name: label, exact: true }).scrollIntoViewIfNeeded();
			await shoot(page, `protocol-fields-${width}-${label}`);
		}
	}
});

test('listen and exit headings are spaced below the preceding separators', async ({ page }) => {
	await page.goto('/?lang=zh#/rules/lab');
	await expect(heading(page)).toHaveText('lab');
	for (const width of [1280, 390]) {
		await page.setViewportSize({ width, height: 844 });
		for (const [name, label] of [
			['listen', '监听'],
			['exit', '出口']
		]) {
			const section = page.getByRole('group', { name: label, exact: true });
			const gap = await section.evaluate((element) => {
				const title = element.querySelector(':scope > legend')!.getBoundingClientRect();
				const previous = element.previousElementSibling!.getBoundingClientRect();
				return title.top - previous.bottom;
			});
			expect(gap, `${name} heading gap at ${width}px`).toBeGreaterThanOrEqual(20);
			await section.locator(':scope > legend').scrollIntoViewIfNeeded();
			await shoot(page, `section-heading-${width}-${name}`);
		}
	}
});

test('Duplicate on a card opens a dirty copy named "<name>-copy"; once its port is free, Save creates it and the overview lists it', async ({
	page,
	api
}) => {
	await page.goto('/');
	const office = cards(page).nth(0);
	const duplicate = office.getByRole('link', { name: 'Duplicate rule' });
	await expect(duplicate).toHaveAttribute('href', '#/new?rule=office');
	await hover(page, duplicate);
	await expect(page.locator('#tooltip')).toHaveText('Duplicate rule');
	await duplicate.click();
	await expect(page).toHaveURL(/#\/new\?rule=office$/);
	await expect(heading(page)).toHaveText('New rule');
	await expect(page.getByLabel('Rule name')).toHaveValue('office-copy');
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('18097');
	await expect(unsaved(page)).toBeVisible();
	await expect(form(page).locator('[data-form-actions] button')).toHaveText([
		'Save & Apply',
		'Cancel'
	]);
	expect(await chainStages(page)).toContain('Hop 1 · exit node socks5://hop-a.example:1080');

	// The source still holds 127.0.0.1:18097: nothing is sent until the port changes.
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(page.locator('#listen-port-error')).toHaveText('Already used by rule "office".');
	await expect(page.getByLabel('Port', { exact: true })).toBeFocused();
	expect(api.writes).toEqual([]);
	await page.getByLabel('Port', { exact: true }).fill('18197');
	await expect(page.locator('#listen-port-error')).toHaveCount(0);
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page).toHaveURL(/#\/rules\/office-copy$/);
	await expect(heading(page)).toHaveText('office-copy');
	await expect(unsaved(page)).toHaveCount(0);
	expect(api.writes).toEqual([
		{
			method: 'POST',
			path: '/apis/configs/rules',
			body: {
				name: 'office-copy',
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

	await page.getByRole('navigation', { name: 'Breadcrumb' }).getByRole('link').click();
	await expect(cardNames(page)).toHaveText(['office', 'mirror', 'db-tunnel', 'lab', 'office-copy']);
	await expect(cards(page).nth(4)).toContainText('127.0.0.1:18197');
	await expect(cards(page).nth(4).getByRole('link', { name: 'Duplicate rule' })).toHaveAttribute(
		'href',
		'#/new?rule=office-copy'
	);
});

test('the editor duplicates through the router: a dirty draft asks first, a clean one opens the copy', async ({
	page,
	api
}) => {
	await page.goto('/#/rules/mirror');
	await expect(heading(page)).toHaveText('mirror');
	await page.getByLabel('Port', { exact: true }).fill('18198');
	await page.getByRole('button', { name: 'Duplicate rule' }).click();
	await expect(confirmDialog(page)).toContainText('Discard unsaved changes?');
	await confirmDialog(page).getByRole('button', { name: 'Cancel' }).click();
	await expect(page).toHaveURL(/#\/rules\/mirror$/);
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('18198');

	await page.getByLabel('Port', { exact: true }).fill('18098');
	await expect(unsaved(page)).toHaveCount(0);
	await page.getByRole('button', { name: 'Duplicate rule' }).click();
	await expect(page).toHaveURL(/#\/new\?rule=mirror$/);
	await expect(heading(page)).toHaveText('New rule');
	await expect(page.getByLabel('Rule name')).toHaveValue('mirror-copy');
	await expect(page.getByLabel('Target port')).toHaveValue('5432');
	await expect(unsaved(page)).toBeVisible();
	await expect(page.getByRole('button', { name: 'Duplicate rule' })).toHaveCount(0);
	expect(api.writes).toEqual([]);

	await page.goto('/?lang=zh#/');
	await expect(cards(page).nth(0).getByRole('link', { name: '复制规则' })).toHaveAttribute(
		'href',
		'#/new?rule=office'
	);
});

test('a taken name, a "/" in it or a taken listen address is refused inline before any request', async ({
	page,
	api
}) => {
	const name = page.getByLabel('Rule name');
	const port = page.getByLabel('Port', { exact: true });
	await page.goto('/#/rules/mirror');
	await expect(heading(page)).toHaveText('mirror');
	await name.fill('office');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page.locator('#rule-name-error')).toHaveText(
		'A rule with this name already exists.'
	);
	await expect(name).toHaveAttribute('aria-invalid', 'true');
	await expect(name).toBeFocused();
	await expect(unsaved(page)).toBeVisible();
	expect(api.writes).toEqual([]);
	await name.fill('office/2');
	await expect(page.locator('#rule-name-error')).toHaveCount(0);
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page.locator('#rule-name-error')).toHaveText('Names must not contain "/".');

	// Its own name is no clash; office's port is, and so is the web UI's.
	await name.fill('mirror');
	await port.fill('18097');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page.locator('#listen-port-error')).toHaveText('Already used by rule "office".');
	await expect(port).toBeFocused();
	await port.fill('1088');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page.locator('#listen-port-error')).toHaveText('Already used by the web UI.');
	expect(api.writes).toEqual([]);

	// lab is disabled: its port is free, and the save goes through.
	await port.fill('18100');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(writes(api)).toEqual(['PUT /apis/configs/rules/mirror']);
	expect(api.writes[0].body).toMatchObject({ name: 'mirror', listen: { port: 18100 } });

	await page.goto('/?lang=zh#/rules/mirror');
	await page.getByLabel('规则名称').fill('office');
	await page.getByLabel('端口', { exact: true }).fill('18097');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page.locator('#rule-name-error')).toHaveText('已有同名规则。');
	await expect(page.locator('#listen-port-error')).toHaveText('已被规则“office”使用。');
	expect(api.writes).toHaveLength(1);
});

test('a hop URL without a scheme, or an added hop left blank, is marked inline and focused; nothing is sent until fixed', async ({
	page,
	api
}) => {
	await page.goto('/#/rules/office');
	await expect(heading(page)).toHaveText('office');
	const rows = page.locator('input[id^="exit-url-"]');
	const first = rows.first();
	await first.fill('127.0.0.1:1080');
	await exitEditor(page).getByRole('button', { name: 'Add hop' }).click();
	await expect(rows).toHaveCount(4);
	await page.keyboard.press('ControlOrMeta+s');
	const errorOf = async (row: Locator) => page.locator(`#${await row.getAttribute('id')}-error`);
	await expect(first).toHaveAttribute('aria-invalid', 'true');
	await expect(await errorOf(first)).toHaveText('Include the scheme (e.g. socks5://host:1080).');
	await expect(first).toBeFocused();
	await expect(rows.nth(1)).not.toHaveAttribute('aria-invalid', /./);
	await expect(rows.nth(3)).toHaveAttribute('aria-invalid', 'true');
	await expect(await errorOf(rows.nth(3))).toHaveText('Enter a proxy URL.');
	expect(api.writes).toEqual([]);

	await first.fill('socks5://127.0.0.1:1080');
	await expect(first).not.toHaveAttribute('aria-invalid', /./);
	await expect(await errorOf(first)).toHaveCount(0);
	await exitEditor(page)
		.locator('li[id^="exit-hop-"]')
		.nth(2)
		.getByRole('button', { name: 'Delete hop' })
		.click();
	await expect(rows).toHaveCount(3);
	await page.keyboard.press('ControlOrMeta+s');
	await expect(unsaved(page)).toHaveCount(0);
	expect(writes(api)).toEqual(['PUT /apis/configs/rules/office']);
	expect(api.writes[0].body).toMatchObject({
		forward: {
			way: [
				{ lb: ['socks5://127.0.0.1:1080'] },
				{ lb: ['ssh://ops@bastion.example:22', 'ssh://ops@bastion-2.example:22'] }
			]
		}
	});
});

test.describe('screenshots', () => {
	for (const [viewport, label] of [
		[{ width: 1280, height: 800 }, 'desktop'],
		[{ width: 375, height: 812 }, 'mobile']
	] as const) {
		test(`${label} overview, editor and builder in en light and zh dark`, async ({ page }) => {
			await page.setViewportSize(viewport);
			await page.goto('/');
			await expect(cards(page)).toHaveCount(4);
			await shoot(page, `${label}-overview-en-light`);
			await page.goto('/#/rules/office');
			await expect(heading(page)).toHaveText('office');
			await shoot(page, `${label}-editor-en-light`);
			await page
				.locator('input[id^="exit-url-"]')
				.first()
				.locator('..')
				.getByRole('button', { name: 'Build…' })
				.click();
			await expect(builder(page)).toBeVisible();
			await page.screenshot({
				path: test.info().outputPath(`${label}-builder-en-light.png`),
				animations: 'disabled'
			});
			await page.keyboard.press('Escape');

			await page.goto('/?lang=zh#/');
			await expect(cards(page)).toHaveCount(4);
			await page.evaluate(() => localStorage.setItem('jumpway.theme', 'dark'));
			await page.reload();
			await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
			await expect(cards(page)).toHaveCount(4);
			await shoot(page, `${label}-overview-zh-dark`);
			await page.goto('/?lang=zh#/rules/office');
			await expect(heading(page)).toHaveText('office');
			await shoot(page, `${label}-editor-zh-dark`);
			await page
				.locator('input[id^="exit-url-"]')
				.first()
				.locator('..')
				.getByRole('button', { name: '拼装…' })
				.click();
			await expect(builder(page)).toBeVisible();
			await page.screenshot({
				path: test.info().outputPath(`${label}-builder-zh-dark.png`),
				animations: 'disabled'
			});
		});
	}
});
