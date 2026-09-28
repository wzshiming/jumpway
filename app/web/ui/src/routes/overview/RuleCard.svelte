<script lang="ts">
	import IconChartColumn from '~icons/lucide/chart-column';
	import IconCopy from '~icons/lucide/copy';
	import IconPencil from '~icons/lucide/pencil';
	import IconTrash from '~icons/lucide/trash-2';
	import { tooltip } from '../../lib/actions/tooltip.svelte';
	import DirectionArrow from '../../lib/components/stats/DirectionArrow.svelte';
	import Sparkline from '../../lib/components/stats/Sparkline.svelte';
	import VirtualPeers from '../../lib/components/rules/VirtualPeers.svelte';
	import HelpTip from '../../lib/components/ui/HelpTip.svelte';
	import IconButton from '../../lib/components/ui/IconButton.svelte';
	import StatusChip, { type ChipState } from '../../lib/components/ui/StatusChip.svelte';
	import { DASH, formatRate, redactCredentials } from '../../lib/format';
	import { t } from '../../lib/i18n.svelte';
	import { duplicateRoute, ruleRoute, statsRoute } from '../../lib/routes';
	import { forwardTarget, isForward, listenAddress } from '../../lib/rule';
	import { cardState } from '../../lib/status.svelte';
	import { DIRECTIONS } from '../../lib/traffic';
	import { trend } from '../../lib/trend.svelte';
	import type { Rule, RuleStatus, Stats } from '../../lib/types';
	import { normalizeWay } from '../../lib/way';

	interface Props {
		rule: Rule;
		// Every configured rule, for virtual peer links; null while unknown.
		rules: readonly Rule[] | null;
		runtime: RuleStatus | null;
		stats: Stats | null;
		// A write or a delete question for this rule is in flight; its switch and Delete wait.
		pending: boolean;
		onToggle: (enabled: boolean) => void;
		onDelete: () => void;
	}

	let { rule, rules, runtime, stats, pending, onToggle, onDelete }: Props = $props();

	const enabled = $derived(!rule.disabled);
	let input = $state<HTMLInputElement | null>(null);
	// A disabled focused control loses focus to body on the next key press; the answer gives it back.
	let refocus = false;
	$effect.pre(() => {
		if (pending) refocus = document.activeElement === input;
	});
	$effect(() => {
		if (pending || !refocus) return;
		refocus = false;
		if (input?.isConnected && document.activeElement === document.body)
			input.focus({ preventScroll: true });
	});
	const chip = $derived.by((): { state: ChipState; label: string } => {
		const state = cardState(rule, runtime);
		if (state === 'disabled') return { state, label: t('disabled') };
		if (state === 'unknown') return { state, label: t('checking') };
		return { state, label: t(state, { attempt: runtime?.attempt ?? 0 }) };
	});
	const address = $derived(runtime?.address ?? listenAddress(rule));
	const target = $derived(runtime?.target ?? forwardTarget(rule));
	const remote = $derived(runtime?.remote ?? normalizeWay(rule.listen.way).length > 0);
	const hops = $derived(normalizeWay(rule.forward.way).length);
	const titleId = $derived('rule-' + encodeURIComponent(rule.name));
</script>

<article
	class="flex flex-col gap-3 rounded-lg border border-line bg-surface p-4"
	aria-labelledby={titleId}
	aria-busy={pending || undefined}
