import type { Action } from 'svelte/action';

export interface ShortcutOptions {
	key: string;
	handler: (event: KeyboardEvent) => void;
}

export const shortcut: Action<HTMLElement, ShortcutOptions> = (node, options) => {
	let current = options;
	const onKeydown = (event: KeyboardEvent) => {
		if (!(event.ctrlKey || event.metaKey)) return;
		if (event.key.toLowerCase() !== current.key.toLowerCase() || !node.isConnected) return;
		event.preventDefault();
		current.handler(event);
	};
	document.addEventListener('keydown', onKeydown);
	return {
		update(next) {
			current = next;
		},
		destroy() {
			document.removeEventListener('keydown', onKeydown);
		}
	};
};
