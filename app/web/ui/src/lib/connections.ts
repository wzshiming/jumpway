import { displayClient, displayURL, text } from './format';
import { i18n, t } from './i18n.svelte';
import { DIRECTIONS, TRAFFIC_METRICS, TRAFFIC_METRIC_LABELS } from './traffic';
import {
	list,
	type Connection,
	type Nullable,
	type PathHop,
	type RuleStats,
	type Stats
} from './types';

export type RuleConnection = Connection & { rule: string };

export type ConnectionSortKey = 'rule' | 'client' | 'target' | 'started' | keyof Stats;
export type SortDirection = 'ascending' | 'descending';

export interface ConnectionSort {
	key: ConnectionSortKey;
	direction: SortDirection;
}

export const DEFAULT_CONNECTION_SORT: ConnectionSort = { key: 'started', direction: 'descending' };

export const MAX_CONNECTION_ROWS = 200;

export function currentConnections(rules: Nullable<RuleStats[]>): RuleConnection[] {
	return list(rules).flatMap((rule) =>
		list(rule.connections).map((connection) => ({ ...connection, rule: rule.name }))
	);
}

export function clientLabel(connection: Pick<Connection, 'client' | 'process'>): string {
	return connection.process?.name || displayClient(connection.client);
}

// Subsequence match: "1809" hits "127.0.0.1:18094".
export function filterConnections(
	connections: readonly RuleConnection[],
	rule: string,
	query: string
): RuleConnection[] {
	const language = i18n.language;
	const needle = text(query).trim().toLocaleLowerCase(language);
	const matches = (value: unknown) => {
		let position = 0;
		const haystack = text(value).toLocaleLowerCase(language);
		for (const character of needle) {
			position = haystack.indexOf(character, position);
			if (position < 0) return false;
			position++;
		}
		return true;
	};
	return connections.filter(
		(connection) =>
			(!rule || connection.rule === rule) &&
			[connection.target, connection.client, connection.rule, connection.process?.name].some(
				matches
			)
	);
}

const STRING_KEYS = new Set<ConnectionSortKey>(['rule', 'client', 'target']);
const TIME_KEYS = new Set<ConnectionSortKey>(['started', 'last_up', 'last_down']);

export function sortConnections(
	connections: readonly RuleConnection[],
	sort: ConnectionSort = DEFAULT_CONNECTION_SORT
): RuleConnection[] {
	const language = i18n.language;
	const direction = sort.direction === 'ascending' ? 1 : -1;
	const value = (connection: RuleConnection): unknown =>
		sort.key === 'client'
			? clientLabel(connection)
			: sort.key in connection
				? connection[sort.key as keyof RuleConnection]
				: connection.stats?.[sort.key as keyof Stats];
	return connections.slice().sort((left, right) => {
		const comparison = STRING_KEYS.has(sort.key)
			? text(value(left)).localeCompare(text(value(right)), language, { numeric: true })
			: TIME_KEYS.has(sort.key)
				? (Date.parse(text(value(left))) || 0) - (Date.parse(text(value(right))) || 0)
				: (Number(value(left)) || 0) - (Number(value(right)) || 0);
		return comparison * direction || left.id - right.id;
	});
}

// The sort options in menu order: download before upload within each metric, as the pairs read.
export const CONNECTION_SORT_KEYS: readonly ConnectionSortKey[] = [
	'started',
	'target',
	'client',
	'rule',
	...TRAFFIC_METRICS.flatMap((metric) => [DIRECTIONS[1][metric], DIRECTIONS[0][metric]])
];

export function connectionSortLabel(key: ConnectionSortKey): string {
	switch (key) {
		case 'started':
			return t('duration');
		case 'target':
			return t('target');
		case 'client':
			return t('client');
		case 'rule':
			return t('ruleLabel');
	}
	for (const direction of DIRECTIONS) {
		for (const metric of TRAFFIC_METRICS) {
			if (direction[metric] === key) {
				return t(direction.label) + ' \u00b7 ' + t(TRAFFIC_METRIC_LABELS[metric]);
			}
		}
	}
	return key;
}

export function formatConnectionPath(connection: {
	path: Nullable<PathHop[]>;
	target?: string;
}): string {
	const path = list(connection.path);
	if (!path.length) return t('direct');
	const hops = path
		.slice()
		.reverse()
		.map((hop) => {
			const label = hop.url ? displayURL(hop.url) : t('hop', { number: hop.index + 1 });
			return label + (hop.dialed === false ? ' (' + t('reused') + ')' : '');
		});
	return [t('chainLocal'), ...hops, connection.target || t('chainTarget')].join(' \u2192 ');
}
