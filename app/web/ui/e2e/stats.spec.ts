import type { Locator, Page } from '@playwright/test';
import { hostsTotals, snapshotFixture } from './fixtures/api.ts';
import { expect, test } from './fixtures/test.ts';
import { count, type MockApi } from './mockApi.ts';

// Statistics, Hosts and Connections in real Chrome against the mocked snapshot: fixture values,
// keyed rows across polls, expansion, filters, sorting, disconnect, reset, errors and layout.

const FIXED_NOW = new Date('2026-09-19T09:00:00Z');

const heading = (page: Page) => page.getByRole('heading', { level: 1 });
const ruleRows = (page: Page) => page.locator('article[data-rule]');
const hostRows = (page: Page) => page.locator('article[data-host]');
// Connections are the same kind of item at every width.
const connectionRows = (page: Page) => page.locator('article[data-connection]');
const connectionOrder = (page: Page) =>
	connectionRows(page).evaluateAll((rows) =>
		rows.map((row) => (row as HTMLElement).dataset.connection)
	);
const confirmDialog = (page: Page) =>
	page.locator('dialog[open][aria-describedby="confirm-message"]');
const tooltip = (page: Page) => page.locator('#tooltip');
// The values of one band in its reading order: column by column, upload above download
// (pass the `[data-traffic]` element itself).
const metrics = (band: Locator) => band.locator('[data-metric]');
// The expanded content of an item, located through its toggle's aria-controls.
const detailsOf = async (page: Page, toggle: Locator) =>
	page.locator('#' + (await toggle.getAttribute('aria-controls')));
const writes = (api: MockApi) => api.writes.map((write) => `${write.method} ${write.path}`);
const expected400 = (entry: string) =>
	entry.startsWith('http 400') || entry.includes('status of 400');

// Tooltips hide on scroll; scrolling the target into view first keeps the hover's tooltip.
async function hover(page: Page, locator: Locator) {
	await locator.scrollIntoViewIfNeeded();
	await page.waitForTimeout(100);
	await locator.hover();
}

// Connection items: identity, client, traffic and duration are fully visible, every item names
// each of the eight traffic values exactly once with a visible label and value in four columns
// of two, and every expanded detail adds only what the item lacks: the start time and the path,
// no metric, connection counts, latency, failures or client.
async function expectConnectionsFit(page: Page, name: string) {
	await expectMetricsFit(page, name);
	const report = await page.evaluate(() => {
		const out: string[] = [];
		const main = document.getElementById('main')!;
		const visible = (element: Element) => {
			const box = element.getBoundingClientRect();
			return box.width > 0 && box.height > 0 && getComputedStyle(element).visibility !== 'hidden';
		};
		for (const row of main.querySelectorAll<HTMLElement>('[data-connection]')) {
			const id = row.dataset.connection;
			const details = document.getElementById(`connection-details-${id}`);
			const metrics = Array.from(row.querySelectorAll<HTMLElement>('[data-metric]')).filter(
				(element) => !details?.contains(element)
			);
			const keys = metrics.map((element) => element.dataset.metric);
			if (keys.length !== 8 || new Set(keys).size !== 8) out.push(`${id} row metrics ${keys}`);
			for (const metric of metrics) {
				const label = metric.closest('[data-column]')?.querySelector('dt');
				if (!label || !visible(label) || !visible(metric) || !metric.textContent?.trim())
					out.push(`${id} ${metric.dataset.metric} lacks a visible label or value`);
			}
			const columns = Array.from(row.querySelectorAll('[data-traffic] [data-column]'))
				.filter((column) => !details?.contains(column))
				.map((column) => column.querySelectorAll('[data-metric]').length);
			if (columns.join() !== '2,2,2,2') out.push(`${id} values per column ${columns}`);
			if (!details) continue;
			if (details.querySelector('[data-metric], [data-traffic]'))
				out.push(`${details.id} repeats traffic`);
			if (details.querySelector('[data-connections], [data-latency], [data-failures]'))
				out.push(`${details.id} shows connection meta`);
			if (details.querySelector('[data-client], [data-client-address]'))
				out.push(`${details.id} repeats the client`);
			const facts = details.querySelectorAll('[data-connection-facts] dt').length;
			if (facts !== 2) out.push(`${details.id} has ${facts} facts`);
		}
		return out.slice(0, 8);
	});
	expect(report, name).toEqual([]);
}

// Every label and value of the bands and connection items is fully visible: nothing is clipped
// inside its box and nothing sticks out of #main's width. Every rule and host item, collapsed or
// not, shows its own eight traffic values with visible labels plus connections, latency and
// failures outside its expansion.
async function expectMetricsFit(page: Page, name: string) {
	const clipped = await page.evaluate(() => {
		const main = document.getElementById('main')!;
		const limit = main.getBoundingClientRect().right + 1;
		return Array.from(
			main.querySelectorAll(
				'[data-traffic] dt, [data-traffic] dd, [data-connections] dd, [data-latency] dd, [data-failures] dd, [data-address], [data-endpoint], [data-endpoints], [data-host-name], [data-usage] a, [data-use], [data-mode], [data-target-address], [data-via], [data-evicted], [data-connection] [data-target], [data-connection] [data-client], [data-connection] [data-rule-link], [data-connection] [data-metric], [data-connection] [data-duration], [data-connection-facts] dt, [data-connection-facts] dd'
			)
		)
			.filter((element) => {
				const box = element.getBoundingClientRect();
				return (
					box.width > 0 && (element.scrollWidth > element.clientWidth + 1 || box.right > limit)
				);
			})
			.slice(0, 8)
			.map(
				(element) =>
					`${element.tagName}[${element.getAttribute('data-metric') ?? element.className.toString().slice(0, 40)}] "${element.textContent?.trim().slice(0, 40)}" scroll=${element.scrollWidth}/${element.clientWidth} right=${Math.round(element.getBoundingClientRect().right)}`
			);
	});
	expect(clipped, name).toEqual([]);
	const items = await page.evaluate(() => {
		const out: string[] = [];
		const visible = (element: Element | null | undefined) => {
			const box = element?.getBoundingClientRect();
			return !!box && box.width > 0 && box.height > 0 && !!element?.textContent?.trim();
		};
		for (const item of document.querySelectorAll<HTMLElement>(
			'article[data-rule], article[data-host]'
		)) {
			const id = item.dataset.rule ?? item.dataset.host;
			const controls = item.querySelector('button[aria-controls]')?.getAttribute('aria-controls');
			const details = controls ? document.getElementById(controls) : null;
			const own = (selector: string) =>
				Array.from(item.querySelectorAll<HTMLElement>(selector)).filter(
					(element) => !details?.contains(element)
				);
			const metrics = own('[data-metric]');
			const keys = metrics.map((element) => element.dataset.metric);
			if (keys.length !== 8 || new Set(keys).size !== 8) out.push(`${id} own metrics ${keys}`);
			for (const metric of metrics) {
				if (!visible(metric.closest('[data-column]')?.querySelector('dt')) || !visible(metric))
					out.push(`${id} ${metric.dataset.metric} lacks a visible label or value`);
			}
			for (const meta of ['[data-connections]', '[data-latency]', '[data-failures]']) {
				const cells = own(meta);
				if (cells.length !== 1 || !visible(cells[0].querySelector('dd')))
					out.push(`${id} ${meta} ×${cells.length}`);
			}
		}
		return out.slice(0, 8);
	});
	expect(items, name).toEqual([]);
}

// The matrix holds still whatever the numbers: within every item's own band the upload and
// download values of one counter start at the same x, every column's edges are the same in
// every item on the page, and the connections / latency / failures cells sit on the first three
// of those columns. Measured on the items' own bands (not the nested hop and endpoint bands).
async function expectColumnsAligned(page: Page, name: string) {
	const report = await page.evaluate(() => {
		const out: string[] = [];
		const round = (value: number) => Math.round(value * 2) / 2;
		const edges = new Map<string, { left: number; right: number; item: string }>();
		const bands = Array.from(
			document.querySelectorAll<HTMLElement>(
				'article[data-rule] [data-traffic], article[data-host] [data-traffic], article[data-connection] [data-traffic]'
			)
		).filter(
			(band) => !band.closest('[data-hop-stats], [data-url], [data-target], [data-endpoint-row]')
		);
		if (bands.length < 2) out.push(`only ${bands.length} bands`);
		for (const band of bands) {
			const item = band.closest<HTMLElement>('article')!;
			const id = item.dataset.rule ?? item.dataset.host ?? item.dataset.connection ?? '?';
			const columns = Array.from(band.querySelectorAll<HTMLElement>('[data-column]'));
			if (columns.length !== 4) out.push(`${id} has ${columns.length} columns`);
			const lefts: number[] = [];
			for (const column of columns) {
				const key = column.dataset.column!;
				const values = Array.from(column.querySelectorAll<HTMLElement>('[data-metric]'));
				const starts = values.map((value) => round(value.getBoundingClientRect().left));
				if (new Set(starts).size !== 1) out.push(`${id} ${key} values start at ${starts}`);
				const box = column.getBoundingClientRect();
				lefts.push(round(box.left));
				const seen = edges.get(key);
				if (!seen) edges.set(key, { left: round(box.left), right: round(box.right), item: id });
				else if (Math.abs(seen.left - box.left) > 0.5 || Math.abs(seen.right - box.right) > 0.5)
					out.push(
						`${id} ${key} spans ${round(box.left)}..${round(box.right)}, ${seen.item} ${seen.left}..${seen.right}`
					);
			}
			const meta = ['connections', 'latency', 'failures']
				.map(
					(cell) =>
						[cell, item.querySelector<HTMLElement>(`[data-stats] > dl > [data-${cell}]`)] as const
				)
				.filter((entry): entry is readonly [string, HTMLElement] => entry[1] !== null);
			meta.forEach(([cell, element], index) => {
				const left = round(element.getBoundingClientRect().left);
				if (index < lefts.length && left !== lefts[index])
					out.push(`${id} ${cell} at ${left}, column at ${lefts[index]}`);
			});
		}
		return out.slice(0, 8);
	});
	expect(report, name).toEqual([]);
}

