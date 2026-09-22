/// <reference lib="dom" />
import { expect, test, type Page } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import { createServer, type ViteDevServer } from 'vite';
import { installMockApi } from './mockApi.ts';

// Native Back/Forward on a dirty page is decided inside popstate with a blocking window.confirm,
// so page.on('dialog') is installed before anything triggers a traversal.
// The state modules are imported straight from src/ through the Vite dev server.

interface Router {
	readonly route: { kind: string; hash: string };
	start(): () => void;
	navigate(hash: string, options?: { replace?: boolean }): Promise<boolean>;
	registerLeaveGuard(isDirty: () => boolean): () => void;
}

interface I18n {
	readonly language: string;
	init(): void;
	setLanguage(language: 'en' | 'zh'): void;
}

interface ConfirmService {
	register(host: (options: { message: string }) => Promise<boolean>): () => void;
}

interface Busy {
	run<T>(task: () => Promise<T>): Promise<T>;
}

interface Probe {
	router: Router;
	i18n: I18n;
	confirmService: ConfirmService;
	busy: Busy;
	hashLinks(node: HTMLElement): { destroy(): void };
	// `popstate ${search}${hash} ${index}` recorded after the router handled it.
	events: string[];
	dirty: boolean;
	prompts: string[];
	navigated: string[];
	// Runs a traversal and resolves true once its popstate (and a trailing task) has run.
	step(move: () => void): Promise<boolean>;
	// Rewinds to the entry stamped 0, then lists `${hash} ${index}` of every entry in order.
	walk(): Promise<string[]>;
	stop: () => void;
	decideA?: (value: boolean) => void;
	decideB?: (value: boolean) => void;
	release?: () => void;
	first?: Promise<boolean>;
	second?: Promise<boolean>;
}

type ProbeWindow = Window & { probe: Probe };

const MODULES = [
	'/src/lib/router.svelte.ts',
	'/src/lib/i18n.svelte.ts',
	'/src/lib/confirm.ts',
	'/src/lib/busy.svelte.ts',
	'/src/lib/actions/links.ts'
];

let server: ViteDevServer;
let origin: string;

test.beforeAll(async () => {
	server = await createServer({
		root: fileURLToPath(new URL('..', import.meta.url)),
		server: { host: '127.0.0.1', port: 0, strictPort: true },
		logLevel: 'error'
	});
	await server.listen();
	origin = server.resolvedUrls!.local[0].replace(/\/$/, '');
});

test.afterAll(async () => {
	await server?.close();
});

function dialogs(page: Page) {
	const seen: string[] = [];
	const control = { accept: false };
	page.on('dialog', (dialog) => {
		seen.push(`${dialog.type()} ${dialog.message()}`);
		void (control.accept ? dialog.accept() : dialog.dismiss());
	});
	return { seen, control };
}

// Starts i18n, then the router, and registers a guard reading `probe.dirty`.
async function load(page: Page, url: string) {
	await page.goto(`${origin}/${url}`);
	await page.evaluate(async (modules) => {
		const [{ router }, { i18n }, { confirmService }, { busy }, { hashLinks }] = (await Promise.all(
			modules.map((path) => import(path))
		)) as [
			{ router: Router },
			{ i18n: I18n },
			{ confirmService: ConfirmService },
			{ busy: Busy },
			{ hashLinks: Probe['hashLinks'] }
		];
		const step = (move: () => void) =>
			new Promise<boolean>((resolve) => {
				const landed = () => {
					clearTimeout(timer);
					setTimeout(() => resolve(true), 0);
				};
				const timer = setTimeout(() => {
					window.removeEventListener('popstate', landed);
					resolve(false);
				}, 400);
				window.addEventListener('popstate', landed, { once: true });
				move();
			});
		const index = () => (history.state as Record<string, unknown> | null)?.jumpwayHistoryIndex;
		const label = () => `${location.hash} ${index()}`;
		const walk = async () => {
			// Playwright's about:blank precedes the app's entries, so never step below the stamp 0.
			while (Number(index()) > 0 && (await step(() => history.back())));
			const entries = [label()];
			while (await step(() => history.forward())) entries.push(label());
			return entries;
		};
		i18n.init();
		const probe: Probe = {
			router,
			i18n,
			confirmService,
			busy,
			hashLinks,
			events: [],
			dirty: false,
			prompts: [],
			navigated: [],
			step,
			walk,
			stop: router.start()
		};
		router.registerLeaveGuard(() => probe.dirty);
		window.addEventListener('popstate', () =>
			probe.events.push(`popstate ${location.search}${location.hash} ${index()}`)
		);
		(window as unknown as ProbeWindow).probe = probe;
	}, MODULES);
}

