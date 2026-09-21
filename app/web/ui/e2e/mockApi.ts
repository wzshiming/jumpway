import type { Page } from '@playwright/test';
import type {
	Address,
	NoProxy,
	RawConfig,
	Rule,
	RuleStatus,
	Snapshot,
	Stats,
	Status
} from '../src/lib/types.ts';
import {
	noProxyFixture,
	rawFixture,
	rulesFixture,
	snapshotFixture,
	statsOf,
	statusFixture,
	webUIFixture
} from './fixtures/api.ts';

export interface Write {
	method: string;
	path: string;
	body: unknown;
}

export interface MockApi {
	status: Status;
	rules: Rule[];
	snapshot: Snapshot;
	webUI: Address;
	noProxy: NoProxy;
	raw: RawConfig;
	// A PUT that changes the web UI address closes the listener serving this page: status stops answering.
	stopOnMove: boolean;
	// The new web UI listener fails to bind: like the tray, the status then names the configured
	// address (port 0 included) with running false, and the PUT answers "saved, but".
	bindFails: boolean;
	// Paths that currently fail at the network level (connection refused).
	down: Set<string>;
	// Paths whose write is applied but whose answer is lost on the way back (connection reset).
	lost: Set<string>;
	// `${method} ${path}` whose next answer waits for the promise; its body is fixed when the
	// request arrives, like a server that answered slowly.
	delay: Map<string, Promise<void>>;
	// `${method} ${path}` of every request, in order.
	requests: string[];
	// Every POST/PUT/DELETE with its JSON body, in order.
	writes: Write[];
	// `${method} ${path}` → 400 text/plain answer. A "saved, but " text still applies the write first.
	fail: Map<string, string>;
}

const SAVED_PREFIX = 'saved, but ';
const RULE_PATH = /^\/apis\/configs\/rules\/([^/]+)$/;
const CONNECTION_PATH = /^\/apis\/stats\/connections\/(\d+)$/;

const text = (status: number, body: string) => ({ status, contentType: 'text/plain', body });

// Like metrics.Registry.Reset: every counter (live connections included) restarts from zero,
// only targets with a live connection survive, and `since` moves to now.
function resetSnapshot(snapshot: Snapshot, now: string): void {
	const zero = (): Stats => statsOf({});
	snapshot.since = now;
	for (const rule of snapshot.rules ?? []) {
		rule.stats = zero();
		for (const way of [rule.listen, rule.forward]) {
			for (const hop of way ?? []) {
				hop.stats = zero();
				for (const url of hop.urls ?? []) url.stats = zero();
			}
		}
		const live = new Set((rule.connections ?? []).map((connection) => connection.target));
		rule.targets = (rule.targets ?? []).filter((target) => live.has(target.address));
		for (const target of rule.targets) target.stats = zero();
		for (const connection of rule.connections ?? []) {
			connection.stats = zero();
			connection.started = now;
		}
		rule.targets_evicted = 0;
	}
}

// Like the status API: virtual endpoints report virtual://channel and never count as remote.
const runtimeOf = (rule: Rule): RuleStatus => ({
	name: rule.name,
	address: rule.listen.virtual
		? 'virtual://' + rule.listen.virtual
		: (rule.listen.host || '127.0.0.1') + ':' + rule.listen.port,
	target: rule.forward.virtual
		? 'virtual://' + rule.forward.virtual
		: rule.forward.port
			? (rule.forward.host || '127.0.0.1') + ':' + rule.forward.port
			: undefined,
	remote: !rule.listen.virtual && Array.isArray(rule.listen.way) && rule.listen.way.length > 0,
	running: !rule.disabled
});

