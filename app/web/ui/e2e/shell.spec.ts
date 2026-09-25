import type { Locator, Page } from '@playwright/test';
import { readFile } from 'node:fs/promises';
import type { Rule } from '../src/lib/types.ts';
import { SECRET, rulesFixture, snapshotTotals } from './fixtures/api.ts';
import { expect, test } from './fixtures/test.ts';
import { count } from './mockApi.ts';

const heading = (page: Page) => page.getByRole('heading', { level: 1 });
const nav = (page: Page) => page.getByRole('navigation', { name: /Navigation|导航/ });
const kpi = (page: Page, name: string) => page.locator(`[data-kpi="${name}"]`);
const compact = async (locator: Locator) => (await locator.innerText()).replace(/\s+/g, ' ').trim();

// Sidebar labels wrap onto at most two lines inside the 224px column; nothing is clipped.
async function expectNavLabelsFit(page: Page, labels: string[]) {
	const links = nav(page).getByRole('link');
	await expect(links).toHaveText(labels);
	const problems = await links.evaluateAll((elements) =>
		elements.flatMap((link) => {
			const out: string[] = [];
			const label = link.textContent?.trim();
			const style = getComputedStyle(link);
			const lineHeight = parseFloat(style.lineHeight) || parseFloat(style.fontSize) * 1.5;
			const box = link.getBoundingClientRect();
			const column = link.closest('aside, dialog')!.getBoundingClientRect();
			if (link.scrollWidth > link.clientWidth + 1)
				out.push(`${label} clipped ${link.scrollWidth}/${link.clientWidth}`);
			if (box.height > lineHeight * 2 + 13) out.push(`${label} taller than two lines`);
			if (box.right > column.right + 1 || box.left < column.left - 1)
				out.push(`${label} outside the sidebar`);
			return out;
		})
	);
	expect(problems).toEqual([]);
}

// Vite may inline the brand PNGs as data URLs, so match the bitmap by its bytes, not its src.
async function expectBrandBitmap(page: Page, name: 'icon_white' | 'icon_black') {
	const expected = Array.from(
		await readFile(new URL(`../../../../icon/${name}.png`, import.meta.url))
	);
	await expect
		.poll(() =>
			page
				.locator('a[href="#/"] img')
				.first()
				.evaluate(async (img: HTMLImageElement) => {
					if (!img.complete || !img.naturalWidth) return null;
					return Array.from(new Uint8Array(await (await fetch(img.currentSrc)).arrayBuffer()));
				})
		)
		.toEqual(expected);
}

const EN_LABELS = [
	'Overview',
	'Rule Traffic',
	'Proxy Hosts',
	'Live Connections',
	'Global Settings',
	'Configuration File'
];
const ZH_LABELS = ['概览', '规则流量', '跳板主机', '活动连接', '全局设置', '配置文件'];

test('the overview shows KPIs and one card per rule from the API', async ({ page, api }) => {
	await page.goto('/');
	await expect(heading(page)).toHaveText('Overview');
	await expect(page).toHaveTitle('Overview · JumpWay');
	await expect(kpi(page, 'rules')).toHaveText('2 Running');
	await expect(kpi(page, 'rule-states')).toHaveText('4 configured · 1 Retrying · 1 Disabled');
	await expect(kpi(page, 'active')).toHaveText(String(snapshotTotals.active));
	// The direction arrows carry their names for assistive technology.
	expect(await compact(kpi(page, 'rate'))).toBe(
		`Upload ${snapshotTotals.rateUp} Download ${snapshotTotals.rateDown}`
	);
	expect(await compact(kpi(page, 'total'))).toBe(
		`Upload ${snapshotTotals.up} Download ${snapshotTotals.down}`
	);

	const cards = page.getByRole('article');
	await expect(cards).toHaveCount(4);
	await expect(cards.locator('[data-state]')).toHaveText([
		'Running',
		'Running',
		'Retrying (3)',
		'Disabled'
	]);
	const office = cards.nth(0);
	await expect(office).toContainText('office');
	await expect(office).toContainText('127.0.0.1:18097');
	await expect(office).toContainText('2 hops');
	await expect(office).toContainText('12.0 KB/s');
	await expect(office.getByRole('link', { name: 'Edit' })).toHaveAttribute(
		'href',
		'#/rules/office'
	);
	await expect(office.getByRole('link', { name: 'Rule Traffic' })).toHaveAttribute(
		'href',
		'#/stats?rule=office'
	);
	await expect(cards.nth(1)).toContainText('10.0.0.5:5432');
	await expect(cards.nth(2)).toContainText('connection refused');
	await expect(cards.nth(2)).toContainText('listen through "ssh://xxxxx@edge.example:22"');

	await expect(page.getByText('Running', { exact: true }).first()).toBeVisible();
	await expect(page.getByText('127.0.0.1:1088')).toBeVisible();
	// The backend echoes proxy URLs verbatim in errors; the UI masks their userinfo everywhere.
	await expect(page.locator('aside')).toContainText(
		'invalid proxy URL "socks5://xxxxx@host:bad": parse "socks5://xxxxx@host:bad": invalid port ":bad" after host (e.g. socks5://host:1080)'
	);
	const html = await page.content();
	expect(html).not.toContain(SECRET);
	expect(html).not.toContain('review-user');
	expect(api.requests).toContain('GET /apis/configs/rules');
	expect(api.requests).toContain('GET /apis/configs/status');
});

