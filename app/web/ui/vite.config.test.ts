// @vitest-environment node
import { readFile } from 'node:fs/promises';
import { describe, expect, it } from 'vitest';
import { brandInDev } from './vite.config.ts';

const indexHtml = await readFile(new URL('./index.html', import.meta.url), 'utf8');

async function rewrite(dir: string) {
	const hook = brandInDev(dir).transformIndexHtml;
	if (typeof hook !== 'object') throw new Error('expected an object hook');
	expect(hook.order).toBe('pre');
	// The hook never reads its plugin context.
	return hook.handler.call(undefined as never, indexHtml, {
		path: '/index.html',
		filename: 'index.html'
	});
}

describe('brandInDev', () => {
	it('is a dev-server-only plugin', () => {
		expect(brandInDev('/repo/icon').apply).toBe('serve');
	});

	it('rewrites the brand href to a /@fs/ URL on POSIX, percent-encoding spaces', async () => {
		const html = await rewrite('/Users/zs m/jumpway/icon');
		expect(html).toContain('href="/@fs/Users/zs%20m/jumpway/icon/icon_black.png"');
		expect(html).not.toContain('../../../icon/');
	});

	it('keeps the Windows drive letter behind the /@fs/ prefix', async () => {
		const html = await rewrite('C:\\work\\jumpway\\icon');
		expect(html).toContain('href="/@fs/C:/work/jumpway/icon/icon_black.png"');
	});

	it.each([
		['/Users/work#1/project?copy/icon', '/@fs/Users/work%231/project%3Fcopy/icon/'],
		['C:\\work#1\\project%copy\\icon', '/@fs/C:/work%231/project%25copy/icon/']
	])('encodes reserved path characters in %s', async (dir, expected) => {
		const html = await rewrite(dir);
		expect(html).toContain(`href="${expected}icon_black.png"`);
	});
});
