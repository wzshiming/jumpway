import type { Action } from 'svelte/action';
import { router } from '../router.svelte';

export interface HashLinksOptions {
	// Runs once router.navigate accepted a click's destination (not on refusal or cancel).
	onNavigated?: () => void;
}

// Delegated: plain left clicks on in-app hash links go through router.navigate.
// A nested instance marks the click handled (defaultPrevented) so an outer one never runs it twice.
export const hashLinks: Action<HTMLElement, HashLinksOptions | undefined> = (node, options) => {
	let current = options;
	const onClick = (event: MouseEvent) => {
		if (event.defaultPrevented || event.button !== 0) return;
		if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
		const anchor = (event.target as Element).closest('a[href^="#/"]');
		if (!(anchor instanceof HTMLAnchorElement) || anchor.target || anchor.hasAttribute('download'))
			return;
		event.preventDefault();
		void router.navigate(anchor.getAttribute('href')!).then((accepted) => {
			if (accepted) current?.onNavigated?.();
		});
	};
	node.addEventListener('click', onClick);
	return {
		update(next) {
			current = next;
		},
		destroy() {
			node.removeEventListener('click', onClick);
		}
	};
};
