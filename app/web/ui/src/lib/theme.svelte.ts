export type Theme = 'system' | 'light' | 'dark';
export type ResolvedTheme = 'light' | 'dark';

export const THEMES: readonly Theme[] = ['system', 'light', 'dark'];
export const THEME_STORAGE_KEY = 'jumpway.theme';

const DARK_QUERY = '(prefers-color-scheme: dark)';

let value = $state<Theme>('system');
let systemDark = $state(false);

const resolve = (): ResolvedTheme => (value === 'system' ? (systemDark ? 'dark' : 'light') : value);

function apply() {
	document.documentElement.dataset.theme = resolve();
}

function readStored(): Theme {
	try {
		const stored = localStorage.getItem(THEME_STORAGE_KEY);
		return THEMES.includes(stored as Theme) ? (stored as Theme) : 'system';
	} catch {
		return 'system';
	}
}

export const theme = {
	get value() {
		return value;
	},
	get resolved() {
		return resolve();
	},
	set(next: Theme) {
		value = next;
		try {
			localStorage.setItem(THEME_STORAGE_KEY, next);
		} catch {
			// Storage may be denied; the choice still applies for this page.
		}
		apply();
	},
	init(): () => void {
		value = readStored();
		const query = typeof matchMedia === 'function' ? matchMedia(DARK_QUERY) : null;
		systemDark = query?.matches ?? false;
		const onChange = (event: { matches: boolean }) => {
			systemDark = event.matches;
			apply();
		};
		query?.addEventListener('change', onChange);
		apply();
		return () => {
			query?.removeEventListener('change', onChange);
		};
	}
};
