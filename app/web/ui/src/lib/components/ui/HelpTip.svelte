<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import type { Action } from 'svelte/action';
	import IconHelp from '~icons/lucide/circle-help';
	import {
		hideTooltip,
		releaseTooltip,
		retainTooltipListeners,
		showTooltip,
		tooltipState
	} from '../../actions/tooltip.svelte';
	import { t } from '../../i18n.svelte';

	// A circled question mark whose definition shows on hover/focus and pins on click or tap.
	interface Props {
		concept: string;
		text: string;
	}

	let { concept, text }: Props = $props();
	let node = $state<HTMLButtonElement | null>(null);
	let pinned = $state(false);

	// Whatever hid the tooltip (Escape, scroll, another owner) also unpins it.
	$effect(() => {
		if (tooltipState.owner !== node) pinned = false;
	});
	// A language switch while shown updates the definition in place.
	$effect(() => {
		const current = text;
		untrack(() => {
			if (node && tooltipState.owner === node) showTooltip(node, current);
		});
	});
	$effect(() => {
		if (!pinned) return;
		const outside = (event: PointerEvent) => {
			if (node && !node.contains(event.target as Node)) hideTooltip();
		};
		document.addEventListener('pointerdown', outside, true);
		return () => document.removeEventListener('pointerdown', outside, true);
	});
	onMount(retainTooltipListeners);

	// Real listeners rather than delegated ones: the click must stop before any enclosing surface.
	const events: Action<HTMLButtonElement> = (button) => {
		const show = () => showTooltip(button, text);
		const leave = () => {
			if (!pinned) releaseTooltip(button);
		};
		const release = () => releaseTooltip(button);
		const toggle = (event: MouseEvent) => {
			event.stopPropagation();
			if (pinned) {
				hideTooltip();
				return;
			}
			show();
			pinned = true;
		};
		button.addEventListener('click', toggle);
		button.addEventListener('pointerenter', show);
		button.addEventListener('pointerleave', leave);
		button.addEventListener('focusin', show);
		button.addEventListener('focusout', release);
		return {
			destroy() {
				release();
				button.removeEventListener('click', toggle);
				button.removeEventListener('pointerenter', show);
				button.removeEventListener('pointerleave', leave);
				button.removeEventListener('focusin', show);
				button.removeEventListener('focusout', release);
			}
		};
	};
</script>

<button
	bind:this={node}
	type="button"
	class="relative z-10 -my-1 inline-flex size-5 shrink-0 items-center justify-center rounded-full align-middle text-fg-subtle transition-colors hover:text-fg focus-visible:text-fg"
	aria-label={t('aboutConcept', { concept })}
	data-help
	use:events
>
	<IconHelp class="size-3.5" aria-hidden="true" />
</button>
