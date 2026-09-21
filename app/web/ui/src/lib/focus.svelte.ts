import { hideTooltip, tooltipState } from './actions/tooltip.svelte';

// Keyed DOM moves can drop focus even when the control remains connected.
export function retainFocus(rows: string, track: () => unknown): void {
	let focused: HTMLElement | null = null;
	$effect.pre(() => {
		track();
		const active = document.activeElement;
		focused = active instanceof HTMLElement && active.closest(rows) ? active : null;
	});
	$effect(() => {
		track();
		const control = focused;
		focused = null;
		if (!control?.isConnected || document.activeElement !== document.body) return;
		// Shown afresh, so the tooltip is measured at the control's new place.
		if (tooltipState.owner === control) hideTooltip();
		control.focus({ preventScroll: true });
	});
}
