<script lang="ts">
	import type { Snippet } from 'svelte';
	import HelpTip from './HelpTip.svelte';

	interface Props {
		title: string;
		description: string | null;
		// What the page shows and how it counts, behind a help tip beside the title.
		help?: string | null;
		children?: Snippet;
	}

	let { title, description, help = null, children }: Props = $props();
</script>

<header class="mb-6 flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
	<div class="min-w-0">
		<div class="flex items-center gap-1.5">
			<h1 class="text-xl leading-tight font-semibold text-fg">{title}</h1>
			{#if help}
				<HelpTip concept={title} text={help} />
			{/if}
		</div>
		{#if description}
			<p class="mt-1 text-sm text-fg-muted">{description}</p>
		{/if}
	</div>
	{#if children}
		<div class="flex flex-wrap items-center gap-2">
			{@render children()}
		</div>
	{/if}
</header>
