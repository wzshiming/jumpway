<script module lang="ts">
	import type { RuleCardState } from '../../status.svelte';

	export type ChipState = RuleCardState;

	// The text colour per state; the KPI strip's dots reuse it so they match the chips.
	export const TEXT_CLASSES: Record<ChipState, string> = {
		running: 'text-running',
		retrying: 'text-warning',
		stopped: 'text-danger',
		unknown: 'text-fg-muted',
		disabled: 'text-fg-subtle'
	};

	const CLASSES: Record<ChipState, string> = {
		running: 'bg-running-soft ' + TEXT_CLASSES.running,
		retrying: 'bg-warning-soft ' + TEXT_CLASSES.retrying,
		stopped: 'bg-danger-soft ' + TEXT_CLASSES.stopped,
		unknown: 'bg-surface-2 ' + TEXT_CLASSES.unknown,
		disabled: 'bg-surface-2 ' + TEXT_CLASSES.disabled
	};
</script>

<script lang="ts">
	interface Props {
		state: ChipState;
		label: string;
	}

	let { state, label }: Props = $props();
</script>

<span
	class="inline-flex max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs font-medium whitespace-nowrap {CLASSES[
		state
	]}"
	data-state={state}
>
	<span class="size-1.5 shrink-0 rounded-full bg-current" aria-hidden="true"></span>
	<span class="truncate">{label}</span>
</span>
