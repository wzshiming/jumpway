<script lang="ts">
	import { tooltip } from '../../actions/tooltip.svelte';
	import { redactCredentials } from '../../format';
	import { t } from '../../i18n.svelte';
	import { sidebar } from '../../sidebar.svelte';
	import { runtimeState, status } from '../../status.svelte';

	interface Props {
		compact?: boolean;
	}

	let { compact = false }: Props = $props();

	const DOT = {
		running: 'bg-running',
		stopped: 'bg-danger',
		unknown: 'bg-unknown'
	} as const;

	const DETAILS_ID = 'runtime-details';

	const state = $derived(runtimeState());
	const label = $derived(t(state === 'unknown' ? 'checking' : state));
	const address = $derived(status.data?.address ?? '');
	const error = $derived(status.data?.error ? redactCredentials(status.data.error) : '');
	const heading = $derived(`${t('runtime')}: ${label}`);
	const details = $derived([address, error].filter(Boolean).join('\n'));
</script>

{#if compact}
	<div class="flex justify-center border-y border-line px-2 py-2">
		<button
			type="button"
			class="icon-btn"
			data-state={state}
			aria-label={heading}
			aria-describedby={details ? DETAILS_ID : undefined}
			use:tooltip={details ? heading + '\n' + details : heading}
			onclick={() => sidebar.set(false)}
		>
			<span class="size-2.5 rounded-full {DOT[state]}" aria-hidden="true"></span>
		</button>
		{#if details}
			<span id={DETAILS_ID} class="sr-only">{details}</span>
		{/if}
	</div>
{:else}
	<div class="border-y border-line px-4 py-3">
		<p class="text-xs text-fg-subtle">{t('runtime')}</p>
		<p class="mt-1 flex items-center gap-2 text-sm font-medium" data-state={state}>
			<span class="size-2 shrink-0 rounded-full {DOT[state]}" aria-hidden="true"></span>
			{label}
		</p>
		{#if address}
			<p class="mt-1 font-mono text-xs break-all text-fg-muted">{address}</p>
		{/if}
		{#if error}
			<p class="mt-1 text-xs break-words text-danger">{error}</p>
		{/if}
	</div>
{/if}
