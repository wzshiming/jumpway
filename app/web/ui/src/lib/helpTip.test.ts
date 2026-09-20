import { flushSync, mount, unmount } from 'svelte';
import { afterEach, beforeEach, expect, test } from 'vitest';
import { hideTooltip, tooltipState } from './actions/tooltip.svelte';
import HelpTip from './components/ui/HelpTip.svelte';

// The circled question mark after a concept label: its definition shows on hover, focus and
// tap, and goes away on leave, Escape, blur or a tap elsewhere.

let target: HTMLElement;
let app: ReturnType<typeof mount> | null = null;

const fire = (node: EventTarget, type: string, init: EventInit = {}) =>
	node.dispatchEvent(new Event(type, { bubbles: true, ...init }));
const pointer = (node: EventTarget, type: string) =>
	node.dispatchEvent(new PointerEvent(type, { bubbles: true, pointerType: 'touch' }));
const click = (node: Element) => {
	node.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
	flushSync();
};
// Hiding is deferred by one microtask (see the tooltip action).
const settled = () => Promise.resolve();
const button = () => target.querySelector('button')!;

beforeEach(() => {
	target = document.body.appendChild(document.createElement('div'));
	app = mount(HelpTip, { target, props: { concept: 'peak', text: 'The highest rate.' } });
	flushSync();
});

afterEach(() => {
	if (app) unmount(app);
	app = null;
	hideTooltip();
	document.body.innerHTML = '';
});

test('renders an icon-only button named after the concept, with no visible text and no native title', () => {
	const help = button();
	expect(help.type).toBe('button');
	expect(help.getAttribute('aria-label')).toBe('About peak');
	expect(help.hasAttribute('title')).toBe(false);
	expect(help.textContent?.trim()).toBe('');
	expect(help.querySelector('svg')).not.toBeNull();
	expect(tooltipState.visible).toBe(false);
});

test('hover shows the definition and links it via aria-describedby; leaving hides it', async () => {
	const help = button();
	fire(help, 'pointerenter', { bubbles: false });
	expect(tooltipState).toMatchObject({ visible: true, text: 'The highest rate.', owner: help });
	expect(help.getAttribute('aria-describedby')).toBe('tooltip');
	fire(help, 'pointerleave', { bubbles: false });
	await settled();
	expect(tooltipState.visible).toBe(false);
	expect(help.hasAttribute('aria-describedby')).toBe(false);
});

test('focus shows it, Escape hides it while focus stays, and a click reopens it for the keyboard', async () => {
	const help = button();
	help.focus();
	fire(help, 'focusin');
	expect(tooltipState.owner).toBe(help);
	document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
	expect(tooltipState.visible).toBe(false);
	expect(document.activeElement).toBe(help);
	click(help);
	expect(tooltipState).toMatchObject({ visible: true, owner: help });
	fire(help, 'focusout');
	await settled();
	expect(tooltipState.visible).toBe(false);
});

test('a tap pins the definition: the pointer leaving keeps it, a tap elsewhere or a second tap hides it', async () => {
	const help = button();
	// A touch tap: enter, leave (touch pointers are transient), then click.
	pointer(help, 'pointerenter');
	pointer(help, 'pointerleave');
	await settled();
	expect(tooltipState.visible).toBe(false);
	click(help);
	expect(tooltipState).toMatchObject({ visible: true, owner: help });
	fire(help, 'pointerleave', { bubbles: false });
	await settled();
	expect(tooltipState.visible).toBe(true);

	pointer(document.body, 'pointerdown');
	await settled();
	expect(tooltipState.visible).toBe(false);

	click(help);
	expect(tooltipState.visible).toBe(true);
	pointer(help, 'pointerdown');
	click(help);
	await settled();
	expect(tooltipState.visible).toBe(false);
});

test('a click does not bubble as an action to an enclosing surface', () => {
	if (app) unmount(app);
	app = null;
	let surfaceClicks = 0;
	target.innerHTML = '';
	const surface = target.appendChild(document.createElement('div'));
	surface.addEventListener('click', () => surfaceClicks++);
	app = mount(HelpTip, { target: surface, props: { concept: 'now', text: 'Now.' } });
	flushSync();
	click(surface.querySelector('button')!);
	expect(surfaceClicks).toBe(0);
	expect(tooltipState.text).toBe('Now.');
});
