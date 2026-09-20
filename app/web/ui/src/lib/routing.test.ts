import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { busy } from './busy.svelte';
import { confirmService } from './confirm';
import { i18n } from './i18n.svelte';
import { HISTORY_INDEX_KEY, router } from './router.svelte';
import { NEW_RULE_ROUTE, routeFor, ruleRoute, statsRoute, type Route } from './routes';

const route = (kind: Route['kind'], hash: string, extra: Partial<Route> = {}): Route => ({
	kind,
	hash,
	name: null,
	rule: '',
	...extra
});

describe('routeFor', () => {
	test.each<[string, Route]>([
		['', route('overview', '#/')],
		['#', route('overview', '#/')],
		['#/', route('overview', '#/')],
		['#/?rule=z', route('overview', '#/?rule=z', { rule: 'z' })],
		['#/new', route('new', '#/new')],
		['#/rules/a%20b', route('rule', '#/rules/a%20b', { name: 'a b' })],
		['#/rules/new', route('rule', '#/rules/new', { name: 'new' })],
		['#/rules/', route('overview', '#/')],
		['#/rules/a/b', route('overview', '#/')],
		['#/rules/%E4', route('overview', '#/')],
		['#/stats', route('stats', '#/stats')],
		['#/stats?rule=x%20y', route('stats', '#/stats?rule=x%20y', { rule: 'x y' })],
		['#/stats?rule=%E4%B8%AD+b', route('stats', '#/stats?rule=%E4%B8%AD+b', { rule: '中 b' })],
		['#/stats?%', route('overview', '#/')],
		['#/stats?rule=%GG', route('overview', '#/')],
		['#/stats?rule=%E0%A4%A', route('overview', '#/')],
		['#/connections?rule=%E4', route('overview', '#/')],
		['#/rules/a?rule=%', route('overview', '#/')],
		['#/connections?rule=r&x=1', route('connections', '#/connections?rule=r&x=1', { rule: 'r' })],
		['#/hosts', route('hosts', '#/hosts')],
		['#/settings', route('settings', '#/settings')],
		['#/yaml', route('yaml', '#/yaml')],
		['#/nope', route('overview', '#/')],
		['garbage', route('overview', '#/')],
		// Retired addresses are unknown ones: no page of their own, no query carried anywhere.
		['#/rules', route('overview', '#/')],
		['#/rules?rule=z', route('overview', '#/')],
		['#/web-ui', route('overview', '#/')],
		['#/stats/connections', route('overview', '#/')]
	])('%s', (hash, expected) => {
		expect(routeFor(hash)).toEqual(expected);
	});

	test('hash builders round-trip through routeFor', () => {
		expect(ruleRoute('a/b c')).toBe('#/rules/a%2Fb%20c');
		expect(routeFor(ruleRoute('a/b c')).name).toBe('a/b c');
		expect(statsRoute('stats')).toBe('#/stats');
		expect(statsRoute('connections', 'r 1')).toBe('#/connections?rule=r+1');
		expect(routeFor(statsRoute('connections', 'r 1')).rule).toBe('r 1');
		expect(routeFor(NEW_RULE_ROUTE).kind).toBe('new');
	});
});

