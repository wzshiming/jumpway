<script lang="ts">
	import type { Snippet } from 'svelte';
	import { tooltip } from '../../actions/tooltip.svelte';

	// label is both the accessible name and the tooltip; pressed/expanded only render when given.
	interface Props {
		label: string;
		onclick: (event: MouseEvent) => void;
		id?: string;
		pressed?: boolean;
		expanded?: boolean;
		controls?: string;
		disabled?: boolean;
		element?: HTMLButtonElement | null;
		children: Snippet;
	}

	let {
		label,
		onclick,
		id,
		pressed,
		expanded,
		controls,
		disabled = false,
		element = $bindable(null),
		children
	}: Props = $props();
</script>

<button
	bind:this={element}
	{id}
	type="button"
	class="icon-btn"
	aria-label={label}
	aria-pressed={pressed}
	aria-expanded={expanded}
	aria-controls={controls}
	{disabled}
	{onclick}
	use:tooltip={label}
>
	{@render children()}
</button>
