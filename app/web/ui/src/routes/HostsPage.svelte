<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { tooltip } from '../lib/actions/tooltip.svelte';
	import { errorMessage } from '../lib/api';
	import Disclosure from '../lib/components/stats/Disclosure.svelte';
	import ListControls from '../lib/components/stats/ListControls.svelte';
	import StatsToolbar from '../lib/components/stats/StatsToolbar.svelte';
	import TrafficDetail from '../lib/components/stats/TrafficDetail.svelte';
	import Banner from '../lib/components/ui/Banner.svelte';
	import Button from '../lib/components/ui/Button.svelte';
	import HelpTip from '../lib/components/ui/HelpTip.svelte';
	import PageHeader from '../lib/components/ui/PageHeader.svelte';
	import SkeletonRows from '../lib/components/ui/SkeletonRows.svelte';
	import { retainFocus } from '../lib/focus.svelte';
	import { formatCount, redactURL } from '../lib/format';
	import {
		DEFAULT_HOST_SORT,
		HOST_SORT_KEYS,
		aggregateHosts,
		filterHosts,
		hostSortLabel,
		sortHosts,
		type EndpointUse,
		type HostSort
	} from '../lib/hosts';
	import { i18n, t } from '../lib/i18n.svelte';
	import { statsRoute } from '../lib/routes';
	import { stats } from '../lib/stats.svelte';

	onMount(() => stats.subscribe());

	const hosts = $derived(aggregateHosts(stats.data?.rules));
	let query = $state('');
	let sort = $state<HostSort>(DEFAULT_HOST_SORT);
	const filtering = $derived(query.trim() !== '');
	const filtered = $derived(filterHosts(hosts, query, i18n.language));
	const shown = $derived(sortHosts(filtered, sort, i18n.language));
	retainFocus('[data-host]', () => shown);
	// Host and endpoint peaks are sums over hop URLs, so they are only an upper bound.
	const peakHint = $derived(t('peakUpperBound'));

	const expanded = new SvelteSet<string>();

	const useLabel = (use: EndpointUse) =>
		[
			use.rule,
			t(use.way === 'listen' ? 'listenThrough' : 'exitChain'),
			t('hop', { number: use.index + 1 })
		].join(' \u00b7 ');

	function toggleRow(host: string) {
		if (expanded.has(host)) expanded.delete(host);
		else expanded.add(host);
	}
</script>

<PageHeader title={t('page.hosts')} description={null} help={t('help.page.hosts')}>
	<StatsToolbar />
</PageHeader>

<section aria-label={t('hosts')} aria-busy={!stats.data && !stats.error}>
	{#if stats.error}
		<Banner kind="error" title={errorMessage(stats.error)} message={null}>
			<Button variant="secondary" onclick={() => void stats.refresh()}>{t('retry')}</Button>
		</Banner>
	{/if}
	<ListControls bind:query bind:sort sortKeys={HOST_SORT_KEYS} labelFor={hostSortLabel}>
		{#snippet trailing()}
			{#if filtering}
				<p class="text-sm text-fg-muted tabular-nums" data-host-count>
					{t('filteredHosts', { count: filtered.length, total: hosts.length })}
				</p>
			{/if}
		{/snippet}
	</ListControls>
	{#if !stats.data}
		{#if !stats.error}
			<SkeletonRows />
		{/if}
	{:else if hosts.length === 0}
		<p class="text-sm text-fg-muted">{t('noHosts')}</p>
	{:else if shown.length === 0}
		<p class="text-sm text-fg-muted">{t('noMatches')}</p>
	{:else}
		<div class="space-y-3">
			{#each shown as host (host.host)}
				<Disclosure
					prefix="host-details"
					name={host.host}
					open={expanded.has(host.host)}
					ontoggle={() => toggleRow(host.host)}
					data-host={host.host}
				>
					{#snippet identity()}
						<p
							class="font-mono text-[15px] font-semibold [overflow-wrap:anywhere] text-fg"
							data-host-name
						>
							{host.host}
						</p>
					{/snippet}
					{#snippet detail()}
						<div class="flex flex-wrap items-center gap-1.5 text-xs" data-usage>
							<span class="text-fg-subtle">{t('usedBy')}</span>
							{#each host.rules as rule (rule)}
								<a
									href={statsRoute('stats', rule)}
									class="relative z-10 max-w-full min-w-0 rounded-md bg-surface-2 px-1.5 py-0.5 [overflow-wrap:anywhere] text-fg hover:text-accent"
								>
									{rule}
								</a>
							{/each}
						</div>
						<dl class="mt-1 flex items-center gap-x-1.5 text-xs">
							<dt class="flex items-center gap-1 text-fg-subtle">
								{t('endpoints')}
								<HelpTip concept={t('endpoints')} text={t('help.endpoints')} />
							</dt>
							<dd class="font-mono text-[13px] tabular-nums" data-endpoints>
								{formatCount(host.endpoints.length)}
							</dd>
						</dl>
					{/snippet}
					{#snippet stats()}
						<TrafficDetail stats={host.stats} {peakHint} latencyHelp={t('help.latencyAggregate')} />
					{/snippet}
					<ul class="divide-y divide-line border-l-2 border-line pl-3">
						{#each host.endpoints as endpoint (endpoint.endpoint)}
							<li class="py-3 first:pt-0 last:pb-0" data-endpoint-row>
								<div class="mb-2 flex flex-wrap items-center gap-x-3 gap-y-1">
									<span
										class="max-w-full min-w-0 font-mono text-[13px] [overflow-wrap:anywhere] text-fg"
										data-endpoint
										use:tooltip={endpoint.urls.map(redactURL).join('\n')}
									>
										{endpoint.endpoint}
									</span>
									{#each endpoint.uses as use (use.rule + '/' + use.way + '/' + use.index)}
										<span
											class="max-w-full min-w-0 rounded-md border border-line px-1.5 py-0.5 text-xs [overflow-wrap:anywhere] text-fg-muted"
											data-use
										>
											{useLabel(use)}
										</span>
									{/each}
								</div>
								{#if host.endpoints.length > 1}
									<TrafficDetail stats={endpoint.stats} {peakHint} compact />
								{/if}
							</li>
						{/each}
					</ul>
				</Disclosure>
			{/each}
		</div>
	{/if}
</section>
