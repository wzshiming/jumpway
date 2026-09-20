<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { SvelteMap, SvelteSet } from 'svelte/reactivity';
	import { tooltip } from '../lib/actions/tooltip.svelte';
	import { configsApi, errorMessage, isAborted } from '../lib/api';
	import ChainFlow, { type HopStage } from '../lib/components/rules/ChainFlow.svelte';
	import Disclosure from '../lib/components/stats/Disclosure.svelte';
	import StatsToolbar from '../lib/components/stats/StatsToolbar.svelte';
	import TrafficDetail from '../lib/components/stats/TrafficDetail.svelte';
	import Banner from '../lib/components/ui/Banner.svelte';
	import Button from '../lib/components/ui/Button.svelte';
	import PageHeader from '../lib/components/ui/PageHeader.svelte';
	import StatusChip, { type ChipState } from '../lib/components/ui/StatusChip.svelte';
	import { DASH, displayURL, redactURL } from '../lib/format';
	import { t } from '../lib/i18n.svelte';
	import { router } from '../lib/router.svelte';
	import { statsRoute } from '../lib/routes';
	import { forwardTarget, isForward, listenAddress } from '../lib/rule';
	import { stats } from '../lib/stats.svelte';
	import { ruleState, status } from '../lib/status.svelte';
	import {
		list,
		type Hop,
		type Nullable,
		type Rule,
		type RuleStats,
		type RuleStatus,
		type WayHop,
		type WayNode
	} from '../lib/types';
	import { normalizeWay } from '../lib/way';

	let rules = $state.raw<Rule[] | null>(null);
	let error = $state.raw<unknown>(null);
	let loading = $state(false);
	let controller: AbortController | null = null;

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
			controller?.abort();
			controller = null;
			unsubscribe();
		};
	});

	interface Row {
		name: string;
		rule: Rule | null;
		runtime: RuleStatus | null;
		entry: RuleStats | null;
	}

	const runtimeRules = $derived(list(status.data?.rules));
	const snapshotRules = $derived(list(stats.data?.rules));
	// Configured rules first; a snapshot may still name a rule the config no longer has.
	const rows = $derived.by((): Row[] => {
		const names = rules ? rules.map((rule) => rule.name) : [];
		const seen = new Set(names);
		for (const entry of snapshotRules) {
			if (!seen.has(entry.name)) {
				seen.add(entry.name);
				names.push(entry.name);
			}
		}
		return names.map((name) => ({
			name,
			rule: rules?.find((rule) => rule.name === name) ?? null,
			runtime: runtimeRules.find((rule) => rule.name === name) ?? null,
			entry: snapshotRules.find((entry) => entry.name === name) ?? null
		}));
	});
	const ready = $derived(rules !== null || stats.data !== null);

	function chip(row: Row): { state: ChipState; label: string } {
		if (row.rule?.disabled) return { state: 'disabled', label: t('disabled') };
		if (!row.runtime) return { state: 'unknown', label: t('checking') };
		const state = ruleState(row.runtime);
		return { state, label: t(state, { attempt: row.runtime.attempt ?? 0 }) };
	}

	const addressOf = (row: Row) => row.runtime?.address ?? (row.rule ? listenAddress(row.rule) : '');
	const targetOf = (row: Row) => row.runtime?.target ?? (row.rule ? forwardTarget(row.rule) : '');
	const remoteOf = (row: Row) =>
		row.runtime?.remote ?? (row.rule ? normalizeWay(row.rule.listen.way).length > 0 : false);

	// Hops the server has seen carry the statistics; the configured way is the fallback.
	function wayOf(hops: Nullable<Hop[]>, configured: Nullable<WayNode[]>): WayHop[] {
		const seen = list(hops)
			.slice()
			.sort((left, right) => left.index - right.index);
		return seen.length
			? seen.map((hop) => ({ lb: list(hop.urls).map((entry) => entry.url) }))
			: normalizeWay(configured);
	}

	const hopOf = (entry: RuleStats | null, stage: HopStage): Hop | null =>
		list(entry?.[stage.role]).find((hop) => hop.index === stage.index) ?? null;

	const parentOf = (hop: Hop) =>
		t('viaHop', {
			parent: hop.parent_index < 0 ? t('chainLocal') : t('hop', { number: hop.parent_index + 1 })
		});

	const expanded = new SvelteSet<string>();
	// A Map: a rule named "toString" or "__proto__" must not resolve to an Object.prototype member.
	const toggles = new SvelteMap<string, HTMLButtonElement>();
	// The rule named by ?rule= is expanded and focused once its row exists.
	let focusPending = $state<string | null>(null);
	const route = $derived(router.route);

	$effect(() => {
		const name = route.rule;
		if (!name) return;
		untrack(() => {
			expanded.add(name);
			focusPending = name;
		});
	});
	$effect(() => {
		const name = focusPending;
		const toggle = name === null ? undefined : toggles.get(name);
		if (!toggle) return;
		focusPending = null;
		toggle.focus();
		toggle.scrollIntoView?.({ block: 'center' });
	});

	function toggleRow(name: string) {
		if (expanded.has(name)) expanded.delete(name);
		else expanded.add(name);
	}
