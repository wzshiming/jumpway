<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAttributes } from 'svelte/elements';
	import IconChevronDown from '~icons/lucide/chevron-down';
	import { tooltip } from '../../actions/tooltip.svelte';
	import { elementId } from '../../dom';
	import { t } from '../../i18n.svelte';

	// One expandable statistics item; links in the header sit above its stretched toggle (`relative z-10`).
	interface Props extends HTMLAttributes<HTMLElement> {
		// Namespaces the details id.
		prefix: string;
		name: string;
		// Identifies the details when several items may share a name.
		key?: string;
		open: boolean;
		ontoggle: () => void;
		// The toggle button, for focusing an item by name.
		toggle?: HTMLButtonElement | null;
		identity: Snippet;
		detail: Snippet;
		// A duration and its action beside the chevron, within a 9rem track.
		summary?: Snippet;
		// The item's own statistics, always visible.
		stats: Snippet;
		children: Snippet;
	}

	let {
		prefix,
		name,
		key = name,
		open,
		ontoggle,
		toggle = $bindable(null),
		identity,
		detail,
		summary,
		stats,
		children,
		...rest
	}: Props = $props();

	const id = $derived(elementId(prefix, key));
	const label = $derived(t(open ? 'collapse' : 'expand') + ': ' + name);
</script>

<!-- Fixed action tracks keep header columns aligned across items. -->
<article
	class="@container rounded-lg border bg-surface {open ? 'border-accent' : 'border-line'}"
	aria-label={name}
	{...rest}
>
	<div
		class="relative grid grid-cols-[minmax(0,1fr)_2rem] items-center gap-x-6 gap-y-2 rounded-t-[7px] px-4 py-3 transition-colors hover:bg-surface-2/40 @md:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_2rem] {summary
			? '@3xl:grid-cols-[minmax(10rem,1.2fr)_minmax(8rem,1fr)_9rem_2rem]'
			: '@3xl:grid-cols-[minmax(10rem,1.2fr)_minmax(8rem,1fr)_2rem]'}"
	>
		<div class="min-w-0" data-header-identity>{@render identity()}</div>
		<div class="col-span-2 min-w-0 @md:col-span-1" data-header-detail>{@render detail()}</div>
		{#if summary}
			<div class="col-span-2 min-w-0 @md:col-span-3 @3xl:col-span-1" data-header-summary>
				{@render summary()}
			</div>
		{/if}
		<button
			type="button"
			class="icon-btn col-start-2 row-start-1 after:absolute after:inset-0 after:content-[''] @md:col-start-3 {summary
				? '@3xl:col-start-4'
				: '@3xl:col-start-3'}"
			aria-label={label}
			aria-expanded={open}
			aria-controls={id}
			data-header-toggle
			bind:this={toggle}
			use:tooltip={label}
			onclick={ontoggle}
		>
			<IconChevronDown
				class="size-4 transition-transform {open ? 'rotate-180' : ''}"
				aria-hidden="true"
			/>
		</button>
	</div>
	<div class="border-t border-line px-4 py-3">{@render stats()}</div>
	{#if open}
		<div {id} class="border-t border-line px-4 py-4">
			{@render children()}
		</div>
	{/if}
</article>