const snapshot = () => {
	const { probe } = window as unknown as ProbeWindow;
	return {
		route: probe.router.route.kind,
		url: location.search + location.hash,
		language: probe.i18n.language,
		htmlLang: document.documentElement.lang,
		events: [...probe.events],
		prompts: [...probe.prompts],
		navigated: [...probe.navigated],
		state: history.state as unknown
	};
};

const clean = () => {
	(window as unknown as ProbeWindow).probe.dirty = false;
};
const walk = () => (window as unknown as ProbeWindow).probe.walk();
const DISCARD = 'confirm Discard unsaved changes?';

test('R5 a dirty Back that also changes ?lang= is refused or accepted with URL, route and language together', async ({
	page
}) => {
	const { seen, control } = dialogs(page);
	await load(page, '?lang=en#/settings');
	await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		await probe.router.navigate('#/yaml');
		probe.i18n.setLanguage('zh');
		probe.dirty = true;
		history.back();
	});
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'yaml',
			url: '?lang=zh#/yaml',
			language: 'zh',
			htmlLang: 'zh-CN',
			state: { jumpwayHistoryIndex: 1 },
			events: ['popstate ?lang=en#/settings 0', 'popstate ?lang=zh#/yaml 1']
		});
	expect(seen).toEqual([DISCARD]);

	control.accept = true;
	await page.evaluate(() => history.back());
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'settings',
			url: '?lang=en#/settings',
			language: 'en',
			htmlLang: 'en',
			state: { jumpwayHistoryIndex: 0 }
		});
	expect(seen).toEqual([DISCARD, DISCARD]);

	await page.evaluate(clean);
	await page.evaluate(() => history.forward());
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({ route: 'yaml', url: '?lang=zh#/yaml', language: 'zh' });
	expect(seen).toHaveLength(2);
});

test('R1 after a traversal between same-URL entries a refused Back stays put with every entry intact and an accepted one lands on it', async ({
	page
}) => {
	const { seen, control } = dialogs(page);
	await load(page, '#/settings');
	await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		await probe.router.navigate('#/yaml');
		await probe.router.navigate('#/settings');
		probe.dirty = true;
		await probe.step(() => history.go(-2));
		probe.dirty = false;
		await probe.router.navigate('#/hosts');
		probe.dirty = true;
		probe.events.length = 0;
		history.back();
	});
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'hosts',
			url: '#/hosts',
			state: { jumpwayHistoryIndex: 1 },
			events: ['popstate #/settings 0', 'popstate #/hosts 1']
		});
	expect(seen).toEqual([DISCARD]);
	await page.evaluate(clean);
	expect(await page.evaluate(walk)).toEqual(['#/settings 0', '#/hosts 1']);
	expect(await page.evaluate(snapshot)).toMatchObject({ route: 'hosts', url: '#/hosts' });

	control.accept = true;
	await page.evaluate(() => {
		(window as unknown as ProbeWindow).probe.dirty = true;
		history.back();
	});
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({ route: 'settings', url: '#/settings', state: { jumpwayHistoryIndex: 0 } });
	expect(seen).toEqual([DISCARD, DISCARD]);
});

test('R7 after a same-URL traversal a refused Forward returns to the entry the browser was on', async ({
	page
}) => {
	const { seen } = dialogs(page);
	await load(page, '#/settings');
	await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		await probe.router.navigate('#/yaml');
		await probe.router.navigate('#/settings');
		await probe.step(() => history.go(-2));
		probe.dirty = true;
		probe.events.length = 0;
		history.forward();
	});
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'settings',
			url: '#/settings',
			state: { jumpwayHistoryIndex: 0 },
			events: ['popstate #/yaml 1', 'popstate #/settings 0']
		});
	expect(seen).toEqual([DISCARD]);
	await page.waitForTimeout(300);
	expect(await page.evaluate(snapshot)).toMatchObject({
		state: { jumpwayHistoryIndex: 0 },
		events: ['popstate #/yaml 1', 'popstate #/settings 0']
	});
	await page.evaluate(clean);
	expect(await page.evaluate(walk)).toEqual(['#/settings 0', '#/yaml 1', '#/settings 2']);
});