async function expectNoDocumentOverflow(page: Page, name: string) {
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
}

// The header cells of every item on the page share their column edges: identity, detail, the
// summary (where the page has one) and the chevron start and end at the same x in every item,
// whatever the names, durations and actions inside them. Connection clients start together and
// their durations line up on the edge nearest the action; the actions share both edges.
async function expectHeadersAligned(page: Page, name: string) {
	const report = await page.evaluate(() => {
		const out: string[] = [];
		const round = (value: number) => Math.round(value * 2) / 2;
		const items = Array.from(
			document.querySelectorAll<HTMLElement>(
				'article[data-rule], article[data-host], article[data-connection]'
			)
		);
		if (items.length < 2) out.push(`only ${items.length} items`);
		const seen = new Map<string, { edge: number; item: string }>();
		const check = (key: string, id: string, edge: number) => {
			const first = seen.get(key);
			if (!first) seen.set(key, { edge: round(edge), item: id });
			else if (Math.abs(first.edge - edge) > 0.5)
				out.push(`${id} ${key} ${round(edge)}, ${first.item} ${first.edge}`);
		};
		for (const item of items) {
			const id = item.dataset.rule ?? item.dataset.host ?? item.dataset.connection ?? '?';
			const cells: Record<string, HTMLElement | null> = {};
			for (const cell of ['identity', 'detail', 'summary', 'toggle']) {
				cells[cell] = item.querySelector<HTMLElement>(`[data-header-${cell}]`);
				if (!cells[cell]) {
					if (cell !== 'summary') out.push(`${id} lacks the ${cell} cell`);
					continue;
				}
				const box = cells[cell]!.getBoundingClientRect();
				check(`${cell} left`, id, box.left);
				check(`${cell} right`, id, box.right);
			}
			const client = item.querySelector('[data-client]');
			if (client) check('client left', id, client.getBoundingClientRect().left);
			const duration = item.querySelector('[data-duration]');
			const action = cells.summary?.querySelector('button');
			if (duration && cells.summary && cells.identity) {
				const box = duration.getBoundingClientRect();
				const summary = cells.summary.getBoundingClientRect();
				const identity = cells.identity.getBoundingClientRect();
				const ownRow = summary.top >= identity.bottom - 1 || identity.top >= summary.bottom - 1;
				check(ownRow ? 'duration left' : 'duration right', id, ownRow ? box.left : box.right);
			}
			if (action) {
				check('action left', id, action.getBoundingClientRect().left);
				check('action right', id, action.getBoundingClientRect().right);
			}
		}
		return out.slice(0, 8);
	});
	expect(report, name).toEqual([]);
}

// Labels squeezed to a character per line are neither clipped nor sticking out, so fitting is
// not enough: each matched label stays within its line budget, its text lines lie inside its own
// box and #main, and the three areas of a collapsed header (identity, detail, summary) never
// overlap.
async function expectHeadersReadable(page: Page, name: string, maxLines: Record<string, number>) {
	const problems = await page.evaluate((maxLines) => {
		const out: string[] = [];
		const main = document.getElementById('main')!.getBoundingClientRect();
		const lines = (element: Element) => {
			const style = getComputedStyle(element);
			const lineHeight = parseFloat(style.lineHeight) || parseFloat(style.fontSize) * 1.5;
			return Math.round(element.getBoundingClientRect().height / lineHeight);
		};
		for (const [selector, max] of Object.entries(maxLines)) {
			for (const element of document.querySelectorAll(selector)) {
				const box = element.getBoundingClientRect();
				if (box.width === 0) continue;
				const label = `${selector} "${element.textContent?.trim().slice(0, 40)}"`;
				if (lines(element) > max) out.push(`${label} ${lines(element)} lines > ${max}`);
				if (box.right > main.right + 1 || box.left < main.left - 1)
					out.push(`${label} outside main ${Math.round(box.left)}..${Math.round(box.right)}`);
				const range = document.createRange();
				range.selectNodeContents(element);
				const outside = Array.from(range.getClientRects()).find(
					(rect) => rect.width > 0 && (rect.right > box.right + 1 || rect.left < box.left - 1)
				);
				if (outside) out.push(`${label} text sticks out to ${Math.round(outside.right)}`);
			}
		}
		for (const article of document.querySelectorAll(
			'article[data-rule], article[data-host], article[data-connection]'
		)) {
			const areas = Array.from(article.firstElementChild!.children).map((area) =>
				area.getBoundingClientRect()
			);
			for (let first = 0; first < areas.length; first++) {
				for (let second = first + 1; second < areas.length; second++) {
					const [a, b] = [areas[first], areas[second]];
					if (
						a.left < b.right - 1 &&
						b.left < a.right - 1 &&
						a.top < b.bottom - 1 &&
						b.top < a.bottom - 1
					)
						out.push(`${article.getAttribute('aria-label')} areas ${first}/${second} overlap`);
				}
			}
		}
		return out.slice(0, 8);
	}, maxLines);
	expect(problems, name).toEqual([]);
}

