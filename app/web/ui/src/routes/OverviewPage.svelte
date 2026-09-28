<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import IconPlus from '~icons/lucide/plus';
	import { tooltip } from '../lib/actions/tooltip.svelte';
	import { ApiError, configsApi, errorMessage, isAborted } from '../lib/api';
	import { busy } from '../lib/busy.svelte';
	import DirectionArrow from '../lib/components/stats/DirectionArrow.svelte';
	import Sparkline from '../lib/components/stats/Sparkline.svelte';
	import Banner from '../lib/components/ui/Banner.svelte';
	import Button from '../lib/components/ui/Button.svelte';
	import HelpTip from '../lib/components/ui/HelpTip.svelte';
	import PageHeader from '../lib/components/ui/PageHeader.svelte';
	import Skeleton from '../lib/components/ui/Skeleton.svelte';
	import { TEXT_CLASSES } from '../lib/components/ui/StatusChip.svelte';
	import { confirm } from '../lib/confirm';
	import {
		DASH,
		formatBytes,
		formatCount,
		formatDateTime,
		formatRate,
		formatShortTime
	} from '../lib/format';
	import { sumStats } from '../lib/hosts';
	import { t, type MessageKey } from '../lib/i18n.svelte';
	import { NEW_RULE_ROUTE } from '../lib/routes';
	import { stats } from '../lib/stats.svelte';
	import { countRuleStates, status, type RuleCardState } from '../lib/status.svelte';
	import { toasts } from '../lib/toast.svelte';
	import { DIRECTIONS, type Direction } from '../lib/traffic';
	import { trend } from '../lib/trend.svelte';
	import { list, type Rule } from '../lib/types';
	import RuleCard from './overview/RuleCard.svelte';

	let rules = $state.raw<Rule[] | null>(null);
	let error = $state.raw<unknown>(null);
	let loading = $state(false);
	let controller: AbortController | null = null;
	// Names with a write or a delete question in flight; only their own switch and Delete wait.
	const pending = new SvelteSet<string>();
	// Aborted when the page is left: a re-read still in flight then never becomes a write.
	const scope = new AbortController();

	function load() {
		controller?.abort();
		const own = new AbortController();
		controller = own;
		loading = true;
		error = null;
		configsApi
			.listRules(own.signal)
			.then(
				(value) => {
					if (controller === own) rules = list(value);
				},
				(reason: unknown) => {
					if (controller === own && !isAborted(reason)) error = reason;
				}
			)
			.finally(() => {
				if (controller !== own) return;
				controller = null;
				loading = false;
			});
	}

	onMount(() => {
		load();
		const unsubscribe = stats.subscribe();
		return () => {
			scope.abort();
			controller?.abort();
			controller = null;
			unsubscribe();
		};
	});

	// What the backend now stores replaces the overview's copy; after unmount nothing is touched.
	function adopt(stored: Rule) {
		if (scope.signal.aborted || !rules) return;
		rules = rules.map((rule) => (rule.name === stored.name ? stored : rule));
		void status.refresh();
		// A list read still open was asked before this write: its answer may predate it, so ask again.
		if (controller) load();
	}

	async function setEnabled(name: string, enabled: boolean) {
		if (pending.has(name)) return;
		pending.add(name);
		try {
			// Re-read first so fields hidden from this page are written back as they are now.
			let latest: Rule;
			try {
				latest = await configsApi.getRule(name, scope.signal);
			} catch (reason) {
				if (!scope.signal.aborted && !isAborted(reason)) toasts.error(errorMessage(reason));
				return;
			}
			// A read that outlived the abort still answers after the page is left: too late to write.
			if (scope.signal.aborted) return;
			const { disabled: _, ...rest } = latest;
			const next: Rule = enabled ? rest : { ...rest, disabled: true };
			let applied = true;
			try {
				await busy.run(() => configsApi.updateRule(name, next));
			} catch (reason) {
				if (reason instanceof ApiError && reason.saved) {
					applied = false;
					toasts.warning(errorMessage(reason));
				} else if (reason instanceof ApiError && reason.unreachable) {
					// The answer was lost: the stored rule, if it can be read, is the only truth.
					toasts.warning(t('saveUncertain'));
					if (scope.signal.aborted) return;
					try {
						adopt(await configsApi.getRule(name, scope.signal));
					} catch {
						// Still unreachable; the switch keeps showing the last known state.
					}
					return;
				} else {
					toasts.error(errorMessage(reason));
					return;
				}
			}
			adopt(next);
			if (applied) toasts.success(t('saved'));
		} finally {
			pending.delete(name);
		}
	}

	async function remove(name: string) {
		if (pending.has(name)) return;
		pending.add(name);
		try {
			const ok = await confirm({
				message: t('confirmDeleteRule', { name }),
				confirmLabel: t('deleteRule'),
				danger: true
			});
			// The page may have been left while the question was open: then nothing is deleted.
			if (!ok || scope.signal.aborted) return;
			let applied = true;
			try {
				await busy.run(() => configsApi.deleteRule(name));
			} catch (reason) {
				if (reason instanceof ApiError && reason.saved) {
					applied = false;
					toasts.warning(errorMessage(reason));
				} else if (reason instanceof ApiError && reason.unreachable) {
					toasts.warning(t('saveUncertain'));
					if (!scope.signal.aborted) load();
					return;
				} else {
					toasts.error(errorMessage(reason));
					return;
				}
			}
			if (applied) toasts.success(t('deleted'));
			if (scope.signal.aborted || !rules) return;
			rules = rules.filter((rule) => rule.name !== name);
			void status.refresh();
			load();
		} finally {
			pending.delete(name);
		}
	}

	const runtimeRules = $derived(list(status.data?.rules));
	const counts = $derived(countRuleStates(rules, status.data?.rules));
	// The states other than running, most severe first; zero counts are left out.
	const STATE_SEGMENTS: readonly { state: RuleCardState; label: MessageKey }[] = [
		{ state: 'stopped', label: 'stopped' },
		{ state: 'retrying', label: 'retryingLabel' },
		{ state: 'unknown', label: 'checking' },
		{ state: 'disabled', label: 'disabled' }
	];
	const segments = $derived(
		STATE_SEGMENTS.filter((segment) => counts[segment.state] > 0).map((segment) => ({
			...segment,
			count: counts[segment.state]
		}))
	);
	const totals = $derived(
		stats.data ? sumStats(list(stats.data.rules).map((rule) => rule.stats)) : null
	);
	// The time sits mid-sentence: the words around it are rendered apart so only it carries the tooltip.
	const sinceWords = $derived(t('sinceTime', { time: '\0' }).split('\0'));
	const runtimeOf = (name: string) => runtimeRules.find((rule) => rule.name === name) ?? null;
	const statsOf = (name: string) =>
		list(stats.data?.rules).find((rule) => rule.name === name)?.stats ?? null;