>
	<header class="flex items-start justify-between gap-3">
		<div class="min-w-0">
			<a
				id={titleId}
				href={ruleRoute(rule.name)}
				class="line-clamp-2 text-[15px] font-semibold break-words text-fg hover:text-accent"
			>
				{rule.name}
			</a>
			<p class="mt-0.5 text-xs text-fg-muted">
				{isForward(rule) ? t('portForward') : t('proxy')}{#if remote}
					&nbsp;&middot; {t('remote')}{/if}
				<HelpTip concept={t('mode')} text={t('help.mode')} />
			</p>
		</div>
		<div class="flex shrink-0 items-center gap-2">
			<span class="inline-flex items-center gap-1">
				<StatusChip state={chip.state} label={chip.label} />
				<HelpTip concept={t('state')} text={t('help.state')} />
			</span>
			<!-- preventDefault keeps the box on the stored value until the write is answered. -->
			<input
				bind:this={input}
				type="checkbox"
				role="switch"
				class="switch"
				checked={enabled}
				disabled={pending}
				aria-busy={pending || undefined}
				aria-label={t('enabled')}
				use:tooltip={t('enabled')}
				onclick={(event) => {
					event.preventDefault();
					if (!pending) onToggle(!enabled);
				}}
			/>
		</div>
	</header>
	<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1 text-sm">
		<dt class="flex items-center gap-1 text-fg-muted">
			{t('listen')}
			<HelpTip concept={t('listen')} text={t('help.listen')} />
		</dt>
		<dd class="font-mono text-[13px] [overflow-wrap:anywhere]">
			{address}
			{#if rule.listen.virtual}
				<VirtualPeers side="listen" channel={rule.listen.virtual} {rules} self={rule.name} />
			{/if}
		</dd>
		<dt class="flex items-center gap-1 text-fg-muted">
			{t('target')}
			<HelpTip concept={t('target')} text={t('help.target')} />
		</dt>
		<dd class="font-mono text-[13px] [overflow-wrap:anywhere]">
			{target || DASH}
			{#if rule.forward.virtual}
				<VirtualPeers side="forward" channel={rule.forward.virtual} {rules} self={rule.name} />
			{/if}
		</dd>
		<dt class="flex items-center gap-1 text-fg-muted">
			{t('exitChain')}
			<HelpTip concept={t('exitChain')} text={t('help.exitChain')} />
		</dt>
		<dd>
			{hops === 0 ? t('direct') : hops === 1 ? t('hopCountOne') : t('hopCount', { count: hops })}
		</dd>
	</dl>
	{#if runtime?.error}
		<p class="line-clamp-2 text-xs break-words text-danger">{redactCredentials(runtime.error)}</p>
	{/if}
	<footer
		class="mt-auto flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t border-line pt-3 text-sm"
	>
		<!-- Upload above download in slots as wide as the longest rate, the line beside them: nothing shifts as the numbers change. -->
		<span class="flex items-center gap-2 font-mono text-[13px] text-fg-muted tabular-nums">
			<span class="flex flex-col gap-y-0.5">
				{#each DIRECTIONS as direction (direction.key)}
					<span class="inline-flex items-center gap-1 whitespace-nowrap">
						<DirectionArrow {direction} />
						<span class="sr-only">{t(direction.label)}</span>
						<span class="min-w-[11ch]">{stats ? formatRate(stats[direction.rate]) : DASH}</span>
					</span>
				{/each}
			</span>
			<Sparkline series={trend.of(rule.name)} class="h-9 w-16 shrink-0" />
		</span>
		<!-- Icon actions one row tall, sitting on the card's own edge. -->
		<span class="-my-1.5 ml-auto flex items-center gap-0.5">
			<a
				href={ruleRoute(rule.name)}
				class="icon-btn"
				aria-label={t('edit')}
				use:tooltip={t('edit')}
			>
				<IconPencil class="size-4" aria-hidden="true" />
			</a>
			<a
				href={duplicateRoute(rule.name)}
				class="icon-btn"
				aria-label={t('duplicateRule')}
				use:tooltip={t('duplicateRule')}
			>
				<IconCopy class="size-4" aria-hidden="true" />
			</a>
			<a
				href={statsRoute('stats', rule.name)}
				class="icon-btn"
				aria-label={t('page.stats')}
				use:tooltip={t('page.stats')}
			>
				<IconChartColumn class="size-4" aria-hidden="true" />
			</a>
			<IconButton label={t('deleteRule')} disabled={pending} onclick={onDelete}>
				<IconTrash class="size-4" aria-hidden="true" />
			</IconButton>
		</span>
	</footer>
</article>
