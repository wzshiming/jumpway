import { t, type MessageKey } from './i18n.svelte';
import type { Rule } from './types';

// forward.port 0 or absent means proxy mode: clients pick their own target.
export const isForward = (rule: Rule): boolean =>
	(rule.forward.port ?? 0) > 0 || !!rule.forward.virtual;

export const virtualAddress = (channel: string): string => 'virtual://' + channel;

// Mirrors the backend channel rule: non-empty, no whitespace, no '/'; case-sensitive.
export const isVirtualChannel = (channel: string): boolean => /^[^\s/]+$/.test(channel);

// Mirrors config.Address.String(): an empty host is bound on 127.0.0.1. Text ports come from the editor.
export function hostPort(host: string | undefined, port: number | string | undefined): string {
	const name = host || '127.0.0.1';
	const wrapped = name.includes(':') && !name.startsWith('[') ? '[' + name + ']' : name;
	return wrapped + ':' + (port === undefined || port === '' ? 0 : port);
}

// Exactly config.Address.String() (net.JoinHostPort), for comparing addresses the way Validate
// does: any host with ':' is bracketed, so '[::1]' and '::1' differ. hostPort is for display.
export function joinHostPort(host: string, port: number | string): string {
	const name = host || '127.0.0.1';
	return (name.includes(':') ? '[' + name + ']' : name) + ':' + port;
}

export const forwardTarget = (rule: Rule): string =>
	rule.forward.virtual
		? virtualAddress(rule.forward.virtual)
		: isForward(rule)
			? hostPort(rule.forward.host, rule.forward.port)
			: '';

export const listenAddress = (rule: Rule): string =>
	rule.listen.virtual
		? virtualAddress(rule.listen.virtual)
		: hostPort(rule.listen.host, rule.listen.port);

export type VirtualSide = 'listen' | 'forward';

const counterpart = (side: VirtualSide): VirtualSide => (side === 'listen' ? 'forward' : 'listen');

// Enabled rules on the other end of a channel; self is the SAVED name so a renamed draft never
// sees its stale copy as a peer. Many entries may feed one exit.
export const virtualPeers = (
	rules: readonly Rule[],
	side: VirtualSide,
	channel: string,
	self = ''
): Rule[] =>
	channel
		? rules.filter(
				(rule) =>
					!rule.disabled && rule.name !== self && rule[counterpart(side)].virtual === channel
			)
		: [];

export const virtualChannels = (rules: readonly Rule[], side: VirtualSide, self = ''): string[] => [
	...new Set(
		rules.flatMap((rule) => {
			const channel = rule[counterpart(side)].virtual;
			return channel && rule.name !== self ? [channel] : [];
		})
	)
];

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
