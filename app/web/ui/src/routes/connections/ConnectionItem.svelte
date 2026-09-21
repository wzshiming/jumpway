<script lang="ts">
	import IconInfo from '~icons/lucide/info';
	import IconUnplug from '~icons/lucide/unplug';
	import { tooltip } from '../../lib/actions/tooltip.svelte';
	import Disclosure from '../../lib/components/stats/Disclosure.svelte';
	import TrafficMetrics from '../../lib/components/stats/TrafficMetrics.svelte';
	import IconButton from '../../lib/components/ui/IconButton.svelte';
	import { formatConnectionPath, type RuleConnection } from '../../lib/connections';
	import { DASH, formatDuration } from '../../lib/format';
	import { t } from '../../lib/i18n.svelte';
	import { statsRoute } from '../../lib/routes';
	import ConnectionDetails from './ConnectionDetails.svelte';

	// One connection as the same kind of item as a rule or a host: everything the snapshot knows
	// about it stays visible; the expansion adds only the start time and the verbose path.
	interface Props {
		connection: RuleConnection;
		expanded: boolean;
		pending: boolean;
		ontoggle: (id: number) => void;
		ondisconnect: (id: number) => void;
	}

	let { connection, expanded, pending, ontoggle, ondisconnect }: Props = $props();

	const name = $derived(connection.target || DASH);
</script>

<Disclosure
	prefix="connection-details"
	{name}
	key={String(connection.id)}
	open={expanded}
	ontoggle={() => ontoggle(connection.id)}
	data-connection={connection.id}
>
	{#snippet identity()}
		<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
			<span
				class="min-w-0 font-mono text-[15px] font-semibold [overflow-wrap:anywhere] text-fg"
				data-target
			>
				{name}
			</span>
			<a
				href={statsRoute('stats', connection.rule)}
				class="relative z-10 max-w-full min-w-0 rounded-md bg-surface-2 px-1.5 py-0.5 text-xs [overflow-wrap:anywhere] text-fg hover:text-accent"
				data-rule-link
			>
				{connection.rule}
			</a>
			<button
				type="button"
				class="icon-btn relative z-10 size-6"
				aria-label={t('path')}
				use:tooltip={formatConnectionPath(connection)}
			>
				<IconInfo class="size-3.5" aria-hidden="true" />
			</button>
		</div>
	{/snippet}
	{#snippet detail()}
		<span class="inline-flex max-w-full min-w-0 flex-wrap items-baseline gap-x-1.5" data-client>
			<span class="max-w-full min-w-0 font-mono text-[13px] [overflow-wrap:anywhere]">
				{connection.client || DASH}
			</span>
			{#if connection.process}
				<span class="text-xs [overflow-wrap:anywhere] text-fg-muted">
					{connection.process.name} ({connection.process.pid})
				</span>
			{/if}
		</span>
	{/snippet}
	{#snippet summary()}
		<div class="flex items-center justify-between gap-2 @3xl:justify-end">
			<span class="text-[13px] whitespace-nowrap text-fg-muted tabular-nums" data-duration>
				{formatDuration(connection.started)}
			</span>
			<span class="relative z-10">
				<IconButton
					label={t('disconnect')}
					disabled={pending}
					onclick={() => ondisconnect(connection.id)}
				>
					<IconUnplug class="size-4" aria-hidden="true" />
				</IconButton>
			</span>
		</div>
	{/snippet}
	{#snippet stats()}
		<TrafficMetrics stats={connection.stats} />
	{/snippet}
	<ConnectionDetails {connection} />
</Disclosure>
