<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import IconPlus from '~icons/lucide/plus';
	import { ApiError, configsApi, errorMessage, isAborted } from '../lib/api';
	import { busy } from '../lib/busy.svelte';
	import DirectionArrow from '../lib/components/stats/DirectionArrow.svelte';
	import Banner from '../lib/components/ui/Banner.svelte';
	import Button from '../lib/components/ui/Button.svelte';
	import HelpTip from '../lib/components/ui/HelpTip.svelte';
	import PageHeader from '../lib/components/ui/PageHeader.svelte';
	import { confirm } from '../lib/confirm';
	import { DASH, formatBytes, formatCount, formatRate } from '../lib/format';
	import { sumStats } from '../lib/hosts';
	import { t } from '../lib/i18n.svelte';
	import { NEW_RULE_ROUTE } from '../lib/routes';
	import { stats } from '../lib/stats.svelte';
	import { status } from '../lib/status.svelte';
	import { toasts } from '../lib/toast.svelte';
	import { DIRECTIONS, type Direction } from '../lib/traffic';
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
	const running = $derived(runtimeRules.filter((rule) => rule.running).length);
	const total = $derived(rules?.length ?? runtimeRules.length);
	const totals = $derived(
		stats.data ? sumStats(list(stats.data.rules).map((rule) => rule.stats)) : null
	);
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

{#snippet directional(kpi: string, value: (direction: Direction) => string)}
	<dl class="mt-1 space-y-0.5 font-mono text-[15px] tabular-nums" data-kpi={kpi}>
		{#each DIRECTIONS as direction (direction.key)}
			<div class="flex items-center gap-1.5">
				<dt class="inline-flex">
					<DirectionArrow {direction} />
					<span class="sr-only">{t(direction.label)}</span>
				</dt>
				<dd>{value(direction)}</dd>
			</div>
		{/each}
	</dl>
{/snippet}

<section
	class="grid grid-cols-2 gap-x-6 gap-y-5 border-b border-line pb-5 md:grid-cols-4"
	aria-label={t('overview')}
>
	<div>
		{@render kpiLabel(t('rules'), t('help.kpi.rules'))}
		<p class="mt-1 font-mono text-2xl leading-none tabular-nums" data-kpi="rules">
			{running}<span class="text-fg-subtle">/{total}</span>
		</p>
		<p class="mt-1.5 text-xs text-fg-muted">{t('rulesRunning', { running, total })}</p>
	</div>
	<div>
		{@render kpiLabel(t('activeConnections'), t('help.kpi.active'))}
		<p class="mt-1 font-mono text-2xl leading-none tabular-nums" data-kpi="active">
			{totals ? formatCount(totals.active) : DASH}
		</p>
		<p class="mt-1.5 text-xs text-fg-muted">
			{totals ? t('connectionsShort', { active: totals.active, total: totals.total }) : DASH}
		</p>
	</div>
	<div>
		{@render kpiLabel(t('currentRate'), t('help.kpi.rate'))}
		{@render directional('rate', (direction) =>
			totals ? formatRate(totals[direction.rate]) : DASH
		)}
	</div>
	<div>
		{@render kpiLabel(t('totalTraffic'), t('help.kpi.total'))}
		{@render directional('total', (direction) =>
			totals ? formatBytes(totals[direction.total]) : DASH
		)}
	</div>
</section>

<section class="pt-5" aria-label={t('rules')} aria-busy={loading}>
	{#if error}
		<Banner kind="error" title={errorMessage(error)} message={null}>
			<Button variant="secondary" onclick={load}>{t('retry')}</Button>
		</Banner>
	{:else if rules}
		<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
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
		</div>
	{:else}
		<p class="text-sm text-fg-muted">{t('loading')}</p>
	{/if}
</section>
