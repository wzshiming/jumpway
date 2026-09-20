<script lang="ts">
	import type { Snippet } from 'svelte';
	import { t } from '../../i18n.svelte';

	// Wires label, hint and error text to the control rendered by `children`.
	interface Props {
		id: string;
		label: string;
		hint?: string | null;
		error?: string | null;
		optional?: boolean;
		children: Snippet<[{ describedBy: string | undefined; invalid: boolean }]>;
	}

	let { id, label, hint = null, error = null, optional = false, children }: Props = $props();

	const hintId = $derived(id + '-hint');
	const errorId = $derived(id + '-error');
	const describedBy = $derived(
		[error ? errorId : null, hint ? hintId : null].filter(Boolean).join(' ') || undefined
	);
</script>

<div class="min-w-0">
	<!-- The optional marker sits beside the label so the control's accessible name stays clean. -->
	<div class="field-label flex items-baseline gap-1">
		<label for={id}>{label}</label>
		{#if optional}
			<span class="font-normal text-fg-subtle">{t('optional')}</span>
		{/if}
	</div>
	{@render children({ describedBy, invalid: error !== null })}
	{#if error}
		<p id={errorId} class="mt-1 text-xs text-danger">{error}</p>
	{/if}
	{#if hint}
		<p id={hintId} class="mt-1 text-xs text-fg-muted">{hint}</p>
	{/if}
</div>
