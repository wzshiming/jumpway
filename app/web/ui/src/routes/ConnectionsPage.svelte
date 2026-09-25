<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { errorMessage } from '../lib/api';
	import ListControls from '../lib/components/stats/ListControls.svelte';
	import StatsToolbar from '../lib/components/stats/StatsToolbar.svelte';
	import Banner from '../lib/components/ui/Banner.svelte';
	import Button from '../lib/components/ui/Button.svelte';
	import PageHeader from '../lib/components/ui/PageHeader.svelte';
	import {
		CONNECTION_SORT_KEYS,
		DEFAULT_CONNECTION_SORT,
		MAX_CONNECTION_ROWS,
		connectionSortLabel,
		currentConnections,
		filterConnections,
		sortConnections,
		type ConnectionSort
	} from '../lib/connections';
	import { retainFocus } from '../lib/focus.svelte';
	import { t } from '../lib/i18n.svelte';
	import { router } from '../lib/router.svelte';
	import { statsRoute } from '../lib/routes';
	import { disconnectConnection, stats } from '../lib/stats.svelte';
	import { toasts } from '../lib/toast.svelte';
	import { list } from '../lib/types';
	import ConnectionItem from './connections/ConnectionItem.svelte';

	onMount(() => stats.subscribe());

	const route = $derived(router.route);
	let rule = $state('');
	let query = $state('');
	let sort = $state<ConnectionSort>(DEFAULT_CONNECTION_SORT);
	// Connection ids whose DELETE is in flight; only their own button is disabled.
	const pending = new SvelteSet<number>();
	// Expanded connection ids; kept across polls.
	const expanded = new SvelteSet<number>();

	// ?rule= owns the selection: it seeds it and follows Back/Forward without remounting.
	$effect(() => {
		const selected = route.rule;
		untrack(() => {
			if (selected !== rule) rule = selected;
		});
	});

	function selectRule(value: string) {
		rule = value;
		void router.navigate(statsRoute('connections', value), { replace: true });
	}

	const all = $derived(currentConnections(stats.data?.rules));
	const filtered = $derived(filterConnections(all, rule, query));
	const sorted = $derived(sortConnections(filtered, sort));
	const shown = $derived(sorted.slice(0, MAX_CONNECTION_ROWS));
	const filtering = $derived(rule !== '' || query.trim() !== '');

	retainFocus('[data-connection]', () => shown);
	// A rule named by the query stays selectable even when the snapshot does not know it.
	const ruleNames = $derived.by(() => {
		const names = list(stats.data?.rules).map((entry) => entry.name);
		if (rule && !names.includes(rule)) names.push(rule);
		return names;
	});

	function toggleRow(id: number) {
		if (expanded.has(id)) expanded.delete(id);
		else expanded.add(id);
	}

	async function disconnect(id: number) {
		if (pending.has(id)) return;
		pending.add(id);
		try {
			await disconnectConnection(id);
			toasts.success(t('disconnected'));
		} catch (reason) {
			toasts.error(errorMessage(reason));
		} finally {
			pending.delete(id);
		}
	}
</script>

<PageHeader title={t('page.connections')} description={null} help={t('help.page.connections')}>
	<StatsToolbar />
</PageHeader>

<section aria-label={t('connections')}>
	{#if stats.error}
		<Banner kind="error" title={errorMessage(stats.error)} message={null}>
			<Button variant="secondary" onclick={() => void stats.refresh()}>{t('retry')}</Button>
		</Banner>
	{/if}
	<ListControls bind:query bind:sort sortKeys={CONNECTION_SORT_KEYS} labelFor={connectionSortLabel}>
		{#snippet leading()}
			<select
				id="connections-rule"
				class="input w-auto max-w-[14rem]"
				aria-label={t('ruleLabel')}
				bind:value={rule}
				onchange={(event) => selectRule(event.currentTarget.value)}
			>
				<option value="">{t('allRules')}</option>
				{#each ruleNames as name (name)}
					<option value={name}>{name}</option>
				{/each}
			</select>
		{/snippet}
		{#snippet trailing()}
			<p class="text-sm text-fg-muted tabular-nums" data-connection-count>
				{filtering
					? t('filteredConnections', { count: filtered.length, total: all.length })
					: t('connectionCount', { count: all.length })}
			</p>
		{/snippet}
	</ListControls>
	{#if !stats.data}
		{#if !stats.error}
			<p class="text-sm text-fg-muted">{t('loading')}</p>
		{/if}
	{:else if shown.length === 0}
		<p class="text-sm text-fg-muted">{t('noConnections')}</p>
	{:else}
		<div class="space-y-3">
			{#each shown as connection (connection.id)}
				<ConnectionItem
					{connection}
					expanded={expanded.has(connection.id)}
					pending={pending.has(connection.id)}
					ontoggle={toggleRow}
					ondisconnect={disconnect}
				/>
			{/each}
		</div>
	{/if}
	{#if sorted.length > shown.length}
		<p class="mt-3 text-sm text-fg-muted">
			{t('showingConnections', { shown: shown.length, total: sorted.length })}
		</p>
	{/if}
</section>
