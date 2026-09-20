<script module lang="ts">
	export type ChipState = 'running' | 'retrying' | 'stopped' | 'unknown' | 'disabled';

	const CLASSES: Record<ChipState, string> = {
		running: 'bg-running-soft text-running',
		retrying: 'bg-warning-soft text-warning',
		stopped: 'bg-danger-soft text-danger',
		unknown: 'bg-surface-2 text-fg-muted',
		disabled: 'bg-surface-2 text-fg-subtle'
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
