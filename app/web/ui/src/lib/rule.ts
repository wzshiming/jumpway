import { t, type MessageKey } from './i18n.svelte';
import type { Rule } from './types';

// forward.port 0 or absent means proxy mode: clients pick their own target.
export const isForward = (rule: Rule): boolean => (rule.forward.port ?? 0) > 0;

// Mirrors config.Address.String(): an empty host is bound on 127.0.0.1. Text ports come from the editor.
export function hostPort(host: string | undefined, port: number | string | undefined): string {
	const name = host || '127.0.0.1';
	const wrapped = name.includes(':') && !name.startsWith('[') ? '[' + name + ']' : name;
	return wrapped + ':' + (port === undefined || port === '' ? 0 : port);
}

export const forwardTarget = (rule: Rule): string =>
	isForward(rule) ? hostPort(rule.forward.host, rule.forward.port) : '';

export const listenAddress = (rule: Rule): string => hostPort(rule.listen.host, rule.listen.port);

const SEPARATOR = ' \u00b7 ';

// Hop 1 is the exit/bind end; the last hop is dialed from this machine.
export function chainSummary(hops: number, target = ''): string {
	if (!hops && !target) return t('direct');
	const names = Array.from({ length: hops }, (_, index) => t('hop', { number: hops - index }));
	return [t('chainLocal'), ...names, target || t('chainTarget')].join(' \u2192 ');
}

export interface HopRoles {
	first: MessageKey;
	last: MessageKey;
}

export const LISTEN_ROLES: HopRoles = { first: 'hopBinds', last: 'hopDialed' };
export const EXIT_ROLES: HopRoles = { first: 'hopExit', last: 'hopDialed' };

export function hopTitle(index: number, count: number, roles: HopRoles): string {
	const labels = [t('hop', { number: index + 1 })];
	if (index === 0) labels.push(t(roles.first));
	if (index === count - 1) labels.push(t(roles.last));
	return labels.join(SEPARATOR);
}

// Read-only chain diagrams also name the entry node of a multi-hop exit chain.
export function stageTitle(index: number, count: number, role: 'listen' | 'forward'): string {
	const labels = [t('hop', { number: index + 1 })];
	if (index === 0) labels.push(t(role === 'listen' ? 'hopBinds' : 'hopExit'));
	if (index === count - 1) {
		if (role === 'forward' && index !== 0) labels.push(t('hopEntry'));
		labels.push(t('hopDialed'));
	}
	return labels.join(SEPARATOR);
}
