import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { en } from './i18n/en';
import { zh } from './i18n/zh';
import { i18n, resolveLanguage, t } from './i18n.svelte';

function setNavigatorLanguages(languages: string[]) {
	Object.defineProperty(navigator, 'language', { value: languages[0], configurable: true });
	Object.defineProperty(navigator, 'languages', { value: languages, configurable: true });
}

// jsdom lands history traversals a few macrotasks later.
async function until(predicate: () => boolean) {
	for (let i = 0; i < 50 && !predicate(); i++) {
		await new Promise((resolve) => setTimeout(resolve, 1));
	}
}

beforeEach(() => {
	history.replaceState(null, '', '/');
	localStorage.clear();
	setNavigatorLanguages(['en-US']);
});

afterEach(() => {
	i18n.dispose();
	vi.restoreAllMocks();
});

describe('dictionaries', () => {
	test('en and zh expose the same keys and keep the legacy copy', () => {
		expect(Object.keys(zh).sort()).toEqual(Object.keys(en).sort());
		expect(Object.keys(en).length).toBeGreaterThanOrEqual(153);
		expect(en.listenThroughHint).toContain('Only ssh://, cmd: and nc hops can bind.');
		expect(zh.showingConnections).toBe('显示 {total} 个连接中的 {shown} 个');
		expect(en['field.encrypto']).toBe('Encryption method');
	});

	test('the zh TUN tooltip relays DNS over TCP and leaves other UDP to the rule options', () => {
		expect(zh['help.tun']).toBe(
			'TUN：虚拟网卡，捕获本机 TCP 和 UDP 流量并经由规则的出口链路发出。DNS 查询（UDP 53 端口）以 DNS over TCP 转发，规则的 DNS 模式为 UDP 时除外；其他 UDP 按规则的 UDP 选项处理。'
		);
	});

	test('the UDP mode hint sends TUN DNS queries by the DNS mode, even when UDP is direct or blocked', () => {
		expect(en.udpModeHint).toContain(
			'DNS queries captured by the TUN (UDP port 53) follow the DNS mode, not the UDP mode: unless the DNS mode is UDP, they are still relayed as DNS over TCP through the exit chain when the UDP mode is Direct or Block.'
		);
		expect(zh.udpModeHint).toContain(
			'TUN 捕获的 DNS 查询（UDP 53 端口）按 DNS 模式而非 UDP 模式处理：除非 DNS 模式为 UDP，即使 UDP 模式为直连或阻止，它们仍以 DNS over TCP 经由出口链路转发。'
		);
	});

	test('the UDP NAT hint bounds full cone by what the exit chain can address', () => {
		expect(en.udpNatHint).toContain(
			"Full cone holds as far as the exit chain's UDP can address every destination; otherwise each destination gets its own connection."
		);
		expect(zh.udpNatHint).toContain(
			'全锥形仅在出口链路的 UDP 能发往每个目标时成立，否则每个目标各用一条连接。'
		);
	});

	test('the zh DNS server texts write 主机:端口 without a stray space', () => {
		expect(zh.dnsServerHint).toBe(
			'仅用于 DNS over TCP：所有查询都发往这里填写的主机:端口，而不是查询原本发往的服务器。'
		);
		expect(zh.dnsServerInvalid).toBe('请输入主机:端口，主机不能为空，端口须为 1 到 65535。');
	});
});

describe('resolveLanguage', () => {
	test('explicit query wins over storage and navigator', () => {
		expect(
			resolveLanguage({ query: '?lang=en', stored: 'zh', navigatorLanguages: ['zh-CN'] })
		).toEqual({ language: 'en', explicit: true });
	});

	test('storage wins over navigator; invalid values are ignored', () => {
		expect(
			resolveLanguage({ query: '?lang=fr', stored: 'zh', navigatorLanguages: ['en'] })
		).toEqual({ language: 'zh', explicit: false });
		expect(
			resolveLanguage({ query: '', stored: 'klingon', navigatorLanguages: ['zh-TW', 'en'] })
		).toEqual({ language: 'zh', explicit: false });
	});

	test('any zh* navigator entry selects zh, otherwise en', () => {
		expect(resolveLanguage({ query: '', stored: null, navigatorLanguages: ['fr', 'ZH'] })).toEqual({
			language: 'zh',
			explicit: false
		});
		expect(resolveLanguage({ query: '', stored: null, navigatorLanguages: [] })).toEqual({
			language: 'en',
			explicit: false
		});
	});
});

