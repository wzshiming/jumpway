<script module lang="ts">
	// A hop stage as handed to the optional `hop` snippet.
	export interface HopStage {
		role: 'listen' | 'forward';
		index: number;
		urls: readonly string[];
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import IconArrowRight from '~icons/lucide/arrow-right';
	import IconGlobe from '~icons/lucide/globe';
	import IconLaptop from '~icons/lucide/laptop';
	import IconMoveRight from '~icons/lucide/move-right';
	import IconServer from '~icons/lucide/server';
	import IconUsers from '~icons/lucide/users';
	import { tooltip } from '../../actions/tooltip.svelte';
	import { displayURL, redactURL } from '../../format';
	import { t } from '../../i18n.svelte';
	import { stageTitle } from '../../rule';
	import type { WayHop } from '../../types';

	// Read-only: clients → listen hops (configured order) → this machine → exit hops (dialed first) → target.
	interface Props {
		listenWay: readonly WayHop[];
		forwardWay: readonly WayHop[];
		target: string;
		// Listen address; shown on the clients' side when a remote hop binds it.
		address: string;
		// `flow` boxes the stages in one wrapping row; `timeline` stacks them for detailed content.
		layout?: 'flow' | 'timeline';
		// Replaces the plain URL list of a hop stage, e.g. with live statistics.
		hop?: Snippet<[HopStage]>;
		// Replaces the target address line.
		targetDetail?: Snippet;
		targetTitle?: string;
	}

	let {
		listenWay,
		forwardWay,
		target,
		address,
		layout = 'flow',
		hop,
		targetDetail,
		targetTitle
	}: Props = $props();

	interface Stage {
		key: string;
		title: string;
		detail: string | null;
		urls: readonly string[];
		hop: HopStage | null;
	}

	const stages = $derived.by((): Stage[] => {
		const remote = listenWay.length > 0;
		const listen = listenWay.map((entry, index) => ({
			key: 'listen-' + index,
			title: stageTitle(index, listenWay.length, 'listen'),
			detail: null,
			urls: entry.lb,
			hop: { role: 'listen' as const, index, urls: entry.lb }
		}));
		const exit = forwardWay
			.map((entry, index) => ({
				key: 'forward-' + index,
				title: stageTitle(index, forwardWay.length, 'forward'),
				detail: null,
				urls: entry.lb,
				hop: { role: 'forward' as const, index, urls: entry.lb }
			}))
			.reverse();
		return [
			{ key: 'clients', title: t('clients'), detail: remote ? address : null, urls: [], hop: null },
			...listen,
			{
				key: 'local',
				title: t('thisMachine'),
				detail: remote ? null : address,
				urls: [],
				hop: null
			},
			...(exit.length
				? exit
				: [{ key: 'direct', title: t('directStage'), detail: null, urls: [], hop: null }]),
			{
				key: 'target',
				title: targetTitle ?? t('target'),
				detail: target || t('chainTarget'),
				urls: [],
				hop: null
			}
		];
	});
</script>

{#snippet content(stage: Stage)}
	<p class="text-xs text-fg-muted">{stage.title}</p>
	{#if stage.key === 'target' && targetDetail}
		{@render targetDetail()}
	{:else if stage.detail}
		<p class="font-mono text-[13px] break-all text-fg">{stage.detail}</p>
	{/if}
	{#if stage.hop && hop}
		{@render hop(stage.hop)}
	{:else if stage.urls.length}
		<ul class="space-y-0.5">
			{#each stage.urls as url, position (position)}
				<li class="font-mono text-[13px] break-all text-fg" use:tooltip={redactURL(url)}>
					{displayURL(url)}
				</li>
			{/each}
		</ul>
	{/if}
{/snippet}

{#snippet marker(stage: Stage)}
	{#if stage.key === 'clients'}
		<IconUsers class="size-3.5" aria-hidden="true" />
	{:else if stage.key === 'local'}
		<IconLaptop class="size-3.5" aria-hidden="true" />
	{:else if stage.key === 'direct'}
		<IconMoveRight class="size-3.5" aria-hidden="true" />
	{:else if stage.key === 'target'}
		<IconGlobe class="size-3.5" aria-hidden="true" />
	{:else}
		<IconServer class="size-3.5" aria-hidden="true" />
	{/if}
{/snippet}

{#if layout === 'timeline'}
	<ol class="space-y-4 text-sm" aria-label={t('chain')}>
		{#each stages as stage, index (stage.key)}
			<li class="relative flex gap-3" data-stage={stage.key}>
				<span
					class="mt-px flex size-6 shrink-0 items-center justify-center rounded-full border bg-surface {stage.key ===
					'local'
						? 'border-accent text-accent'
						: 'border-line-strong text-fg-muted'}"
					aria-hidden="true"
				>
					{@render marker(stage)}
				</span>
				{#if index < stages.length - 1}
					<span class="absolute top-7 -bottom-4 left-3 w-px bg-line" aria-hidden="true"></span>
				{/if}
				<div class="min-w-0 flex-1 pt-0.5">
					{@render content(stage)}
				</div>
			</li>
		{/each}
	</ol>
{:else}
	<ol class="flex flex-wrap items-center gap-y-2 text-sm" aria-label={t('chain')}>
		{#each stages as stage, index (stage.key)}
			<li class="flex min-w-0 items-center" data-stage={stage.key}>
				{#if index > 0}
					<IconArrowRight class="mx-1.5 size-4 shrink-0 text-fg-subtle" aria-hidden="true" />
				{/if}
				<div class="min-w-0 rounded-md border border-line bg-surface px-2.5 py-1.5">
					{@render content(stage)}
				</div>
			</li>
		{/each}
	</ol>
{/if}
