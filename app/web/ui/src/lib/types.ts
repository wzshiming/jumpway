// Wire contract of /apis/configs and /apis/stats; Go nil slices arrive as null.

export type Nullable<T> = T | null | undefined;

// Server encodes one URL as a bare string and several as {lb}; "a|b" strings are accepted on input.
export type WayNode = string | string[] | { lb?: Nullable<string[]> };

export interface WayHop {
	lb: string[];
}

export interface Address {
	host: string;
	port: number;
}

// One scheme served on the entry; empty credentials inherit the shared Listen ones, cipher is ss only.
export interface Protocol {
	type: string;
	username?: string;
	password?: string;
	cipher?: string;
}

// virtual names an in-app channel instead of a socket; the server then reports host '' and port 0.
export interface Listen {
	host: string;
	port: number;
	virtual?: string;
	way?: Nullable<WayNode[]>;
	username?: string;
	password?: string;
	cipher?: string;
	protocols?: Nullable<Protocol[]>;
}

// port 0 or absent means proxy mode: clients pick their own target; virtual is always a forward.
export interface Forward {
	host?: string;
	port?: number;
	virtual?: string;
	way?: Nullable<WayNode[]>;
}

export interface Rule {
	name: string;
	disabled?: boolean;
	listen: Listen;
	forward: Forward;
}

export interface NoProxy {
	list: Nullable<string[]>;
	from_env: Nullable<string[]>;
	from_file: Nullable<string[]>;
}

export interface RawConfig {
	yaml: string;
}

export interface RuleStatus {
	name: string;
	address: string;
	target?: string;
	remote: boolean;
	running: boolean;
	attempt?: number;
	error?: string;
}

export interface Status {
	address: string;
	running: boolean;
	error?: string;
	rules: Nullable<RuleStatus[]>;
	version?: string;
}

export interface Stats {
	up: number;
	down: number;
	rate_up: number;
	rate_down: number;
	peak_rate_up: number;
	peak_rate_down: number;
	active: number;
	total: number;
	dials: number;
	dial_failures: number;
	latency_ms: number;
	avg_latency_ms: number;
	last_active?: string;
	last_up?: string;
	last_down?: string;
}

export interface URLStats {
	url: string;
	stats: Stats;
}

// Hop 0 is the exit/bind end; the last hop is dialed from this machine.
export interface Hop {
	index: number;
	parent_index: number;
	stats: Stats;
	urls: Nullable<URLStats[]>;
}

export interface Target {
	address: string;
	via: string;
	stats: Stats;
}

// dialed false: a cached transport was reused and url was inferred or unknown.
export interface PathHop {
	index: number;
	url: string;
	dialed: boolean;
}

export interface Process {
	pid: number;
	name: string;
}

export interface Connection {
	id: number;
	client?: string;
	process?: Process;
	target: string;
	via: string;
	path: Nullable<PathHop[]>;
	started: string;
	stats: Stats;
}

export interface RuleStats {
	name: string;
	stats: Stats;
	listen: Nullable<Hop[]>;
	forward: Nullable<Hop[]>;
	targets: Nullable<Target[]>;
	connections: Nullable<Connection[]>;
	targets_evicted: number;
}

export interface Snapshot {
	since: string;
	rules: Nullable<RuleStats[]>;
}

export const list = <T>(value: Nullable<T[]>): T[] => (Array.isArray(value) ? value : []);
