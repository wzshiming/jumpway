import { describe, expect, test } from 'vitest';
import { sparklinePoints, type SparklineGeometry } from './sparkline';
import { TREND_SAMPLES, type RateSample } from './trend';

const sample = (up: number, down: number): RateSample => ({ up, down });

const coords = (points: string) =>
	points.split(' ').map((point) => point.split(',').map(Number) as [number, number]);

// 59 slot steps of exactly 2 px between the pads; 20 px of vertical span.
const geometry: SparklineGeometry = { width: 120, height: 22 };

describe('sparklinePoints', () => {
	test('needs at least two samples', () => {
		expect(sparklinePoints([], geometry)).toBeNull();
		expect(sparklinePoints([sample(5, 5)], geometry)).toBeNull();
		expect(sparklinePoints([sample(0, 0), sample(0, 0)], geometry)).not.toBeNull();
	});

	test('defaults to TREND_SAMPLES slots and a 1 px pad: the newest sample sits at width - pad', () => {
		expect(TREND_SAMPLES).toBe(60);
		const paths = sparklinePoints([sample(0, 0), sample(10, 5)], geometry)!;
		expect(paths.up).toBe('117,21 119,1');
		expect(paths.down).toBe('117,21 119,11');
		expect(coords(paths.up)).toHaveLength(2);
		expect(coords(paths.down)).toHaveLength(2);
		expect(coords(paths.up).at(-1)![0]).toBe(geometry.width - 1);
	});

	test('the largest value of either series sits at y = pad, zero at y = height - pad', () => {
		const paths = sparklinePoints([sample(3, 0), sample(0, 12), sample(6, 6)], geometry)!;
		expect(coords(paths.down).map(([, y]) => y)).toEqual([21, 1, 11]);
		expect(coords(paths.up).map(([, y]) => y)).toEqual([16, 21, 11]);
	});

	test('is right-aligned: earlier samples sit one slot to the left, the left side stays empty', () => {
		const paths = sparklinePoints([sample(1, 1), sample(2, 2), sample(3, 3)], geometry)!;
		const step = (geometry.width - 2) / (TREND_SAMPLES - 1);
		expect(step).toBe(2);
		expect(coords(paths.up).map(([x]) => x)).toEqual([119 - 2 * step, 119 - step, 119]);
		expect(coords(paths.down).map(([x]) => x)).toEqual([115, 117, 119]);
		const fuller = sparklinePoints(
			Array.from({ length: 60 }, (_, index) => sample(index, 0)),
			geometry
		)!;
		expect(coords(fuller.up)).toHaveLength(60);
		expect(coords(fuller.up)[0][0]).toBe(1);
		expect(coords(fuller.up).at(-1)![0]).toBe(119);
	});

	test('both series share one scale: down peaking at half of up sits at mid-height', () => {
		const paths = sparklinePoints([sample(100, 50), sample(0, 0)], geometry)!;
		expect(coords(paths.up)[0][1]).toBe(1);
		expect(coords(paths.down)[0][1]).toBe(geometry.height / 2);
		expect(coords(paths.up)[1][1]).toBe(21);
		expect(coords(paths.down)[1][1]).toBe(21);
	});

	test('all zeros draw both series flat along the bottom', () => {
		const paths = sparklinePoints([sample(0, 0), sample(0, 0), sample(0, 0)], geometry)!;
		expect(paths.up).toBe('115,21 117,21 119,21');
		expect(paths.down).toBe(paths.up);
	});

	test('honours custom slots and pad and only draws the newest `slots` samples', () => {
		const paths = sparklinePoints([sample(1000, 0), sample(0, 0), sample(10, 0), sample(5, 0)], {
			width: 12,
			height: 12,
			slots: 3,
			pad: 1
		})!;
		expect(paths.up).toBe('1,11 6,1 11,6');
		expect(paths.down).toBe('1,11 6,11 11,11');
		const padded = sparklinePoints([sample(0, 0), sample(4, 2)], {
			width: 122,
			height: 22,
			pad: 2
		})!;
		expect(padded.up).toBe('118,20 120,2');
		expect(padded.down).toBe('118,20 120,11');
	});

	test('rounds coordinates to at most two decimals', () => {
		const paths = sparklinePoints([sample(1, 1), sample(3, 2)], { width: 60, height: 20 })!;
		expect(paths.up).toBe('58.02,13 59,1');
		expect(paths.down).toBe('58.02,13 59,7');
		for (const path of [paths.up, paths.down]) {
			for (const point of path.split(' '))
				expect(point).toMatch(/^\d+(\.\d{1,2})?,\d+(\.\d{1,2})?$/);
		}
	});
});
