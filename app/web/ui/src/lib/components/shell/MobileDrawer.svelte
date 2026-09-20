<script lang="ts">
	import IconX from '~icons/lucide/x';
	import { hashLinks } from '../../actions/links';
	import { closeModal, openModal } from '../../dialog';
	import { t } from '../../i18n.svelte';
	import IconButton from '../ui/IconButton.svelte';
	import Brand from './Brand.svelte';
	import Sidebar from './Sidebar.svelte';

	interface Props {
		id: string;
		open: boolean;
		// Receives focus again when the drawer closes.
		opener: HTMLElement | null;
	}

	let { id, open = $bindable(), opener }: Props = $props();
	let dialog: HTMLDialogElement;

	$effect(() => {
		if (open) openModal(dialog);
		else closeModal(dialog);
	});

	const close = () => (open = false);
</script>

<!-- The drawer's own hashLinks owns clicks inside it and closes it only once the router accepted the navigation. -->
<dialog
	bind:this={dialog}
	{id}
	class="m-0 h-dvh max-h-none w-full max-w-none bg-transparent p-0 text-fg"
	aria-label={t('navigation')}
	use:hashLinks={{ onNavigated: close }}
	onclick={(event) => {
		if (event.target === dialog) close();
	}}
	onclose={() => {
		open = false;
		opener?.focus();
	}}
>
	<div class="flex h-full w-[min(280px,85vw)] flex-col overflow-y-auto bg-surface shadow-xl">
		<div class="flex items-center justify-between px-4 py-3">
			<Brand />
			<IconButton label={t('closeNavigation')} onclick={close}>
				<IconX class="size-5" aria-hidden="true" />
			</IconButton>
		</div>
		<Sidebar />
	</div>
</dialog>
