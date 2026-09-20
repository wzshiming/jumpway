import { en, type MessageKey } from './i18n/en';
import { zh } from './i18n/zh';

export type Language = 'en' | 'zh';
export type { MessageKey };

export const LANGUAGES: readonly Language[] = ['en', 'zh'];
export const LANGUAGE_STORAGE_KEY = 'jumpway.lang';

const messages: Record<Language, Record<MessageKey, string>> = { en, zh };

const isLanguage = (value: unknown): value is Language => LANGUAGES.includes(value as Language);

export function resolveLanguage(input: {
	query: string;
	stored: string | null;
	navigatorLanguages: readonly string[];
}): { language: Language; explicit: boolean } {
	const override = new URLSearchParams(input.query).get('lang');
	if (isLanguage(override)) return { language: override, explicit: true };
	if (isLanguage(input.stored)) return { language: input.stored, explicit: false };
	const zhPreferred = input.navigatorLanguages.some((value) => /^zh/i.test(value || ''));
	return { language: zhPreferred ? 'zh' : 'en', explicit: false };
}

function readStored(): string | null {
	try {
		return localStorage.getItem(LANGUAGE_STORAGE_KEY);
	} catch {
		return null;
	}
}

function writeStored(language: Language) {
	try {
		localStorage.setItem(LANGUAGE_STORAGE_KEY, language);
	} catch {
		// Storage may be denied (private mode, disabled cookies); the in-memory choice still applies.
	}
}

let language = $state<Language>('en');
let explicit = $state(false);
let listening = false;

function apply(next: Language) {
	language = next;
	document.documentElement.lang = next === 'zh' ? 'zh-CN' : 'en';
}

// Back/Forward can land on an entry whose ?lang= differs from the current one.
function reconcile() {
	const resolved = resolveLanguage({
		query: location.search,
		stored: readStored(),
		navigatorLanguages: [navigator.language, ...(navigator.languages ?? [])]
	});
	explicit = resolved.explicit;
	apply(resolved.language);
}

export function t(key: MessageKey, values: Record<string, string | number> = {}): string {
	return messages[language][key].replace(/\{(\w+)\}/g, (_, name: string) =>
		String(values[name] ?? '')
	);
}

export const i18n = {
	get language() {
		return language;
	},
	get explicit() {
		return explicit;
	},
	init() {
		if (!listening) {
			window.addEventListener('popstate', reconcile);
			listening = true;
		}
		reconcile();
	},
	setLanguage(next: Language) {
		apply(next);
		writeStored(next);
		const params = new URLSearchParams(location.search);
		if (params.has('lang')) {
			params.set('lang', next);
			history.replaceState(history.state, '', location.pathname + '?' + params + location.hash);
			explicit = true;
		}
	},
	dispose() {
		if (listening) {
			window.removeEventListener('popstate', reconcile);
			listening = false;
		}
		explicit = false;
		apply('en');
	}
};
