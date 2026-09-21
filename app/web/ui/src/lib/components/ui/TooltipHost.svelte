<script lang="ts">
	import { TOOLTIP_ID, tooltipState } from '../../actions/tooltip.svelte';

	const MARGIN = 8;
	const GAP = 6;

	let element = $state<HTMLElement | null>(null);
	let left = $state(MARGIN);
	let top = $state(MARGIN);
	let shown: HTMLElement | null = null;

	// Centred below the owner, flipped above when there is no room, clamped to the viewport.
	$effect(() => {
		const owner = tooltipState.owner;
		const tip = element;
		if (!owner || !tip || !tooltipState.text) return;
		// The top layer keeps it above a modal drawer; it must be showing before it can be measured.
		if (shown !== tip) {
			try {
				tip.showPopover?.();
				shown = tip;
			} catch (error) {
				if (!(error instanceof DOMException) || error.name !== 'InvalidStateError') throw error;
				// Focus can return while another popover is still changing its top-layer state.
				const frame = requestAnimationFrame(() => {
					if (tip.isConnected && tooltipState.owner === owner) {
						tip.showPopover?.();
						shown = tip;
						position(owner, tip);
					}
				});
				return () => cancelAnimationFrame(frame);
			}
		}
		position(owner, tip);
	});

	function position(owner: HTMLElement, tip: HTMLElement) {
		const rect = owner.getBoundingClientRect();
		const width = tip.offsetWidth;
		const height = tip.offsetHeight;
		const viewportWidth = document.documentElement.clientWidth;
		const viewportHeight = document.documentElement.clientHeight;
		const centred = rect.left + rect.width / 2 - width / 2;
		left = Math.max(MARGIN, Math.min(centred, viewportWidth - width - MARGIN));
		const below = rect.bottom + GAP;
		top = Math.max(
			MARGIN,
			below + height > viewportHeight - MARGIN ? rect.top - GAP - height : below
		);
	}
</script>

{#if tooltipState.visible}
	<div
		bind:this={element}
		role="tooltip"
		id={TOOLTIP_ID}
		popover="manual"
		class="pointer-events-none fixed inset-auto z-50 m-0 max-w-[min(360px,calc(100vw-16px))] overflow-visible rounded-md border-0 bg-fg px-2 py-1 text-xs break-all whitespace-pre-line text-canvas shadow-md"
		style:left="{left}px"
		style:top="{top}px"
	>
		{tooltipState.text}
	</div>
{/if}