</script>

<PageHeader title={t('page.overview')} description={null} />

{#snippet kpiLabel(label: string, help: string)}
	<p class="flex items-center gap-1 text-xs text-fg-muted">
		{label}
		<HelpTip concept={label} text={help} />
	</p>
{/snippet}

{#snippet stateDot(state: RuleCardState)}
	<span
		class="size-1.5 shrink-0 self-center rounded-full bg-current {TEXT_CLASSES[state]}"
		aria-hidden="true"
	></span>
{/snippet}

{#snippet directional(kpi: string, value: (direction: Direction) => string)}
	<dl class="mt-1 space-y-0.5 font-mono text-[15px] tabular-nums" data-kpi={kpi}>
		{#each DIRECTIONS as direction (direction.key)}
			<div class="flex items-center gap-1.5">
				<dt class="inline-flex">
					<DirectionArrow {direction} />
					<span class="sr-only">{t(direction.label)}</span>
				</dt>
				<!-- As wide as the longest rate ("1023.9 KB/s"), so whatever follows the values holds still. -->
				<dd class="min-w-[11ch]">{value(direction)}</dd>
			</div>
		{/each}
	</dl>
{/snippet}

<!-- @container: four columns once the content beside the sidebar is 42rem wide; else a 2×2 grid. -->
<div class="@container">
	<section
		class="grid grid-cols-2 gap-x-6 gap-y-5 border-b border-line pb-5 @2xl:grid-cols-4"
		aria-label={t('overview')}
	>
		<div>
			{@render kpiLabel(t('rules'), t('help.kpi.rules'))}
			<p class="mt-1 flex flex-wrap items-baseline gap-x-2" data-kpi="rules">
				<span class="font-mono text-2xl leading-none tabular-nums">
					{status.data ? counts.running : DASH}
				</span>
				<span class="inline-flex items-baseline gap-1 text-xs text-fg-muted"
					>{@render stateDot('running')}{t('running')}</span
				>
			</p>
			<!-- A segment carries its trailing separator and never wraps: lines break only between segments. -->
			<p class="mt-1.5 text-xs text-fg-muted" data-kpi="rule-states">
				<span class="whitespace-nowrap"
					>{t('rulesConfigured', {
						total: counts.total
					})}{#if segments.length}&nbsp;&middot;{/if}</span
				>{#each segments as segment, index (segment.state)}
					{' '}<span class="whitespace-nowrap"
						><span class="inline-flex items-baseline gap-1" data-rule-state={segment.state}
							>{@render stateDot(segment.state)}{t('stateCount', {
								count: segment.count,
								state: t(segment.label)
							})}</span
						>{#if index < segments.length - 1}&nbsp;&middot;{/if}</span
					>{/each}
			</p>
		</div>
		<div>
			{@render kpiLabel(t('activeConnections'), t('help.kpi.active'))}
			<p class="mt-1 font-mono text-2xl leading-none tabular-nums" data-kpi="active">
				{totals ? formatCount(totals.active) : DASH}
			</p>
			<p class="mt-1.5 text-xs text-fg-muted" data-kpi="connections-total">
				{#if totals}
					{t('connectionsTotal', { total: formatCount(totals.total) })}&nbsp;&middot;
					{sinceWords[0]}<span use:tooltip={formatDateTime(stats.data?.since)}
						>{formatShortTime(stats.data?.since)}</span
					>{sinceWords[1] ?? ''}
				{:else}
					{DASH}
				{/if}
			</p>
		</div>
		<div>
			{@render kpiLabel(t('currentRate'), t('help.kpi.rate'))}
			<!-- The line needs a tile about 200px wide: from 27rem in two columns, from 55rem in four; hidden in between and on phones. -->
			<div class="flex items-center gap-3">
				{@render directional('rate', (direction) =>
					totals ? formatRate(totals[direction.rate]) : DASH
				)}
				<Sparkline
					series={trend.total}
					class="hidden h-10 w-16 shrink-0 @[27rem]:block @2xl:hidden @[55rem]:block"
				/>
			</div>
		</div>
		<div>
			{@render kpiLabel(t('totalTraffic'), t('help.kpi.total'))}
			{@render directional('total', (direction) =>
				totals ? formatBytes(totals[direction.total]) : DASH
			)}
		</div>
	</section>
</div>

<!-- @container: the grid fills 18rem columns into the width beside the sidebar, not the viewport. -->
<section class="@container pt-5" aria-label={t('rules')} aria-busy={loading}>
	{#if error}
		<Banner kind="error" title={errorMessage(error)} message={null}>
			<Button variant="secondary" onclick={load}>{t('retry')}</Button>
		</Banner>
	{:else}
		{#if !rules}
			<span class="sr-only">{t('loading')}</span>
		{/if}
		<!-- One grid for the skeletons and the cards: what lands does not move. -->
		<div class="grid grid-cols-[repeat(auto-fill,minmax(18rem,1fr))] gap-3">
			{#if rules}
				{#each rules as rule (rule.name)}
					<RuleCard
						{rule}
						{rules}
						runtime={runtimeOf(rule.name)}
						stats={statsOf(rule.name)}
						pending={pending.has(rule.name)}
						onToggle={(enabled) => void setEnabled(rule.name, enabled)}
						onDelete={() => void remove(rule.name)}
					/>
				{/each}
				<a
					href={NEW_RULE_ROUTE}
					class="flex min-h-40 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-line-strong text-sm text-fg-muted transition-colors hover:border-accent hover:text-accent"
				>
					<IconPlus class="size-6" aria-hidden="true" />
					{t('newRule')}
				</a>
			{:else}
				{#each { length: 3 } as _, index (index)}
					<div
						class="flex flex-col gap-3 rounded-lg border border-line bg-surface p-4"
						data-skeleton-card
					>
						<!-- Bars one line (1lh) of the type they stand in for, so a card lands exactly where its placeholder was. -->
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0 flex-1">
								<Skeleton class="h-[1lh] w-2/5 text-[15px]" />
								<Skeleton class="mt-0.5 h-[1lh] w-1/4 text-xs" />
							</div>
							<div class="flex shrink-0 items-center gap-2">
								<Skeleton class="h-5 w-16 rounded-full" />
								<Skeleton class="h-5 w-9 rounded-full" />
							</div>
						</div>
						<div class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1">
							{#each { length: 3 } as _, row (row)}
								<Skeleton class="my-0.5 h-4 w-14" />
								<Skeleton class="my-0.5 h-4 w-3/4" />
							{/each}
						</div>
						<!-- Sized like the stacked rates with their line and the icon actions so the footer wraps where a card's does. -->
						<div
							class="mt-auto flex flex-wrap items-center justify-between gap-x-3 gap-y-1 border-t border-line pt-3"
						>
							<div class="flex items-center gap-2 text-[13px]">
								<div class="flex flex-col gap-y-0.5">
									{#each { length: 2 } as _, line (line)}
										<span class="inline-flex items-center gap-1">
											<Skeleton class="size-3 rounded-sm" />
											<Skeleton class="h-[1lh] w-[11ch] font-mono" />
										</span>
									{/each}
								</div>
								<Skeleton class="h-9 w-16" />
							</div>
							<span class="-my-1.5 ml-auto flex h-8 items-center">
								<Skeleton class="h-4 w-[8.5rem]" />
							</span>
						</div>
					</div>
				{/each}
			{/if}
		</div>
	{/if}
</section>
