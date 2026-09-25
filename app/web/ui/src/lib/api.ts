import { redactCredentials } from './format';
import { t } from './i18n.svelte';
import type { Address, NoProxy, RawConfig, Rule, Snapshot, Status } from './types';

// Mirrors configs.SavedPrefix: the config was written, only the reload failed.
export const SAVED_PREFIX = 'saved, but ';
export const REQUEST_TIMEOUT_MS = 15_000;

const CONFIGS = '/apis/configs';
const STATS = '/apis/stats';

export type ApiErrorKind = 'http' | 'invalid' | 'network' | 'timeout' | 'aborted';

export class ApiError extends Error {
	constructor(
		readonly kind: ApiErrorKind,
		message: string,
		readonly status: number | null
	) {
		super(message);
		this.name = 'ApiError';
	}

	get saved(): boolean {
		return this.kind === 'http' && this.message.startsWith(SAVED_PREFIX);
	}

	get unreachable(): boolean {
		return this.kind === 'network' || this.kind === 'timeout';
	}
}

export const isAborted = (error: unknown): boolean =>
	error instanceof ApiError && error.kind === 'aborted';

// Display text for any failure; server messages may echo proxy URLs, so credentials are masked here.
export function errorMessage(error: unknown): string {
	if (error instanceof ApiError) {
		switch (error.kind) {
			case 'aborted':
				return '';
			case 'invalid':
				return t('invalidResponse');
			case 'network':
			case 'timeout':
				return t('unreachable');
		}
	}
	const message = error instanceof Error ? error.message : '';
	return message ? redactCredentials(message) : t('requestFailed');
}

export interface RequestOptions {
	method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
	body?: unknown;
	signal?: AbortSignal;
	timeoutMs?: number;
}

export async function request<T>(
	url: string,
	{ method = 'GET', body, signal, timeoutMs = REQUEST_TIMEOUT_MS }: RequestOptions = {}
): Promise<T> {
	const controller = new AbortController();
	let timedOut = false;
	const timer = setTimeout(() => {
		timedOut = true;
		controller.abort();
	}, timeoutMs);
	const forwardAbort = () => controller.abort();
	if (signal?.aborted) forwardAbort();
	else signal?.addEventListener('abort', forwardAbort, { once: true });
	try {
		if (controller.signal.aborted) throw new ApiError('aborted', 'aborted', null);
		const response = await fetch(url, {
			method,
			headers:
				body === undefined
					? { Accept: 'application/json' }
					: { Accept: 'application/json', 'Content-Type': 'application/json' },
			body: body === undefined ? undefined : JSON.stringify(body),
			cache: 'no-store',
			signal: controller.signal
		});
		const text = await response.text();
		if (!response.ok) throw new ApiError('http', text, response.status);
		try {
			return JSON.parse(text) as T;
		} catch {
			throw new ApiError('invalid', 'unreadable response', response.status);
		}
	} catch (error) {
		if (error instanceof ApiError) throw error;
		if (controller.signal.aborted) {
			throw new ApiError(timedOut ? 'timeout' : 'aborted', timedOut ? 'timeout' : 'aborted', null);
		}
		throw new ApiError('network', error instanceof Error ? error.message : String(error), null);
	} finally {
		clearTimeout(timer);
		signal?.removeEventListener('abort', forwardAbort);
	}
}

const rulePath = (name: string) => CONFIGS + '/rules/' + encodeURIComponent(name);

export const configsApi = {
	getWebUI: (signal?: AbortSignal) => request<Address>(CONFIGS + '/web-ui', { signal }),
	updateWebUI: (address: Address, signal?: AbortSignal) =>
		request<null>(CONFIGS + '/web-ui', { method: 'PUT', body: address, signal }),
	getNoProxy: (signal?: AbortSignal) => request<NoProxy>(CONFIGS + '/no-proxy', { signal }),
	updateNoProxy: (noProxy: NoProxy, signal?: AbortSignal) =>
		request<null>(CONFIGS + '/no-proxy', { method: 'PUT', body: noProxy, signal }),
	getRaw: (signal?: AbortSignal) => request<RawConfig>(CONFIGS + '/raw', { signal }),
	updateRaw: (raw: RawConfig, signal?: AbortSignal) =>
		request<null>(CONFIGS + '/raw', { method: 'PUT', body: raw, signal }),
	status: (signal?: AbortSignal) => request<Status>(CONFIGS + '/status', { signal }),
	listRules: (signal?: AbortSignal) => request<Rule[] | null>(CONFIGS + '/rules', { signal }),
	createRule: (rule: Rule, signal?: AbortSignal) =>
		request<null>(CONFIGS + '/rules', { method: 'POST', body: rule, signal }),
	getRule: (name: string, signal?: AbortSignal) => request<Rule>(rulePath(name), { signal }),
	updateRule: (name: string, rule: Rule, signal?: AbortSignal) =>
		request<null>(rulePath(name), { method: 'PUT', body: rule, signal }),
	deleteRule: (name: string, signal?: AbortSignal) =>
		request<null>(rulePath(name), { method: 'DELETE', signal })
};

export const statsApi = {
	get: (signal?: AbortSignal) => request<Snapshot>(STATS, { signal }),
	reset: (signal?: AbortSignal) => request<null>(STATS, { method: 'DELETE', signal }),
	disconnect: (id: number, signal?: AbortSignal) =>
		request<null>(STATS + '/connections/' + id, { method: 'DELETE', signal })
};
