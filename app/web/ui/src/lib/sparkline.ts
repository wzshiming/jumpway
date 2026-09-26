import { TREND_SAMPLES, type RateSample } from './trend';

export interface SparklineGeometry {
	width: number;
	height: number;
	// Sample positions across the width; defaults to the trend ring capacity.
	slots?: number;
	pad?: number;
}

// SVG `points` for one polyline per direction.
export interface SparklinePaths {
	up: string;
	down: string;
}

const round = (value: number) => String(Number(value.toFixed(2)));

// Right-aligned: a partly filled ring leaves the left side empty instead of stretching.
export function sparklinePoints(
	samples: readonly RateSample[],
	{ width, height, slots = TREND_SAMPLES, pad = 1 }: SparklineGeometry
): SparklinePaths | null {
	if (samples.length < 2) return null;
	const shown = samples.length > slots ? samples.slice(samples.length - slots) : samples;
	const step = (width - 2 * pad) / (slots - 1);
	const span = height - 2 * pad;
	let max = 1;
	for (const { up, down } of shown) max = Math.max(max, up, down);
	const x = (index: number) => round(width - pad - (shown.length - 1 - index) * step);
	const y = (value: number) => round(pad + span * (1 - value / max));
	const points = (pick: (sample: RateSample) => number) =>
		shown.map((sample, index) => `${x(index)},${y(pick(sample))}`).join(' ');
	return { up: points((sample) => sample.up), down: points((sample) => sample.down) };
}