test('an address-bar hash change on a dirty page is undone with go(-1) and the new entry stays reachable forward', async ({
	page
}) => {
	const { seen, control } = dialogs(page);
	await load(page, '#/settings');
	await page.evaluate(() => {
		(window as unknown as ProbeWindow).probe.dirty = true;
		location.hash = '#/hosts';
	});
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'settings',
			url: '#/settings',
			state: { jumpwayHistoryIndex: 0 },
			events: ['popstate #/hosts 1', 'popstate #/settings 0']
		});
	expect(seen).toEqual([DISCARD]);

	await page.evaluate(() => history.forward());
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'settings',
			url: '#/settings',
			events: [
				'popstate #/hosts 1',
				'popstate #/settings 0',
				'popstate #/hosts 1',
				'popstate #/settings 0'
			]
		});
	expect(seen).toEqual([DISCARD, DISCARD]);

	control.accept = true;
	await page.evaluate(() => history.forward());
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({ route: 'hosts', url: '#/hosts', state: { jumpwayHistoryIndex: 1 } });
	expect(seen).toHaveLength(3);
	await page.evaluate(clean);
	expect(await page.evaluate(walk)).toEqual(['#/settings 0', '#/hosts 1']);
});

test('two consecutive dismissed Backs prompt twice and leave every entry in place', async ({
	page
}) => {
	const { seen } = dialogs(page);
	await load(page, '#/settings');
	await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		await probe.router.navigate('#/yaml');
		await probe.router.navigate('#/hosts');
		probe.dirty = true;
		history.back();
	});
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({ url: '#/hosts', events: ['popstate #/yaml 1', 'popstate #/hosts 2'] });
	await page.evaluate(() => history.back());
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'hosts',
			url: '#/hosts',
			state: { jumpwayHistoryIndex: 2 },
			events: ['popstate #/yaml 1', 'popstate #/hosts 2', 'popstate #/yaml 1', 'popstate #/hosts 2']
		});
	expect(seen).toEqual([DISCARD, DISCARD]);
	await page.evaluate(clean);
	expect(await page.evaluate(walk)).toEqual(['#/settings 0', '#/yaml 1', '#/hosts 2']);
});

test('a Back while busy is undone silently and navigate is refused until the task ends', async ({
	page
}) => {
	const { seen } = dialogs(page);
	await load(page, '#/settings');
	await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		await probe.router.navigate('#/yaml');
		probe.dirty = true;
		void probe.busy.run(() => new Promise<void>((resolve) => (probe.release = resolve)));
		history.back();
	});
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'yaml',
			url: '#/yaml',
			state: { jumpwayHistoryIndex: 1 },
			events: ['popstate #/settings 0', 'popstate #/yaml 1']
		});
	expect(seen).toEqual([]);
	const refused = await page.evaluate(() => {
		const { probe } = window as unknown as ProbeWindow;
		return probe.router.navigate('#/hosts');
	});
	expect(refused).toBe(false);
	const allowed = await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		probe.release!();
		await new Promise((resolve) => setTimeout(resolve, 0));
		probe.dirty = false;
		return probe.router.navigate('#/hosts');
	});
	expect(allowed).toBe(true);
	expect(await page.evaluate(snapshot)).toMatchObject({
		route: 'hosts',
		url: '#/hosts',
		state: { jumpwayHistoryIndex: 2 }
	});
	expect(seen).toEqual([]);
});

test('a dirty in-app link click opens the custom prompt with zero history writes; modified clicks are not intercepted', async ({
	page
}) => {
	const { seen } = dialogs(page);
	await load(page, '#/settings');
	await page.evaluate(() => {
		const { probe } = window as unknown as ProbeWindow;
		document.body.insertAdjacentHTML(
			'beforeend',
			'<a id="hosts" href="#/hosts">hosts</a><a id="yaml" href="#/yaml">yaml</a>'
		);
		probe.hashLinks(document.body);
		const navigate = probe.router.navigate.bind(probe.router);
		probe.router.navigate = (hash, options) => {
			probe.navigated.push(hash);
			return navigate(hash, options);
		};
		probe.confirmService.register(({ message }) => {
			probe.prompts.push(message);
			return new Promise<boolean>((resolve) => (probe.decideA = resolve));
		});
		probe.dirty = true;
	});
	await page.click('#hosts');
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({ prompts: ['Discard unsaved changes?'], navigated: ['#/hosts'] });
	const grown = await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		const before = history.length;
		probe.decideA!(false);
		await new Promise((resolve) => setTimeout(resolve, 0));
		return history.length - before;
	});
	expect(grown).toBe(0);
	expect(await page.evaluate(snapshot)).toMatchObject({
		route: 'settings',
		url: '#/settings',
		state: { jumpwayHistoryIndex: 0 },
		events: []
	});

	await page.click('#hosts');
	await expect
		.poll(() => page.evaluate(() => (window as unknown as ProbeWindow).probe.prompts.length))
		.toBe(2);
	await page.evaluate(() => (window as unknown as ProbeWindow).probe.decideA!(true));
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'hosts',
			url: '#/hosts',
			state: { jumpwayHistoryIndex: 1 },
			navigated: ['#/hosts', '#/hosts'],
			events: []
		});

	const popup = page
		.context()
		.waitForEvent('page', { timeout: 2000 })
		.catch(() => null);
	await page.click('#yaml', { modifiers: ['ControlOrMeta'] });
	await (await popup)?.close();
	await page.waitForTimeout(200);
	expect(await page.evaluate(snapshot)).toMatchObject({
		route: 'hosts',
		url: '#/hosts',
		navigated: ['#/hosts', '#/hosts'],
		prompts: ['Discard unsaved changes?', 'Discard unsaved changes?']
	});
	expect(seen).toEqual([]);
});