test('#/stats shows fixture values per rule; expanding reveals the full band, true hop peaks, URL tooltips and targets', async ({
	page,
	api
}) => {
	await page.clock.setFixedTime(FIXED_NOW);
	await page.goto('/#/stats');
	await expect(heading(page)).toHaveText('Rule Traffic');
	await expect(page).toHaveTitle('Rule Traffic · JumpWay');
	await expect(ruleRows(page)).toHaveCount(4);
	await expect(page.getByRole('main').locator('table')).toHaveCount(0);
	await expect(page.getByRole('main')).toContainText('Since');
	await expect(
		page.getByRole('main').getByRole('link', { name: 'Prometheus metrics' })
	).toHaveAttribute('href', '/metrics');

	// Collapsed: identity, state, mode, address and the full band — four labelled columns of
	// upload over download, then connections, latency and failures.
	const office = ruleRows(page).nth(0);
	await expect(office.locator('[data-state]')).toHaveText('Running');
	await expect(office.locator('[data-mode]')).toHaveText('Proxy');
	await expect(office.locator('[data-address]')).toHaveText('127.0.0.1:18097');
	const block = office.locator('[data-stats]').first();
	const band = block.locator('[data-traffic]');
	await expect(block.locator('dt')).toHaveText([
		'now',
		'peak',
		'total',
		'last',
		/Connections\s+Active \/ total/,
		/Latency\s+Last \/ average/,
		/Failures\s+Failed \/ attempted/
	]);
	await expect(metrics(band)).toHaveText([
		'12.0 KB/s',
		'1.0 MB/s',
		'64.0 KB/s',
		'4.0 MB/s',
		'5.0 MB',
		'100.0 MB',
		'—',
		'—'
	]);
	await expect(band.locator('[data-column="total"] [data-metric]')).toHaveText([
		'5.0 MB',
		'100.0 MB'
	]);
	await expect(block.locator('[data-connections] a')).toHaveText('3 / 120');
	await expect(block.locator('[data-connections] a')).toHaveAttribute(
		'href',
		'#/connections?rule=office'
	);
	await expect(block.locator('[data-latency] dd')).toHaveText('41.2 ms / 38.7 ms');
	await expect(block.locator('[data-failures] dd')).toHaveText('2 / 118');
	const mirror = ruleRows(page).nth(1);
	await expect(mirror.locator('[data-mode]')).toHaveText('Port forward');
	await expect(mirror.locator('[data-address]')).toHaveText(
		/127\.0\.0\.1:18098\s+→ 10\.0\.0\.5:5432/
	);
	await expect(ruleRows(page).nth(2).locator('[data-state]')).toHaveText('Retrying (3)');
	await expect(ruleRows(page).nth(2).locator('[data-mode]')).toHaveText('Port forward · remote');
	await expect(ruleRows(page).nth(2).locator('[data-latency] dd')).toHaveText('— / —');
	await expect(ruleRows(page).nth(3).locator('[data-state]')).toHaveText('Disabled');
	await expect(metrics(ruleRows(page).nth(3).locator('[data-traffic]'))).toHaveText(
		Array(8).fill('—')
	);
	await expectMetricsFit(page, 'stats-collapsed');
	await expectColumnsAligned(page, 'stats-collapsed');

	// The toggle renames itself on expansion, so it is located by its aria-controls. It holds
	// no link: the rule name stays a link beside it.
	const toggle = office.locator('button[aria-controls]');
	await expect(toggle).toHaveAccessibleName('Expand: office');
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	await expect(toggle.locator('a')).toHaveCount(0);
	await expect(office.getByRole('link', { name: 'office' })).toHaveAttribute(
		'href',
		'#/stats?rule=office'
	);
	await toggle.click();
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(toggle).toHaveAccessibleName('Collapse: office');
	const details = await detailsOf(page, toggle);
	await expect(details).toBeVisible();
	// The expansion is the chain: two hops, three URLs and three targets carry statistics, nothing else does.
	await expect(details.locator('[data-stats]')).toHaveCount(8);
	expect(
		await details
			.locator('[data-metric]')
			.evaluateAll((all) =>
				all.every((element) => element.closest('[data-hop-stats], [data-url], [data-target]'))
			)
	).toBe(true);
	await expect(metrics(band)).toHaveCount(8);
	await expectMetricsFit(page, 'stats-expanded');

	const stages = details.getByRole('list', { name: 'Chain' }).locator('> li');
	await expect(stages.locator('> div > p:first-child')).toHaveText([
		'Clients',
		'This machine',
		'Hop 2 · entry node · dialed from this machine',
		'Hop 1 · exit node',
		'Targets'
	]);
	const entry = stages.nth(2);
	await expect(entry.locator('[data-parent]')).toHaveText('via this machine');
	// The hop peak is the server's measured peak (4.1 MB/s), not the 5.7 MB/s sum of its URL peaks.
	await expect(entry.locator('[data-hop-stats] [data-metric="peak_rate_down"]')).toHaveText(
		'4.1 MB/s'
	);
	await expect(entry.locator('[data-url] [data-endpoint]')).toHaveText([
		'ssh://bastion.example:22',
		'ssh://bastion-2.example:22'
	]);
	await expect(entry.locator('[data-url]').nth(1).locator('[data-metric="last_down"]')).toHaveText(
		'15 min ago'
	);
	await expect(entry.locator('[data-url]').nth(1).locator('[data-connections] dd')).toHaveText(
		'1 / 3'
	);
	await hover(page, entry.locator('[data-url] [data-endpoint]').nth(0));
	await expect(tooltip(page)).toHaveText('ssh://xxxxx@bastion.example:22');
	await expect(stages.nth(3).locator('[data-parent]')).toHaveText('via Hop 2');
	await expect(stages.nth(3).locator('[data-endpoint]')).toHaveText('socks5://hop-a.example:1080');
	expect(await page.content()).not.toContain('demo:placeholder');
	await expect(stages.nth(4)).toContainText('3 distinct targets');
	await expect(stages.nth(4).getByRole('link', { name: '3 connections' })).toHaveAttribute(
		'href',
		'#/connections?rule=office'
	);
	// The targets themselves: most recently active first, the never-active one last, each with
	// the hop it was reached through and its own counters; three older ones were dropped.
	await expect(stages.nth(4).locator('[data-evicted]')).toHaveText(
		/3 older targets were dropped from this list/
	);
	const targets = stages.nth(4).locator('[data-target]');
	await expect(targets.locator('[data-target-address]')).toHaveText([
		'cdn.example.net:443',
		'example.com:443',
		'10.1.2.3:8080'
	]);
	await expect(targets.locator('[data-via]')).toHaveText([
		'via ssh://bastion-2.example:22',
		'via ssh://bastion.example:22'
	]);
	await expect(targets.nth(2).locator('[data-via]')).toHaveCount(0);
	await hover(page, targets.nth(1).locator('[data-via]'));
	await expect(tooltip(page)).toHaveText('ssh://xxxxx@bastion.example:22');
	await expect(targets.nth(1).locator('[data-metric="down"]')).toHaveText('57.2 MB');
	await expect(targets.nth(1).locator('[data-failures] dd')).toHaveText('2 / 80');
	await expect(targets.nth(0).locator('[data-latency] dd')).toHaveText('22.0 ms / 24.5 ms');
	await expect(stages.nth(4).locator('[data-targets-more]')).toHaveCount(0);
	await hover(page, stages.nth(4).getByRole('button', { name: 'About Targets' }));
	await expect(tooltip(page)).toContainText('at most 1000 targets');
	expect(await page.content()).not.toContain('ops@');

	// Items and the expansion survive polls unchanged; a changed value shows up within a poll.
	const marked = await office.evaluate((row) => ((row as HTMLElement).dataset.marker = 'kept'));
	expect(marked).toBe('kept');
	api.snapshot.rules![0].stats.rate_up = 24_576;
	await expect(band.locator('[data-metric="rate_up"]')).toHaveText('24.0 KB/s');
	await expect(office).toHaveAttribute('data-marker', 'kept');
	await expect(details).toBeVisible();

	// The band sits below the toggle's surface: clicking it toggles nothing.
	const bandBox = (await band.boundingBox())!;
	await page.mouse.click(bandBox.x + bandBox.width - 4, bandBox.y + bandBox.height / 2);
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');

	// Keyboard: Space on the toggle collapses, Enter re-opens; several items stay open at once.
	await toggle.focus();
	await page.keyboard.press('Space');
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	await expect(details).toHaveCount(0);
	await page.keyboard.press('Enter');
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await mirror.locator('button[aria-controls]').click();
	await expect(mirror.locator('button[aria-controls]')).toHaveAttribute('aria-expanded', 'true');
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	// Clicking the header surface (not the chevron) toggles too: the button stretches over it.
	const surface = (await mirror.locator('[data-address]').boundingBox())!;
	await page.mouse.click(surface.x + surface.width / 2, surface.y + surface.height / 2);
	await expect(mirror.locator('button[aria-controls]')).toHaveAttribute('aria-expanded', 'false');

	// The rule links stay inside statistics: only the query changes, nothing remounts.
	await mirror.getByRole('link', { name: 'mirror' }).click();
	await expect(page).toHaveURL(/#\/stats\?rule=mirror$/);
	await expect(mirror.getByRole('button', { name: 'Collapse: mirror' })).toBeFocused();
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	expect(count(api, 'GET /apis/configs/rules')).toBe(1);
});

test('#/stats search narrows the rules by name, address or target and clearing restores them; sorting by a counter reorders and the direction button flips it', async ({
	page,
	api
}) => {
	await page.goto('/#/stats');
	const names = () =>
		ruleRows(page).evaluateAll((rows) => rows.map((row) => (row as HTMLElement).dataset.rule));
	await expect(ruleRows(page)).toHaveCount(4);
	await expect(page.locator('[data-rule-count]')).toHaveCount(0);
	const search = page.getByRole('searchbox', { name: 'Search' });
	await expect(page.getByRole('button', { name: 'Clear search' })).toHaveCount(0);
	await search.fill('tun');
	await expect(ruleRows(page)).toHaveCount(1);
	await expect.poll(names).toEqual(['db-tunnel']);
	await expect(page.locator('[data-rule-count]')).toHaveText('1 of 4 rules');
	// The forward target is searched too.
	await search.fill('5432');
	await expect.poll(names).toEqual(['mirror', 'db-tunnel']);
	await search.fill('zzz');
	await expect(ruleRows(page)).toHaveCount(0);
	await expect(page.getByRole('main')).toContainText('No matches');
	await expect(page.locator('[data-rule-count]')).toHaveText('0 of 4 rules');
	await page.getByRole('button', { name: 'Clear search' }).click();
	await expect(search).toHaveValue('');
	await expect(ruleRows(page)).toHaveCount(4);
	await expect(page.locator('[data-rule-count]')).toHaveCount(0);
	await expect.poll(names).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);

	// Configured order by default; the download total reorders, ascending first, then flipped.
	const sortBy = page.getByRole('combobox', { name: 'Sort by' });
	const direction = page.getByRole('button', { name: /^(Ascending|Descending)$/ });
	await expect(sortBy).toHaveValue('configured');
	await expect(direction).toHaveAccessibleName('Ascending');
	await sortBy.selectOption('down');
	await expect.poll(names).toEqual(['db-tunnel', 'lab', 'mirror', 'office']);
	await direction.click();
	await expect(direction).toHaveAccessibleName('Descending');
	await expect.poll(names).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	// A poll that changes the counters moves the rows, keeping their elements and expansion.
	const mirror = page.locator('article[data-rule="mirror"]');
	await mirror.getByRole('button', { name: 'Expand: mirror' }).click();
	await mirror.evaluate((row) => ((row as HTMLElement).dataset.marker = 'kept'));
	api.snapshot.rules![1].stats.down = 500_000_000;
	await expect.poll(names).toEqual(['mirror', 'office', 'db-tunnel', 'lab']);
	await expect(mirror).toHaveAttribute('data-marker', 'kept');
	await expect(mirror.locator('button[aria-controls]')).toHaveAttribute('aria-expanded', 'true');
	await sortBy.selectOption('dial_failures');
	await expect.poll(names).toEqual(['db-tunnel', 'office', 'mirror', 'lab']);
	await sortBy.selectOption('configured');
	await expect.poll(names).toEqual(['lab', 'db-tunnel', 'mirror', 'office']);
	await direction.click();
	await expect.poll(names).toEqual(['office', 'mirror', 'db-tunnel', 'lab']);
	// The controls stay usable at a phone width.
	await page.setViewportSize({ width: 375, height: 800 });
	await expect(search).toBeVisible();
	await expect(sortBy).toBeVisible();
	await expect(direction).toBeVisible();
	await expectNoDocumentOverflow(page, 'stats-controls-375');
});

test('#/hosts search matches host names and endpoint labels; sorting by host name orders alphabetically', async ({
	page
}) => {
	await page.goto('/#/hosts');
	const names = () =>
		hostRows(page).evaluateAll((rows) => rows.map((row) => (row as HTMLElement).dataset.host));
	await expect(hostRows(page)).toHaveCount(4);
	await expect(page.locator('[data-host-count]')).toHaveCount(0);
	const search = page.getByRole('searchbox', { name: 'Search' });
	await search.fill('bastion');
	await expect.poll(names).toEqual(['bastion.example', 'bastion-2.example']);
	await expect(page.locator('[data-host-count]')).toHaveText('2 of 4 hosts');
	await search.fill('socks5');
	await expect.poll(names).toEqual(['hop-a.example']);
	await search.fill('zzz');
	await expect(hostRows(page)).toHaveCount(0);
	await expect(page.getByRole('main')).toContainText('No matches');
	await page.getByRole('button', { name: 'Clear search' }).click();
	await expect.poll(names).toEqual(hostsTotals.order);
	await expect(page.locator('[data-host-count]')).toHaveCount(0);

	const sortBy = page.getByRole('combobox', { name: 'Sort by' });
	const direction = page.getByRole('button', { name: /^(Ascending|Descending)$/ });
	await expect(sortBy).toHaveValue('traffic');
	await expect(direction).toHaveAccessibleName('Descending');
	await sortBy.selectOption('host');
	await expect
		.poll(names)
		.toEqual(['hop-a.example', 'edge.example', 'bastion.example', 'bastion-2.example']);
	await direction.click();
	await expect(direction).toHaveAccessibleName('Ascending');
	await expect
		.poll(names)
		.toEqual(['bastion-2.example', 'bastion.example', 'edge.example', 'hop-a.example']);
	await sortBy.selectOption('traffic');
	await expect.poll(names).toEqual([...hostsTotals.order].reverse());
	await direction.click();
	await expect.poll(names).toEqual(hostsTotals.order);
	await expectMetricsFit(page, 'hosts-sorted');
});

