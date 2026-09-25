<script lang="ts">
	import IconActivity from '~icons/lucide/activity';
	import IconBookOpen from '~icons/lucide/book-open';
	import IconFlame from '~icons/lucide/flame';
	import IconMonitor from '~icons/lucide/monitor';
	import IconMoon from '~icons/lucide/moon';
	import IconPanelLeftClose from '~icons/lucide/panel-left-close';
	import IconPanelLeftOpen from '~icons/lucide/panel-left-open';
	import IconSlidersHorizontal from '~icons/lucide/sliders-horizontal';
	import IconSun from '~icons/lucide/sun';
	import IconGithub from '~icons/simple-icons/github';
	import { tooltip } from '../../actions/tooltip.svelte';
	import { i18n, LANGUAGES, t, type Language } from '../../i18n.svelte';
	import { SIDEBAR_ID } from '../../sidebar.svelte';
	import { theme, type Theme } from '../../theme.svelte';
	import IconButton from '../ui/IconButton.svelte';

	// collapsed: the icon rail; language and theme move into one popover, the links stack.
	// ontoggle: the sidebar's collapse control ends the resources row; the drawer passes none.
	interface Props {
		collapsed?: boolean;
		ontoggle?: () => void;
	}

	let { collapsed = false, ontoggle }: Props = $props();

	const LANGUAGE_LABELS: Record<Language, { short: string; full: 'english' | 'chinese' }> = {
		en: { short: 'EN', full: 'english' },
		zh: { short: '\u4e2d\u6587', full: 'chinese' }
	};

	const THEME_OPTIONS: { value: Theme; label: 'themeSystem' | 'themeLight' | 'themeDark' }[] = [
		{ value: 'system', label: 'themeSystem' },
		{ value: 'light', label: 'themeLight' },
		{ value: 'dark', label: 'themeDark' }
	];

	const RESOURCES = [
		{ href: '/metrics', label: 'prometheus', external: false },
		{ href: '/swaggerui/', label: 'apiDocs', external: false },
		{ href: '/debug/pprof/', label: 'pprof', external: false },
		{ href: 'https://github.com/wzshiming/jumpway', label: 'github', external: true }
	] as const;

	const MENU_ID = 'preferences-menu';
	const MARGIN = 8;

	let menuButton = $state<HTMLButtonElement | null>(null);
	let menu = $state<HTMLElement | null>(null);
	let menuOpen = $state(false);
	let menuLeft = $state(MARGIN);
	let menuTop = $state(MARGIN);

	function positionMenu() {
		if (!menuButton || !menu) return;
		const rect = menuButton.getBoundingClientRect();
		menuLeft = Math.max(
			MARGIN,
			Math.min(
				rect.right + MARGIN,
				document.documentElement.clientWidth - menu.offsetWidth - MARGIN
			)
		);
		menuTop = Math.max(
			MARGIN,
			Math.min(rect.top, document.documentElement.clientHeight - menu.offsetHeight - MARGIN)
		);
	}

	function onMenuToggle(event: ToggleEvent) {
		menuOpen = event.newState === 'open';
		if (menuOpen) positionMenu();
	}

	$effect(() => {
		if (!menuOpen) return;
		window.addEventListener('resize', positionMenu);
		document.addEventListener('scroll', positionMenu, true);
		return () => {
			window.removeEventListener('resize', positionMenu);
			document.removeEventListener('scroll', positionMenu, true);
		};
	});

	// Tabbing away from both the popover and its button closes it, like clicking elsewhere does.
	function onMenuFocusOut(event: FocusEvent) {
		const next = event.relatedTarget;
		if (!(next instanceof Node)) return;
		if (menu?.contains(next) || menuButton?.contains(next)) return;
		if (menu?.matches(':popover-open')) menu.hidePopover();
	}
</script>