// Serves /apis from in-memory fixtures; mutations mirror the backend's validation messages.
export async function installMockApi(
	page: Page,
	overrides: Partial<MockApi> = {}
): Promise<MockApi> {
	const api: MockApi = {
		status: structuredClone(statusFixture),
		rules: structuredClone(rulesFixture),
		snapshot: structuredClone(snapshotFixture),
		webUI: structuredClone(webUIFixture),
		noProxy: structuredClone(noProxyFixture),
		raw: structuredClone(rawFixture),
		stopOnMove: false,
		bindFails: false,
		down: new Set(),
		lost: new Set(),
		delay: new Map(),
		requests: [],
		writes: [],
		fail: new Map(),
		...overrides
	};
	const statusRules = () => (api.status.rules ??= []);

	function create(rule: Rule): string | null {
		if (!rule.name) return 'rule name is empty';
		if (api.rules.some((entry) => entry.name === rule.name))
			return `rule ${JSON.stringify(rule.name)} already exists`;
		api.rules.push(rule);
		statusRules().push(runtimeOf(rule));
		return null;
	}

	function update(name: string, rule: Rule): string | null {
		const index = api.rules.findIndex((entry) => entry.name === name);
		if (index < 0) return `rule ${JSON.stringify(name)} not found`;
		if (rule.name !== name && api.rules.some((entry) => entry.name === rule.name))
			return `rule ${JSON.stringify(rule.name)} already exists`;
		api.rules[index] = rule;
		const runtime = statusRules().findIndex((entry) => entry.name === name);
		if (runtime >= 0) statusRules()[runtime] = { ...statusRules()[runtime], ...runtimeOf(rule) };
		return null;
	}

	function remove(name: string): string | null {
		const index = api.rules.findIndex((entry) => entry.name === name);
		if (index < 0) return `rule ${JSON.stringify(name)} not found`;
		api.rules.splice(index, 1);
		api.status.rules = statusRules().filter((entry) => entry.name !== name);
		return null;
	}

	// Port 0 lets the server pick one; the mock's pick is fixed so tests can assert the link.
	function updateWebUI(address: Address): string | null {
		if (!Number.isInteger(address.port) || address.port < 0 || address.port > 65535)
			return `web_ui.port ${address.port} is out of range (0-65535)`;
		const changed = address.host !== api.webUI.host || address.port !== api.webUI.port;
		api.webUI = address;
		if (api.bindFails) {
			api.status.address = (address.host || '127.0.0.1') + ':' + address.port;
			api.status.running = false;
			return `${SAVED_PREFIX}web_ui: listen tcp ${api.status.address}: bind: address already in use`;
		}
		api.status.address = address.port
			? (address.host || '127.0.0.1') + ':' + address.port
			: '127.0.0.1:43210';
		if (api.stopOnMove && changed) api.down.add('/apis/configs/status');
		return null;
	}

	function updateRaw(raw: RawConfig): string | null {
		if (!raw.yaml.trim()) return 'config is empty';
		api.raw = { yaml: raw.yaml };
		return null;
	}

	function disconnect(id: number): string | null {
		for (const rule of api.snapshot.rules ?? []) {
			const index = (rule.connections ?? []).findIndex((connection) => connection.id === id);
			if (index >= 0) {
				rule.connections!.splice(index, 1);
				return null;
			}
		}
		return `connection ${id} not found`;
	}

	await page.route('**/apis/**', async (route) => {
		const request = route.request();
		const path = new URL(request.url()).pathname;
		const method = request.method();
		api.requests.push(`${method} ${path}`);
		if (api.down.has(path)) return route.abort('connectionrefused');
		const held = api.delay.get(`${method} ${path}`);
		api.delay.delete(`${method} ${path}`);
		const json = async (body: unknown) => {
			const payload = JSON.stringify(body);
			if (held) await held;
			return route.fulfill({ status: 200, contentType: 'application/json', body: payload });
		};
		const injected = api.fail.get(`${method} ${path}`);
		if (method !== 'GET') {
			api.writes.push({
				method,
				path,
				body: method === 'DELETE' ? undefined : request.postDataJSON()
			});
		}
		if (injected !== undefined && !injected.startsWith(SAVED_PREFIX))
			return route.fulfill(text(400, injected));
		const match = RULE_PATH.exec(path);
		const name = match ? decodeURIComponent(match[1]) : null;
		const connection = CONNECTION_PATH.exec(path);
		if (method === 'GET') {
			switch (path) {
				case '/apis/configs/status':
					return json(api.status);
				case '/apis/configs/rules':
					return json(api.rules);
				case '/apis/configs/web-ui':
					return json(api.webUI);
				case '/apis/configs/no-proxy':
					return json(api.noProxy);
				case '/apis/configs/raw':
					return json(api.raw);
				case '/apis/stats':
					return json(api.snapshot);
			}
			if (name !== null) {
				const rule = api.rules.find((entry) => entry.name === name);
				return rule
					? json(rule)
					: route.fulfill(text(400, `rule ${JSON.stringify(name)} not found`));
			}
			return route.fulfill(text(400, `mock API: unsupported ${method} ${path}`));
		}
		const body = api.writes.at(-1)!.body;
		let error: string | null;
		if (method === 'POST' && path === '/apis/configs/rules') error = create(body as Rule);
		else if (method === 'PUT' && name !== null) error = update(name, body as Rule);
		else if (method === 'DELETE' && name !== null) error = remove(name);
		else if (method === 'PUT' && path === '/apis/configs/web-ui')
			error = updateWebUI(body as Address);
		else if (method === 'PUT' && path === '/apis/configs/no-proxy') {
			api.noProxy = body as NoProxy;
			error = null;
		} else if (method === 'PUT' && path === '/apis/configs/raw')
			error = updateRaw(body as RawConfig);
		else if (method === 'DELETE' && path === '/apis/stats') {
			resetSnapshot(api.snapshot, new Date().toISOString());
			error = null;
		} else if (method === 'DELETE' && connection) error = disconnect(Number(connection[1]));
		else return route.fulfill(text(400, `mock API: unsupported ${method} ${path}`));
		if (error !== null) return route.fulfill(text(400, error));
		if (api.lost.has(path)) return route.abort('connectionreset');
		return injected === undefined ? json(null) : route.fulfill(text(400, injected));
	});
	return api;
}

export const count = (api: MockApi, entry: string): number =>
	api.requests.filter((request) => request === entry).length;
