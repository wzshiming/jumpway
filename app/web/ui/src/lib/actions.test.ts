import { afterEach, describe, expect, test, vi } from 'vitest';
import { hashLinks } from './actions/links';
import { shortcut } from './actions/shortcut';
import { TOOLTIP_ID, hideTooltip, tooltip, tooltipState } from './actions/tooltip.svelte';
import { router } from './router.svelte';

const fire = (target: EventTarget, type: string, init: EventInit = {}) =>
	target.dispatchEvent(new Event(type, { bubbles: true, ...init }));
const key = (init: KeyboardEventInit) => {
	const event = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init });
	document.dispatchEvent(event);
	return event;
};

const mountedActions: { destroy?: () => void }[] = [];
// Hiding is deferred by one microtask (see the tooltip action).
const hidden = () => Promise.resolve();

afterEach(() => {
	mountedActions.splice(0).forEach((action) => action.destroy?.());
	hideTooltip();
	document.body.innerHTML = '';
	vi.restoreAllMocks();
});

describe('tooltip action', () => {
	function mount(text: string) {
		const button = document.body.appendChild(document.createElement('button'));
		const action = tooltip(button, text)!;
		mountedActions.push(action);
		return { button, action };
	}

	test('shows immediately on pointerenter or focus and links the host via aria-describedby', async () => {
		const { button } = mount('socks5://u:p@h:1080');
		expect(tooltipState.visible).toBe(false);
		fire(button, 'pointerenter', { bubbles: false });
		expect(tooltipState).toMatchObject({
			visible: true,
			text: 'socks5://u:p@h:1080',
			owner: button
		});
		expect(button.getAttribute('aria-describedby')).toBe(TOOLTIP_ID);
		fire(button, 'pointerleave', { bubbles: false });
		await hidden();
		expect(tooltipState.visible).toBe(false);
		expect(button.hasAttribute('aria-describedby')).toBe(false);
		fire(button, 'focusin');
		expect(tooltipState.owner).toBe(button);
		fire(button, 'focusout');
		await hidden();
		expect(tooltipState.owner).toBeNull();
	});

	test('hides on Escape, scroll and resize', () => {
		const { button } = mount('tip');
		for (const hide of [
			() => key({ key: 'Escape' }),
			() => fire(document.body, 'scroll', { bubbles: false }),
			() => fire(window, 'resize', { bubbles: false })
		]) {
			fire(button, 'pointerenter', { bubbles: false });
			expect(tooltipState.visible).toBe(true);
			hide();
			expect(tooltipState.visible).toBe(false);
		}
	});

	test('a hovered host can change its text, and blank text hides it', () => {
		const { button, action } = mount('old');
		fire(button, 'pointerenter', { bubbles: false });
		action.update!('new');
		expect(tooltipState.text).toBe('new');
		action.update!('');
		expect(tooltipState.visible).toBe(false);
		fire(button, 'pointerenter', { bubbles: false });
		expect(tooltipState.visible).toBe(false);
	});

	test('moving to another host transfers ownership', async () => {
		const first = mount('one');
		const second = mount('two');
		fire(first.button, 'pointerenter', { bubbles: false });
		fire(second.button, 'pointerenter', { bubbles: false });
		expect(tooltipState.owner).toBe(second.button);
		expect(first.button.hasAttribute('aria-describedby')).toBe(false);
		expect(second.button.getAttribute('aria-describedby')).toBe(TOOLTIP_ID);
		fire(first.button, 'pointerleave', { bubbles: false });
		await hidden();
		expect(tooltipState.owner).toBe(second.button);
		// Leaving the old owner and entering the new one in the same task never hides the new one.
		fire(second.button, 'pointerleave', { bubbles: false });
		fire(first.button, 'pointerenter', { bubbles: false });
		await hidden();
		expect(tooltipState.owner).toBe(first.button);
	});

	test('blur and refocus of the same host in one task keeps it shown; a plain blur still hides', async () => {
		const { button } = mount('again');
		fire(button, 'focusin');
		fire(button, 'focusout');
		fire(button, 'focusin');
		await hidden();
		expect(tooltipState).toMatchObject({ visible: true, owner: button, text: 'again' });
		expect(button.getAttribute('aria-describedby')).toBe(TOOLTIP_ID);
		fire(button, 'focusout');
		await hidden();
		expect(tooltipState.visible).toBe(false);
		expect(button.hasAttribute('aria-describedby')).toBe(false);
	});

	test('focusing an empty host does not cancel the previous tooltip hide', async () => {
		const first = mount('one');
		const empty = mount('');
		fire(first.button, 'focusin');
		fire(first.button, 'focusout');
		fire(empty.button, 'focusin');
		await hidden();
		expect(tooltipState.visible).toBe(false);
		expect(first.button.hasAttribute('aria-describedby')).toBe(false);
		expect(empty.button.hasAttribute('aria-describedby')).toBe(false);
	});

	test('A → B → A in one task leaves the tooltip on A; the deferred hides of A and B are stale', async () => {
		const a = mount('a');
		const b = mount('b');
		fire(a.button, 'focusin');
		fire(a.button, 'focusout');
		fire(b.button, 'focusin');
		fire(b.button, 'focusout');
		fire(a.button, 'focusin');
		await hidden();
		expect(tooltipState).toMatchObject({ visible: true, owner: a.button, text: 'a' });
		expect(a.button.getAttribute('aria-describedby')).toBe(TOOLTIP_ID);
		expect(b.button.hasAttribute('aria-describedby')).toBe(false);
		fire(a.button, 'focusout');
		await hidden();
		expect(tooltipState.visible).toBe(false);
	});

	test('destroying the owner (row removed while hovered) hides it and releases the global listeners', async () => {
		const removeWindow = vi.spyOn(window, 'removeEventListener');
		const { button, action } = mount('gone');
		fire(button, 'pointerenter', { bubbles: false });
		mountedActions.length = 0;
		action.destroy!();
		await hidden();
		expect(tooltipState.visible).toBe(false);
		fire(button, 'pointerenter', { bubbles: false });
		expect(tooltipState.visible).toBe(false);
		expect(removeWindow).toHaveBeenCalledWith('resize', expect.any(Function));
	});

	test('adds its id next to existing aria-describedby tokens and removes only its own on hide', async () => {
		const { button } = mount('tip');
		button.setAttribute('aria-describedby', 'hint');
		fire(button, 'pointerenter', { bubbles: false });
		expect(button.getAttribute('aria-describedby')).toBe('hint ' + TOOLTIP_ID);
		fire(button, 'pointerleave', { bubbles: false });
		await hidden();
		expect(button.getAttribute('aria-describedby')).toBe('hint');
	});

	test('tokens added while shown survive an update and the hide', async () => {
		const { button, action } = mount('old');
		fire(button, 'pointerenter', { bubbles: false });
		button.setAttribute('aria-describedby', `${TOOLTIP_ID} error-1`);
		action.update!('new');
		expect(button.getAttribute('aria-describedby')).toBe(`${TOOLTIP_ID} error-1`);
		fire(button, 'pointerleave', { bubbles: false });
		await hidden();
		expect(button.getAttribute('aria-describedby')).toBe('error-1');
	});

	test('a removed owner and a superseded owner keep their unrelated tokens', async () => {
		const first = mount('one');
		const second = mount('two');
		first.button.setAttribute('aria-describedby', 'a');
		second.button.setAttribute('aria-describedby', 'b');
		fire(first.button, 'pointerenter', { bubbles: false });
		fire(second.button, 'pointerenter', { bubbles: false });
		expect(first.button.getAttribute('aria-describedby')).toBe('a');
		expect(second.button.getAttribute('aria-describedby')).toBe('b ' + TOOLTIP_ID);
		mountedActions.splice(mountedActions.indexOf(second.action), 1);
		second.action.destroy!();
		await hidden();
		expect(second.button.getAttribute('aria-describedby')).toBe('b');
		expect(tooltipState.visible).toBe(false);
	});
});

