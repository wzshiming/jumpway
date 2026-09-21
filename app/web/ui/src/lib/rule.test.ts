import { describe, expect, test } from 'vitest';
import {
	chainSummary,
	EXIT_ROLES,
	forwardTarget,
	hopTitle,
	hostPort,
	isForward,
	LISTEN_ROLES,
	listenAddress,
	stageTitle
} from './rule';
import type { Rule } from './types';

const rule = (forward: Rule['forward'], listen: Partial<Rule['listen']> = {}): Rule => ({
	name: 'r',
	listen: { host: '127.0.0.1', port: 1080, ...listen },
	forward
});

describe('rule helpers', () => {
	test('a forward port above zero means port-forward mode; zero or absent means proxy mode', () => {
		expect(isForward(rule({ host: '10.0.0.5', port: 5432 }))).toBe(true);
		expect(isForward(rule({ host: '10.0.0.5', port: 0 }))).toBe(false);
		expect(isForward(rule({}))).toBe(false);
	});

	test('hostPort brackets bare IPv6 hosts and keeps everything else verbatim', () => {
		expect(hostPort('127.0.0.1', 1080)).toBe('127.0.0.1:1080');
		expect(hostPort('::1', 1080)).toBe('[::1]:1080');
		expect(hostPort('[::1]', 1080)).toBe('[::1]:1080');
	});

	// config.Address.String(): an empty host means 127.0.0.1.
	test('an empty or missing host is the loopback address, as the backend binds it', () => {
		expect(hostPort('', 18099)).toBe('127.0.0.1:18099');
		expect(hostPort(undefined, 5432)).toBe('127.0.0.1:5432');
		expect(hostPort('', 0)).toBe('127.0.0.1:0');
		expect(hostPort(undefined, undefined)).toBe('127.0.0.1:0');
		expect(listenAddress(rule({}, { host: '', port: 18099 }))).toBe('127.0.0.1:18099');
		expect(forwardTarget(rule({ port: 5432 }))).toBe('127.0.0.1:5432');
		expect(forwardTarget(rule({ host: '', port: 5432 }))).toBe('127.0.0.1:5432');
	});

	test('forwardTarget is empty for proxy rules and host:port for forward rules', () => {
		expect(forwardTarget(rule({}))).toBe('');
		expect(forwardTarget(rule({ host: '', port: 0 }))).toBe('');
		expect(forwardTarget(rule({ host: 'fd00::5', port: 5432 }))).toBe('[fd00::5]:5432');
	});

	test('listenAddress joins the configured listen host and port', () => {
		expect(listenAddress(rule({}, { host: '0.0.0.0', port: 18099 }))).toBe('0.0.0.0:18099');
	});
});

// Hop 1 is the exit end; the last hop is dialed from this machine (legacy chainSummary/hopStage).
describe('chain labels', () => {
	test('chainSummary walks from this machine through the hops in reverse to the target', () => {
		expect(chainSummary(0)).toBe('direct');
		expect(chainSummary(0, '10.0.0.5:5432')).toBe('this machine \u2192 10.0.0.5:5432');
		expect(chainSummary(2)).toBe('this machine \u2192 Hop 2 \u2192 Hop 1 \u2192 target');
		expect(chainSummary(1, '[fd00::5]:5432')).toBe(
			'this machine \u2192 Hop 1 \u2192 [fd00::5]:5432'
		);
	});

	test('hopTitle names the first and last roles, both for a single hop', () => {
		expect(hopTitle(0, 1, EXIT_ROLES)).toBe(
			'Hop 1 \u00b7 exit node \u00b7 dialed from this machine'
		);
		expect(hopTitle(0, 3, EXIT_ROLES)).toBe('Hop 1 \u00b7 exit node');
		expect(hopTitle(1, 3, EXIT_ROLES)).toBe('Hop 2');
		expect(hopTitle(2, 3, EXIT_ROLES)).toBe('Hop 3 \u00b7 dialed from this machine');
		expect(hopTitle(0, 2, LISTEN_ROLES)).toBe('Hop 1 \u00b7 binds the port');
		expect(hopTitle(1, 2, LISTEN_ROLES)).toBe('Hop 2 \u00b7 dialed from this machine');
	});

	test('stageTitle adds the entry role to a multi-hop exit chain only', () => {
		expect(stageTitle(1, 2, 'forward')).toBe(
			'Hop 2 \u00b7 entry node \u00b7 dialed from this machine'
		);
		expect(stageTitle(0, 2, 'forward')).toBe('Hop 1 \u00b7 exit node');
		expect(stageTitle(0, 1, 'forward')).toBe(
			'Hop 1 \u00b7 exit node \u00b7 dialed from this machine'
		);
		expect(stageTitle(1, 2, 'listen')).toBe('Hop 2 \u00b7 dialed from this machine');
		expect(stageTitle(0, 1, 'listen')).toBe(
			'Hop 1 \u00b7 binds the port \u00b7 dialed from this machine'
		);
	});
});
