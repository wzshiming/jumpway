import { flushSync, mount, unmount } from 'svelte';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { statusFixture } from '../../e2e/fixtures/api';
import App from '../App.svelte';
import { toasts } from '../lib/toast.svelte';

// Its own file: the snapshot poller is a module singleton and only a fresh one has no answer,
// which is the state in which a failed snapshot must not leave the placeholders standing.

let target: HTMLElement;
let app: ReturnType<typeof mount> | null = null;

const json = (body: unknown) =>
	new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});

beforeEach(() => {
	vi.useFakeTimers();
	vi.stubGlobal(
		'fetch',
		vi.fn(async (url: string) => {
			if (url === '/apis/stats') throw new TypeError('Failed to fetch');
			if (url === '/apis/configs/status') return json(statusFixture);
			if (url === '/apis/configs/rules') return new Promise<Response>(() => {});
			return new Response('not found', { status: 404 });
		})
	);
});

afterEach(() => {
	if (app) unmount(app);
	app = null;
	target?.remove();
	toasts.clear();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	document.body.innerHTML = '';
	history.replaceState(null, '', '/');
});

const PAGES = [
	['#/stats', 'Statistics'],
	['#/hosts', 'Hosts'],
	['#/connections', 'Connections']
] as const;

test('a snapshot that fails while the rules are still pending shows its error instead of placeholders on every list page', async () => {
	for (const [hash, label] of PAGES) {
		history.replaceState(null, '', '/' + hash);
		target = document.body.appendChild(document.createElement('div'));
		app = mount(App, { target });
		flushSync();
		await vi.advanceTimersByTimeAsync(0);
		const section = target.querySelector<HTMLElement>(`main section[aria-label="${label}"]`)!;
		expect(section.querySelector('[role="alert"]')?.textContent, hash).toContain(
			'Cannot reach JumpWay.'
		);
		expect(section.querySelectorAll('[data-skeleton], [data-skeleton-row]'), hash).toHaveLength(0);
		expect(
			Array.from(section.querySelectorAll('.sr-only')).some(
				(element) => element.textContent?.trim() === 'Loading...'
			),
			hash
		).toBe(false);
		unmount(app);
		app = null;
		target.remove();
	}
});
