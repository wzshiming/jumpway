import type { Action } from 'svelte/action';

export const TOOLTIP_ID = 'tooltip';

let text = $state('');
let owner = $state.raw<HTMLElement | null>(null);

// The single TooltipHost renders `text` anchored to `owner` under id TOOLTIP_ID.
export const tooltipState = {
	get text() {
		return text;
	},
	get owner() {
		return owner;
	},
	get visible() {
		return owner !== null && text !== '';
	}
};

const tokensOf = (node: HTMLElement) =>
	(node.getAttribute('aria-describedby') ?? '').split(/\s+/).filter(Boolean);

function describe(node: HTMLElement) {
	const tokens = tokensOf(node);
	if (!tokens.includes(TOOLTIP_ID)) tokens.push(TOOLTIP_ID);
	node.setAttribute('aria-describedby', tokens.join(' '));
}

function undescribe(node: HTMLElement) {
	const tokens = tokensOf(node).filter((token) => token !== TOOLTIP_ID);
	if (tokens.length > 0) node.setAttribute('aria-describedby', tokens.join(' '));
	else node.removeAttribute('aria-describedby');
}

export function hideTooltip() {
	if (owner) undescribe(owner);
	owner = null;
	text = '';
}

// Makes `node` the owner showing `value`; blank text hides the tooltip it owned.
function show(node: HTMLElement, value: string) {
	if (!value) {
		if (owner === node) hideTooltip();
		return;
	}
	if (owner && owner !== node) undescribe(owner);
	owner = node;
	text = value;
	describe(node);
}

// An activation: a pending deferred hide from before it no longer applies.
export function showTooltip(node: HTMLElement, value: string) {
	if (value) activation++;
	show(node, value);
}

// Hides the tooltip `node` owns, one microtask later: Chrome can fire focusout during Svelte
// teardown, when state writes are unsafe, and a re-show in between keeps it.
export function releaseTooltip(node: HTMLElement) {
	if (owner !== node) return;
	const token = activation;
	queueMicrotask(() => {
		if (owner === node && activation === token) hideTooltip();
	});
}

const onKeydown = (event: KeyboardEvent) => {
	if (event.key === 'Escape') hideTooltip();
};

let hosts = 0;

function attachGlobalListeners() {
	document.addEventListener('scroll', hideTooltip, { capture: true, passive: true });
	document.addEventListener('keydown', onKeydown);
	window.addEventListener('resize', hideTooltip);
}

function detachGlobalListeners() {
	document.removeEventListener('scroll', hideTooltip, { capture: true });
	document.removeEventListener('keydown', onKeydown);
	window.removeEventListener('resize', hideTooltip);
}

// Bumped on every activation so a deferred hide can tell whether the host was re-shown meanwhile.
let activation = 0;

// The global listeners live while any tooltip host or help tip is mounted.
export function retainTooltipListeners(): () => void {
	if (hosts++ === 0) attachGlobalListeners();
	return () => {
		if (--hosts === 0) detachGlobalListeners();
	};
}

export const tooltip: Action<HTMLElement, string | null | undefined> = (node, value) => {
	let current = value ?? '';
	const enter = () => showTooltip(node, current);
	const leave = () => releaseTooltip(node);
	node.addEventListener('pointerenter', enter);
	node.addEventListener('focusin', enter);
	node.addEventListener('pointerleave', leave);
	node.addEventListener('focusout', leave);
	const release = retainTooltipListeners();
	return {
		update(next) {
			current = next ?? '';
			if (owner === node) show(node, current);
		},
		destroy() {
			leave();
			node.removeEventListener('pointerenter', enter);
			node.removeEventListener('focusin', enter);
			node.removeEventListener('pointerleave', leave);
			node.removeEventListener('focusout', leave);
			release();
		}
	};
};
