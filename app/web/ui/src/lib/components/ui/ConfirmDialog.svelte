<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { confirmService, type ConfirmOptions } from '../../confirm';
	import { closeModal, openModal } from '../../dialog';
	import { t } from '../../i18n.svelte';
	import Button from './Button.svelte';

	let dialog: HTMLDialogElement;
	let cancelButton = $state<HTMLButtonElement | null>(null);
	let confirmButton = $state<HTMLButtonElement | null>(null);
	let options = $state<ConfirmOptions | null>(null);
	let pending: ((value: boolean) => void) | null = null;
	let opener: Element | null = null;

	// Resolves exactly once; Escape, close and unmount all count as "no".
	function settle(value: boolean) {
		const resolve = pending;
		pending = null;
		options = null;
		if (dialog?.open) closeModal(dialog);
		if (opener instanceof HTMLElement) opener.focus();
		opener = null;
		resolve?.(value);
	}

	onMount(() => {
		const unregister = confirmService.register((request) => {
			if (pending) return Promise.resolve(false);
			return new Promise<boolean>((resolve) => {
				pending = resolve;
				opener = document.activeElement;
				options = request;
				void tick().then(() => {
					if (pending !== resolve) return;
					openModal(dialog);
					// A destructive question defaults to the safe answer.
					(request.danger ? cancelButton : confirmButton)?.focus();
				});
			});
		});
		return () => {
			unregister();
			settle(false);
		};
	});
</script>

<dialog
	bind:this={dialog}
	class="m-auto w-[min(420px,calc(100vw-2rem))] rounded-lg border border-line bg-surface p-0 text-fg shadow-xl"
	aria-labelledby={options?.title ? 'confirm-title' : undefined}
	aria-describedby={options ? 'confirm-message' : undefined}
	onclose={() => {
		// dialog.close() delivers its event a task later; if a new question reopened the dialog by then, the event is stale.
		if (pending && !dialog.open) settle(false);
	}}
>
	{#if options}
		<div class="p-5">
			{#if options.title}
				<h2 id="confirm-title" class="text-base font-semibold">{options.title}</h2>
			{/if}
			<p id="confirm-message" class="text-sm text-fg-muted {options.title ? 'mt-1.5' : ''}">
				{options.message}
			</p>
			<div class="mt-5 flex justify-end gap-2">
				<Button variant="secondary" bind:element={cancelButton} onclick={() => settle(false)}>
					{options.cancelLabel ?? t('cancel')}
				</Button>
				<Button
					variant={options.danger ? 'danger' : 'primary'}
					bind:element={confirmButton}
					onclick={() => settle(true)}
				>
					{options.confirmLabel ?? t('confirm')}
				</Button>
			</div>
		</div>
	{/if}
</dialog>
