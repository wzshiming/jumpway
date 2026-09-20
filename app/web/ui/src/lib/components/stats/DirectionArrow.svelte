<script lang="ts">
	import IconArrowDown from '~icons/lucide/arrow-down';
	import IconArrowUp from '~icons/lucide/arrow-up';
	import type { Direction } from '../../traffic';

	let { direction }: { direction: Direction } = $props();
</script>

{#if direction.key === 'up'}
	<IconArrowUp class="size-3 shrink-0 text-link" aria-hidden="true" />
{:else}
	<IconArrowDown class="size-3 shrink-0 text-accent" aria-hidden="true" />
{/if}
