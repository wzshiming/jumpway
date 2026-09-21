<script lang="ts">
	import { onDestroy, untrack } from 'svelte';
	import IconMenu from '~icons/lucide/menu';
	import IconPanelLeftClose from '~icons/lucide/panel-left-close';
	import IconPanelLeftOpen from '~icons/lucide/panel-left-open';
	import { hashLinks } from './lib/actions/links';
	import { ApiError } from './lib/api';
	import Brand from './lib/components/shell/Brand.svelte';
	import MobileDrawer from './lib/components/shell/MobileDrawer.svelte';
	import Sidebar from './lib/components/shell/Sidebar.svelte';
	import Banner from './lib/components/ui/Banner.svelte';
	import Button from './lib/components/ui/Button.svelte';
	import ConfirmDialog from './lib/components/ui/ConfirmDialog.svelte';
	import IconButton from './lib/components/ui/IconButton.svelte';
	import StatusChip from './lib/components/ui/StatusChip.svelte';
	import ToastHost from './lib/components/ui/ToastHost.svelte';
	import TooltipHost from './lib/components/ui/TooltipHost.svelte';
	import { i18n, t } from './lib/i18n.svelte';
	import { DESKTOP_QUERY, mediaQuery } from './lib/media.svelte';
	import { moved } from './lib/moved.svelte';
	import { pageTitle } from './lib/nav';
	import { router } from './lib/router.svelte';
	import { sidebar } from './lib/sidebar.svelte';
	import { runtimeState, status } from './lib/status.svelte';
	import { theme } from './lib/theme.svelte';
	import ConnectionsPage from './routes/ConnectionsPage.svelte';
	import HostsPage from './routes/HostsPage.svelte';
	import OverviewPage from './routes/OverviewPage.svelte';
	import RuleEditorPage from './routes/RuleEditorPage.svelte';
	import SettingsPage from './routes/SettingsPage.svelte';
	import StatsRulesPage from './routes/StatsRulesPage.svelte';
	import YamlPage from './routes/YamlPage.svelte';

	const DRAWER_ID = 'navigation-drawer';
	const SIDEBAR_ID = 'sidebar';

	// Language, route and sidebar width are resolved before the first render so nothing flashes.
	i18n.init();
	sidebar.init();
	const desktop = mediaQuery(DESKTOP_QUERY, true);
	const stops = [theme.init(), router.start(), status.subscribe(), desktop.start()];
	onDestroy(() => {
		for (const stop of stops.reverse()) stop();
		i18n.dispose();
	});

	const route = $derived(router.route);
	const title = $derived(pageTitle(route));
	const unreachable = $derived(status.error instanceof ApiError && status.error.unreachable);
	const runtime = $derived(runtimeState());
	const notice = $derived(moved.notice);
	const collapsed = $derived(sidebar.collapsed);

	let drawerOpen = $state(false);
	let menuButton = $state<HTMLButtonElement | null>(null);
	let main = $state<HTMLElement | null>(null);

	$effect(() => {
		document.title = title + ' \u00b7 ' + t('appName');
	});
	// The drawer only exists below the breakpoint; growing past it must not leave it flagged open.
	$effect(() => {
		if (desktop.matches) drawerOpen = false;
	});
	$effect(() => {
		const current = status.data;
		if (current) untrack(() => moved.reconcile(current, status.answered));
	});
	// #main is the scroller, not the document: a new page starts at its top.
	$effect(() => {
		void route.kind;
		void route.name;
		if (main) main.scrollTop = 0;
	});
</script>

<div
	class="relative flex h-dvh flex-col overflow-hidden md:grid md:grid-rows-1 {collapsed
		? 'md:grid-cols-[64px_minmax(0,1fr)]'
		: 'md:grid-cols-[224px_minmax(0,1fr)]'}"
	use:hashLinks