</script>

<PageHeader title={t('page.stats')} description={null} help={t('help.page.stats')}>
	<StatsToolbar />
</PageHeader>

<section aria-label={t('stats')} aria-busy={loading}>
	{#if error}
		<Banner kind="error" title={errorMessage(error)} message={null}>
			<Button variant="secondary" onclick={load}>{t('retry')}</Button>
		</Banner>
	{/if}
	{#if stats.error}
		<Banner kind="error" title={errorMessage(stats.error)} message={null}>
			<Button variant="secondary" onclick={() => void stats.refresh()}>{t('retry')}</Button>
		</Banner>
	{/if}
	{#if !ready}
		{#if !error}
			<p class="text-sm text-fg-muted">{t('loading')}</p>
		{/if}
	{:else if rows.length === 0}
		<p class="text-sm text-fg-muted">{t('noRules')}</p>
	{:else}
		<div class="space-y-3">
			{#each rows as row (row.name)}
				{@const state = chip(row)}
				{@const address = addressOf(row)}
				{@const target = targetOf(row)}
				{@const entry = row.entry}
				{#snippet hopDetail(stage: HopStage)}
					{@const hop = hopOf(entry, stage)}
					{#if hop}
						<p class="text-xs text-fg-subtle" data-parent>{parentOf(hop)}</p>
						<div class="mt-2" data-hop-stats>
							<TrafficDetail stats={hop.stats} compact />
						</div>
						<ul class="mt-3 divide-y divide-line border-l-2 border-line pl-3">
							{#each list(hop.urls) as url (url.url)}
								<li class="py-2.5 first:pt-0 last:pb-0" data-url>
									<p
										class="mb-1.5 font-mono text-[13px] [overflow-wrap:anywhere] text-fg"
										data-endpoint
										use:tooltip={redactURL(url.url)}
									>
										{displayURL(url.url)}
									</p>
									<TrafficDetail stats={url.stats} compact />
								</li>
							{/each}
						</ul>
					{:else}
						<ul class="space-y-0.5">
							{#each stage.urls as url, position (position)}
								<li
									class="font-mono text-[13px] [overflow-wrap:anywhere] text-fg"
									data-endpoint
									use:tooltip={redactURL(url)}
								>
									{displayURL(url)}
								</li>
							{/each}
						</ul>
					{/if}
				{/snippet}
				{#snippet targetsDetail()}
					{@const distinct = new Set(list(entry?.targets).map((item) => item.address)).size}
					{@const live = list(entry?.connections).length}
					{#if target}
						<p class="font-mono text-[13px] [overflow-wrap:anywhere] text-fg">{target}</p>
					{/if}
					{#if distinct === 0 && live === 0}
						<p class="text-[13px] text-fg-muted">{t('noTargets')}</p>
					{:else}
						<p class="text-[13px] text-fg">{t('targetCount', { count: distinct })}</p>
						{#if live > 0}
							<a
								href={statsRoute('connections', row.name)}
								class="text-[13px] text-link hover:underline"
							>
								{t('connectionCount', { count: live })}
							</a>
						{/if}
					{/if}
				{/snippet}
				<Disclosure
					prefix="stats-details"
					name={row.name}
					open={expanded.has(row.name)}
					ontoggle={() => toggleRow(row.name)}
					data-rule={row.name}
					bind:toggle={
						() => toggles.get(row.name) ?? null,
						(node) => (node ? toggles.set(row.name, node) : toggles.delete(row.name))
					}
				>
					{#snippet identity()}
						<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
							<a
								href={statsRoute('stats', row.name)}
								class="relative z-10 min-w-0 text-[15px] font-semibold [overflow-wrap:anywhere] text-fg hover:text-accent"
							>
								{row.name}
							</a>
							<StatusChip state={state.state} label={state.label} />
						</div>
						{#if row.rule}
							<p class="mt-0.5 text-xs text-fg-muted" data-mode>
								{isForward(row.rule) ? t('portForward') : t('proxy')}{#if remoteOf(row)}
									&nbsp;&middot; {t('remote')}{/if}
							</p>
						{/if}
					{/snippet}
					{#snippet detail()}
						<p class="font-mono text-[13px] [overflow-wrap:anywhere] text-fg" data-address>
							{address || DASH}
							{#if target}
								<span class="block text-xs text-fg-muted">&rarr; {target}</span>
							{/if}
						</p>
					{/snippet}
					{#snippet stats()}
						<TrafficDetail
							stats={entry?.stats}
							connectionsHref={statsRoute('connections', row.name)}
						/>
					{/snippet}
					<ChainFlow
						layout="timeline"
						listenWay={wayOf(entry?.listen, row.rule?.listen.way)}
						forwardWay={wayOf(entry?.forward, row.rule?.forward.way)}
						{target}
						{address}
						hop={hopDetail}
						targetDetail={targetsDetail}
						targetTitle={t('targets')}
					/>
				</Disclosure>
			{/each}
		</div>
	{/if}
</section>
