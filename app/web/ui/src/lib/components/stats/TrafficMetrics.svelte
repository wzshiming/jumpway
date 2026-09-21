<script lang="ts">
	import { tooltip } from '../../actions/tooltip.svelte';
	import { DASH, formatAgo, formatBytes, formatDateTime, formatRate } from '../../format';
	import { t } from '../../i18n.svelte';
	import {
		DIRECTIONS,
		TRAFFIC_METRICS,
		TRAFFIC_METRIC_HELP,
		TRAFFIC_METRIC_LABELS,
		type Direction,
		type TrafficMetric
	} from '../../traffic';
	import type { Stats } from '../../types';
	import HelpTip from '../ui/HelpTip.svelte';
	import DirectionArrow from './DirectionArrow.svelte';

	// The eight traffic counters as a matrix: a column per metric, upload above download, equal tracks.
	interface Props {
		stats: Stats | null | undefined;
		// Shown on both peak values when they are sums rather than measured peaks.
		peakHint?: string | null;
		// Tighter spacing and no help tips for the bands nested under hop URLs and host endpoints.
		compact?: boolean;
	}

	let { stats, peakHint = null, compact = false }: Props = $props();

	const labelOf = (metric: TrafficMetric) =>
		t(metric === 'peak' && peakHint ? 'peakSum' : TRAFFIC_METRIC_LABELS[metric]);
	const helpOf = (metric: TrafficMetric) =>
		t(metric === 'peak' && peakHint ? 'help.peakSum' : TRAFFIC_METRIC_HELP[metric]);

	function valueOf(direction: Direction, metric: TrafficMetric): string {
		if (!stats) return DASH;
		switch (metric) {
			case 'rate':
				return formatRate(stats[direction.rate]);
			case 'peak':
				return formatRate(stats[direction.peak]);
			case 'total':
				return formatBytes(stats[direction.total]);
			case 'last':
				return formatAgo(stats[direction.last]);
		}
	}

	function hintOf(direction: Direction, metric: TrafficMetric): string | null {
		if (metric === 'peak') return peakHint;
		if (metric === 'last')
			return stats?.[direction.last] ? formatDateTime(stats[direction.last]) : null;
		return null;
	}
</script>

<!-- @container: four columns once the band is 29rem wide (an 11-character rate plus its arrow each). -->
<div class="@container" data-traffic role="group" aria-label={t('traffic')}>
	<dl
		class="grid grid-cols-2 {compact
			? 'gap-x-3 gap-y-1.5'
			: 'gap-x-4 gap-y-2'} font-mono text-[13px] tabular-nums @[29rem]:grid-cols-4"
	>
		{#each TRAFFIC_METRICS as metric (metric)}
			<div class="min-w-0" data-column={metric}>
				<dt class="mb-0.5 flex items-center gap-1 font-sans text-[11px] text-fg-subtle">
					{labelOf(metric)}
					{#if !compact}
						<HelpTip concept={labelOf(metric)} text={helpOf(metric)} />
					{/if}
				</dt>
				{#each DIRECTIONS as direction (direction.key)}
					<dd
						class="flex items-center gap-1 whitespace-nowrap {metric === 'last'
							? 'text-fg-muted'
							: ''}"
					>
						<span class="inline-flex" use:tooltip={t(direction.help)}>
							<DirectionArrow {direction} />
							<span class="sr-only">{t(direction.label)}</span>
						</span>
						<span data-metric={direction[metric]} use:tooltip={hintOf(direction, metric)}>
							{valueOf(direction, metric)}
						</span>
					</dd>
				{/each}
			</div>
		{/each}
	</dl>
</div>
