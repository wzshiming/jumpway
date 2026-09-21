<script lang="ts">
	import IconCheck from '~icons/lucide/check';
	import IconCircleAlert from '~icons/lucide/circle-alert';
	import IconInfo from '~icons/lucide/info';
	import IconTriangleAlert from '~icons/lucide/triangle-alert';
	import IconX from '~icons/lucide/x';
	import { t } from '../../i18n.svelte';
	import { toasts, type ToastKind } from '../../toast.svelte';
	import IconButton from './IconButton.svelte';

	const CLASSES: Record<ToastKind, string> = {
		success: 'border-running/40 text-running',
		info: 'border-line text-fg-muted',
		warning: 'border-warning/40 text-warning',
		error: 'border-danger/40 text-danger'
	};
</script>

<div
	class="pointer-events-none fixed inset-x-3 bottom-3 z-40 flex flex-col items-end gap-2 sm:inset-x-auto sm:right-4 sm:bottom-4"
	aria-label={t('notifications')}
>
	{#each toasts.list as toast (toast.id)}
		<div
			role={toast.kind === 'error' || toast.kind === 'warning' ? 'alert' : 'status'}
			class="pointer-events-auto flex w-full max-w-sm items-start gap-2 rounded-md border bg-surface py-1.5 pr-1.5 pl-3 text-sm shadow-lg {CLASSES[
				toast.kind
			]}"
		>
			{#if toast.kind === 'success'}
				<IconCheck class="mt-2 size-4 shrink-0" aria-hidden="true" />
			{:else if toast.kind === 'warning'}
				<IconTriangleAlert class="mt-2 size-4 shrink-0" aria-hidden="true" />
			{:else if toast.kind === 'error'}
				<IconCircleAlert class="mt-2 size-4 shrink-0" aria-hidden="true" />
			{:else}
				<IconInfo class="mt-2 size-4 shrink-0" aria-hidden="true" />
			{/if}
			<p class="min-w-0 flex-1 py-1.5 break-words text-fg">{toast.message}</p>
			<IconButton label={t('dismiss')} onclick={() => toasts.dismiss(toast.id)}>
				<IconX class="size-4" aria-hidden="true" />
			</IconButton>
		</div>
	{/each}
</div>