{#snippet preferences()}
	<div
		role="group"
		aria-label={t('language')}
		class="inline-flex rounded-md border border-line p-0.5 text-xs font-medium"
	>
		{#each LANGUAGES as language (language)}
			<button
				type="button"
				class="rounded px-2 py-1 text-fg-muted transition-colors hover:text-fg aria-pressed:bg-accent-soft aria-pressed:text-accent"
				aria-pressed={i18n.language === language}
				aria-label={t(LANGUAGE_LABELS[language].full)}
				lang={language === 'zh' ? 'zh-CN' : 'en'}
				onclick={() => i18n.setLanguage(language)}
			>
				{LANGUAGE_LABELS[language].short}
			</button>
		{/each}
	</div>
	<div role="group" aria-label={t('theme')} class="inline-flex gap-0.5">
		{#each THEME_OPTIONS as option (option.value)}
			<IconButton
				label={t(option.label)}
				pressed={theme.value === option.value}
				onclick={() => theme.set(option.value)}
			>
				{#if option.value === 'system'}
					<IconMonitor class="size-4" aria-hidden="true" />
				{:else if option.value === 'light'}
					<IconSun class="size-4" aria-hidden="true" />
				{:else}
					<IconMoon class="size-4" aria-hidden="true" />
				{/if}
			</IconButton>
		{/each}
	</div>
{/snippet}

{#snippet resources()}
	<ul class="flex items-center gap-0.5 {collapsed ? 'flex-col' : ''}" aria-label={t('resources')}>
		{#each RESOURCES as resource (resource.href)}
			<li>
				<a
					href={resource.href}
					class="icon-btn"
					aria-label={t(resource.label)}
					target={resource.external ? '_blank' : undefined}
					rel={resource.external ? 'noopener noreferrer' : undefined}
					use:tooltip={t(resource.label)}
				>
					{#if resource.label === 'prometheus'}
						<IconActivity class="size-4" aria-hidden="true" />
					{:else if resource.label === 'apiDocs'}
						<IconBookOpen class="size-4" aria-hidden="true" />
					{:else if resource.label === 'pprof'}
						<IconFlame class="size-4" aria-hidden="true" />
					{:else}
						<IconGithub class="size-4" aria-hidden="true" />
					{/if}
				</a>
			</li>
		{/each}
	</ul>
{/snippet}

<div
	class="border-t border-line {collapsed
		? 'flex flex-col items-center gap-1 px-2 py-2'
		: 'space-y-3 px-3 py-3'}"
>
	{#if collapsed}
		<button
			bind:this={menuButton}
			type="button"
			class="icon-btn"
			popovertarget={MENU_ID}
			aria-label={t('preferences')}
			aria-expanded={menuOpen}
			aria-controls={MENU_ID}
			use:tooltip={t('preferences')}
		>
			<IconSlidersHorizontal class="size-4" aria-hidden="true" />
		</button>
		<div
			bind:this={menu}
			id={MENU_ID}
			popover="auto"
			class="fixed inset-auto m-0 hidden flex-col items-start gap-3 rounded-md border border-line bg-surface p-3 text-fg shadow-lg [&:popover-open]:flex"
			style:left="{menuLeft}px"
			style:top="{menuTop}px"
			ontoggle={onMenuToggle}
			onfocusout={onMenuFocusOut}
		>
			{@render preferences()}
		</div>
	{:else}
		<div class="flex items-center justify-between gap-2">
			{@render preferences()}
		</div>
	{/if}
	<div
		class="flex {collapsed
			? 'flex-col items-center gap-1'
			: 'items-center justify-between gap-0.5'}"
	>
		{@render resources()}
		{#if ontoggle}
			<IconButton
				label={t(collapsed ? 'expandSidebar' : 'collapseSidebar')}
				expanded={!collapsed}
				controls={SIDEBAR_ID}
				onclick={ontoggle}
			>
				{#if collapsed}
					<IconPanelLeftOpen class="size-4" aria-hidden="true" />
				{:else}
					<IconPanelLeftClose class="size-4" aria-hidden="true" />
				{/if}
			</IconButton>
		{/if}
	</div>
</div>