describe('router', () => {
	let stop: () => void;
	const cleanups: (() => void)[] = [];
	const flush = () => new Promise((resolve) => setTimeout(resolve, 0));
	// jsdom lands history traversals a few macrotasks later.
	const until = async (predicate: () => boolean) => {
		for (let i = 0; i < 50 && !predicate(); i++) {
			await new Promise((resolve) => setTimeout(resolve, 1));
		}
	};
	const index = () => (history.state as Record<string, unknown> | null)?.[HISTORY_INDEX_KEY];
	const dirty = () => cleanups.push(router.registerLeaveGuard(() => true));
	const nativeConfirm = (answer: boolean) => {
		const seen: string[] = [];
		const spy = vi.spyOn(window, 'confirm').mockImplementation(() => {
			seen.push(`${location.hash} ${router.route.kind}`);
			return answer;
		});
		return { spy, seen };
	};

	beforeEach(() => {
		history.replaceState(null, '', '/#/settings');
		stop = router.start();
	});

	afterEach(() => {
		cleanups.splice(0).forEach((cleanup) => cleanup());
		stop();
		vi.restoreAllMocks();
	});

	test('a native Back on a dirty page asks window.confirm synchronously; dismiss keeps route and entries, accept follows the URL', async () => {
		await router.navigate('#/yaml');
		const length = history.length;
		dirty();
		const { spy, seen } = nativeConfirm(false);
		history.back();
		await until(() => spy.mock.calls.length === 1 && location.hash === '#/yaml');
		expect(spy).toHaveBeenCalledWith('Discard unsaved changes?');
		expect(seen).toEqual(['#/settings yaml']);
		expect(router.route.kind).toBe('yaml');
		expect(location.hash).toBe('#/yaml');
		expect(index()).toBe(1);
		expect(history.length).toBe(length);

		spy.mockReturnValue(true);
		history.back();
		await until(() => router.route.kind === 'settings');
		expect(spy).toHaveBeenCalledTimes(2);
		expect(router.route.kind).toBe('settings');
		expect(location.hash).toBe('#/settings');
		expect(index()).toBe(0);
	});

	test('start canonicalises an unknown initial hash to the overview, and a clean page follows an external hash change without a dialog', async () => {
		stop();
		history.replaceState(null, '', '/#/web-ui');
		stop = router.start();
		expect(router.route.kind).toBe('overview');
		expect(location.hash).toBe('#/');
		expect(index()).toBe(0);
		const { spy } = nativeConfirm(false);
		location.hash = '#/stats?rule=a';
		await until(() => router.route.kind === 'stats');
		expect(router.route).toEqual(route('stats', '#/stats?rule=a', { rule: 'a' }));
		expect(location.hash).toBe('#/stats?rule=a');
		expect(index()).toBe(1);
		expect(spy).not.toHaveBeenCalled();
	});

	test('navigate pushes or replaces history and reports success', async () => {
		const length = history.length;
		await expect(router.navigate('#/yaml')).resolves.toBe(true);
		expect(location.hash).toBe('#/yaml');
		expect(history.length).toBe(length + 1);
		expect(index()).toBe(1);
		await expect(router.navigate(ruleRoute('x'), { replace: true })).resolves.toBe(true);
		expect(router.route.name).toBe('x');
		expect(history.length).toBe(length + 1);
		expect(index()).toBe(1);
	});

	test('navigating to the current route never asks and writes nothing', async () => {
		dirty();
		const { spy } = nativeConfirm(false);
		const host = vi.fn(async () => false);
		cleanups.push(confirmService.register(host));
		const push = vi.spyOn(history, 'pushState');
		await expect(router.navigate('#/settings')).resolves.toBe(true);
		expect(spy).not.toHaveBeenCalled();
		expect(host).not.toHaveBeenCalled();
		expect(push).not.toHaveBeenCalled();
		expect(location.hash).toBe('#/settings');
	});

	test('navigate on a dirty page asks the custom confirm host before any history write', async () => {
		dirty();
		const { spy } = nativeConfirm(true);
		let answer = false;
		const host = vi.fn(async () => answer);
		cleanups.push(confirmService.register(host));
		const push = vi.spyOn(history, 'pushState');
		const replace = vi.spyOn(history, 'replaceState');
		await expect(router.navigate('#/yaml')).resolves.toBe(false);
		expect(host).toHaveBeenCalledWith({ message: 'Discard unsaved changes?', danger: true });
		expect(push).not.toHaveBeenCalled();
		expect(replace).not.toHaveBeenCalled();
		expect(router.route.kind).toBe('settings');
		expect(location.hash).toBe('#/settings');

		answer = true;
		await expect(router.navigate('#/yaml')).resolves.toBe(true);
		expect(push).toHaveBeenCalledTimes(1);
		expect(router.route.kind).toBe('yaml');
		expect(index()).toBe(1);
		await expect(router.navigate('#/hosts', { replace: true })).resolves.toBe(true);
		expect(replace).toHaveBeenCalledTimes(1);
		expect(push).toHaveBeenCalledTimes(1);
		expect(index()).toBe(1);
		expect(host).toHaveBeenCalledTimes(3);
		expect(spy).not.toHaveBeenCalled();
	});

	test('a custom host that rejects its promise counts as a refusal and releases the prompt', async () => {
		dirty();
		nativeConfirm(true);
		let fail = true;
		cleanups.push(
			confirmService.register(() =>
				fail ? Promise.reject(new Error('closed')) : Promise.resolve(true)
			)
		);
		const push = vi.spyOn(history, 'pushState');
		await expect(router.navigate('#/yaml')).resolves.toBe(false);
		expect(push).not.toHaveBeenCalled();
		expect(router.route.kind).toBe('settings');
		fail = false;
		await expect(router.navigate('#/yaml')).resolves.toBe(true);
		expect(push).toHaveBeenCalledTimes(1);
	});

	test('while the custom prompt is open navigate is refused and a native Back is restored without a native dialog', async () => {
		await router.navigate('#/yaml');
		dirty();
		const { spy } = nativeConfirm(true);
		let decide!: (value: boolean) => void;
		const host = vi.fn(() => new Promise<boolean>((resolve) => (decide = resolve)));
		cleanups.push(confirmService.register(host));
		const first = router.navigate('#/hosts');
		await flush();
		expect(host).toHaveBeenCalledTimes(1);
		await expect(router.navigate('#/stats')).resolves.toBe(false);
		expect(host).toHaveBeenCalledTimes(1);

		const events: string[] = [];
		const record = () => events.push(location.hash);
		window.addEventListener('popstate', record);
		cleanups.push(() => window.removeEventListener('popstate', record));
		history.back();
		await until(() => events.length === 2);
		expect(events).toEqual(['#/settings', '#/yaml']);
		expect(spy).not.toHaveBeenCalled();
		expect(router.route.kind).toBe('yaml');
		expect(index()).toBe(1);

		decide(true);
		await expect(first).resolves.toBe(true);
		expect(router.route.kind).toBe('hosts');
		expect(location.hash).toBe('#/hosts');
		expect(index()).toBe(2);
	});

	test('a prompt answered after stop/start neither navigates nor unlocks the new lifecycle', async () => {
		dirty();
		let decideA!: (value: boolean) => void;
		const hostA = vi.fn(() => new Promise<boolean>((resolve) => (decideA = resolve)));
		const unregisterA = confirmService.register(hostA);
		const first = router.navigate('#/yaml');
		await flush();
		expect(hostA).toHaveBeenCalledTimes(1);
		unregisterA();
		stop();
		stop = router.start();
		dirty();
		let decideB!: (value: boolean) => void;
		const hostB = vi.fn(() => new Promise<boolean>((resolve) => (decideB = resolve)));
		cleanups.push(confirmService.register(hostB));
		const second = router.navigate('#/hosts');
		await flush();
		expect(hostB).toHaveBeenCalledTimes(1);

		decideA(true);
		await expect(first).resolves.toBe(false);
		expect(router.route.kind).toBe('settings');
		expect(location.hash).toBe('#/settings');
		await expect(router.navigate('#/stats')).resolves.toBe(false);
		expect(hostB).toHaveBeenCalledTimes(1);

		decideB(true);
		await expect(second).resolves.toBe(true);
		expect(router.route.kind).toBe('hosts');
		expect(location.hash).toBe('#/hosts');
		expect(index()).toBe(1);
	});

	test('a prompt answered while the correction traversal is in flight writes nothing', async () => {
		await router.navigate('#/yaml');
		dirty();
		nativeConfirm(true);
		let decide!: (value: boolean) => void;
		cleanups.push(
			confirmService.register(() => new Promise<boolean>((resolve) => (decide = resolve)))
		);
		const first = router.navigate('#/hosts');
		await flush();
		const push = vi.spyOn(history, 'pushState');
		const events: string[] = [];
		const record = () => {
			events.push(location.hash);
			if (events.length === 1) decide(true);
		};
		window.addEventListener('popstate', record);
		cleanups.push(() => window.removeEventListener('popstate', record));
		history.back();
		await expect(first).resolves.toBe(false);
		await until(() => events.length === 2);
		expect(events).toEqual(['#/settings', '#/yaml']);
		expect(push).not.toHaveBeenCalled();
		expect(router.route.kind).toBe('yaml');
		expect(index()).toBe(1);
		const again = router.navigate('#/hosts');
		await flush();
		decide(true);
		await expect(again).resolves.toBe(true);
		expect(index()).toBe(2);
	});

	test('busy refuses navigate and undoes a Back without any dialog', async () => {
		await router.navigate('#/yaml');
		dirty();
		const { spy } = nativeConfirm(true);
		const host = vi.fn(async () => true);
		cleanups.push(confirmService.register(host));
		let release!: () => void;
		const task = busy.run(() => new Promise<void>((resolve) => (release = resolve)));
		await expect(router.navigate('#/hosts')).resolves.toBe(false);
		expect(host).not.toHaveBeenCalled();
		const events: string[] = [];
		const record = () => events.push(location.hash);
		window.addEventListener('popstate', record);
		cleanups.push(() => window.removeEventListener('popstate', record));
		history.back();
		await until(() => events.length === 2);
		expect(events).toEqual(['#/settings', '#/yaml']);
		expect(spy).not.toHaveBeenCalled();
		expect(router.route.kind).toBe('yaml');
		expect(index()).toBe(1);
		release();
		await task;
		await expect(router.navigate('#/hosts')).resolves.toBe(true);
		expect(host).toHaveBeenCalledTimes(1);
	});

	test('a traversal to a same-route entry is accepted silently and later refusals return to it (R1)', async () => {
		await router.navigate('#/yaml');
		await router.navigate('#/settings');
		dirty();
		const { spy, seen } = nativeConfirm(false);
		history.go(-2);
		await until(() => index() === 0);
		expect(spy).not.toHaveBeenCalled();
		expect(router.route.kind).toBe('settings');
		cleanups.splice(0).forEach((cleanup) => cleanup());
		await expect(router.navigate('#/hosts')).resolves.toBe(true);
		expect(index()).toBe(1);
		dirty();
		history.back();
		await until(() => spy.mock.calls.length === 1 && location.hash === '#/hosts');
		expect(seen).toEqual(['#/settings hosts']);
		expect(router.route.kind).toBe('hosts');
		expect(index()).toBe(1);
		spy.mockReturnValue(true);
		history.back();
		await until(() => router.route.kind === 'settings');
		expect(location.hash).toBe('#/settings');
		expect(index()).toBe(0);
	});

	test('a refused Forward to a same-URL entry returns to the entry the browser was on (R7)', async () => {
		await router.navigate('#/yaml');
		await router.navigate('#/settings');
		history.go(-2);
		await until(() => index() === 0);
		dirty();
		const { spy, seen } = nativeConfirm(false);
		history.forward();
		await until(() => spy.mock.calls.length === 1 && index() === 0);
		expect(seen).toEqual(['#/yaml settings']);
		expect(router.route.kind).toBe('settings');
		expect(location.hash).toBe('#/settings');
		expect(index()).toBe(0);
		history.go(2);
		await until(() => index() === 2);
		expect(spy).toHaveBeenCalledTimes(1);
		expect(router.route.kind).toBe('settings');
		expect(location.hash).toBe('#/settings');
		expect(index()).toBe(2);
	});

	test('a Back that also changes ?lang= is decided synchronously and keeps URL, route and language together', async () => {
		stop();
		history.replaceState(null, '', '/?lang=en#/settings');
		i18n.init();
		cleanups.push(() => {
			i18n.dispose();
			localStorage.clear();
		});
		stop = router.start();
		await router.navigate('#/yaml');
		i18n.setLanguage('zh');
		expect(location.search + location.hash).toBe('?lang=zh#/yaml');
		dirty();
		const { spy } = nativeConfirm(false);
		history.back();
		await until(() => spy.mock.calls.length === 1 && location.hash === '#/yaml');
		expect(spy).toHaveBeenCalledTimes(1);
		expect(router.route.kind).toBe('yaml');
		expect(location.search + location.hash).toBe('?lang=zh#/yaml');
		expect(i18n.language).toBe('zh');
		expect(document.documentElement.lang).toBe('zh-CN');
		expect(index()).toBe(1);

		spy.mockReturnValue(true);
		history.back();
		await until(() => router.route.kind === 'settings');
		expect(location.search + location.hash).toBe('?lang=en#/settings');
		expect(i18n.language).toBe('en');
		expect(document.documentElement.lang).toBe('en');
		expect(index()).toBe(0);
	});

	test('a refused external hash push is undone with go(-1) and the pushed entry stays reachable forward', async () => {
		dirty();
		const { spy } = nativeConfirm(false);
		const events: string[] = [];
		const record = () => events.push(`${location.hash} ${index()}`);
		window.addEventListener('popstate', record);
		cleanups.push(() => window.removeEventListener('popstate', record));
		location.hash = '#/hosts';
		await until(() => events.length === 2);
		expect(events).toEqual(['#/hosts 1', '#/settings 0']);
		expect(spy).toHaveBeenCalledTimes(1);
		expect(router.route.kind).toBe('settings');
		history.forward();
		await until(() => events.length === 4);
		expect(events.slice(2)).toEqual(['#/hosts 1', '#/settings 0']);
		expect(spy).toHaveBeenCalledTimes(2);
		spy.mockReturnValue(true);
		history.forward();
		await until(() => router.route.kind === 'hosts');
		expect(location.hash).toBe('#/hosts');
		expect(index()).toBe(1);
	});

	test('two consecutive dismissed Backs ask twice and leave every entry in place', async () => {
		await router.navigate('#/yaml');
		await router.navigate('#/hosts');
		const length = history.length;
		dirty();
		const { spy, seen } = nativeConfirm(false);
		history.back();
		await until(() => spy.mock.calls.length === 1 && location.hash === '#/hosts');
		history.back();
		await until(() => spy.mock.calls.length === 2 && location.hash === '#/hosts');
		expect(seen).toEqual(['#/yaml hosts', '#/yaml hosts']);
		expect(router.route.kind).toBe('hosts');
		expect(index()).toBe(2);
		expect(history.length).toBe(length);
		cleanups.splice(0).forEach((cleanup) => cleanup());
		history.go(-2);
		await until(() => router.route.kind === 'settings');
		expect(index()).toBe(0);
		history.forward();
		await until(() => router.route.kind === 'yaml');
		expect(index()).toBe(1);
		history.forward();
		await until(() => router.route.kind === 'hosts');
		expect(index()).toBe(2);
	});

	test('a position collision left by stop/start rewrites only the entry the browser is on', async () => {
		await router.navigate('#/yaml');
		stop();
		history.back();
		await until(() => location.hash === '#/settings');
		location.hash = '#/hosts';
		stop = router.start();
		expect(router.route.kind).toBe('hosts');
		expect(index()).toBe(0);
		const length = history.length;
		dirty();
		const { spy } = nativeConfirm(false);
		const push = vi.spyOn(history, 'pushState');
		history.back();
		await until(() => spy.mock.calls.length === 1);
		expect(router.route.kind).toBe('hosts');
		expect(location.hash).toBe('#/hosts');
		expect(index()).toBe(0);
		expect(push).not.toHaveBeenCalled();
		expect(history.length).toBe(length);
		await flush();
		expect(location.hash).toBe('#/hosts');
		history.forward();
		await until(() => location.hash === '#/hosts' && index() === 0);
		expect(spy).toHaveBeenCalledTimes(1);
		expect(router.route.kind).toBe('hosts');
	});

	test('start listens to popstate only and stop removes it', async () => {
		stop();
		const added = vi.spyOn(window, 'addEventListener');
		const removed = vi.spyOn(window, 'removeEventListener');
		stop = router.start();
		expect(added.mock.calls.map(([type]) => type)).toEqual(['popstate']);
		stop();
		expect(removed.mock.calls.map(([type]) => type)).toEqual(['popstate']);
		location.hash = '#/yaml';
		await flush();
		expect(router.route.kind).toBe('settings');
		stop = router.start();
		expect(router.route.kind).toBe('yaml');
	});

	test('a correction left in flight by a stopped lifecycle is not mistaken for a landing after start', async () => {
		await router.navigate('#/yaml');
		dirty();
		const { spy, seen } = nativeConfirm(false);
		const events: string[] = [];
		const record = () => {
			events.push(`${location.hash} ${index()}`);
			if (events.length === 1) {
				stop();
				stop = router.start();
				dirty();
			}
		};
		window.addEventListener('popstate', record);
		cleanups.push(() => window.removeEventListener('popstate', record));
		history.back();
		await until(() => events.length === 3);
		expect(events).toEqual(['#/settings 0', '#/yaml 1', '#/settings 0']);
		expect(seen).toEqual(['#/settings yaml', '#/yaml settings']);
		expect(router.route.kind).toBe('settings');
		cleanups.splice(0).forEach((cleanup) => cleanup());
		history.forward();
		await until(() => router.route.kind === 'yaml');
		expect(location.hash).toBe('#/yaml');
		expect(index()).toBe(1);
		expect(spy).toHaveBeenCalledTimes(2);
	});

	test('history.state metadata written by others survives start, navigate and Back', async () => {
		stop();
		history.replaceState({ scroll: 42 }, '', '/#/settings');
		stop = router.start();
		expect(location.hash).toBe('#/settings');
		expect(history.state).toMatchObject({ scroll: 42 });
		await router.navigate('#/yaml', { replace: true });
		expect(history.state).toMatchObject({ scroll: 42 });
		await router.navigate('#/hosts');
		history.back();
		await until(() => location.hash === '#/yaml');
		expect(history.state).toMatchObject({ scroll: 42 });
	});
});