test('the KPI strip folds to two columns beside the expanded sidebar and never splits a state segment', async ({
	page,
	api
}) => {
	// 768px wide: the sidebar is expanded and the content beside it is about 480px.
	await page.setViewportSize({ width: 768, height: 900 });
	// A slow status: meanwhile every enabled rule counts as "Status unknown", the longest segment.
	let release!: () => void;
	api.delay.set(
		'GET /apis/configs/status',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	await page.goto('/');
	await expect(page.getByRole('article')).toHaveCount(4);
	const states = kpi(page, 'rule-states');
	await expect(states).toHaveText('4 configured · 3 Status unknown · 1 Disabled');
	// A segment is its count, its name and the separator after it: one line of text-xs each, so
	// the line only ever breaks between segments and a separator never starts a line. The columns
	// here are wide enough for every segment, so the nowrap itself is asserted too.
	const expectSegmentsOnOneLineEach = async (texts: string[]) => {
		const segments = await states.locator('> span').evaluateAll((spans) =>
			spans.map((span) => ({
				text: span.textContent!.replace(/\s+/g, ' ').trim(),
				height: span.getBoundingClientRect().height,
				whiteSpace: getComputedStyle(span).whiteSpace
			}))
		);
		for (const segment of segments) {
			expect(segment.height, segment.text).toBeLessThan(20);
			expect(segment.whiteSpace, segment.text).toBe('nowrap');
		}
		expect(segments.map((segment) => segment.text)).toEqual(texts);
	};
	const overflow = () =>
		page.evaluate(() => {
			const main = document.getElementById('main')!;
			return main.scrollWidth - main.clientWidth;
		});
	const box = async (name: string) => (await kpi(page, name).boundingBox())!;
	await expectSegmentsOnOneLineEach(['4 configured ·', '3 Status unknown ·', '1 Disabled']);
	expect(await overflow()).toBeLessThanOrEqual(0);
	// Two columns: the rate tile sits below the rules tile.
	await expect
		.poll(async () => {
			const rules = await box('rules');
			return (await box('rate')).y - (rules.y + rules.height);
		})
		.toBeGreaterThan(0);

	// On a desktop the four tiles share one row.
	await page.setViewportSize({ width: 1280, height: 800 });
	await expect
		.poll(async () => Math.abs((await box('total')).y - (await box('rules')).y))
		.toBeLessThanOrEqual(2);
	release();
	await expect(kpi(page, 'rules')).toHaveText('2 Running');
	await expect(states).toHaveText('4 configured · 1 Retrying · 1 Disabled');
	await expectSegmentsOnOneLineEach(['4 configured ·', '1 Retrying ·', '1 Disabled']);
	expect(await overflow()).toBeLessThanOrEqual(0);
});

test('while the rule list is pending the overview grid holds three card skeletons, and the cards replace them in place', async ({
	page,
	api
}) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	const hold = () => {
		let release!: () => void;
		api.delay.set(
			'GET /apis/configs/rules',
			new Promise<void>((resolve) => {
				release = resolve;
			})
		);
		return release;
	};
	const release = hold();
	await page.goto('/');
	const section = page.locator('section[aria-label="Rules"]');
	const grid = section.locator('> div').first();
	const skeletonCards = grid.locator('[data-skeleton-card]');
	await expect(section).toHaveAttribute('aria-busy', 'true');
	await expect(skeletonCards).toHaveCount(3);
	await expect(skeletonCards.locator('[data-skeleton]').first()).toBeVisible();
	await expect(section.locator('.sr-only', { hasText: 'Loading...' })).toHaveCount(1);
	await expect(page.getByRole('article')).toHaveCount(0);
	// Tops are relative to the grid: the KPI line above it gains its state dots when the rules land.
	const boxes = (locator: Locator) =>
		locator.evaluateAll((elements) =>
			elements.map((element) => {
				const box = element.getBoundingClientRect();
				const grid = element.parentElement!.getBoundingClientRect();
				return { top: Math.round(box.top - grid.top), height: Math.round(box.height) };
			})
		);
	const overflow = () =>
		page.evaluate(() => {
			const main = document.getElementById('main')!;
			return main.scrollWidth - main.clientWidth;
		});
	// Three columns at 1280: the placeholders share one row and stand about as tall as a card.
	const placeholders = await boxes(skeletonCards);
	expect(new Set(placeholders.map((box) => box.top)).size).toBe(1);
	for (const box of placeholders) expect(box.height).toBeGreaterThan(160);
	expect(await overflow()).toBeLessThanOrEqual(0);
	const gridBefore = (await grid.boundingBox())!;
	if (process.env.JW_SHOTS)
		await page.screenshot({
			path: '/tmp/jw-polish-s5-shots/overview-skeleton-1280.png',
			animations: 'disabled'
		});
	release();
	await expect(page.getByRole('article')).toHaveCount(4);
	await expect(skeletonCards).toHaveCount(0);
	await expect(page.locator('[data-skeleton]')).toHaveCount(0);
	await expect(section).not.toHaveAttribute('aria-busy', 'true');
	await expect(section.locator('.sr-only', { hasText: 'Loading...' })).toHaveCount(0);
	// The same grid, as wide and in the same column; the first row of cards starts where the
	// placeholders did.
	const gridAfter = (await grid.boundingBox())!;
	expect([gridAfter.x, gridAfter.width]).toEqual([gridBefore.x, gridBefore.width]);
	expect(Math.abs(gridAfter.y - gridBefore.y)).toBeLessThan(2);
	const cards = await boxes(page.getByRole('article'));
	expect(cards.slice(0, 3).map((box) => box.top)).toEqual(placeholders.map((box) => box.top));
	expect(await overflow()).toBeLessThanOrEqual(0);
	// One column on a phone, where the card footer wraps its icons under the rates: the
	// placeholders must wrap the same way, or every card below the first lands lower than its
	// placeholder stood.
	await page.setViewportSize({ width: 375, height: 900 });
	const releasePhone = hold();
	await page.goto('/');
	await expect(skeletonCards).toHaveCount(3);
	const phonePlaceholders = await boxes(skeletonCards);
	expect(new Set(phonePlaceholders.map((box) => box.top)).size).toBe(3);
	expect(await overflow()).toBeLessThanOrEqual(0);
	releasePhone();
	await expect(page.getByRole('article')).toHaveCount(4);
	// The first two fixture rules carry no runtime error, so their cards are exactly a placeholder tall.
	const phoneCards = await boxes(page.getByRole('article'));
	expect(phoneCards.slice(0, 3).map((box) => box.top)).toEqual(
		phonePlaceholders.map((box) => box.top)
	);
	expect(phoneCards.slice(0, 2).map((box) => box.height)).toEqual(
		phonePlaceholders.slice(0, 2).map((box) => box.height)
	);
	expect(await overflow()).toBeLessThanOrEqual(0);
	if (process.env.JW_SHOTS) {
		await page.addInitScript(() => localStorage.setItem('jumpway.theme', 'dark'));
		const releaseDark = hold();
		await page.goto('/');
		await expect(skeletonCards).toHaveCount(3);
		await page.screenshot({
			path: '/tmp/jw-polish-s5-shots/overview-skeleton-1280-dark.png',
			animations: 'disabled'
		});
		releaseDark();
		await expect(page.getByRole('article')).toHaveCount(4);
	}
});

