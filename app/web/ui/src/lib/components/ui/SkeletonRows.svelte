<script lang="ts">
	import { t } from '../../i18n.svelte';
	import Skeleton from './Skeleton.svelte';

	// Three collapsed statistics items (header columns, chevron, metric band) standing in for a list.
</script>

<div class="space-y-3">
	<span class="sr-only">{t('loading')}</span>
	{#each { length: 3 } as _, row (row)}
		<div class="@container rounded-lg border border-line bg-surface" data-skeleton-row>
			<div
				class="grid grid-cols-[minmax(0,1fr)_2rem] items-center gap-x-6 gap-y-2 px-4 py-3 @md:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_2rem]"
			>
				<div class="space-y-2">
					<Skeleton class="h-4 w-2/5" />
					<Skeleton class="h-3 w-1/4" />
				</div>
				<div class="col-span-2 space-y-2 @md:col-span-1">
					<Skeleton class="h-4 w-3/5" />
					<Skeleton class="h-3 w-2/5" />
				</div>
				<Skeleton class="col-start-2 row-start-1 size-8 rounded-md @md:col-start-3" />
			</div>
			<div
				class="grid grid-cols-2 gap-x-4 gap-y-2 border-t border-line px-4 py-3 @[29rem]:grid-cols-4"
			>
				{#each { length: 4 } as _, column (column)}
					<div class="space-y-1.5">
						<Skeleton class="h-3 w-10" />
						<Skeleton class="h-4 w-24" />
						<Skeleton class="h-4 w-20" />
					</div>
				{/each}
			</div>
		</div>
	{/each}
</div>
