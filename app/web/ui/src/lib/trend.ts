import { list, type Snapshot, type Stats } from './types';

export interface RateSample {
	up: number;
	down: number;
}

// Ring capacity: about a minute at the stats poller's cadence.
export const TREND_SAMPLES = 60;
// A longer silence between accepted snapshots (hidden page, outage) restarts the series.
export const TREND_GAP_MS = 10_000;
// A refresh right after a tick is not a new sample.
export const TREND_MIN_STEP_MS = 500;
// Key of the aggregate series; rule names are never empty.
export const TREND_TOTAL = '';

export interface TrendState {
	readonly since: string | null;
	readonly lastAt: number | null;
	readonly series: ReadonlyMap<string, readonly RateSample[]>;
}

export const EMPTY_TREND: TrendState = { since: null, lastAt: null, series: new Map() };

const finite = (value: number) => (Number.isFinite(value) ? value : 0);

const sample = (stats: Stats): RateSample => ({
	up: finite(stats.rate_up),
	down: finite(stats.rate_down)
});

function append(samples: readonly RateSample[] | undefined, next: RateSample): RateSample[] {
	const all = samples ? [...samples, next] : [next];
	return all.length > TREND_SAMPLES ? all.slice(all.length - TREND_SAMPLES) : all;
}

// Copy-on-write; a new `since` (stats reset) always records, so the old series vanish at once.
export function recordTrend(state: TrendState, snapshot: Snapshot, now: number): TrendState {
	const elapsed = state.lastAt === null ? Infinity : now - state.lastAt;
	const restart = snapshot.since !== state.since || elapsed > TREND_GAP_MS;
	if (!restart && elapsed < TREND_MIN_STEP_MS) return state;
	const previous = restart ? EMPTY_TREND.series : state.series;
	const series = new Map<string, readonly RateSample[]>();
	const total: RateSample = { up: 0, down: 0 };
	for (const rule of list(snapshot.rules)) {
		const current = sample(rule.stats);
		series.set(rule.name, append(previous.get(rule.name), current));
		total.up += current.up;
		total.down += current.down;
	}
	series.set(TREND_TOTAL, append(previous.get(TREND_TOTAL), total));
	return { since: snapshot.since, lastAt: now, series };
}
