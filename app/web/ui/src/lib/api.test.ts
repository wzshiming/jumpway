import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import {
	ApiError,
	REQUEST_TIMEOUT_MS,
	SAVED_PREFIX,
	configsApi as configs,
	errorMessage,
	request,
	statsApi as stats
} from './api';
import { i18n } from './i18n.svelte';

type Call = { url: string; init: RequestInit };
let calls: Call[];

function stubFetch(handler: (call: Call) => Response | Promise<Response>) {
	calls = [];
	vi.stubGlobal(
		'fetch',
		vi.fn((url: string, init: RequestInit) => {
			const call = { url, init };
			calls.push(call);
			return Promise.resolve().then(() => handler(call));
		})
	);
}

/** A request that only ever settles when its signal aborts. */
function stubHanging() {
	stubFetch(
		({ init }) =>
			new Promise<Response>((_, reject) => {
				const signal = init.signal!;
				const fail = () => reject(signal.reason ?? new DOMException('aborted', 'AbortError'));
				if (signal.aborted) fail();
				else signal.addEventListener('abort', fail, { once: true });
			})
	);
}

const json = (value: unknown, status = 200) =>
	new Response(JSON.stringify(value), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});

beforeEach(() => i18n.init());
afterEach(() => {
	vi.unstubAllGlobals();
	vi.useRealTimers();
	i18n.dispose();
});

describe('request', () => {
	test('GET asks for JSON with no-store and parses the body', async () => {
		stubFetch(() => json({ ok: 1 }));
		await expect(request('/apis/configs/status')).resolves.toEqual({ ok: 1 });
		expect(calls[0].url).toBe('/apis/configs/status');
		expect(calls[0].init).toMatchObject({
			method: 'GET',
			cache: 'no-store',
			headers: { Accept: 'application/json' }
		});
		expect((calls[0].init.headers as Record<string, string>)['Content-Type']).toBeUndefined();
		expect(calls[0].init.body).toBeUndefined();
	});

	test('mutations send a JSON body and resolve null like the server does', async () => {
		stubFetch(() => json(null));
		await expect(
			request('/apis/configs/raw', { method: 'PUT', body: { yaml: 'a: 1' } })
		).resolves.toBeNull();
		expect(calls[0].init.method).toBe('PUT');
		expect(calls[0].init.headers).toEqual({
			Accept: 'application/json',
			'Content-Type': 'application/json'
		});
		expect(calls[0].init.body).toBe('{"yaml":"a: 1"}');
	});

	test('non-2xx responses surface the plain-text body as an http error', async () => {
		stubFetch(() => new Response('rules[0].name "a/b" must not contain "/"', { status: 400 }));
		const error = await request('/apis/configs/rules', { method: 'POST', body: {} }).catch(
			(e) => e
		);
		expect(error).toBeInstanceOf(ApiError);
		expect(error).toMatchObject({
			kind: 'http',
			status: 400,
			message: 'rules[0].name "a/b" must not contain "/"',
			saved: false,
			unreachable: false
		});
	});

	test('the saved-but-reload-failed prefix is recognised', async () => {
		stubFetch(() => new Response(SAVED_PREFIX + 'listen tcp: address in use', { status: 400 }));
		const error = await request('/apis/configs/rules/a', { method: 'PUT', body: {} }).catch(
			(e) => e
		);
		expect(error).toMatchObject({
			kind: 'http',
			saved: true,
			message: 'saved, but listen tcp: address in use'
		});
	});

	test('unparseable 2xx bodies are an invalid-response error, distinct from http errors', async () => {
		stubFetch(() => new Response('<html>oops</html>', { status: 200 }));
		const error = await request('/apis/stats').catch((e) => e);
		expect(error).toMatchObject({ kind: 'invalid', status: 200, unreachable: false });
	});

	test('a failed fetch is a network error and counts as unreachable', async () => {
		stubFetch(() => {
			throw new TypeError('Failed to fetch');
		});
		const error = await request('/apis/configs/status').catch((e) => e);
		expect(error).toMatchObject({ kind: 'network', unreachable: true, status: null });
	});

	test('requests time out after 15 s as unreachable and leave no timers behind', async () => {
		vi.useFakeTimers();
		stubHanging();
		const pending = request('/apis/configs/status');
		const settled = pending.catch((e) => e);
		expect(REQUEST_TIMEOUT_MS).toBe(15_000);
		await vi.advanceTimersByTimeAsync(REQUEST_TIMEOUT_MS - 1);
		expect(vi.getTimerCount()).toBe(1);
		await vi.advanceTimersByTimeAsync(1);
		expect(await settled).toMatchObject({ kind: 'timeout', unreachable: true });
		expect(vi.getTimerCount()).toBe(0);
	});

	test("a caller's abort is reported as aborted, not unreachable, and its listener is removed", async () => {
		vi.useFakeTimers();
		stubHanging();
		const controller = new AbortController();
		const remove = vi.spyOn(controller.signal, 'removeEventListener');
		const settled = request('/apis/stats', { signal: controller.signal }).catch((e) => e);
		controller.abort();
		expect(await settled).toMatchObject({ kind: 'aborted', unreachable: false });
		expect(remove).toHaveBeenCalledWith('abort', expect.any(Function));
		expect(vi.getTimerCount()).toBe(0);
	});

	test('an already aborted signal never reaches the network', async () => {
		stubHanging();
		const controller = new AbortController();
		controller.abort();
		const error = await request('/apis/stats', { signal: controller.signal }).catch((e) => e);
		expect(error).toMatchObject({ kind: 'aborted' });
		expect(calls).toHaveLength(0);
	});

	test('a successful request clears its timeout', async () => {
		vi.useFakeTimers();
		stubFetch(() => json([]));
		await request('/apis/configs/rules');
		expect(vi.getTimerCount()).toBe(0);
	});
});

