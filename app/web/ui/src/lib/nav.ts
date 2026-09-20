import type { Component } from 'svelte';
import type { SvelteHTMLElements } from 'svelte/elements';
import IconCable from '~icons/lucide/cable';
import IconChartColumn from '~icons/lucide/chart-column';
import IconFileCode from '~icons/lucide/file-code';
import IconLayoutDashboard from '~icons/lucide/layout-dashboard';
import IconServer from '~icons/lucide/server';
import IconSettings from '~icons/lucide/settings';
import { t, type MessageKey } from './i18n.svelte';
import type { PageKind, Route } from './routes';

export interface NavItem {
	hash: string;
	label: MessageKey;
	// The routes this entry is current for; every PageKind belongs to exactly one entry.
	kinds: readonly PageKind[];
	icon: Component<SvelteHTMLElements['svg']>;
}

export const NAV_ITEMS: readonly NavItem[] = [
	{
		hash: '#/',
		label: 'page.overview',
		kinds: ['overview', 'new', 'rule'],
		icon: IconLayoutDashboard
	},
	{ hash: '#/stats', label: 'page.stats', kinds: ['stats'], icon: IconChartColumn },
	{ hash: '#/hosts', label: 'page.hosts', kinds: ['hosts'], icon: IconServer },
	{ hash: '#/connections', label: 'page.connections', kinds: ['connections'], icon: IconCable },
	{ hash: '#/settings', label: 'page.settings', kinds: ['settings'], icon: IconSettings },
	{ hash: '#/yaml', label: 'page.yaml', kinds: ['yaml'], icon: IconFileCode }
];

export const navItemFor = (kind: PageKind): NavItem =>
	NAV_ITEMS.find((item) => item.kinds.includes(kind))!;

// The editor is titled by its rule and the new-rule page by itself; the rest by their nav entry.
export function pageTitle(route: Route): string {
	if (route.kind === 'rule') return route.name ?? '';
	if (route.kind === 'new') return t('newRule');
	return t(navItemFor(route.kind).label);
}
