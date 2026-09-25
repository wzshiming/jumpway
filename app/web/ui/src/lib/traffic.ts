import { t, type MessageKey } from './i18n.svelte';

export interface Direction {
	key: 'up' | 'down';
	label: 'upload' | 'download';
	// The definition shown on the arrow.
	help: 'help.upload' | 'help.download';
	rate: 'rate_up' | 'rate_down';
	peak: 'peak_rate_up' | 'peak_rate_down';
	total: 'up' | 'down';
	last: 'last_up' | 'last_down';
}

export const DIRECTIONS: readonly Direction[] = [
	{
		key: 'up',
		label: 'upload',
		help: 'help.upload',
		rate: 'rate_up',
		peak: 'peak_rate_up',
		total: 'up',
		last: 'last_up'
	},
	{
		key: 'down',
		label: 'download',
		help: 'help.download',
		rate: 'rate_down',
		peak: 'peak_rate_down',
		total: 'down',
		last: 'last_down'
	}
];

export type TrafficMetric = 'rate' | 'peak' | 'total' | 'last';

export const TRAFFIC_METRICS: readonly TrafficMetric[] = ['rate', 'peak', 'total', 'last'];

// The short caption of each metric, as the detail bands and the sort options show it.
export const TRAFFIC_METRIC_LABELS: Record<TrafficMetric, MessageKey> = {
	rate: 'nowShort',
	peak: 'peak',
	total: 'totalShort',
	last: 'lastShort'
};

// The definition behind each caption's help tip.
export const TRAFFIC_METRIC_HELP: Record<TrafficMetric, MessageKey> = {
	rate: 'help.now',
	peak: 'help.peak',
	total: 'help.total',
	last: 'help.last'
};

export type TrafficKey = Direction[TrafficMetric];

// The eight counters in sort-menu order: download before upload within each metric, as the pairs read.
export const TRAFFIC_SORT_KEYS: readonly TrafficKey[] = TRAFFIC_METRICS.flatMap((metric) => [
	DIRECTIONS[1][metric],
	DIRECTIONS[0][metric]
]);

// "Download · now"; a key that is no traffic counter falls back to itself.
export function trafficSortLabel(key: string): string {
	for (const direction of DIRECTIONS) {
		for (const metric of TRAFFIC_METRICS) {
			if (direction[metric] === key) {
				return t(direction.label) + ' \u00b7 ' + t(TRAFFIC_METRIC_LABELS[metric]);
			}
		}
	}
	return key;
}