test('the rate KPI and every card trace the recent rates: the lines fill in with the polls, sized by their tiles, in the arrow colours of both themes', async ({
	page,
	api
}) => {
	const shots = process.env.JW_SHOTS ? '/tmp/jw-polish-s4b-shots/' : '';
	const shoot = async (name: string, target: Page | Locator = page) => {
		if (shots) await target.screenshot({ path: shots + name + '.png', animations: 'disabled' });
	};
	const kpiLine = page.locator('[data-kpi="rate"] ~ svg[data-sparkline]');
	const cards = page.getByRole('article');
	const cardLine = cards.nth(0).locator('footer svg[data-sparkline]');
	const office = api.snapshot.rules![0].stats;
	// Rates that move each second, so the lines have a shape worth looking at.
	const wobble = async (steps: number) => {
		for (let step = 1; step <= steps; step++) {
			office.rate_up = 12_288 * (1 + (step % 3));
			office.rate_down = 1_048_576 / (1 + (step % 4));
			await page.waitForResponse((response) => response.url().endsWith('/apis/stats'));
		}
	};
	const points = (locator: Locator) =>
		locator.locator('polyline').evaluateAll((lines) =>
			lines.map((line) =>
				line
					.getAttribute('points')!
					.split(' ')
					.map((p) => p.split(',').map(Number))
			)
		);
	const overflow = () =>
		page.evaluate(() => {
			const main = document.getElementById('main')!;
			return main.scrollWidth - main.clientWidth;
		});
	// The lines' resolved stroke colours and the opaque background behind the first one.
	const colours = (locator: Locator) =>
		locator.locator('polyline').evaluateAll((lines) => {
			const opaque = (element: Element | null): string => {
				for (let node = element; node; node = node.parentElement) {
					const background = getComputedStyle(node).backgroundColor;
					if (background && background !== 'rgba(0, 0, 0, 0)' && background !== 'transparent')
						return background;
				}
				return getComputedStyle(document.body).backgroundColor;
			};
			return {
				strokes: lines.map((line) => {
					const style = getComputedStyle(line);
					return style.stroke === 'currentcolor' ? style.color : style.stroke;
				}),
				background: opaque(lines[0]?.closest('svg') ?? null)
			};
		});

	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/');
	await expect(cards).toHaveCount(4);
	await expect(kpiLine).toHaveCount(1);
	await expect(cards.locator('footer svg[data-sparkline]')).toHaveCount(4);
	await expect(page.locator('[data-kpi="total"] ~ svg[data-sparkline]')).toHaveCount(0);
	await wobble(4);
	await expect.poll(async () => (await points(kpiLine)).length).toBe(2);
	await expect.poll(async () => (await points(cardLine)).length).toBe(2);
	for (const locator of [kpiLine, cardLine]) {
		const lines = await points(locator);
		for (const line of lines) {
			expect(line.length).toBeGreaterThanOrEqual(3);
			expect(line.at(-1)![0]).toBe(119);
		}
		await expect(locator).toHaveAttribute('aria-hidden', 'true');
	}
	// The tile's width and 24px tall; 64×16 inside a card footer, after both rates.
	const kpiBox = (await kpiLine.boundingBox())!;
	expect(kpiBox.width).toBeGreaterThanOrEqual(100);
	expect(Math.round(kpiBox.height)).toBe(24);
	const cardBox = (await cardLine.boundingBox())!;
	expect([Math.round(cardBox.width), Math.round(cardBox.height)]).toEqual([64, 16]);
	const rates = (await cards.nth(0).locator('footer > span').first().boundingBox())!;
	expect(cardBox.x).toBeGreaterThan(rates.x);
	expect(cardBox.x + cardBox.width).toBeLessThanOrEqual(rates.x + rates.width + 1);
	// The numbers read as before; the line adds no text.
	expect(await compact(kpi(page, 'rate'))).toMatch(/^Upload \S+ \S+ Download \S+ \S+$/);
	expect(await overflow()).toBeLessThanOrEqual(0);
	const light = await colours(kpiLine);
	expect(new Set([...light.strokes, light.background]).size).toBe(3);
	await shoot('overview-1280-trend');
	await shoot('kpi-rate', kpi(page, 'rate').locator('..'));
	await shoot('card-footer', cards.nth(0).locator('footer'));

	// Dark theme: still two distinct colours, neither the background's.
	await page.addInitScript(() => localStorage.setItem('jumpway.theme', 'dark'));
	await page.goto('/');
	await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
	await expect(cards).toHaveCount(4);
	await wobble(3);
	await expect.poll(async () => (await points(kpiLine)).length).toBe(2);
	const dark = await colours(kpiLine);
	expect(new Set([...dark.strokes, dark.background]).size).toBe(3);
	expect(dark.background).not.toBe(light.background);
	expect(await overflow()).toBeLessThanOrEqual(0);
	await shoot('overview-1280-trend-dark');

	// A phone: the footer wraps its actions under the rates; nothing leaves the card.
	await page.addInitScript(() => localStorage.removeItem('jumpway.theme'));
	await page.setViewportSize({ width: 375, height: 812 });
	await page.goto('/');
	await expect(cards).toHaveCount(4);
	await wobble(3);
	await expect.poll(async () => (await points(cardLine)).length).toBe(2);
	expect(await overflow()).toBeLessThanOrEqual(0);
	const inside = await cards.evaluateAll((articles) =>
		articles.every((article) => {
			const box = article.getBoundingClientRect();
			return Array.from(
				article.querySelectorAll('svg[data-sparkline], footer a, footer button')
			).every((element) => {
				const rect = element.getBoundingClientRect();
				return rect.left >= box.left - 1 && rect.right <= box.right + 1;
			});
		})
	);
	expect(inside).toBe(true);
	await shoot('overview-375-trend');
});