describe('shortcut action', () => {
	test('Ctrl/Cmd+key runs the handler once, prevents the default and stops after destroy', () => {
		const handler = vi.fn();
		const node = document.body.appendChild(document.createElement('form'));
		const action = shortcut(node, { key: 's', handler })!;
		expect(key({ key: 's', ctrlKey: true }).defaultPrevented).toBe(true);
		expect(key({ key: 'S', metaKey: true }).defaultPrevented).toBe(true);
		expect(key({ key: 's' }).defaultPrevented).toBe(false);
		expect(key({ key: 'k', ctrlKey: true }).defaultPrevented).toBe(false);
		expect(handler).toHaveBeenCalledTimes(2);

		const replacement = vi.fn();
		action.update!({ key: 's', handler: replacement });
		key({ key: 's', ctrlKey: true });
		expect(replacement).toHaveBeenCalledTimes(1);
		expect(handler).toHaveBeenCalledTimes(2);

		action.destroy!();
		expect(key({ key: 's', ctrlKey: true }).defaultPrevented).toBe(false);
		expect(replacement).toHaveBeenCalledTimes(1);
	});
});

describe('hashLinks action', () => {
	test('plain left clicks on in-app hash links go through router.navigate; everything else is left to the browser', () => {
		const navigate = vi.spyOn(router, 'navigate').mockResolvedValue(true);
		const root = document.body.appendChild(document.createElement('div'));
		root.innerHTML = `
			<a id="app" href="#/rules/a%20b"><span id="inner">a b</span></a>
			<a id="blank" href="#/yaml" target="_blank">y</a>
			<a id="download" href="#/yaml" download>d</a>
			<a id="external" href="https://example.invalid/#/yaml">x</a>
			<a id="anchor" href="#top">t</a>
			<button id="button">b</button>`;
		// Reads the action's decision after bubbling, then stops jsdom from navigating.
		const outcomes: boolean[] = [];
		const observe = (event: Event) => {
			outcomes.push(event.defaultPrevented);
			event.preventDefault();
		};
		document.addEventListener('click', observe);
		mountedActions.push({ destroy: () => document.removeEventListener('click', observe) });
		const click = (
			id: string,
			{ handled = false, ...init }: MouseEventInit & { handled?: boolean } = {}
		) => {
			const event = new MouseEvent('click', {
				bubbles: true,
				cancelable: true,
				button: 0,
				...init
			});
			if (handled) event.preventDefault();
			document.getElementById(id)!.dispatchEvent(event);
			return outcomes.at(-1);
		};
		const action = hashLinks(root)!;
		mountedActions.push(action);

		expect(click('inner')).toBe(true);
		expect(navigate).toHaveBeenCalledWith('#/rules/a%20b');
		for (const init of [
			{ ctrlKey: true },
			{ metaKey: true },
			{ shiftKey: true },
			{ altKey: true },
			{ button: 1 },
			{ handled: true }
		]) {
			expect(click('app', init), JSON.stringify(init)).toBe(init.handled === true);
		}
		for (const id of ['blank', 'download', 'external', 'anchor', 'button']) {
			expect(click(id), id).toBe(false);
		}
		expect(navigate).toHaveBeenCalledTimes(1);

		mountedActions.splice(mountedActions.indexOf(action), 1);
		action.destroy!();
		expect(click('inner')).toBe(false);
		expect(navigate).toHaveBeenCalledTimes(1);
	});
});
