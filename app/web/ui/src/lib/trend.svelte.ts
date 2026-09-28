import { EMPTY_TREND, TREND_TOTAL, recordTrend, type RateSample, type TrendState } from './trend';
import type { Snapshot } from './types';

const EMPTY_SAMPLES: readonly RateSample[] = Object.freeze([]);

let state = $state.raw<TrendState>(EMPTY_TREND);

export const trend = {
	record(snapshot: Snapshot, now = Date.now()) {
		state = recordTrend(state, snapshot, now);
	},
	of(name: string): readonly RateSample[] {
		return state.series.get(name) ?? EMPTY_SAMPLES;
	},
	get total(): readonly RateSample[] {
		return state.series.get(TREND_TOTAL) ?? EMPTY_SAMPLES;
	},
	reset() {
		state = EMPTY_TREND;
	}
};
