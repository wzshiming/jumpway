<script lang="ts">
	import { sparklinePoints } from '../../sparkline';
	import type { RateSample } from '../../trend';

	// The recent rates as two lines in the arrows' colours, upload under download; the caller
	// sizes the box, which stays while there is too little to draw.
	interface Props {
		series: readonly RateSample[];
		class?: string;
	}

	let { series, class: className = '' }: Props = $props();

	const WIDTH = 120;
	const HEIGHT = 24;
	const paths = $derived(sparklinePoints(series, { width: WIDTH, height: HEIGHT }));
</script>

<svg
	viewBox="0 0 {WIDTH} {HEIGHT}"
	preserveAspectRatio="none"
	class="block overflow-visible {className}"
	aria-hidden="true"
	data-sparkline
	data-samples={series.length}
>
	<!-- The baseline marks the chart's width: a young series fills in from the right. -->
	<line
		x1="0"
		y1={HEIGHT - 1}
		x2={WIDTH}
		y2={HEIGHT - 1}
		class="text-line-strong"
		stroke="currentColor"
		stroke-width="1"
		vector-effect="non-scaling-stroke"
	/>
	{#if paths}
		<g class="text-link">
			<polyline
				points={paths.up}
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				stroke-linejoin="round"
				stroke-linecap="round"
				vector-effect="non-scaling-stroke"
			/>
		</g>
		<g class="text-accent">
			<polyline
				points={paths.down}
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				stroke-linejoin="round"
				stroke-linecap="round"
				vector-effect="non-scaling-stroke"
			/>
		</g>
	{/if}
</svg>