describe('typed clients', () => {
	test('configs routes and encodings', async () => {
		stubFetch(() => json(null));
		await configs.get();
		await configs.update({
			web_ui: { host: '', port: 1 },
			rules: null,
			no_proxy: { list: null, from_env: null, from_file: null }
		});
		await configs.getWebUI();
		await configs.updateWebUI({ host: '127.0.0.1', port: 1088 });
		await configs.getNoProxy();
		await configs.updateNoProxy({ list: ['a'], from_env: [], from_file: [] });
		await configs.getRaw();
		await configs.updateRaw({ yaml: 'x' });
		await configs.status();
		await configs.listRules();
		await configs.createRule({ name: 'n', listen: { host: '', port: 0 }, forward: {} });
		await configs.getRule('a b');
		await configs.updateRule('a/b', { name: 'c', listen: { host: '', port: 0 }, forward: {} });
		await configs.deleteRule('new');
		expect(calls.map((call) => [call.init.method, call.url])).toEqual([
			['GET', '/apis/configs'],
			['PUT', '/apis/configs'],
			['GET', '/apis/configs/web-ui'],
			['PUT', '/apis/configs/web-ui'],
			['GET', '/apis/configs/no-proxy'],
			['PUT', '/apis/configs/no-proxy'],
			['GET', '/apis/configs/raw'],
			['PUT', '/apis/configs/raw'],
			['GET', '/apis/configs/status'],
			['GET', '/apis/configs/rules'],
			['POST', '/apis/configs/rules'],
			['GET', '/apis/configs/rules/a%20b'],
			['PUT', '/apis/configs/rules/a%2Fb'],
			['DELETE', '/apis/configs/rules/new']
		]);
		expect(calls[3].init.body).toBe('{"host":"127.0.0.1","port":1088}');
	});

	test('stats routes use the numeric connection id', async () => {
		stubFetch(() => json(null));
		await stats.get();
		await stats.reset();
		await stats.disconnect(7);
		expect(calls.map((call) => [call.init.method, call.url])).toEqual([
			['GET', '/apis/stats'],
			['DELETE', '/apis/stats'],
			['DELETE', '/apis/stats/connections/7']
		]);
	});

	test('the caller signal is forwarded through the clients', async () => {
		stubHanging();
		const controller = new AbortController();
		const settled = stats.get(controller.signal).catch((e) => e);
		controller.abort();
		expect(await settled).toMatchObject({ kind: 'aborted' });
	});
});

describe('errorMessage', () => {
	test('maps kinds to localized copy and keeps server text', () => {
		expect(errorMessage(new ApiError('http', 'port 70000 out of range', 400))).toBe(
			'port 70000 out of range'
		);
		expect(errorMessage(new ApiError('http', '', 500))).toBe('The request could not be completed.');
		expect(errorMessage(new ApiError('invalid', 'x', 200))).toBe(
			'JumpWay returned an unreadable response. Try reloading from disk.'
		);
		expect(errorMessage(new ApiError('network', 'Failed to fetch', null))).toBe(
			'Cannot reach JumpWay.'
		);
		expect(errorMessage(new ApiError('timeout', '', null))).toBe('Cannot reach JumpWay.');
		expect(errorMessage(new ApiError('aborted', '', null))).toBe('');
		expect(errorMessage(new Error('boom'))).toBe('boom');
		expect(errorMessage('weird')).toBe('The request could not be completed.');
		i18n.setLanguage('zh');
		expect(errorMessage(new ApiError('timeout', '', null))).toBe('无法连接到 JumpWay。');
	});

	test('masks URL credentials echoed by the server while the raw error keeps its classification', () => {
		const url = 'socks5://review-user:review-secret@host:bad';
		const error = new ApiError(
			'http',
			`saved, but rules[0].forward.way[0]: invalid proxy URL "${url}": parse "${url}": invalid port ":bad" after host`,
			400
		);
		expect(error.saved).toBe(true);
		expect(error.message).toContain('review-secret');
		const shown = errorMessage(error);
		expect(shown).not.toContain('review-secret');
		expect(shown).not.toContain('review-user');
		expect(shown).toBe(
			'saved, but rules[0].forward.way[0]: invalid proxy URL "socks5://xxxxx@host:bad": parse "socks5://xxxxx@host:bad": invalid port ":bad" after host'
		);
		expect(errorMessage(new Error('dial ssh://u:p@h:22: refused'))).toBe(
			'dial ssh://xxxxx@h:22: refused'
		);
	});
});
