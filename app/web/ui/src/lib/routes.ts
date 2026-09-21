export type PageKind =
	'overview' | 'new' | 'rule' | 'stats' | 'hosts' | 'connections' | 'settings' | 'yaml';

export interface Route {
	kind: PageKind;
	hash: string;
	name: string | null;
	rule: string;
}

export const HOME_ROUTE = '#/';
export const NEW_RULE_ROUTE = '#/new';

const PAGES: Record<string, PageKind> = {
	'#/': 'overview',
	'#/new': 'new',
	'#/stats': 'stats',
	'#/hosts': 'hosts',
	'#/connections': 'connections',
	'#/settings': 'settings',
	'#/yaml': 'yaml'
};

const home = (): Route => ({ kind: 'overview', hash: HOME_ROUTE, name: null, rule: '' });

// URLSearchParams tolerates bad escapes ("%GG", truncated UTF-8); treat them like a bad route name.
function wellFormed(component: string): boolean {
	try {
		decodeURIComponent(component);
		return true;
	} catch {
		return false;
	}
}

export function routeFor(hash: string): Route {
	const queryIndex = hash.indexOf('?');
	const path = queryIndex < 0 ? hash : hash.slice(0, queryIndex);
	const query = queryIndex < 0 ? '' : hash.slice(queryIndex + 1);
	if (!wellFormed(query)) return home();
	const canonical = path + (query ? '?' + query : '');
	if (Object.hasOwn(PAGES, path)) {
		return {
			kind: PAGES[path],
			hash: canonical,
			name: null,
			rule: new URLSearchParams(query).get('rule') || ''
		};
	}
	const match = /^#\/rules\/([^/]+)$/.exec(path);
	if (match) {
		try {
			return { kind: 'rule', hash: canonical, name: decodeURIComponent(match[1]), rule: '' };
		} catch {
			return home();
		}
	}
	return home();
}

export const ruleRoute = (name: string): string => '#/rules/' + encodeURIComponent(name);

export const statsRoute = (kind: 'stats' | 'connections', rule = ''): string =>
	'#/' + kind + (rule ? '?' + new URLSearchParams({ rule }) : '');
