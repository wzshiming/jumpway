<script lang="ts" generics="K extends string">
	import type { Snippet } from 'svelte';
	import IconSortAsc from '~icons/lucide/arrow-up-narrow-wide';
	import IconSortDesc from '~icons/lucide/arrow-down-wide-narrow';
	import IconSearch from '~icons/lucide/search';
	import IconX from '~icons/lucide/x';
	import { t } from '../../i18n.svelte';
	import type { ListSort } from '../../search';
	import IconButton from '../ui/IconButton.svelte';

	// The search box and sort controls of a statistics list; `leading` and `trailing` frame them.
	interface Props {
		query: string;
		sort: ListSort<K>;
		sortKeys: readonly K[];
		labelFor: (key: K) => string;
		leading?: Snippet;
		trailing?: Snippet;
	}

	let {
		query = $bindable(),
		sort = $bindable(),
		sortKeys,
		labelFor,
		leading,
		trailing
	}: Props = $props();

	function flipDirection() {
		sort = {
			key: sort.key,
			direction: sort.direction === 'ascending' ? 'descending' : 'ascending'
		};
	}
</script>

<div class="mb-3 flex flex-wrap items-center gap-2" data-list-controls>
	{@render leading?.()}
	<div class="relative w-64 max-w-full">
		<IconSearch
			class="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-fg-subtle"
			aria-hidden="true"
		/>
		<input
			type="search"
			class="input pr-8 pl-8 [&::-webkit-search-cancel-button]:hidden"
			placeholder={t('search')}
			aria-label={t('search')}
			autocomplete="off"
			bind:value={query}
		/>
		{#if query}
			<span class="absolute top-0 right-0">
				<IconButton label={t('clearSearch')} onclick={() => (query = '')}>
					<IconX class="size-4" aria-hidden="true" />
				</IconButton>
			</span>
		{/if}
	</div>
	<div class="flex items-center gap-1">
		<select
			class="input w-auto max-w-[11rem]"
			aria-label={t('sortBy')}
			value={sort.key}
			onchange={(event) =>
				(sort = { key: event.currentTarget.value as K, direction: sort.direction })}
		>
			{#each sortKeys as key (key)}
				<option value={key}>{labelFor(key)}</option>
			{/each}
		</select>
		<IconButton label={t(sort.direction)} onclick={flipDirection}>
			{#if sort.direction === 'ascending'}
				<IconSortAsc class="size-4" aria-hidden="true" />
			{:else}
				<IconSortDesc class="size-4" aria-hidden="true" />
			{/if}
		</IconButton>
	</div>
	{@render trailing?.()}
</div>