test('R4 a custom prompt answered after stop/start neither navigates nor unlocks the new lifecycle', async ({
	page
}) => {
	const { seen } = dialogs(page);
	await load(page, '#/settings');
	const result = await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		const tick = () => new Promise((resolve) => setTimeout(resolve, 0));
		let hostB = 0;
		probe.dirty = true;
		const unregisterA = probe.confirmService.register(
			() => new Promise<boolean>((resolve) => (probe.decideA = resolve))
		);
		const first = probe.router.navigate('#/yaml');
		await tick();
		unregisterA();
		probe.stop();
		probe.stop = probe.router.start();
		probe.router.registerLeaveGuard(() => probe.dirty);
		probe.confirmService.register(() => {
			hostB++;
			return new Promise<boolean>((resolve) => (probe.decideB = resolve));
		});
		const second = probe.router.navigate('#/hosts');
		await tick();
		const hostBAfterSecond = hostB;
		probe.decideA!(true);
		const firstResult = await first;
		const hashAfterA = location.hash;
		const third = await probe.router.navigate('#/stats');
		const hostBAfterThird = hostB;
		probe.decideB!(true);
		const secondResult = await second;
		return {
			hostBAfterSecond,
			firstResult,
			hashAfterA,
			third,
			hostBAfterThird,
			secondResult,
			route: probe.router.route.kind,
			hash: location.hash,
			state: history.state as unknown
		};
	});
	expect(result).toEqual({
		hostBAfterSecond: 1,
		firstResult: false,
		hashAfterA: '#/settings',
		third: false,
		hostBAfterThird: 1,
		secondResult: true,
		route: 'hosts',
		hash: '#/hosts',
		state: { jumpwayHistoryIndex: 1 }
	});
	expect(seen).toEqual([]);
});

test('R8 a Back to an entry visited before a later navigate is still guarded', async ({ page }) => {
	const { seen } = dialogs(page);
	await load(page, '#/settings');
	await page.evaluate(async () => {
		const { probe } = window as unknown as ProbeWindow;
		await probe.router.navigate('#/yaml');
		await probe.step(() => history.back());
		await probe.router.navigate('#/yaml');
		probe.dirty = true;
		probe.events.length = 0;
		history.back();
	});
	await expect
		.poll(() => page.evaluate(snapshot))
		.toMatchObject({
			route: 'yaml',
			url: '#/yaml',
			state: { jumpwayHistoryIndex: 1 },
			events: ['popstate #/settings 0', 'popstate #/yaml 1']
		});
	expect(seen).toEqual([DISCARD]);
});

// The mounted App (index.html -> main.ts) and the test share the dev server's module instances,
// so a guard registered here makes the real shell dirty and its own ConfirmDialog asks the question.

interface AppProbe {
	asked: number;
	navigated: string[];
	answers: boolean[];
}

type AppProbeWindow = Window & { app: AppProbe };

const ROUTER = '/src/lib/router.svelte.ts';
const CONFIRM = '/src/lib/confirm.ts';

async function loadApp(page: Page, url: string) {
	await installMockApi(page);
	await page.goto(`${origin}/${url}`);
	await page.evaluate(async (path) => {
		const { router } = (await import(path)) as { router: Router };
		const app: AppProbe = { asked: 0, navigated: [], answers: [] };
		const navigate = router.navigate.bind(router);
		router.navigate = (hash, options) => {
			app.navigated.push(hash);
			return navigate(hash, options);
		};
		(window as unknown as AppProbeWindow).app = app;
	}, ROUTER);
}

