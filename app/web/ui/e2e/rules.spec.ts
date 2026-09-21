import type { Locator, Page } from '@playwright/test';
import { rulesFixture } from './fixtures/api.ts';
import { expect, test } from './fixtures/test.ts';
import { count, type MockApi } from './mockApi.ts';

// Rule cards on the overview, editor, hop editor, URL builder and the dirty-leave guards, in real
// Chrome against the stateful mock (writes recorded, failures injectable).

const heading = (page: Page) => page.getByRole('heading', { level: 1 });
const cards = (page: Page) => page.getByRole('main').getByRole('article');
const cardNames = (page: Page) => cards(page).locator('header a');
const newRule = (page: Page) =>
	page.getByRole('main').locator('header').getByRole('link', { name: 'New rule' });
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
	await expect(page.locator('[data-kpi="rules"]')).toHaveText('1/3');
	expect(writes(api)).toEqual(['DELETE /apis/configs/rules/mirror']);
	await expect(page.getByRole('status').filter({ hasText: 'Rule deleted.' })).toBeVisible();
	await expect(page).toHaveURL(/#\/$/);
	expect(count(api, 'GET /apis/configs/status')).toBeGreaterThanOrEqual(2);
	expect(count(api, 'GET /apis/configs/rules')).toBeGreaterThanOrEqual(2);

	await newRule(page).click();
	await expect(page).toHaveURL(/#\/new$/);
	await expect(heading(page)).toHaveText('New rule');
});

test('an empty overview and an unknown rule show their empty states', async ({
	page,
	api,
	failures
}) => {
	api.rules = [];
	await page.goto('/#/rules');
	await expect(page).toHaveURL(/#\/$/);
	await expect(page.getByRole('main')).toContainText('No rules yet.');
	await expect(page.getByRole('main').getByRole('link', { name: 'New rule' })).toHaveCount(1);
	await expect(newRule(page)).toBeVisible();

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

	await page.getByLabel('Rule name').fill('lab/2');
	await page.getByLabel('Port', { exact: true }).fill('18101');
	await page.getByLabel('Username').fill('demo');
	await page.getByLabel('Password', { exact: true }).fill('placeholder');
	await expect(unsaved(page)).toBeVisible();
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(page).toHaveURL(/#\/rules\/lab%2F2$/);
	await expect(heading(page)).toHaveText('lab/2');
	expect(api.writes).toEqual([
		{
			method: 'POST',
			path: '/apis/configs/rules',
			body: {
				name: 'lab/2',
				listen: { host: '127.0.0.1', port: 18101, username: 'demo', password: 'placeholder' },
				forward: {}
			}
		}
	]);
	await expect(page.getByRole('status').filter({ hasText: 'Saved and applied.' })).toBeVisible();
	await expect(unsaved(page)).toHaveCount(0);
	await expect(page.getByLabel('Rule name')).toHaveValue('lab/2');
	await expect(page.getByRole('button', { name: 'Delete' })).toBeVisible();

	// Rename: PUT to the old name with the new body, then the route follows.
	await page.getByLabel('Rule name').fill('lab-2');
	await page.keyboard.press('ControlOrMeta+s');
	await expect(page).toHaveURL(/#\/rules\/lab-2$/);
	await expect(heading(page)).toHaveText('lab-2');
	expect(writes(api)).toEqual(['POST /apis/configs/rules', 'PUT /apis/configs/rules/lab%2F2']);
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

test('Cancel sits between Save & Apply and Delete: clean leaves at once, dirty asks, a save in flight holds it, Enter never triggers it', async ({
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
	api.fail.set('PUT /apis/configs/rules/draft', 'rules[4].listen.port 18097 is already in use');
	await page.getByLabel('Port', { exact: true }).fill('18097');
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
		listen: { host: '127.0.0.1', port: 18100, username: 'demo', password: 'placeholder' },
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
	await page.getByLabel('Port', { exact: true }).fill('18097');
	await page.keyboard.press('ControlOrMeta+s');
	const banner = page.getByRole('main').getByRole('alert');
	await expect(banner).toContainText('invalid proxy URL "socks5://xxxxx@bad:port"');
	expect(await page.content()).not.toContain('demo:placeholder@bad');
	await expect(unsaved(page)).toBeVisible();
	await expect(page).toHaveURL(/#\/rules\/mirror$/);
	await expect(page.getByLabel('Port', { exact: true })).toHaveValue('18097');
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

test('the card footer keeps its three 32px actions and long rates inside the card at 375, 820 and 1280; the delete question fits a phone', async ({
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
			return {
				overflow: main.scrollWidth - main.clientWidth,
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
		for (const card of geometry.cards) {
			expect(card.sizes, `${width}px`).toEqual(['32x32', '32x32', '32x32']);
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

test.describe('screenshots', () => {
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
