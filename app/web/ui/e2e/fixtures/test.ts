import { expect, test as base } from '@playwright/test';
import { installMockApi, type MockApi } from '../mockApi.ts';

// The built app served by vite preview, with /apis answered by the in-memory mock.
// Every test also fails on console errors, page errors and unexpected request failures.

export interface Failures {
	entries: string[];
	allow: (entry: string) => boolean;
}

export const test = base.extend<{ api: MockApi; failures: Failures }>({
	api: [
		async ({ page }, use) => {
			await use(await installMockApi(page));
		},
		{ auto: true }
	],
	failures: [
		async ({ page }, use) => {
			const failures: Failures = { entries: [], allow: () => false };
			page.on('console', (message) => {
				if (message.type() === 'error') failures.entries.push('console: ' + message.text());
			});
			page.on('pageerror', (error) => failures.entries.push('pageerror: ' + error.message));
			page.on('requestfailed', (request) =>
				failures.entries.push('requestfailed: ' + request.url())
			);
			page.on('response', (response) => {
				if (response.status() >= 400)
					failures.entries.push(`http ${response.status()}: ${response.url()}`);
			});
			await use(failures);
			expect(failures.entries.filter((entry) => !failures.allow(entry))).toEqual([]);
		},
		{ auto: true }
	]
});

export { expect };
