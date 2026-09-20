<script lang="ts">
	import type { Snippet } from 'svelte';
	import IconCircleAlert from '~icons/lucide/circle-alert';
	import IconInfo from '~icons/lucide/info';
	import IconTriangleAlert from '~icons/lucide/triangle-alert';

	interface Props {
		kind: 'error' | 'warning' | 'info';
		title: string;
		message: string | null;
		children?: Snippet;
	}

	let { kind, title, message, children }: Props = $props();

	const CLASSES = {
		error: 'border-danger/40 bg-danger-soft text-fg',
		warning: 'border-warning/40 bg-warning-soft text-fg',
		info: 'border-line bg-surface-2 text-fg'
	} as const;
	const ICON_CLASSES = {
		error: 'text-danger',
		warning: 'text-warning',
		info: 'text-fg-muted'
	} as const;
</script>

<div
	role={kind === 'info' ? 'status' : 'alert'}
	class="mb-6 flex flex-wrap items-start gap-3 rounded-md border px-3 py-2.5 text-sm {CLASSES[
		kind
	]}"
>
	{#if kind === 'error'}
		<IconCircleAlert class="mt-0.5 size-4 shrink-0 {ICON_CLASSES[kind]}" aria-hidden="true" />
	{:else if kind === 'warning'}
		<IconTriangleAlert class="mt-0.5 size-4 shrink-0 {ICON_CLASSES[kind]}" aria-hidden="true" />
	{:else}
		<IconInfo class="mt-0.5 size-4 shrink-0 {ICON_CLASSES[kind]}" aria-hidden="true" />
	{/if}
	<div class="min-w-0 flex-1 basis-48">
		<p class="font-medium">{title}</p>
		{#if message}
			<p class="mt-0.5 text-fg-muted">{message}</p>
		{/if}
	</div>
	{#if children}
		<div class="flex items-center gap-2">
			{@render children()}
		</div>
	{/if}
</div>
