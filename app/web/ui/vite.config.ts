import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import Icons from 'unplugin-icons/vite';
import { defineConfig, type Plugin } from 'vitest/config';

const backend = process.env.JUMPWAY_URL ?? 'http://127.0.0.1:1088';
// The tray icon bitmaps double as the web brand; the dev server must be allowed to read them.
const brandDir = fileURLToPath(new URL('../../../icon', import.meta.url));

// Vite serves dev assets outside its root under /@fs/.
export const brandInDev = (dir: string): Plugin => {
	const encoded = dir
		.replaceAll('\\', '/')
		.split('/')
		.map((part, index) => (index === 0 && /^[a-z]:$/i.test(part) ? part : encodeURIComponent(part)))
		.join('/');
	const href = path.posix.join('/@fs/', encoded, '/');
	return {
		name: 'jumpway:brand-in-dev',
		apply: 'serve',
		transformIndexHtml: {
			order: 'pre',
			handler: (html) => html.replaceAll('href="../../../icon/', `href="${href}`)
		}
	};
};

export default defineConfig({
	plugins: [svelte(), tailwindcss(), Icons({ compiler: 'svelte' }), brandInDev(brandDir)],
	build: {
		outDir: '../statics',
		emptyOutDir: true
	},
	server: {
		fs: { allow: ['.', brandDir] },
		proxy: Object.fromEntries(
			['/apis', '/metrics', '/swaggerui', '/debug'].map((prefix) => [prefix, backend])
		)
	},
	resolve: {
		alias: { $brand: brandDir },
		// Svelte's browser build is required for mount() under Vitest.
		conditions: process.env.VITEST ? ['browser'] : undefined
	},
	test: {
		environment: 'jsdom',
		include: ['src/**/*.test.ts', 'vite.config.test.ts']
	}
});