>
	<a
		href="#main"
		class="sr-only rounded-md bg-accent px-3 py-2 text-on-accent focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50"
		onclick={(event) => {
			event.preventDefault();
			main?.focus();
		}}
	>
		{t('skipToContent')}
	</a>
	{#if desktop.matches}
		<aside
			id={SIDEBAR_ID}
			class="flex min-h-0 flex-col overflow-y-auto border-r border-line bg-surface"
		>
			<div
				class="flex items-center {collapsed
					? 'flex-col gap-2 px-2 py-3'
					: 'justify-between py-4 pr-2 pl-4'}"
			>
				<Brand compact={collapsed} />
				<IconButton
					label={t(collapsed ? 'expandSidebar' : 'collapseSidebar')}
					expanded={!collapsed}
					controls={SIDEBAR_ID}
					onclick={() => sidebar.toggle()}
				>
					{#if collapsed}
						<IconPanelLeftOpen class="size-4" aria-hidden="true" />
					{:else}
						<IconPanelLeftClose class="size-4" aria-hidden="true" />
					{/if}
				</IconButton>
			</div>
			<Sidebar {collapsed} />
		</aside>
	{:else}
		<header class="flex h-12 shrink-0 items-center gap-2 border-b border-line bg-surface px-2">
			<IconButton
				label={t('openNavigation')}
				expanded={drawerOpen}
				controls={DRAWER_ID}
				bind:element={menuButton}
				onclick={() => (drawerOpen = true)}
			>
				<IconMenu class="size-5" aria-hidden="true" />
			</IconButton>
			<Brand />
			<span class="ml-auto">
				<StatusChip state={runtime} label={t(runtime === 'unknown' ? 'checking' : runtime)} />
			</span>
		</header>
		<MobileDrawer id={DRAWER_ID} bind:open={drawerOpen} opener={menuButton} />
	{/if}
	<main
		id="main"
		bind:this={main}
		tabindex="-1"
		class="relative min-h-0 min-w-0 flex-1 overflow-y-auto px-4 py-(--main-inset) outline-none [--main-inset:--spacing(5)] md:px-8 md:[--main-inset:--spacing(8)]"
	>
		<div class="mx-auto w-full max-w-[1200px]">
			{#if notice}
				<Banner
					kind="warning"
					title={t(
						notice.kind === 'moved'
							? 'movedTo'
							: notice.kind === 'maybe'
								? 'saveUncertain'
								: notice.kind === 'unconfirmed'
									? 'movedUnconfirmed'
									: 'movedUnknown'
					)}
					message={notice.kind === 'maybe'
						? t('movedMaybe')
						: notice.kind === 'unconfirmed'
							? t('recoveryHint')
							: null}
				>
					{#if notice.link}
						{#if notice.kind === 'unconfirmed'}
							<span class="text-fg-muted">{t('movedUnconfirmedAt')}</span>
						{/if}
						<a href={notice.link.href} class="btn btn-secondary font-mono">{notice.link.address}</a>
					{/if}
				</Banner>
			{/if}
			{#if unreachable && (!notice || notice.kind === 'maybe')}
				<Banner kind="error" title={t('unreachable')} message={t('recoveryHint')}>
					<Button variant="secondary" onclick={() => void status.refresh()}>{t('retry')}</Button>
				</Banner>
			{/if}
			{#key route.kind + '\u0000' + (route.name ?? '')}
				<div class="motion-safe:animate-page-enter">
					{#if route.kind === 'overview'}
						<OverviewPage />
					{:else if route.kind === 'new'}
						<RuleEditorPage name={null} />
					{:else if route.kind === 'rule'}
						<RuleEditorPage name={route.name ?? ''} />
					{:else if route.kind === 'stats'}
						<StatsRulesPage />
					{:else if route.kind === 'hosts'}
						<HostsPage />
					{:else if route.kind === 'connections'}
						<ConnectionsPage />
					{:else if route.kind === 'settings'}
						<SettingsPage />
					{:else}
						<YamlPage />
					{/if}
				</div>
			{/key}
		</div>
	</main>
</div>
<TooltipHost />
<ToastHost />
<ConfirmDialog />