const appProbe = () => (window as unknown as AppProbeWindow).app;

// Asks through the app's confirm() and records the answer.
const ask = (page: Page, message: string) =>
	page.evaluate(
		async ([path, text]) => {
			const { confirm } = (await import(path)) as {
				confirm: (options: { message: string }) => Promise<boolean>;
			};
			void confirm({ message: text }).then((answer) =>
				(window as unknown as AppProbeWindow).app.answers.push(answer)
			);
		},
		[CONFIRM, message]
	);

test.describe('mobile shell', () => {
	test.use({ viewport: { width: 375, height: 812 } });

	test('a dirty link inside the drawer is handled once: cancel keeps the drawer open, confirm closes it and returns focus', async ({
		page
	}) => {
		const { seen } = dialogs(page);
		await loadApp(page, '#/');
		await page.evaluate(async (path) => {
			const { router } = (await import(path)) as { router: Router };
			router.registerLeaveGuard(() => {
				(window as unknown as AppProbeWindow).app.asked++;
				return true;
			});
		}, ROUTER);
		const menu = page.getByRole('button', { name: 'Open navigation' });
		const drawer = page.locator('dialog#navigation-drawer');
		const hosts = drawer.getByRole('link', { name: 'Proxy Hosts' });
		const question = page.getByRole('dialog').filter({ hasText: 'Discard unsaved changes?' });

		await menu.click();
		await expect(drawer).toHaveAttribute('open', '');
		await hosts.click();
		await expect(question).toBeVisible();
		expect(await page.evaluate(appProbe)).toMatchObject({ asked: 1, navigated: ['#/hosts'] });
		await question.getByRole('button', { name: 'Cancel' }).click();
		await expect(question).toBeHidden();
		await expect(page).toHaveURL(/#\/$/);
		await expect(drawer).toHaveAttribute('open', '');
		await expect(hosts).toBeVisible();

		await hosts.click();
		await expect(question).toBeVisible();
		await question.getByRole('button', { name: 'Confirm' }).click();
		await expect(page).toHaveURL(/#\/hosts$/);
		await expect(page.getByRole('heading', { level: 1 })).toHaveText('Proxy Hosts');
		await expect(drawer).not.toHaveAttribute('open', '');
		await expect(menu).toBeFocused();
		expect(await page.evaluate(appProbe)).toMatchObject({
			asked: 2,
			navigated: ['#/hosts', '#/hosts']
		});
		expect(seen).toEqual([]);
	});
});

test('a question asked right after the previous answer stays open until it is answered itself', async ({
	page
}) => {
	const { seen } = dialogs(page);
	await loadApp(page, '#/settings');
	const opener = page.getByRole('navigation').getByRole('link', { name: 'Proxy Hosts' });
	await opener.focus();
	await page.evaluate(async (path) => {
		const { confirm } = (await import(path)) as {
			confirm: (options: { message: string }) => Promise<boolean>;
		};
		const { app } = window as unknown as AppProbeWindow;
		void confirm({ message: 'first question' }).then((first) => {
			app.answers.push(first);
			return confirm({ message: 'second question' }).then((second) => app.answers.push(second));
		});
	}, CONFIRM);
	const question = page.getByRole('dialog').filter({ hasText: 'question' });
	await expect(question).toContainText('first question');
	await question.getByRole('button', { name: 'Confirm' }).click();
	await expect(question).toContainText('second question');
	// The close event from the first dialog.close() lands after the reopen and must not answer this one.
	await page.waitForTimeout(300);
	await expect(question).toBeVisible();
	await expect(question).toContainText('second question');
	expect((await page.evaluate(appProbe)).answers).toEqual([true]);

	await question.getByRole('button', { name: 'Cancel' }).click();
	await expect(question).toBeHidden();
	expect((await page.evaluate(appProbe)).answers).toEqual([true, false]);
	await expect(opener).toBeFocused();

	// Escape on a dialog that was reopened is still a genuine "no".
	await ask(page, 'third question');
	await expect(question).toContainText('third question');
	await page.keyboard.press('Escape');
	await expect(question).toBeHidden();
	// Chrome removes `open` first and delivers the close event a task later.
	await expect
		.poll(async () => (await page.evaluate(appProbe)).answers)
		.toEqual([true, false, false]);
	await expect(opener).toBeFocused();
	expect(seen).toEqual([]);
});
