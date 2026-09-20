<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant: 'primary' | 'secondary' | 'danger' | 'ghost';
		type?: 'button' | 'submit';
		id?: string;
		disabled?: boolean;
		onclick?: (event: MouseEvent) => void;
		element?: HTMLButtonElement | null;
		children: Snippet;
	}

	let {
		variant,
		type = 'button',
		id,
		disabled = false,
		onclick,
		element = $bindable(null),
		children
	}: Props = $props();
</script>

<button bind:this={element} {id} {type} class="btn btn-{variant}" {disabled} {onclick}>
	{@render children()}
</button>
