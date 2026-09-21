// jsdom implements neither showModal nor close; the open attribute keeps the flows testable there.
export function openModal(dialog: HTMLDialogElement) {
	if (dialog.open) return;
	if (typeof dialog.showModal === 'function') dialog.showModal();
	else dialog.open = true;
}

export function closeModal(dialog: HTMLDialogElement) {
	if (!dialog.open) return;
	if (typeof dialog.close === 'function') {
		dialog.close();
		return;
	}
	dialog.open = false;
	dialog.dispatchEvent(new Event('close'));
}