describe('i18n store', () => {
	test('t interpolates placeholders and follows the active language', () => {
		i18n.init();
		expect(t('retrying', { attempt: 3 })).toBe('Retrying (3)');
		expect(t('ago', { time: '5 s' })).toBe('5 s ago');
		i18n.setLanguage('zh');
		expect(t('ago', { time: '5 s' })).toBe('5 s前');
		expect(t('connectionCount', {})).toBe(' 条连接');
	});

	test('init applies the document language from the query', () => {
		history.replaceState(null, '', '/?lang=zh#/rules');
		i18n.init();
		expect(i18n.language).toBe('zh');
		expect(i18n.explicit).toBe(true);
		expect(document.documentElement.lang).toBe('zh-CN');
	});

	test('setLanguage persists and updates an explicit ?lang= without touching the hash', () => {
		history.replaceState(null, '', '/?lang=zh&x=1#/stats?rule=a');
		i18n.init();
		i18n.setLanguage('en');
		expect(i18n.language).toBe('en');
		expect(localStorage.getItem('jumpway.lang')).toBe('en');
		expect(location.search).toBe('?lang=en&x=1');
		expect(location.hash).toBe('#/stats?rule=a');
		expect(document.documentElement.lang).toBe('en');
	});

	test('setLanguage leaves the URL alone when no ?lang= is present', () => {
		history.replaceState(null, '', '/#/yaml');
		i18n.init();
		i18n.setLanguage('zh');
		expect(location.search).toBe('');
		expect(location.hash).toBe('#/yaml');
		expect(i18n.explicit).toBe(false);
		expect(localStorage.getItem('jumpway.lang')).toBe('zh');
	});

	test('stored preference is honoured on init and storage failures are tolerated', () => {
		localStorage.setItem('jumpway.lang', 'zh');
		i18n.init();
		expect(i18n.language).toBe('zh');
		i18n.dispose();

		vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
			throw new DOMException('denied', 'SecurityError');
		});
		vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
			throw new DOMException('denied', 'SecurityError');
		});
		setNavigatorLanguages(['zh-CN']);
		i18n.init();
		expect(i18n.language).toBe('zh');
		expect(() => i18n.setLanguage('en')).not.toThrow();
		expect(i18n.language).toBe('en');
	});

	test('Back/Forward to an entry with another ?lang= re-applies the language priority', async () => {
		history.replaceState(null, '', '/?lang=zh#/');
		history.pushState({ scroll: 42 }, '', '/?lang=zh#/rules');
		i18n.init();
		expect(i18n.language).toBe('zh');
		i18n.setLanguage('en');
		expect(location.search).toBe('?lang=en');
		expect(history.state).toEqual({ scroll: 42 });

		history.back();
		await until(() => location.search === '?lang=zh');
		expect(location.search).toBe('?lang=zh');
		expect(i18n.language).toBe('zh');
		expect(i18n.explicit).toBe(true);
		expect(document.documentElement.lang).toBe('zh-CN');

		history.forward();
		await until(() => location.search === '?lang=en');
		expect(i18n.language).toBe('en');
		expect(document.documentElement.lang).toBe('en');

		history.replaceState(null, '', '/#/rules');
		history.back();
		await until(() => location.search === '?lang=zh');
		history.forward();
		await until(() => location.search === '');
		expect(i18n.language).toBe('en');
		expect(i18n.explicit).toBe(false);
	});

	test('init attaches a single history listener that dispose removes', async () => {
		const added = vi.spyOn(window, 'addEventListener');
		i18n.init();
		i18n.init();
		expect(added.mock.calls.filter(([type]) => type === 'popstate')).toHaveLength(1);
		history.replaceState(null, '', '/?lang=zh#/');
		history.pushState(null, '', '/?lang=en#/rules');
		i18n.dispose();
		history.back();
		await until(() => location.search === '?lang=zh');
		expect(location.search).toBe('?lang=zh');
		expect(i18n.language).toBe('en');
		i18n.init();
		expect(i18n.language).toBe('zh');
	});
});
