import { defineConfig } from '@playwright/test';

const previewURL = 'http://127.0.0.1:4173';
// JUMPWAY_URL points the opt-in `real` project at a running backend; the mocked preview server is
// then not started, so run that project alone: `playwright test --project real`.
const realURL = process.env.JUMPWAY_URL;

export default defineConfig({
	testDir: 'e2e',
	webServer: realURL
		? undefined
		: {
				command: 'pnpm build:dist && pnpm preview:dist --host 127.0.0.1 --port 4173 --strictPort',
				url: previewURL,
				reuseExistingServer: !process.env.CI
			},
	use: {
		baseURL: previewURL,
		// PW_CHANNEL=chrome runs against the locally installed Chrome instead of a downloaded browser.
		channel: process.env.PW_CHANNEL
	},
	projects: [
		{ name: 'chromium', testIgnore: 'real/**', use: { browserName: 'chromium' } },
		{
			name: 'real',
			testMatch: 'real/**/*.spec.ts',
			use: { browserName: 'chromium', baseURL: realURL }
		}
	]
});
