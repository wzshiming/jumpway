<script lang="ts">
	import { tooltip } from '../../actions/tooltip.svelte';
	import { t } from '../../i18n.svelte';
	import { NAV_ITEMS, type NavItem } from '../../nav';
	import { router } from '../../router.svelte';
	import RuntimeStatus from './RuntimeStatus.svelte';
	import SidebarFooter from './SidebarFooter.svelte';

	// collapsed: the icon rail; every link keeps its name through aria-label and a tooltip.
	// ontoggle: renders the collapse control in the footer; the drawer passes none.
	interface Props {
		collapsed?: boolean;
		ontoggle?: () => void;
	}

	let { collapsed = false, ontoggle }: Props = $props();

	const current = (item: NavItem) => item.kinds.includes(router.route.kind);
</script>

<RuntimeStatus compact={collapsed} />
<nav aria-label={t('navigation')} class="flex-1 px-2 py-2">
	<ul class="space-y-0.5">
		{#each NAV_ITEMS as item (item.hash)}
			<li>
				<a
					href={item.hash}
					aria-current={current(item) ? 'page' : undefined}
					aria-label={collapsed ? t(item.label) : undefined}
					class="flex items-center gap-2.5 rounded-md py-1.5 text-sm text-fg-muted transition-colors hover:bg-surface-2 hover:text-fg aria-[current=page]:bg-accent-soft aria-[current=page]:font-medium aria-[current=page]:text-accent {collapsed
						? 'justify-center px-0'
						: 'px-2.5'}"
					use:tooltip={collapsed ? t(item.label) : null}
				>
					<item.icon class="size-4 shrink-0" aria-hidden="true" />
					{#if !collapsed}
						{t(item.label)}
					{/if}
				</a>
			</li>
		{/each}
	</ul>
</nav>
<SidebarFooter {collapsed} {ontoggle} />