test('sidebar links use current routes and unknown addresses return home', async ({ page }) => {
	await page.goto('/');
	await expectNavLabelsFit(page, EN_LABELS);
	await nav(page).getByRole('link', { name: 'Rule Traffic' }).click();
	await expect(page).toHaveURL(/#\/stats$/);
	await expect(heading(page)).toHaveText('Rule Traffic');
	await expect(nav(page).locator('a[aria-current="page"]')).toHaveAttribute('href', '#/stats');
	await expect(page).toHaveTitle('Rule Traffic · JumpWay');

	for (const route of [
		'web-ui',
		'no-proxy',
		'stats/rules',
		'stats/connections',
		'rules',
		'nowhere'
	]) {
		await page.goto(`/#/${route}?rule=office`);
		await expect(page).toHaveURL(/#\/$/);
		await expect(heading(page)).toHaveText('Overview');
		await expect(nav(page).locator('a[aria-current="page"]')).toHaveAttribute('href', '#/');
	}

	await page.goto('/#/rules/office');
	await expect(heading(page)).toHaveText('office');
	await expect(nav(page).locator('a[aria-current="page"]')).toHaveAttribute('href', '#/');
	await page.goto('/#/new');
	await expect(heading(page)).toHaveText('New rule');
	await expect(nav(page).locator('a[aria-current="page"]')).toHaveAttribute('href', '#/');

	await page.goto('/#/nowhere');
	await expect(page).toHaveURL(/#\/$/);
	await expect(heading(page)).toHaveText('Overview');
});

test('desktop sidebar becomes a persistent icon rail with working preferences', async ({
	page
}) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/');
	const aside = page.locator('aside');
	const main = page.getByRole('main');
	await expect.poll(async () => (await aside.boundingBox())?.width).toBe(224);
	const initialMainWidth = (await main.boundingBox())!.width;
	await page.getByRole('button', { name: 'Collapse sidebar', exact: true }).click();
	const expand = page.getByRole('button', { name: 'Expand sidebar', exact: true });
	await expect(expand).toBeFocused();
	await expect(expand).toHaveAttribute('aria-expanded', 'false');
	await expect.poll(async () => (await aside.boundingBox())?.width).toBe(64);
	expect((await main.boundingBox())!.width).toBe(initialMainWidth + 160);
	expect(await page.evaluate(() => localStorage.getItem('jumpway.sidebarCollapsed'))).toBe('true');
	await expect(nav(page).getByRole('link')).toHaveCount(6);
	const connections = nav(page).getByRole('link', { name: 'Live Connections', exact: true });
	await connections.focus();
	await expect(page.locator('#tooltip')).toHaveText('Live Connections');
	await connections.click();
	await expect(heading(page)).toHaveText('Live Connections');
	await expect(nav(page).locator('a[aria-current="page"]')).toHaveAttribute(
		'href',
		'#/connections'
	);
	await page.reload();
	await expect.poll(async () => (await aside.boundingBox())?.width).toBe(64);

	const preferences = aside.getByRole('button', { name: 'Preferences', exact: true });
	const popup = page.locator('#preferences-menu');
	await preferences.click();
	await expect(popup).toBeVisible();
	await expect(preferences).toHaveAttribute('aria-expanded', 'true');
	await page.setViewportSize({ width: 1024, height: 600 });
	await expect
		.poll(async () => {
			const box = (await popup.boundingBox())!;
			return box.y >= 8 && box.y + box.height <= 592 && box.x + box.width <= 1016;
		})
		.toBe(true);
	await page.screenshot({
		path: test.info().outputPath('collapsed-sidebar-preferences.png'),
		animations: 'disabled'
	});
	await page.setViewportSize({ width: 1280, height: 800 });
	await popup.getByRole('button', { name: 'Dark', exact: true }).click();
	await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
	await popup.getByRole('button', { name: '中文', exact: true }).click();
	await expect(heading(page)).toHaveText('活动连接');
	await page.keyboard.press('Escape');
	await expect(popup).not.toBeVisible();
	await expect(aside.locator('button[popovertarget="preferences-menu"]')).toHaveAttribute(
		'aria-expanded',
		'false'
	);
	await expect(aside.locator('ul[aria-label] a[href="/metrics"]')).toBeVisible();
	await aside.locator('button[popovertarget="preferences-menu"]').click();
	await expect(popup).toBeVisible();
	await popup.getByRole('button', { name: '深色', exact: true }).focus();
	await page.keyboard.press('Tab');
	await expect(popup).not.toBeVisible();
	await page.screenshot({
		path: test.info().outputPath('collapsed-sidebar-zh-dark.png'),
		animations: 'disabled'
	});

	await page.setViewportSize({ width: 375, height: 812 });
	await expect(aside).toHaveCount(0);
	await page.getByRole('button', { name: '打开导航', exact: true }).click();
	await expect(page.locator('#navigation-drawer')).toBeVisible();
	await expect(page.locator('#navigation-drawer nav')).toContainText('活动连接');
	await page.keyboard.press('Escape');
	await page.setViewportSize({ width: 1024, height: 600 });
	await expect.poll(async () => (await aside.boundingBox())?.width).toBe(64);
	await aside.locator('a[href="/metrics"]').scrollIntoViewIfNeeded();
	await expect(aside.locator('a[href="/metrics"]')).toBeVisible();
	await expect(page.locator('#navigation-drawer[open]')).toHaveCount(0);
	await page.getByRole('button', { name: '展开侧边栏', exact: true }).click();
	await expect.poll(async () => (await aside.boundingBox())?.width).toBe(224);
	await expect(nav(page)).toContainText('活动连接');
});

// JW_SHOTS=1 saves review screenshots of the sidebar states under /tmp.
const shot = (page: Page, name: string) =>
	process.env.JW_SHOTS
		? page.screenshot({ path: `/tmp/jw-polish-s2-shots/${name}.png`, animations: 'disabled' })
		: undefined;

test('the sidebar clamps the runtime error to three lines until it is clicked open', async ({
	page
}) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/');
	const aside = page.locator('aside');
	const error = aside.locator('button[aria-expanded]', { hasText: 'invalid proxy URL' });
	const fits = () => aside.evaluate((element) => element.scrollWidth <= element.clientWidth);
	await expect(error).toHaveAttribute('aria-expanded', 'false');
	// text-xs lines are 16px tall: three of them and nothing more.
	const clamped = (await error.boundingBox())!.height;
	expect(clamped).toBeLessThan(3 * 16 + 6);
	expect(await fits()).toBe(true);

	await error.click();
	await expect(error).toHaveAttribute('aria-expanded', 'true');
	await expect(error).toBeFocused();
	await expect.poll(async () => (await error.boundingBox())!.height).toBeGreaterThan(clamped);
	await expect(error).toHaveText(
		'rules[3].forward.way[0]: invalid proxy URL "socks5://xxxxx@host:bad": parse "socks5://xxxxx@host:bad": invalid port ":bad" after host (e.g. socks5://host:1080)'
	);
	expect(await error.evaluate((element) => element.scrollHeight <= element.clientHeight + 1)).toBe(
		true
	);
	expect(await fits()).toBe(true);
	await shot(page, 'sidebar-error-expanded');

	await error.click();
	await expect(error).toHaveAttribute('aria-expanded', 'false');
	await expect.poll(async () => (await error.boundingBox())!.height).toBe(clamped);
});

test('the sidebar toggle ends the sidebar: beside the resource links, centred in the rail, absent from the drawer', async ({
	page
}) => {
	await page.setViewportSize({ width: 1280, height: 800 });
	await page.goto('/');
	const aside = page.locator('aside');
	const toggle = aside.locator('button[aria-controls="sidebar"]');
	await expect(toggle).toHaveAttribute('aria-expanded', 'true');
	await expect(aside.locator(':scope > :first-child button')).toHaveCount(0);
	const rail = (await aside.boundingBox())!;
	const box = (await toggle.boundingBox())!;
	const github = (await aside.getByRole('link', { name: 'GitHub', exact: true }).boundingBox())!;
	expect(rail.y + rail.height - (box.y + box.height)).toBeLessThan(80);
	expect(box.x).toBeGreaterThanOrEqual(github.x + github.width);
	await shot(page, 'sidebar-1280-expanded');

	await toggle.click();
	await expect(toggle).toBeFocused();
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	await expect.poll(async () => (await aside.boundingBox())?.width).toBe(64);
	await expect
		.poll(() =>
			aside.evaluate((element) => {
				const button = element.querySelector('button[aria-controls="sidebar"]')!;
				const box = button.getBoundingClientRect();
				const rail = element.getBoundingClientRect();
				const others = Array.from(element.querySelectorAll('a, button')).filter(
					(other) => other !== button
				);
				return {
					lowest: others.every((other) => other.getBoundingClientRect().y < box.y),
					centred: Math.abs(box.x + box.width / 2 - rail.x - 32) < 2
				};
			})
		)
		.toEqual({ lowest: true, centred: true });
	await expect(aside.locator(':scope > :first-child button')).toHaveCount(0);
	await shot(page, 'sidebar-1280-collapsed');

	// The drawer ignores the rail choice and never offers the toggle.
	await page.setViewportSize({ width: 375, height: 800 });
	await expect(aside).toHaveCount(0);
	await page.getByRole('button', { name: 'Open navigation', exact: true }).click();
	const drawer = page.locator('dialog#navigation-drawer');
	await expect(drawer).toHaveAttribute('open', '');
	await expectNavLabelsFit(page, EN_LABELS);
	await expect(drawer.getByRole('link', { name: 'GitHub', exact: true })).toBeVisible();
	await expect(drawer.locator('[aria-controls="sidebar"]')).toHaveCount(0);
	await shot(page, 'sidebar-375-drawer');
});

test('?lang=zh renders Chinese; the switch persists across reloads and ?lang= still wins', async ({
	page
}) => {
	await page.goto('/?lang=zh#/');
	await expect(heading(page)).toHaveText('概览');
	await expect(page.locator('html')).toHaveAttribute('lang', 'zh-CN');
	await expectNavLabelsFit(page, ZH_LABELS);

	await page.goto('/');
	await expect(heading(page)).toHaveText('Overview');
	await page.getByRole('button', { name: '中文' }).click();
	await expect(heading(page)).toHaveText('概览');
	await expect(page.getByRole('button', { name: '中文' })).toHaveAttribute('aria-pressed', 'true');
	expect(await page.evaluate(() => localStorage.getItem('jumpway.lang'))).toBe('zh');

	await page.reload();
	await expect(heading(page)).toHaveText('概览');
	await expect(page).toHaveTitle('概览 · JumpWay');

	await page.goto('/?lang=en#/settings');
	await expect(heading(page)).toHaveText('Global Settings');
	await page.getByRole('button', { name: '中文' }).click();
	await expect(page).toHaveURL(/\?lang=zh#\/settings$/);
	await expect(heading(page)).toHaveText('全局设置');
});

test('the theme switch stamps data-theme, swaps the brand bitmap and persists', async ({
	page
}) => {
	await page.goto('/');
	const html = page.locator('html');
	await expect(html).toHaveAttribute('data-theme', /light|dark/);
	await page.getByRole('button', { name: 'Dark' }).click();
	await expect(html).toHaveAttribute('data-theme', 'dark');
	await expect(page.getByRole('button', { name: 'Dark' })).toHaveAttribute('aria-pressed', 'true');
	await expectBrandBitmap(page, 'icon_white');
	const background = await page.evaluate(
		() => getComputedStyle(document.documentElement).backgroundColor
	);
	expect(background).toBe('rgb(23, 25, 27)');
	expect(await page.evaluate(() => localStorage.getItem('jumpway.theme'))).toBe('dark');

	await page.reload();
	await expect(html).toHaveAttribute('data-theme', 'dark');
	await expectBrandBitmap(page, 'icon_white');
	await page.getByRole('button', { name: 'Light' }).click();
	await expect(html).toHaveAttribute('data-theme', 'light');
	await expectBrandBitmap(page, 'icon_black');
});

test.describe('mobile', () => {
	test.use({ viewport: { width: 375, height: 812 } });

	test('the drawer opens from the header, closes on Escape and after navigating, and restores focus', async ({
		page
	}) => {
		await page.goto('/');
		await expect(page.locator('aside')).toHaveCount(0);
		const menu = page.getByRole('button', { name: 'Open navigation' });
		const drawer = page.locator('dialog#navigation-drawer');
		await expect(menu).toHaveAttribute('aria-expanded', 'false');
		await expect(drawer).not.toHaveAttribute('open', '');

		await menu.click();
		await expect(drawer).toHaveAttribute('open', '');
		await expect(menu).toHaveAttribute('aria-expanded', 'true');
		await expect(drawer.getByRole('link', { name: 'Global Settings' })).toBeVisible();
		await expectNavLabelsFit(page, EN_LABELS);
		await page.keyboard.press('Escape');
		await expect(drawer).not.toHaveAttribute('open', '');
		await expect(menu).toBeFocused();

		await menu.click();
		await drawer.getByRole('link', { name: 'Global Settings' }).click();
		await expect(page).toHaveURL(/#\/settings$/);
		await expect(heading(page)).toHaveText('Global Settings');
		await expect(drawer).not.toHaveAttribute('open', '');
		await expect(menu).toBeFocused();

		await page.goto('/');
		await expect(page.getByRole('article')).toHaveCount(4);
		const overflow = await page.evaluate(() => {
			const main = document.getElementById('main')!;
			return { scrollWidth: main.scrollWidth, clientWidth: main.clientWidth };
		});
		expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth);
	});

	test('tooltips inside the open drawer paint above it, hide on blur and Escape, and keep the focus flow', async ({
		page
	}) => {
		await page.goto('/');
		const menu = page.getByRole('button', { name: 'Open navigation' });
		const drawer = page.locator('dialog#navigation-drawer');
		const tip = page.locator('#tooltip');
		await menu.click();
		await expect(drawer).toHaveAttribute('open', '');
		expect(await drawer.evaluate((element) => element.matches(':modal'))).toBe(true);

		const github = drawer.getByRole('link', { name: 'GitHub' });
		await github.focus();
		await expect(tip).toHaveText('GitHub');
		await expect(github).toHaveAttribute('aria-describedby', 'tooltip');
		// In the top layer after the modal drawer, so it is painted over it rather than behind it.
		expect(await tip.evaluate((element) => element.matches(':popover-open'))).toBe(true);
		const box = (await tip.boundingBox())!;
		const panel = (await drawer.locator('> div').boundingBox())!;
		expect(box.x + box.width).toBeLessThanOrEqual(panel.x + panel.width);
		const clip = { x: box.x, y: box.y, width: box.width, height: box.height };
		const evidence = test.info().outputPath('mobile-drawer-tooltip.png');
		await page.screenshot({ path: evidence, animations: 'disabled' });
		test.info().attach('mobile-drawer-tooltip', { path: evidence, contentType: 'image/png' });
		const shown = await page.screenshot({ clip, animations: 'disabled' });
		await github.blur();
		await expect(tip).toHaveCount(0);
		const hidden = await page.screenshot({ clip, animations: 'disabled' });
		expect(shown.equals(hidden)).toBe(false);

		await drawer.getByRole('button', { name: 'Dark' }).hover();
		await expect(tip).toHaveText('Dark');
		expect(await tip.evaluate((element) => element.matches(':popover-open'))).toBe(true);
		await page.keyboard.press('Escape');
		await expect(drawer).not.toHaveAttribute('open', '');
		await expect(menu).toBeFocused();
		// The drawer's tooltip went with it. The pointer still rests where Dark was, over whatever the
		// page shows there now, so bring it to the focused button before reading its own tooltip.
		await expect(tip).not.toHaveText('Dark');
		await menu.hover();
		await expect(tip).toHaveText('Open navigation');
		expect(await tip.evaluate((element) => element.matches(':popover-open'))).toBe(true);
		await page.mouse.move(200, 400);
		await menu.blur();
		await expect(tip).toHaveCount(0);
	});
});

test('an unreachable backend shows the recovery banner until a retry succeeds', async ({
	page,
	api,
	failures
}) => {
	// The refused status request itself is the point of this test.
	failures.allow = (entry) =>
		entry.includes('/apis/configs/status') || entry.includes('ERR_CONNECTION_REFUSED');
	api.down.add('/apis/configs/status');
	await page.goto('/');
	const banner = page.getByRole('main').getByRole('alert');
	await expect(banner).toContainText('Cannot reach JumpWay.');
	await expect(banner).toContainText('Edit Config');
	const runtime = page.locator('aside [data-state]').first();
	await expect(runtime).toHaveAttribute('data-state', 'unknown');
	await expect(runtime).toHaveText('Status unknown');
	await expect(kpi(page, 'active')).toHaveText(String(snapshotTotals.active));

	api.down.delete('/apis/configs/status');
	await banner.getByRole('button', { name: 'Retry' }).click();
	await expect(banner).toHaveCount(0);
	await expect(runtime).toHaveAttribute('data-state', 'running');
	await expect(runtime).toHaveText('Running');
});

test('stats poll every second on the overview and pause on a placeholder page', async ({
	page,
	api
}) => {
	await page.goto('/');
	await expect.poll(() => count(api, 'GET /apis/stats')).toBeGreaterThanOrEqual(3);
	await nav(page).getByRole('link', { name: 'Global Settings' }).click();
	await expect(heading(page)).toHaveText('Global Settings');
	const paused = count(api, 'GET /apis/stats');
	await page.waitForTimeout(2_500);
	expect(count(api, 'GET /apis/stats')).toBeLessThanOrEqual(paused + 1);
	await nav(page).getByRole('link', { name: 'Overview' }).click();
	await expect.poll(() => count(api, 'GET /apis/stats')).toBeGreaterThan(paused + 1);
});

test('a card switch re-reads the rule, writes it back with only `disabled` changed and reports honestly', async ({
	page,
	api,
	failures
}) => {
	failures.allow = (entry) => entry.startsWith('http 400') || entry.includes('status of 400');
	await page.goto('/');
	const cards = page.getByRole('article');
	await expect(cards).toHaveCount(4);
	const office = cards.nth(0);
	const toggle = office.getByRole('switch', { name: 'Enabled' });
	await expect(toggle).toBeChecked();
	await expect(cards.nth(3).getByRole('switch', { name: 'Enabled' })).not.toBeChecked();

	// The overview's copy is stale: the exit chain grew behind its back; the write must carry it.
	const latestWay = [...(api.rules[0].forward.way as unknown[]), 'ssh://ops@third.example:22'];
	api.rules[0].forward.way = latestWay as Rule['forward']['way'];
	const statusReads = count(api, 'GET /apis/configs/status');
	let release!: () => void;
	api.delay.set(
		'PUT /apis/configs/rules/office',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	await toggle.click();
	await expect(toggle).toBeDisabled();
	await expect(toggle).toBeChecked();
	await expect(office.locator('[data-state]')).toHaveText('Running');
	// A second toggle while pending (Space on the still-focused control) must not write again.
	await page.keyboard.press('Space');
	release();
	await expect(toggle).not.toBeChecked();
	await expect(toggle).toBeEnabled();
	// Keyboard users keep their place: the answered switch takes focus back.
	await expect(toggle).toBeFocused();
	await expect(office.locator('[data-state]')).toHaveText('Disabled');
	expect(api.requests.filter((entry) => entry.endsWith('/rules/office'))).toEqual([
		'GET /apis/configs/rules/office',
		'PUT /apis/configs/rules/office'
	]);
	expect(api.writes).toEqual([
		{
			method: 'PUT',
			path: '/apis/configs/rules/office',
			body: { ...rulesFixture[0], forward: { way: latestWay }, disabled: true }
		}
	]);
	await expect(page.getByRole('status').filter({ hasText: 'Saved and applied.' })).toBeVisible();
	await expect.poll(() => count(api, 'GET /apis/configs/status')).toBeGreaterThan(statusReads);
	expect(await page.content()).not.toContain('third.example');

	// Enabling drops `disabled` and keeps the credentials exactly as stored.
	const lab = cards.nth(3);
	await lab.getByRole('switch', { name: 'Enabled' }).click();
	await expect(lab.getByRole('switch', { name: 'Enabled' })).toBeChecked();
	const { disabled: _, ...labEnabled } = rulesFixture[3];
	expect(api.writes[1].body).toEqual(labEnabled);
	expect(await page.content()).not.toContain('placeholder');

	// A rejected write leaves the switch where it was and says why.
	const mirror = cards.nth(1).getByRole('switch', { name: 'Enabled' });
	api.fail.set('PUT /apis/configs/rules/mirror', 'rules[1].listen.port 18098 is in use');
	await mirror.click();
	await expect(page.getByRole('alert').filter({ hasText: '18098 is in use' })).toBeVisible();
	await expect(mirror).toBeChecked();
	await expect(mirror).toBeEnabled();
	expect(api.rules[1].disabled).toBeUndefined();

	// Written to disk but not applied: the stored value shows, with a warning instead of a success.
	const tunnel = cards.nth(2).getByRole('switch', { name: 'Enabled' });
	api.fail.set('PUT /apis/configs/rules/db-tunnel', 'saved, but reload failed: store is locked');
	await tunnel.click();
	await expect(page.getByRole('alert').filter({ hasText: 'store is locked' })).toBeVisible();
	await expect(tunnel).not.toBeChecked();
	expect(api.rules[2].disabled).toBe(true);
	expect(api.writes).toHaveLength(4);
});

test('a switch whose re-read is still pending when the page is left never writes what it read', async ({
	page,
	api,
	failures
}) => {
	// Leaving the overview abandons the read; Chrome reports the aborted fetch as a failed request.
	failures.allow = (entry) =>
		entry.startsWith('requestfailed: ') && entry.endsWith('/apis/configs/rules/office');
	await page.goto('/');
	const cards = page.getByRole('article');
	await expect(cards).toHaveCount(4);
	const office = cards.nth(0);
	const toggle = office.getByRole('switch', { name: 'Enabled' });
	let release!: () => void;
	api.delay.set(
		'GET /apis/configs/rules/office',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	await toggle.click();
	await expect(toggle).toBeDisabled();
	await expect.poll(() => count(api, 'GET /apis/configs/rules/office')).toBe(1);
	expect(api.writes).toEqual([]);

	// Meanwhile the rule is edited and saved in the editor.
	await office.getByRole('link', { name: 'Edit' }).click();
	await expect(heading(page)).toHaveText('office');
	const port = page.getByLabel('Port', { exact: true });
	await expect(port).toHaveValue('18097');
	await port.fill('18123');
	await page.getByRole('button', { name: 'Save & Apply' }).click();
	await expect(page.getByRole('status').filter({ hasText: 'Saved and applied.' })).toBeVisible();
	expect(api.rules[0].listen.port).toBe(18123);
	expect(api.writes).toHaveLength(1);

	// The stale answer (port 18097) arrives now: it must not turn into a second write.
	release();
	await page.waitForTimeout(500);
	expect(api.writes).toHaveLength(1);
	expect(api.rules[0].listen.port).toBe(18123);
	expect(api.rules[0].disabled).toBeUndefined();
	await expect(port).toHaveValue('18123');
	expect(
		await page.getByRole('status').filter({ hasText: 'Saved and applied.' }).count()
	).toBeLessThanOrEqual(1);
	await expect(page.getByRole('alert')).toHaveCount(0);
});

test('a list answer that predates an accepted write cannot undo it: a switch answered while the re-read after a delete is still open', async ({
	page,
	api,
	failures
}) => {
	// The superseded re-read is abandoned; Chrome reports the aborted fetch as a failed request.
	failures.allow = (entry) =>
		entry.startsWith('requestfailed: ') && entry.endsWith('/apis/configs/rules');
	await page.goto('/');
	const cards = page.getByRole('article');
	await expect(cards).toHaveCount(4);
	const section = page.getByRole('main').locator('section[aria-label="Rules"]');
	const confirmDialog = page.locator('dialog[open][aria-describedby="confirm-message"]');
	// The re-read after the delete is slow; its payload is fixed when the request arrives.
	let release!: () => void;
	api.delay.set(
		'GET /apis/configs/rules',
		new Promise<void>((resolve) => {
			release = resolve;
		})
	);
	await cards.nth(0).getByRole('button', { name: 'Delete' }).click();
	await expect(confirmDialog).toContainText('Delete rule "office"?');
	await confirmDialog.getByRole('button', { name: 'Delete' }).click();
	await expect(cards).toHaveCount(3);
	await expect(cards.locator('header a')).toHaveText(['mirror', 'db-tunnel', 'lab']);
	await expect.poll(() => count(api, 'GET /apis/configs/rules')).toBe(2);
	await expect(section).toHaveAttribute('aria-busy', 'true');

	// Meanwhile mirror is switched off and the backend confirms it.
	const mirror = cards.nth(0);
	const toggle = mirror.getByRole('switch', { name: 'Enabled' });
	await toggle.click();
	await expect(toggle).not.toBeChecked();
	await expect(toggle).toBeEnabled();
	await expect(mirror.locator('[data-state]')).toHaveText('Disabled');
	expect(api.rules.map((rule) => rule.name)).toEqual(['mirror', 'db-tunnel', 'lab']);
	expect(api.rules[0].disabled).toBe(true);
	expect(api.writes.map((write) => `${write.method} ${write.path}`)).toEqual([
		'DELETE /apis/configs/rules/office',
		'PUT /apis/configs/rules/mirror'
	]);

	// The slow answer (mirror still enabled) arrives now: it is superseded, not applied.
	release();
	await page.waitForTimeout(500);
	await expect(toggle).not.toBeChecked();
	await expect(toggle).toBeEnabled();
	await expect(mirror.locator('[data-state]')).toHaveText('Disabled');
	await expect(mirror.getByRole('button', { name: 'Delete' })).toBeEnabled();
	await expect(cards.locator('header a')).toHaveText(['mirror', 'db-tunnel', 'lab']);
	await expect(section).toHaveAttribute('aria-busy', 'false');
	await expect(kpi(page, 'rules')).toHaveText('0 Running');
	await expect(kpi(page, 'rule-states')).toHaveText('3 configured · 1 Retrying · 2 Disabled');
	await expect(page.getByRole('status').filter({ hasText: 'Rule deleted.' })).toBeVisible();
	await expect(page.getByRole('status').filter({ hasText: 'Saved and applied.' })).toBeVisible();
	await expect(page.getByRole('alert')).toHaveCount(0);
	expect(api.writes).toHaveLength(2);
	// What the overview shows is what the backend stores.
	expect(
		await cards
			.getByRole('switch', { name: 'Enabled' })
			.evaluateAll((elements) => elements.map((element) => (element as HTMLInputElement).checked))
	).toEqual(api.rules.map((rule) => !rule.disabled));
});

test.describe('application shell', () => {
	// The viewport is the shell: the document never scrolls, #main does.
	const shell = (page: Page) =>
		page.evaluate(() => {
			const main = document.getElementById('main')!;
			const root = document.documentElement;
			const rect = (element: Element | null) => {
				const box = element?.getBoundingClientRect();
				return box ? { top: box.top, bottom: box.bottom, height: box.height } : null;
			};
			return {
				innerHeight: window.innerHeight,
				pageScrollY: window.scrollY,
				documentScrollable: root.scrollHeight > root.clientHeight,
				main: {
					scrollTop: main.scrollTop,
					scrollHeight: main.scrollHeight,
					clientHeight: main.clientHeight,
					scrollWidth: main.scrollWidth,
					clientWidth: main.clientWidth
				},
				aside: rect(document.querySelector('aside')),
				header: rect(document.querySelector('header'))
			};
		});
	// Brand, runtime, nav and footer stack in order inside the sidebar without overlapping.
	const sidebarStack = (page: Page) =>
		page.locator('aside').evaluate((aside) => {
			const parts = [
				aside.querySelector('a[href="#/"]'),
				aside.querySelector('[data-state]'),
				aside.querySelector('nav'),
				aside.querySelector('[aria-label="Resources"]')
			];
			return parts.map((part) => {
				const box = part!.getBoundingClientRect();
				return { top: Math.round(box.top), bottom: Math.round(box.bottom) };
			});
		});
	const scrollMain = async (page: Page, x: number, y: number) => {
		await page.mouse.move(x, y);
		await page.mouse.wheel(0, 600);
		await expect.poll(async () => (await shell(page)).main.scrollTop).toBeGreaterThan(0);
	};

	test('desktop: the sidebar spans the viewport, main owns the scroll, and a resize keeps it that way', async ({
		page
	}) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/#/rules/office');
		await expect(heading(page)).toHaveText('office');
		const save = page.getByRole('button', { name: 'Save & Apply' });
		await expect(save).toBeVisible();
		let state = await shell(page);
		expect(state.documentScrollable).toBe(false);
		expect(state.main.scrollHeight).toBeGreaterThan(state.main.clientHeight);
		expect(state.main.scrollWidth).toBeLessThanOrEqual(state.main.clientWidth);
		expect(state.aside).toMatchObject({ top: 0, bottom: 800 });
		// The page-level sticky action bar is pinned to the main scroller's bottom edge.
		const bar = (await save.boundingBox())!;
		expect(bar.y + bar.height).toBeLessThanOrEqual(800);
		expect(bar.y).toBeGreaterThan(600);

		await scrollMain(page, 800, 400);
		state = await shell(page);
		expect(state.pageScrollY).toBe(0);
		expect(state.aside).toMatchObject({ top: 0, bottom: 800 });
		await expect(nav(page).getByRole('link', { name: 'Overview' })).toBeInViewport();
		await expect(page.locator('aside').getByRole('link', { name: 'GitHub' })).toBeInViewport();
		await expect(save).toBeInViewport();

		// A short viewport: the sidebar shrinks with it and scrolls internally instead of clipping.
		await page.setViewportSize({ width: 1024, height: 600 });
		state = await shell(page);
		expect(state.documentScrollable).toBe(false);
		expect(state.aside).toMatchObject({ top: 0, bottom: 600 });
		expect(state.main.clientHeight).toBe(600);
		const github = page.locator('aside').getByRole('link', { name: 'GitHub' });
		await github.scrollIntoViewIfNeeded();
		await expect(github).toBeInViewport();
		await expect(nav(page).getByRole('link', { name: 'Configuration File' })).toBeInViewport();
		const stack = await sidebarStack(page);
		for (let index = 1; index < stack.length; index++) {
			expect(stack[index].top, `sidebar part ${index}`).toBeGreaterThanOrEqual(
				stack[index - 1].bottom
			);
		}
		expect(stack[0].top).toBeGreaterThanOrEqual(0);
		expect(stack.at(-1)!.bottom).toBeLessThanOrEqual(600);

		// Growing back: no stale height from the smaller viewport.
		await page.setViewportSize({ width: 1440, height: 900 });
		state = await shell(page);
		expect(state.aside).toMatchObject({ top: 0, bottom: 900 });
		expect(state.main.clientHeight).toBe(900);
		expect(state.documentScrollable).toBe(false);

		// Navigating to another page starts it at the top.
		await scrollMain(page, 900, 450);
		await nav(page).getByRole('link', { name: 'Rule Traffic' }).click();
		await expect(heading(page)).toHaveText('Rule Traffic');
		expect((await shell(page)).main.scrollTop).toBe(0);
	});

	for (const viewport of [
		{ width: 375, height: 812 },
		{ width: 390, height: 844 }
	]) {
		test(`mobile ${viewport.width}x${viewport.height}: the top bar stays put, main scrolls under it and the drawer fits the viewport`, async ({
			page
		}) => {
			await page.setViewportSize(viewport);
			await page.goto('/#/rules/office');
			await expect(heading(page)).toHaveText('office');
			let state = await shell(page);
			expect(state.documentScrollable).toBe(false);
			expect(state.header).toMatchObject({ top: 0, height: 48 });
			expect(state.main.clientHeight).toBe(viewport.height - 48);
			expect(state.main.scrollHeight).toBeGreaterThan(state.main.clientHeight);
			expect(state.main.scrollWidth).toBeLessThanOrEqual(state.main.clientWidth);

			await scrollMain(page, viewport.width / 2, viewport.height / 2);
			state = await shell(page);
			expect(state.pageScrollY).toBe(0);
			expect(state.header).toMatchObject({ top: 0, height: 48 });
			await expect(page.getByRole('button', { name: 'Save & Apply' })).toBeInViewport();

			await page.getByRole('button', { name: 'Open navigation' }).click();
			const drawer = page.locator('dialog#navigation-drawer');
			await expect(drawer).toHaveAttribute('open', '');
			const panel = (await drawer.locator('> div').boundingBox())!;
			expect(panel.y).toBe(0);
			expect(panel.height).toBeLessThanOrEqual(viewport.height);
			await expect(drawer.getByRole('link', { name: 'GitHub' })).toBeInViewport();
		});
	}
});

test.describe('screenshots', () => {
	// Viewport shots: the shell is the viewport, so a "full page" is what the user sees.
	const shoot = async (page: Page, name: string) => {
		const path = test.info().outputPath(name + '.png');
		await page.screenshot({ path, animations: 'disabled' });
		test.info().attach(name, { path, contentType: 'image/png' });
	};

	test('desktop en light and zh dark', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/');
		await expect(page.getByRole('article')).toHaveCount(4);
		await shoot(page, 'desktop-en-light');
		await page.getByRole('button', { name: 'Dark' }).click();
		await page.getByRole('button', { name: '中文' }).click();
		await expect(heading(page)).toHaveText('概览');
		await shoot(page, 'desktop-zh-dark');
	});

	test('desktop long page scrolled and a short viewport', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/#/rules/office');
		await expect(heading(page)).toHaveText('office');
		await page.mouse.move(800, 400);
		await page.mouse.wheel(0, 600);
		await expect
			.poll(() => page.evaluate(() => document.getElementById('main')!.scrollTop))
			.toBeGreaterThan(0);
		await shoot(page, 'desktop-editor-scrolled');
		await page.setViewportSize({ width: 1024, height: 600 });
		await page.goto('/');
		await expect(page.getByRole('article')).toHaveCount(4);
		await shoot(page, 'short-1024x600-overview');
	});

	test('mobile en light and zh dark drawer', async ({ page }) => {
		await page.setViewportSize({ width: 375, height: 812 });
		await page.goto('/');
		await expect(page.getByRole('article')).toHaveCount(4);
		await shoot(page, 'mobile-en-light');
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/?lang=zh#/');
		await expect(page.getByRole('article')).toHaveCount(4);
		await page.getByRole('button', { name: '打开导航' }).click();
		await page.getByRole('button', { name: '深色' }).click();
		await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
		await shoot(page, 'mobile-zh-dark-drawer');
	});
});