test('?rule= pre-expands and focuses once; polls leave focus alone', async ({ page, api }) => {
	await page.goto('/#/stats?rule=db-tunnel');
	await expect(page).toHaveURL(/#\/stats\?rule=db-tunnel$/);
	const toggle = page.getByRole('button', { name: 'Collapse: db-tunnel' });
	await expect(toggle).toBeFocused();
	const stages = page.getByRole('list', { name: 'Chain' }).locator('> li');
	await expect(stages).toHaveText([
		/Clients\s+0\.0\.0\.0:18099/,
		/Hop 1 · binds the port · dialed from this machine/,
		/This machine/,
		/Direct/,
		/Targets\s+127\.0\.0\.1:5432\s+No connections yet/
	]);
	await expect(stages.nth(1).locator('[data-failures] dd').first()).toHaveText('3 / 3');
	await expectMetricsFit(page, 'stats-deep-link');

	const link = page.getByRole('main').getByRole('link', { name: 'Prometheus metrics' });
	await link.focus();
	const polls = count(api, 'GET /apis/stats');
	await expect.poll(() => count(api, 'GET /apis/stats')).toBeGreaterThan(polls + 1);
	await expect(link).toBeFocused();
});

test('rules named like Object.prototype members expand and focus from ?rule= once their row exists', async ({
	page,
	api
}) => {
	const listen = (port: number) => ({ host: '127.0.0.1', port });
	api.rules.push(
		{ name: 'toString', listen: listen(18101), forward: {} },
		{ name: '__proto__', listen: listen(18102), forward: {} },
		{ name: 'constructor', listen: listen(18103), forward: {} }
	);
	let release!: () => void;
	api.delay.set('GET /apis/configs/rules', new Promise<void>((resolve) => (release = resolve)));
	await page.goto('/#/stats?rule=toString');
	// The snapshot's rules are rows before the config answers; the deep-linked one is not yet.
	await expect(ruleRows(page)).toHaveCount(3);
	expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true);
	release();
	await expect(ruleRows(page)).toHaveCount(7);
	const toggle = page.getByRole('button', { name: 'Collapse: toString' });
	await expect(toggle).toBeFocused();
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(page.getByRole('button', { name: 'Expand: __proto__' })).toHaveAttribute(
		'aria-expanded',
		'false'
	);

	await page
		.locator('article[data-rule="__proto__"]')
		.getByRole('link', { name: '__proto__' })
		.click();
	await expect(page).toHaveURL(/#\/stats\?rule=__proto__$/);
	await expect(page.getByRole('button', { name: 'Collapse: __proto__' })).toBeFocused();
	await page.goBack();
	await expect(page).toHaveURL(/#\/stats\?rule=toString$/);
	await expect(toggle).toBeFocused();
	await page
		.locator('article[data-rule="constructor"]')
		.getByRole('link', { name: 'constructor' })
		.click();
	await expect(page.getByRole('button', { name: 'Collapse: constructor' })).toBeFocused();
	expect(count(api, 'GET /apis/configs/rules')).toBe(1);
});

test('reset asks first; accepting DELETEs /apis/stats and the pages show the zeroed snapshot', async ({
	page,
	api
}) => {
	await page.goto('/#/connections');
	await expect(connectionRows(page)).toHaveCount(4);
	await page.getByRole('button', { name: 'Reset statistics' }).click();
	await expect(confirmDialog(page)).toContainText('Reset statistics for all rules?');
	await confirmDialog(page).getByRole('button', { name: 'Cancel' }).click();
	await expect(confirmDialog(page)).toHaveCount(0);
	expect(api.writes).toEqual([]);

	await page.getByRole('button', { name: 'Reset statistics' }).click();
	await confirmDialog(page).getByRole('button', { name: 'Confirm' }).click();
	await expect(page.getByRole('status').filter({ hasText: 'Statistics reset.' })).toBeVisible();
	expect(writes(api)).toEqual(['DELETE /apis/stats']);
	// Live connections survive a reset with their counters back at zero, like the backend.
	await expect(connectionRows(page)).toHaveCount(4);
	await expect(connectionRows(page).nth(0).locator('[data-metric="down"]')).toHaveText('0 B');
	await expect(connectionRows(page).nth(0).locator('[data-metric="last_down"]')).toHaveText('—');
	await expect(connectionRows(page).nth(0).locator('[data-metric="peak_rate_down"]')).toHaveText(
		'0 B/s'
	);

	await page
		.getByRole('navigation', { name: 'Navigation' })
		.getByRole('link', { name: 'Rule Traffic' })
		.click();
	// Every level of the rule shows the zeroed counters: its own band and, expanded, the chain's.
	const office = ruleRows(page).nth(0);
	const block = office.locator('[data-stats]').first();
	await expect(block.locator('[data-metric="down"]')).toHaveText('0 B');
	await expect(block.locator('[data-connections] dd')).toHaveText('0 / 0');
	await office.locator('button[aria-controls]').click();
	const details = await detailsOf(page, office.locator('button[aria-controls]'));
	await expect(details.locator('[data-hop-stats] [data-metric="down"]')).toHaveText(['0 B', '0 B']);
	await expect(details.locator('[data-url] [data-metric="down"]')).toHaveText([
		'0 B',
		'0 B',
		'0 B'
	]);
	await page.getByRole('button', { name: 'Reset statistics' }).click();
	await confirmDialog(page).getByRole('button', { name: 'Cancel' }).click();
	expect(writes(api)).toEqual(['DELETE /apis/stats']);
});

test('a failing poll keeps the items behind a retry banner; polling stops off the statistics pages', async ({
	page,
	api,
	failures
}) => {
	failures.allow = (entry) =>
		entry.includes('/apis/stats') || entry.includes('ERR_CONNECTION_REFUSED');
	await page.goto('/#/stats');
	await expect(ruleRows(page)).toHaveCount(4);
	api.down.add('/apis/stats');
	const banner = page.getByRole('main').getByRole('alert');
	await expect(banner).toContainText('Cannot reach JumpWay.');
	await expect(ruleRows(page)).toHaveCount(4);
	await expect(ruleRows(page).nth(0).locator('[data-metric="rate_up"]')).toHaveText('12.0 KB/s');
	api.down.delete('/apis/stats');
	await banner.getByRole('button', { name: 'Retry' }).click();
	await expect(banner).toHaveCount(0);

	await page
		.getByRole('navigation', { name: 'Navigation' })
		.getByRole('link', { name: 'Global Settings' })
		.click();
	await expect(heading(page)).toHaveText('Global Settings');
	const settled = count(api, 'GET /apis/stats');
	await page.waitForTimeout(2_500);
	expect(count(api, 'GET /apis/stats')).toBe(settled);
});

test('#/hosts aggregates by hostname: weighted latency, upper-bound peaks, stats links and endpoint chips', async ({
	page,
	api
}) => {
	await page.clock.setFixedTime(FIXED_NOW);
	await page.goto('/#/hosts');
	await expect(heading(page)).toHaveText('Proxy Hosts');
	await expect(hostRows(page)).toHaveCount(4);
	await expect(page.getByRole('main').locator('table')).toHaveCount(0);
	await expect(hostRows(page).locator('[data-host-name]')).toHaveText(hostsTotals.order);
	const bastion = hostRows(page).nth(1);
	// Usage links live in their own line beside the toggle, never inside it.
	await expect(bastion.locator('[data-usage] a')).toHaveText(['office', 'mirror']);
	await expect(bastion.locator('[data-usage] a').nth(0)).toHaveAttribute(
		'href',
		'#/stats?rule=office'
	);
	await expect(bastion.locator('button a')).toHaveCount(0);
	await expect(bastion.locator('[data-endpoints]')).toHaveText('1');
	await expect(bastion.locator('[data-usage] + dl')).toHaveText(/Endpoints\s+1/);
	// The band is visible while collapsed: eight values, summed peaks labelled and explained.
	const block = bastion.locator('[data-stats]').first();
	const band = block.locator('[data-traffic]');
	await expect(band.locator('[data-metric="rate_down"]')).toHaveText('785.3 KB/s');
	await expect(band.locator('[data-metric="down"]')).toHaveText(hostsTotals.bastionDown);
	await expect(band.locator('[data-metric="peak_rate_down"]')).toHaveText(
		hostsTotals.bastionPeakDown
	);
	await expect(block.locator('[data-connections] dd')).toHaveText(hostsTotals.bastionConnections);
	await expect(block.locator('[data-latency] dd')).toHaveText(
		'18.4 ms / ' + hostsTotals.bastionAvgLatency
	);
	await expect(band.locator('dt').nth(1)).toHaveText('peak (sum)');
	await hover(page, band.locator('[data-metric="peak_rate_down"]'));
	await expect(tooltip(page)).toHaveText('Sum of endpoint peaks — an upper bound');
	await expectMetricsFit(page, 'hosts-collapsed');

	await bastion.getByRole('button', { name: 'Expand: bastion.example' }).click();
	const toggle = bastion.getByRole('button', { name: 'Collapse: bastion.example' });
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	const details = await detailsOf(page, toggle);
	await expect(details.locator('[data-stats]')).toHaveCount(0);
	const endpoints = details.locator('[data-endpoint-row]');
	await expect(endpoints).toHaveCount(1);
	await expect(endpoints.locator('[data-endpoint]')).toHaveText('ssh://bastion.example:22');
	await expect(endpoints.locator('[data-use]')).toHaveText([
		'office · Exit chain · Hop 2',
		'mirror · Exit chain · Hop 1'
	]);
	await expect(bastion.locator('[data-metric]')).toHaveCount(8);
	await hover(page, endpoints.locator('[data-endpoint]'));
	await expect(tooltip(page)).toHaveText('ssh://xxxxx@bastion.example:22');
	await expectMetricsFit(page, 'hosts-expanded');

	const urls = api.snapshot.rules![0].forward![1].urls!;
	const additional = structuredClone(urls[0]);
	additional.url = 'ssh://ops@bastion.example:2222';
	additional.stats.down = 2048;
	urls.push(additional);
	await expect(endpoints).toHaveCount(2);
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(endpoints.locator('[data-metric="down"]')).toHaveText([
		hostsTotals.bastionDown,
		'2.0 KB'
	]);
	await expect(endpoints.locator('[data-metric]')).toHaveCount(16);
	await expect(
		endpoints.locator('[data-connections], [data-latency], [data-failures]')
	).toHaveCount(6);
	await expectMetricsFit(page, 'hosts-multiple-endpoints');
	urls.pop();
	await expect(endpoints).toHaveCount(1);
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(details.locator('[data-stats], [data-metric]')).toHaveCount(0);
	await expect(bastion.locator('[data-metric]')).toHaveCount(8);

	// Following a usage link lands on statistics with that rule expanded, not on the editor.
	await bastion.locator('[data-usage] a').nth(1).click();
	await expect(page).toHaveURL(/#\/stats\?rule=mirror$/);
	await expect(page.getByRole('button', { name: 'Collapse: mirror' })).toBeFocused();
});

test('a focused host toggle keeps focus, state and tooltip when a poll re-sorts its row; a removed host gives focus up', async ({
	page,
	api
}) => {
	await page.goto('/#/hosts');
	await expect(hostRows(page)).toHaveCount(4);
	const order = () =>
		hostRows(page).evaluateAll((rows) => rows.map((row) => (row as HTMLElement).dataset.host));
	expect(await order()).toEqual(hostsTotals.order);
	const row = page.locator('article[data-host="bastion-2.example"]');
	await row.getByRole('button', { name: 'Expand: bastion-2.example' }).click();
	// Park the pointer off the items so nothing sliding under it re-owns the tooltip.
	await heading(page).hover();
	const nav = page
		.getByRole('navigation', { name: 'Navigation' })
		.getByRole('link', { name: 'Proxy Hosts' });
	// The click left the toggle focused; refocusing it from elsewhere shows its tooltip.
	await nav.focus();
	const toggle = row.getByRole('button', { name: 'Collapse: bastion-2.example' });
	await toggle.focus();
	await expect(toggle).toBeFocused();
	await expect(tooltip(page)).toHaveText('Collapse: bastion-2.example');
	await toggle.evaluate((element) => ((element as HTMLElement).dataset.marker = 'kept'));
	const details = await detailsOf(page, toggle);
	await expect(details).toBeVisible();

	// Its endpoint's download total grows past every other host: the next poll moves the row first.
	api.snapshot.rules![0].forward![1].urls![1].stats.down = 300_000_000;
	await expect
		.poll(order)
		.toEqual(['bastion-2.example', 'hop-a.example', 'bastion.example', 'edge.example']);
	await expect(toggle).toBeFocused();
	await expect(toggle).toHaveAttribute('data-marker', 'kept');
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(details).toBeVisible();
	await expect(tooltip(page)).toHaveText('Collapse: bastion-2.example');
	// The tooltip followed the button to its new row instead of floating over the old one.
	const gap = await page.evaluate(() => {
		const button = document.activeElement!.getBoundingClientRect();
		const tip = document.getElementById('tooltip')!.getBoundingClientRect();
		return tip.top - button.bottom;
	});
	expect(gap).toBeGreaterThanOrEqual(0);
	expect(gap).toBeLessThan(12);

	// Focus outside the rows is not touched by a re-sort.
	await nav.focus();
	api.snapshot.rules![0].forward![1].urls![1].stats.down = 25_000_000;
	await expect.poll(order).toEqual(hostsTotals.order);
	await expect(nav).toBeFocused();

	// The host's only URL disappears: focus is not sent back to the detached button.
	await toggle.focus();
	await expect(toggle).toBeFocused();
	api.snapshot.rules![0].forward![1].urls!.splice(1, 1);
	await expect(row).toHaveCount(0);
	await expect
		.poll(async () => await order())
		.toEqual(['hop-a.example', 'bastion.example', 'edge.example']);
	expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true);
	await expect(tooltip(page)).toHaveCount(0);
});

test('from 768px up, collapsed rule and host headers keep names and addresses on their lines beside a summary of its own', async ({
	page
}) => {
	await page.clock.setFixedTime(FIXED_NOW);
	// Fixture names fit one line and addresses two (address plus target) at every width.
	const budget = {
		'article[data-rule] a[href^="#/stats?rule="]': 1,
		'[data-address]': 2,
		'[data-host-name]': 1,
		'[data-usage] a': 1
	};
	for (const [route, rows] of [
		['stats', ruleRows],
		['hosts', hostRows]
	] as const) {
		await page.goto(`/#/${route}`);
		await expect(rows(page)).toHaveCount(4);
		// 768 is the sidebar's breakpoint: 224px of sidebar leave the items about 480px.
		for (const width of [768, 820, 1024, 1280]) {
			await page.setViewportSize({ width, height: 800 });
			await expect(page.getByRole('navigation', { name: 'Navigation' })).toBeVisible();
			await expectMetricsFit(page, `${route}-${width}`);
			await expectHeadersReadable(page, `${route}-${width}`, budget);
			await expectNoDocumentOverflow(page, `${route}-${width}`);
			if (width === 820) {
				await page.screenshot({
					path: test.info().outputPath(`tablet-820-${route}.png`),
					animations: 'disabled'
				});
			}
		}
	}
});

test('long rule names wrap inside the usage and endpoint chips at 375px; the chips still link and the row still toggles', async ({
	page,
	api
}) => {
	const long = 'ProductionInternationalHeadquartersTunnel';
	const longer = 'ProductionInternationalHeadquartersTunnelForEngineeringAndDataTeams';
	for (const [from, to] of [
		['office', long],
		['mirror', longer]
	] as const) {
		api.rules.find((rule) => rule.name === from)!.name = to;
		api.status.rules!.find((rule) => rule.name === from)!.name = to;
		api.snapshot.rules!.find((rule) => rule.name === from)!.name = to;
	}
	await page.setViewportSize({ width: 375, height: 812 });
	const bastion = page.locator('article[data-host="bastion.example"]');
	const budget = {
		'[data-host-name]': 1,
		'[data-usage] a': 3,
		'[data-use]': 3,
		'[data-endpoint]': 2
	};
	for (const [lang, expand, uses] of [
		['en', 'Expand: ', [`${long} · Exit chain · Hop 2`, `${longer} · Exit chain · Hop 1`]],
		['zh', '展开: ', [`${long} · 出口链路 · 跳板节点 2`, `${longer} · 出口链路 · 跳板节点 1`]]
	] as const) {
		await page.goto(`/?lang=${lang}#/hosts`);
		await expect(hostRows(page)).toHaveCount(4);
		await expect(bastion.locator('[data-usage] a')).toHaveText([long, longer]);
		await expect(bastion.locator('[data-usage] a').nth(1)).toHaveAttribute(
			'href',
			`#/stats?rule=${longer}`
		);
		await expectMetricsFit(page, `hosts-long-${lang}`);
		await expectHeadersReadable(page, `hosts-long-${lang}`, budget);
		await expectNoDocumentOverflow(page, `hosts-long-${lang}`);
		await bastion.getByRole('button', { name: expand + 'bastion.example' }).click();
		await expect(bastion.locator('[data-use]')).toHaveText(uses);
		await expectMetricsFit(page, `hosts-long-expanded-${lang}`);
		await expectHeadersReadable(page, `hosts-long-expanded-${lang}`, budget);
		await expectNoDocumentOverflow(page, `hosts-long-expanded-${lang}`);
		await bastion.locator('[data-endpoint-row]').first().scrollIntoViewIfNeeded();
		await page.screenshot({
			path: test.info().outputPath(`mobile-375-hosts-long-${lang}.png`),
			animations: 'disabled'
		});
	}

	// The header surface beside the chips still toggles; a wrapped chip still navigates.
	const toggle = bastion.locator('button[aria-controls]');
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await bastion.locator('[data-host-name]').scrollIntoViewIfNeeded();
	const surface = (await bastion.locator('[data-host-name]').boundingBox())!;
	await page.mouse.click(surface.x + surface.width / 2, surface.y + surface.height / 2);
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	await bastion.locator('[data-usage] a').nth(1).click();
	await expect(page).toHaveURL(new RegExp(`#/stats\\?rule=${longer}$`));
	await expect(page.getByRole('button', { name: `收起: ${longer}` })).toBeFocused();
});

test('#/connections: filters, ?rule= preselection, sorting from the toolbar, expansion and path tooltips', async ({
	page
}) => {
	await page.clock.setFixedTime(FIXED_NOW);
	await page.goto('/#/connections?rule=mirror');
	await expect(page).toHaveURL(/#\/connections\?rule=mirror$/);
	await expect(heading(page)).toHaveText('Live Connections');
	const select = page.getByRole('combobox', { name: 'Rule' });
	await expect(select).toHaveValue('mirror');
	await expect(connectionRows(page)).toHaveCount(1);
	await expect(page.locator('[data-connection-count]')).toHaveText('1 of 4 connections');
	await select.selectOption('');
	await expect(page).toHaveURL(/#\/connections$/);
	await expect(connectionRows(page)).toHaveCount(4);
	await expect(connectionRows(page).locator('[data-target]')).toHaveText([
		'10.1.2.3:8080',
		'cdn.example.net:443',
		'example.com:443',
		'10.0.0.5:5432'
	]);
	await expect(page.locator('[data-connection-count]')).toHaveText('4 connections');
	// The same kind of item as a rule or host at every width: no table, no column headers.
	await expect(page.getByRole('main').locator('table')).toHaveCount(0);
	await expectConnectionsFit(page, 'connections-collapsed');
	await expectColumnsAligned(page, 'connections-collapsed');

	const curl = connectionRows(page).nth(2);
	// The client keeps its port beside the process; nothing about it is left for the expansion.
	await expect(curl.locator('[data-client]')).toHaveText(/127\.0\.0\.1:18094\s+curl \(4242\)/);
	await hover(page, curl.getByRole('button', { name: 'Path' }));
	await expect(tooltip(page)).toHaveText(
		'this machine → ssh://bastion.example:22 → socks5://hop-a.example:1080 → example.com:443'
	);
	await hover(page, connectionRows(page).nth(1).getByRole('button', { name: 'Path' }));
	await expect(tooltip(page)).toHaveText(
		'this machine → ssh://bastion-2.example:22 (reused) → Hop 1 (reused) → cdn.example.net:443'
	);
	await expect(curl.locator('[data-duration]')).toHaveText('1 min 30 s');
	// The item's band: four labelled columns, upload above download, the absolute time on hover.
	await expect(curl.locator('[data-traffic] dt')).toHaveText(['now', 'peak', 'total', 'last']);
	await expect(metrics(curl.locator('[data-traffic]'))).toHaveText([
		'1.0 KB/s',
		'512.0 KB/s',
		'4.0 KB/s',
		'1.0 MB/s',
		'12.0 KB',
		'1.0 MB',
		'3 s ago',
		'2 s ago'
	]);
	await hover(page, curl.locator('[data-metric="last_down"]'));
	await expect(tooltip(page)).toHaveText(new Date('2026-09-19T08:59:58Z').toLocaleString('en'));
	await expect(curl.locator('[data-rule-link]')).toHaveAttribute('href', '#/stats?rule=office');

	// The chevron expands the item to what it lacks: the start time and the path; Space and
	// Enter work on it too, and several items stay open together. It renames itself on
	// expansion, so it is re-located by its aria-controls.
	await curl.getByRole('button', { name: 'Expand: example.com:443' }).click();
	const toggle = curl.locator('button[aria-controls]');
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(toggle).toHaveAccessibleName('Collapse: example.com:443');
	const details = await detailsOf(page, toggle);
	await expect(details.locator('[data-traffic], [data-metric]')).toHaveCount(0);
	await expect(details.locator('dt')).toHaveText(['Started', 'Path']);
	await expect(details.locator('[data-started] time')).toHaveAttribute(
		'datetime',
		'2026-09-19T08:58:30.000Z'
	);
	await expect(details.locator('[data-path]')).toHaveText(
		'this machine → ssh://bastion.example:22 → socks5://hop-a.example:1080 → example.com:443'
	);
	await expect(details).not.toContainText('curl');
	await expect(details).not.toContainText('127.0.0.1');
	const expanded = page.locator('[id^="connection-details-"]');
	await expect(expanded).toHaveCount(1);
	await connectionRows(page)
		.nth(1)
		.getByRole('button', { name: 'Expand: cdn.example.net:443' })
		.focus();
	await page.keyboard.press('Space');
	await expect(expanded).toHaveCount(2);
	await expectConnectionsFit(page, 'connections-expanded');
	await expectColumnsAligned(page, 'connections-expanded');
	await page.keyboard.press('Enter');
	await expect(expanded).toHaveCount(1);
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');

	const search = page.getByRole('searchbox', { name: 'Search' });
	await expect(page.getByRole('button', { name: 'Clear search' })).toHaveCount(0);
	await search.fill('1809');
	await expect(connectionRows(page)).toHaveCount(2);
	await expect(page.locator('[data-connection-count]')).toHaveText('2 of 4 connections');
	// The expanded item keeps its detail through the filter.
	await expect(expanded).toHaveCount(1);
	await search.fill('chrome help');
	await expect(connectionRows(page)).toHaveCount(1);
	await expect(connectionRows(page).locator('[data-target]')).toHaveText('cdn.example.net:443');
	await search.fill('zzz');
	await expect(connectionRows(page)).toHaveCount(0);
	await expect(page.getByRole('main')).toContainText('No current connections');
	await page.getByRole('button', { name: 'Clear search' }).click();
	await expect(search).toHaveValue('');
	await expect(connectionRows(page)).toHaveCount(4);

	// Sorting: the select picks the key and keeps the direction; the button flips it.
	const sortBy = page.getByRole('combobox', { name: 'Sort by' });
	const direction = page.getByRole('button', { name: /^(Ascending|Descending)$/ });
	await expect(sortBy).toHaveValue('started');
	await expect(direction).toHaveAccessibleName('Descending');
	await sortBy.selectOption('target');
	await expect(direction).toHaveAccessibleName('Descending');
	await expect(connectionRows(page).locator('[data-target]')).toHaveText([
		'example.com:443',
		'cdn.example.net:443',
		'10.1.2.3:8080',
		'10.0.0.5:5432'
	]);
	await direction.click();
	await expect(direction).toHaveAccessibleName('Ascending');
	await expect(connectionRows(page).locator('[data-target]')).toHaveText([
		'10.0.0.5:5432',
		'10.1.2.3:8080',
		'cdn.example.net:443',
		'example.com:443'
	]);
	await sortBy.selectOption('rate_down');
	expect(await connectionOrder(page)).toEqual(['103', '201', '102', '101']);
	await direction.click();
	await expect(connectionRows(page).locator('[data-target]').first()).toHaveText('example.com:443');
	await sortBy.selectOption('peak_rate_down');
	await expect(direction).toHaveAccessibleName('Descending');
	expect(await connectionOrder(page)).toEqual(['101', '102', '201', '103']);
	await hover(page, direction);
	await expect(tooltip(page)).toHaveText('Descending');
	await direction.click();
	await expect(direction).toHaveAccessibleName('Ascending');
	await expect(tooltip(page)).toHaveText('Ascending');
	expect(await connectionOrder(page)).toEqual(['103', '201', '102', '101']);
	await sortBy.selectOption('last_up');
	expect(await connectionOrder(page)).toEqual(['103', '201', '102', '101']);
	await sortBy.selectOption('rate_up');
	expect(await connectionOrder(page)).toEqual(['103', '102', '101', '201']);
	await direction.click();
	expect(await connectionOrder(page)).toEqual(['201', '101', '102', '103']);
	await sortBy.selectOption('started');
	expect(await connectionOrder(page)).toEqual(['103', '102', '101', '201']);
	// The expansion is keyed by connection, so it followed its item through every re-sort.
	await expect(page.locator('#connection-details-101')).toHaveCount(1);

	// A query-only route change (Back) re-selects without remounting or stealing focus: build the
	// entries through in-app links first, since the select itself replaces the current entry.
	await page
		.getByRole('navigation', { name: 'Navigation' })
		.getByRole('link', { name: 'Rule Traffic' })
		.click();
	// The connections link sits in the collapsed band, outside the toggle's surface.
	await ruleRows(page).nth(0).locator('[data-connections] a').click();
	await expect(page).toHaveURL(/#\/connections\?rule=office$/);
	await expect(select).toHaveValue('office');
	await expect(connectionRows(page)).toHaveCount(3);
	await page
		.getByRole('navigation', { name: 'Navigation' })
		.getByRole('link', { name: 'Live Connections' })
		.click();
	await expect(page).toHaveURL(/#\/connections$/);
	await expect(connectionRows(page)).toHaveCount(4);
	await select.evaluate((element) => ((element as HTMLElement).dataset.marker = 'kept'));
	await search.focus();
	await page.goBack();
	await expect(page).toHaveURL(/#\/connections\?rule=office$/);
	await expect(select).toHaveValue('office');
	await expect(connectionRows(page)).toHaveCount(3);
	await expect(search).toBeFocused();
	await expect(select).toHaveAttribute('data-marker', 'kept');
});

test('disconnect DELETEs that id only, the row leaves on the next poll and its tooltip with it', async ({
	page,
	api,
	failures
}) => {
	await page.goto('/#/connections');
	await expect(connectionRows(page)).toHaveCount(4);
	const chrome = connectionRows(page).nth(1);
	await expect(chrome).toHaveAttribute('data-connection', '102');
	const disconnect = chrome.getByRole('button', { name: 'Disconnect' });
	await hover(page, disconnect);
	await expect(tooltip(page)).toHaveText('Disconnect');
	await disconnect.click();
	await expect(connectionRows(page)).toHaveCount(3);
	expect(await connectionOrder(page)).toEqual(['103', '101', '201']);
	expect(writes(api)).toEqual(['DELETE /apis/stats/connections/102']);
	await expect(page.getByRole('status').filter({ hasText: 'Connection closed.' })).toBeVisible();
	// The hovered row is gone; Chrome may re-hover whatever slid under the pointer, so the
	// tooltip is either hidden or owned by a live element of another row.
	await expect(page.locator('[data-connection="102"]')).toHaveCount(0);
	const owners = page.locator('[aria-describedby~="tooltip"]');
	expect(await owners.count()).toBeLessThanOrEqual(1);
	if ((await owners.count()) === 1) {
		await expect(owners).toBeVisible();
		await expect(tooltip(page)).toHaveText('Disconnect');
	} else {
		await expect(tooltip(page)).toHaveCount(0);
	}
	await expect(
		connectionRows(page).nth(0).getByRole('button', { name: 'Disconnect' })
	).toBeEnabled();

	failures.allow = expected400;
	api.fail.set('DELETE /apis/stats/connections/103', 'connection 103 not found');
	await connectionRows(page).nth(0).getByRole('button', { name: 'Disconnect' }).click();
	await expect(
		page.getByRole('alert').filter({ hasText: 'connection 103 not found' })
	).toBeVisible();
	await expect(connectionRows(page)).toHaveCount(3);
	await expect(
		connectionRows(page).nth(0).getByRole('button', { name: 'Disconnect' })
	).toBeEnabled();
});

test('a focused row control keeps focus and its tooltip when a poll moves its row; a removed row gives focus up', async ({
	page,
	api
}) => {
	await page.goto('/#/connections');
	await expect(connectionRows(page)).toHaveCount(4);
	// Ascending download total, chosen through the toolbar.
	await page.getByRole('combobox', { name: 'Sort by' }).selectOption('down');
	await page.getByRole('button', { name: 'Descending' }).click();
	const order = () => connectionOrder(page);
	expect(await order()).toEqual(['103', '102', '101', '201']);
	const path = page.locator('[data-connection="102"]').getByRole('button', { name: 'Path' });
	await path.focus();
	await expect(path).toBeFocused();
	const pathText =
		'this machine → ssh://bastion-2.example:22 (reused) → Hop 1 (reused) → cdn.example.net:443';
	await expect(tooltip(page)).toHaveText(pathText);
	await path.evaluate((element) => ((element as HTMLElement).dataset.marker = 'kept'));

	// Its download total grows past every other row: the next poll moves the row to the end.
	api.snapshot.rules![0].connections![1].stats.down = 3_000_000;
	await expect.poll(order).toEqual(['103', '101', '201', '102']);
	await expect(path).toBeFocused();
	await expect(path).toHaveAttribute('data-marker', 'kept');
	await expect(tooltip(page)).toHaveText(pathText);
	// The tooltip followed the button to its new row instead of floating over the old one.
	const gap = await page.evaluate(() => {
		const button = document.activeElement!.getBoundingClientRect();
		const tip = document.getElementById('tooltip')!.getBoundingClientRect();
		return tip.top - button.bottom;
	});
	expect(gap).toBeGreaterThanOrEqual(0);
	expect(gap).toBeLessThan(12);

	// The connection closes: focus is not sent back to the detached button, nothing is thrown.
	api.snapshot.rules![0].connections!.splice(1, 1);
	await expect(page.locator('[data-connection="102"]')).toHaveCount(0);
	expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true);
	await expect(tooltip(page)).toHaveCount(0);

	// Filtering the focused row away is what the user asked for.
	await connectionRows(page).first().getByRole('button', { name: 'Disconnect' }).focus();
	await page.getByRole('combobox', { name: 'Rule' }).selectOption('mirror');
	await expect(connectionRows(page)).toHaveCount(1);
	expect(
		await page.evaluate(() => document.activeElement?.closest('[data-connection]') === null)
	).toBe(true);
});

test('250 connections render as 200 rows with a note; the count still says 251', async ({
	page,
	api
}) => {
	const template = snapshotFixture.rules![0].connections![0];
	api.snapshot.rules![0].connections = Array.from({ length: 250 }, (_, index) => ({
		...structuredClone(template),
		id: 1_000 + index,
		client: `10.0.0.${index % 250}:${20_000 + index}`,
		started: new Date(Date.now() - (250 - index) * 1_000).toISOString()
	}));
	await page.goto('/#/connections');
	await expect(connectionRows(page)).toHaveCount(200);
	await expect(connectionRows(page).first()).toHaveAttribute('data-connection', '1249');
	await expect(page.locator('[data-connection-count]')).toHaveText('251 connections');
	await expect(page.getByRole('main')).toContainText('Showing 200 of 251');
	await expectNoDocumentOverflow(page, 'connections-250');
	await expectConnectionsFit(page, 'connections-250');
	await expectColumnsAligned(page, 'connections-250');
	// The same 200 keyed items at a tablet width.
	await page.setViewportSize({ width: 820, height: 1180 });
	await expect(connectionRows(page)).toHaveCount(200);
	await expect(connectionRows(page).first()).toHaveAttribute('data-connection', '1249');
	await expectNoDocumentOverflow(page, 'connections-250-tablet');
	await expectColumnsAligned(page, 'connections-250-tablet');
});

test('connections are the same items at every width; an expanded item stays expanded across widths and everything fits', async ({
	page,
	api
}) => {
	await page.clock.setFixedTime(FIXED_NOW);
	await page.goto('/#/connections');
	await expect(connectionRows(page)).toHaveCount(4);
	const budget = {
		'[data-connection] [data-target]': 1,
		'[data-connection] [data-rule-link]': 1,
		'[data-connection] [data-duration]': 1
	};
	for (const width of [768, 820, 1024, 1099, 1100, 1280, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		await expect(page.getByRole('navigation', { name: 'Navigation' })).toBeVisible();
		await expect(page.getByRole('main').locator('table')).toHaveCount(0);
		await expect(connectionRows(page)).toHaveCount(4);
		if (width === 768) {
			await connectionRows(page)
				.nth(2)
				.getByRole('button', { name: 'Expand: example.com:443' })
				.click();
		}
		await expect(page.locator('#connection-details-101')).toBeVisible();
		await expect(
			page
				.locator('[data-connection="101"]')
				.getByRole('button', { name: 'Collapse: example.com:443' })
		).toHaveAttribute('aria-expanded', 'true');
		await expectConnectionsFit(page, `connections-${width}`);
		await expectColumnsAligned(page, `connections-${width}`);
		await expectHeadersReadable(page, `connections-${width}`, budget);
		await expectNoDocumentOverflow(page, `connections-${width}`);
		if (width === 820) {
			await page.screenshot({
				path: test.info().outputPath('tablet-820-connections.png'),
				animations: 'disabled'
			});
		}
	}
	// The item header toggles from its surface too; Disconnect and the rule badge sit above
	// that surface and keep their own actions.
	await page.setViewportSize({ width: 820, height: 900 });
	const card = page.locator('article[data-connection="101"]');
	const surface = (await card.locator('[data-client]').boundingBox())!;
	await page.mouse.click(surface.x + surface.width / 2, surface.y + surface.height / 2);
	await expect(card.locator('button[aria-controls]')).toHaveAttribute('aria-expanded', 'false');
	await page
		.locator('article[data-connection="103"]')
		.getByRole('button', { name: 'Disconnect' })
		.click();
	await expect(page.locator('[data-connection="103"]')).toHaveCount(0);
	expect(writes(api)).toEqual(['DELETE /apis/stats/connections/103']);
	await expect(card.locator('button[aria-controls]')).toHaveAttribute('aria-expanded', 'false');
	await card.locator('[data-rule-link]').click();
	await expect(page).toHaveURL(/#\/stats\?rule=office$/);
	await expect(page.getByRole('button', { name: 'Collapse: office' })).toBeFocused();
});

test('the metric columns hold still across items with different numbers, on all three pages, at phone, tablet and desktop widths', async ({
	page,
	api
}) => {
	await page.clock.setFixedTime(FIXED_NOW);
	// Heterogeneous values: a huge total and rate on one connection, zeros on another, dashes on
	// the rule and host without traffic.
	api.snapshot.rules![0].connections![1].stats.down = 987_654_321_000;
	api.snapshot.rules![0].connections![1].stats.rate_down = 123_456_789;
	for (const width of [375, 768, 820, 1280, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		for (const [route, rows] of [
			['stats', ruleRows],
			['hosts', hostRows],
			['connections', connectionRows]
		] as const) {
			await page.goto(`/?w=${width}#/${route}`);
			await expect(rows(page)).toHaveCount(4);
			await expectColumnsAligned(page, `${route}-${width}`);
			await expectMetricsFit(page, `${route}-${width}`);
			await expectNoDocumentOverflow(page, `${route}-${width}`);
		}
		// Four columns from the tablet width on; two column groups on a phone and at 768, where
		// the sidebar and scrollbar leave the band under 29rem.
		const columnsPerRow = await page.evaluate(() => {
			const cells = Array.from(
				document.querySelectorAll<HTMLElement>('article[data-connection] [data-column]')
			).slice(0, 4);
			return new Set(cells.map((cell) => Math.round(cell.getBoundingClientRect().top))).size;
		});
		expect(columnsPerRow, `rows of columns at ${width}`).toBe(width <= 768 ? 2 : 1);
	}
});

test('the header columns hold still too: names, clients, durations and actions of every item share their edges on all three pages, in one row on a desktop and two rows on a tablet', async ({
	page,
	api
}) => {
	await page.clock.setFixedTime(FIXED_NOW);
	// Durations from seconds to days beside clients with and without a process name.
	api.snapshot.rules![1].connections![0].started = '2026-09-14T03:56:00Z';
	for (const width of [375, 768, 820, 1280, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		for (const [route, rows] of [
			['stats', ruleRows],
			['hosts', hostRows],
			['connections', connectionRows]
		] as const) {
			await page.goto(`/?h=${width}#/${route}`);
			await expect(rows(page)).toHaveCount(4);
			await expectHeadersAligned(page, `${route}-${width}`);
			await expectNoDocumentOverflow(page, `${route}-${width}`);
		}
		await expect(page.locator('[data-connection="201"] [data-duration]')).toHaveText('125 h 4 min');
		// Desktop: identity, client, summary and chevron in one row; tablet: the summary takes a
		// second row under identity and client; phone: identity beside the chevron, the rest below.
		const rowsOf = await page.evaluate(() => {
			const item = document.querySelector<HTMLElement>('article[data-connection]')!;
			const box = (hook: string) =>
				item.querySelector(`[data-header-${hook}]`)!.getBoundingClientRect();
			const identity = box('identity');
			const besideIdentity = (hook: string) => {
				const cell = box(hook);
				return cell.top < identity.bottom - 1 && identity.top < cell.bottom - 1;
			};
			return {
				detail: besideIdentity('detail'),
				summary: besideIdentity('summary'),
				toggle: besideIdentity('toggle')
			};
		});
		expect(rowsOf, `header rows at ${width}`).toEqual({
			detail: width > 375,
			summary: width >= 1280,
			toggle: true
		});
		if (width === 1280) {
			for (const route of ['connections', 'stats', 'hosts']) {
				await page.goto(`/?s=${width}#/${route}`);
				await expect(page.locator('article')).toHaveCount(4);
				await page.screenshot({
					path: test.info().outputPath(`${route}-1280-en.png`),
					animations: 'disabled'
				});
			}
		}
	}
});

test('circled question marks explain the concepts: hover and focus show the definition, a click pins it without toggling the item, Escape and a click elsewhere hide it', async ({
	page
}) => {
	await page.goto('/#/stats');
	await expect(ruleRows(page)).toHaveCount(4);
	const office = ruleRows(page).nth(0);
	const toggle = office.locator('button[aria-controls]');
	const help = office.getByRole('button', { name: 'About peak' });
	await expect(help).toHaveCount(1);
	// Hover shows the definition; leaving hides it.
	await hover(page, help);
	await expect(tooltip(page)).toHaveText(
		'Peak: the highest one-second rate seen since the statistics were last reset.'
	);
	await expect(help).toHaveAttribute('aria-describedby', 'tooltip');
	await heading(page).hover();
	await expect(tooltip(page)).toHaveCount(0);
	// A click pins it: the pointer may leave, the item does not toggle, Escape hides it and the
	// button keeps focus so Enter brings it back.
	await help.click();
	await expect(tooltip(page)).toContainText('Peak:');
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	await heading(page).hover();
	await expect(tooltip(page)).toContainText('Peak:');
	await page.keyboard.press('Escape');
	await expect(tooltip(page)).toHaveCount(0);
	await expect(help).toBeFocused();
	await page.keyboard.press('Enter');
	await expect(tooltip(page)).toContainText('Peak:');
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	// Tab moves on and takes the definition with it; Shift+Tab back shows it again.
	await page.keyboard.press('Tab');
	await expect(help).not.toBeFocused();
	await expect(tooltip(page)).not.toContainText('Peak:');
	await page.keyboard.press('Shift+Tab');
	await expect(help).toBeFocused();
	await expect(tooltip(page)).toContainText('Peak:');
	// A click elsewhere hides a pinned definition without toggling the item either.
	await help.click();
	await page.mouse.click(5, 5);
	await expect(tooltip(page)).toHaveCount(0);
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	// The heading and the other concepts are explained too, in the current language.
	await hover(page, page.getByRole('button', { name: 'About Rule Traffic' }));
	await expect(tooltip(page)).toContainText('One item per rule');
	await hover(page, office.getByRole('button', { name: 'About Latency' }));
	await expect(tooltip(page)).toContainText('not a ping');
	await page.goto('/?lang=zh#/hosts');
	await expect(hostRows(page)).toHaveCount(4);
	await hover(page, hostRows(page).nth(1).getByRole('button', { name: '峰值（合计）说明' }));
	await expect(tooltip(page)).toContainText('上界');
});

test.describe('touch', () => {
	test.use({ viewport: { width: 375, height: 812 }, hasTouch: true, isMobile: true });

	test('on a phone a tap on a question mark pins its definition inside the item without expanding it; a tap elsewhere hides it', async ({
		page
	}) => {
		await page.goto('/#/connections');
		await expect(connectionRows(page)).toHaveCount(4);
		const item = connectionRows(page).nth(2);
		const help = item.getByRole('button', { name: 'About total' });
		await help.scrollIntoViewIfNeeded();
		await help.tap();
		await expect(tooltip(page)).toContainText('Total:');
		await expect(item.locator('button[aria-controls]')).toHaveAttribute('aria-expanded', 'false');
		// A tap elsewhere (the heading, not the item's toggling surface) hides it.
		await heading(page).tap();
		await expect(tooltip(page)).toHaveCount(0);
		await expect(item.locator('button[aria-controls]')).toHaveAttribute('aria-expanded', 'false');
	});
});

test('full IPv6 clients fit the items at phone, tablet and desktop widths', async ({
	page,
	api
}) => {
	const client = '2001:db8f:1234:5678:9abc:def0:1234:5678';
	api.snapshot.rules![0].connections![0].client = `[${client}]:52000`;
	for (const width of [375, 768, 820, 1100, 1280]) {
		await page.setViewportSize({ width, height: 900 });
		await page.goto(`/?width=${width}#/connections`);
		await expect(connectionRows(page)).toHaveCount(4);
		await expect(page.locator('[data-connection="101"] [data-client]')).toContainText(client);
		await expectConnectionsFit(page, `connections-ipv6-${width}`);
		await expectNoDocumentOverflow(page, `connections-ipv6-${width}`);
	}
});

test('long targets, IPv6 clients, process names and rule names wrap across mobile and desktop in both languages', async ({
	page,
	api
}) => {
	const long = 'ProductionInternationalHeadquartersTunnelForEngineeringAndDataTeams';
	const target = 'a-very-long-subdomain-name.internal.engineering.example-corporation.com:8443';
	api.rules.find((rule) => rule.name === 'office')!.name = long;
	api.status.rules!.find((rule) => rule.name === 'office')!.name = long;
	const office = api.snapshot.rules!.find((rule) => rule.name === 'office')!;
	office.name = long;
	office.connections![0].target = target;
	office.connections![0].client = '[2001:db8f:1234:5678:9abc:def0:1234:5678]:52000';
	office.connections![0].process = {
		pid: 31337,
		name: 'com.example.SomeVeryLongHelperProcessName (Renderer)'
	};
	const budget = {
		'[data-connection] [data-target]': 4,
		'[data-connection] [data-rule-link]': 4,
		'[data-connection] [data-client]': 4,
		'[data-connection] [data-duration]': 1,
		'[data-connection-facts] dd': 6
	};
	for (const [lang, expand] of [
		['en', 'Expand: '],
		['zh', '\u5c55\u5f00: ']
	] as const) {
		for (const width of [375, 390, 1280]) {
			await page.setViewportSize({ width, height: 900 });
			// A distinct query per width: the same URL would be a same-document navigation.
			await page.goto(`/?lang=${lang}&w=${width}#/connections`);
			await expect(connectionRows(page)).toHaveCount(4);
			const row = page.locator('[data-connection="101"]');
			await expect(row.locator('[data-rule-link]')).toHaveText(long);
			await expect(row.locator('[data-target]')).toHaveText(target);
			await expect(row.locator('[data-client]')).toContainText('SomeVeryLongHelperProcessName');
			await expectConnectionsFit(page, `connections-long-${lang}-${width}`);
			await expectHeadersReadable(page, `connections-long-${lang}-${width}`, budget);
			await expectNoDocumentOverflow(page, `connections-long-${lang}-${width}`);
			await row.getByRole('button', { name: expand + target }).click();
			await expect(page.locator('#connection-details-101')).toBeVisible();
			await expect(page.locator('#connection-details-101 [data-path]')).toContainText(target);
			await expectConnectionsFit(page, `connections-long-expanded-${lang}-${width}`);
			await expectHeadersReadable(page, `connections-long-expanded-${lang}-${width}`, budget);
			await expectNoDocumentOverflow(page, `connections-long-expanded-${lang}-${width}`);
			if (width === 375 && lang === 'zh') {
				await row.scrollIntoViewIfNeeded();
				await page.screenshot({
					path: test.info().outputPath('mobile-375-connections-long-zh.png'),
					animations: 'disabled'
				});
			}
		}
	}
	// The rule select stays compact while holding the full name; the detail carries the rule
	// through its statistics link.
	const select = page.locator('#connections-rule');
	await select.selectOption(long);
	await expect(page).toHaveURL(new RegExp(`#/connections\\?rule=${long}$`));
	await expect(connectionRows(page)).toHaveCount(3);
	expect(
		await select.evaluate((element) => element.getBoundingClientRect().width)
	).toBeLessThanOrEqual(14 * 16 + 1);
	await expectNoDocumentOverflow(page, 'connections-long-selected');
});

test.describe('screenshots', () => {
	// #main scrolls on its own, so a shot covers the viewport; `scroll` moves #main first to show
	// the lower part of an expanded item.
	const shoot = async (page: Page, name: string, scroll = 0) => {
		const path = test.info().outputPath(name + '.png');
		await page.evaluate((top) => {
			for (const element of document.querySelectorAll('.overflow-x-auto')) element.scrollLeft = 0;
			document.getElementById('main')!.scrollTop = top;
		}, scroll);
		await page.screenshot({ path, animations: 'disabled' });
		test.info().attach(name, { path, contentType: 'image/png' });
		await expectNoDocumentOverflow(page, name);
	};

	for (const [viewport, label] of [
		[{ width: 1280, height: 800 }, 'desktop'],
		[{ width: 375, height: 812 }, 'mobile']
	] as const) {
		test(`${label} statistics, hosts and connections in en light and zh dark`, async ({ page }) => {
			await page.clock.setFixedTime(FIXED_NOW);
			await page.setViewportSize(viewport);
			for (const [lang, theme, expand, collapse] of [
				['en', 'light', 'Expand: ', 'Collapse: '],
				['zh', 'dark', '展开: ', '收起: ']
			] as const) {
				await page.goto(`/?lang=${lang}#/stats`);
				await page.evaluate((value) => localStorage.setItem('jumpway.theme', value), theme);
				await page.reload();
				await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
				await expect(ruleRows(page)).toHaveCount(4);
				await expectMetricsFit(page, `${label}-stats-${lang}`);
				await shoot(page, `${label}-stats-${lang}-${theme}`);
				await ruleRows(page)
					.nth(0)
					.getByRole('button', { name: expand + 'office' })
					.click();
				await expect(
					ruleRows(page)
						.nth(0)
						.getByRole('button', { name: collapse + 'office' })
				).toHaveAttribute('aria-expanded', 'true');
				await expect(page.locator('[data-url]')).toHaveCount(3);
				await expectMetricsFit(page, `${label}-stats-expanded-${lang}`);
				await shoot(page, `${label}-stats-expanded-${lang}-${theme}`);
				await shoot(page, `${label}-stats-expanded-chain-${lang}-${theme}`, 480);

				await page.goto(`/?lang=${lang}#/hosts`);
				await expect(hostRows(page)).toHaveCount(4);
				await expectMetricsFit(page, `${label}-hosts-${lang}`);
				await shoot(page, `${label}-hosts-${lang}-${theme}`);
				await hostRows(page)
					.nth(1)
					.getByRole('button', { name: expand + 'bastion.example' })
					.click();
				await expect(page.locator('[data-endpoint-row]')).toHaveCount(1);
				await expectMetricsFit(page, `${label}-hosts-expanded-${lang}`);
				await shoot(page, `${label}-hosts-expanded-${lang}-${theme}`);

				await page.goto(`/?lang=${lang}#/connections`);
				await expect(connectionRows(page)).toHaveCount(4);
				await expectConnectionsFit(page, `${label}-connections-${lang}`);
				await shoot(page, `${label}-connections-${lang}-${theme}`);
				await connectionRows(page)
					.nth(2)
					.getByRole('button', { name: expand + 'example.com:443' })
					.click();
				await expect(page.locator('[id^="connection-details-"]')).toHaveCount(1);
				await expectConnectionsFit(page, `${label}-connections-expanded-${lang}`);
				await shoot(page, `${label}-connections-expanded-${lang}-${theme}`);
			}
		});
	}
});
