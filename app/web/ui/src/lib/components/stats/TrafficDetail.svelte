<script lang="ts">
	import { DASH, formatCount, formatLatency } from '../../format';
	import { t } from '../../i18n.svelte';
	import type { Stats } from '../../types';
	import HelpTip from '../ui/HelpTip.svelte';
	import TrafficMetrics from './TrafficMetrics.svelte';

	// Every counter of one Stats: the eight traffic values, then connections, latency and failures.
	interface Props {
		stats: Stats | null | undefined;
		// Shown on both peak values when they are sums rather than measured peaks.
		peakHint?: string | null;
		// Links the connection counts while any connection is active.
		connectionsHref?: string | null;
		// Tighter spacing and no help tips for the bands nested under hop URLs and host endpoints.
		compact?: boolean;
		// Replaces the plain latency definition where the stats aggregate several endpoints.
		latencyHelp?: string | null;
	}

	let {
		stats,
		peakHint = null,
		connectionsHref = null,
		compact = false,
		latencyHelp = null
	}: Props = $props();
</script>

<!-- @container: the meta row flips to four tracks together with the traffic band above it. -->
<div class="@container grid {compact ? 'gap-y-2' : 'gap-y-3'}" data-stats>
	<TrafficMetrics {stats} {peakHint} {compact} />
	<dl
		class="grid grid-cols-2 {compact
			? 'gap-x-3 gap-y-1.5'
			: 'gap-x-4 gap-y-2'} font-mono text-[13px] tabular-nums @[29rem]:grid-cols-4"
	>
		<div class="min-w-0" data-connections>
			<dt class="mb-0.5 flex flex-wrap items-center gap-x-1 font-sans text-[11px] text-fg-subtle">
				{t('connections')}
				<span class="text-fg-subtle/80">{t('connectionsTotals')}</span>
				{#if !compact}
					<HelpTip concept={t('connections')} text={t('help.connections')} />
				{/if}
			</dt>
			<!-- Pairs break only at the slash, never inside a number. -->
			<dd>
				{#if !stats}
					{DASH}
				{:else if connectionsHref && stats.active > 0}
					<a href={connectionsHref} class="text-link hover:underline">
						<span class="whitespace-nowrap">{formatCount(stats.active)}</span> /
						<span class="whitespace-nowrap">{formatCount(stats.total)}</span>
					</a>
				{:else}
					<span class="whitespace-nowrap">{formatCount(stats.active)}</span> /
					<span class="whitespace-nowrap">{formatCount(stats.total)}</span>
				{/if}
			</dd>
		</div>
		<div class="min-w-0" data-latency>
			<dt class="mb-0.5 flex flex-wrap items-center gap-x-1 font-sans text-[11px] text-fg-subtle">
				{t('latency')}
				<span class="text-fg-subtle/80">{t('latencyTotals')}</span>
				{#if !compact}
					<HelpTip concept={t('latency')} text={latencyHelp ?? t('help.latency')} />
				{/if}
			</dt>
			<dd>
				<span class="whitespace-nowrap">{formatLatency(stats)}</span> /
				<span class="whitespace-nowrap">{formatLatency(stats, 'avg_latency_ms')}</span>
			</dd>
		</div>
		<div class="min-w-0" data-failures>
			<dt class="mb-0.5 flex flex-wrap items-center gap-x-1 font-sans text-[11px] text-fg-subtle">
				{t('dialFailures')}
				<span class="text-fg-subtle/80">{t('dialsTotals')}</span>
				{#if !compact}
					<HelpTip concept={t('dialFailures')} text={t('help.failures')} />
				{/if}
			</dt>
			<dd>
				{#if !stats}
					{DASH}
				{:else}
					<span class="whitespace-nowrap">{formatCount(stats.dial_failures)}</span> /
					<span class="whitespace-nowrap">{formatCount(stats.dials)}</span>
				{/if}
			</dd>
		</div>
	</dl>
</div>
