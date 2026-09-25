import type {
	Address,
	NoProxy,
	RawConfig,
	Rule,
	Snapshot,
	Stats,
	Status
} from '../../src/lib/types.ts';

// Shared by the Vitest shell tests and the Playwright mock API. Credentials are placeholders.

// Backend errors echo proxy URLs verbatim; the UI must show these masked and never the secret.
export const LEAKY_URL = 'socks5://review-user:review-secret@host:bad';
export const LEAKY_SSH_URL = 'ssh://ops:review-secret@edge.example:22';
export const SECRET = 'review-secret';

export const statusFixture: Status = {
	address: '127.0.0.1:1088',
	running: true,
	error: `rules[3].forward.way[0]: invalid proxy URL "${LEAKY_URL}": parse "${LEAKY_URL}": invalid port ":bad" after host (e.g. socks5://host:1080)`,
	rules: [
		{ name: 'office', address: '127.0.0.1:18097', remote: false, running: true },
		{
			name: 'mirror',
			address: '127.0.0.1:18098',
			target: '10.0.0.5:5432',
			remote: false,
			running: true
		},
		{
			name: 'db-tunnel',
			address: '0.0.0.0:18099',
			target: '127.0.0.1:5432',
			remote: true,
			running: false,
			attempt: 3,
			error: `listen through "${LEAKY_SSH_URL}": ssh: handshake failed: dial tcp 203.0.113.9:22: connection refused`
		},
		{ name: 'lab', address: '127.0.0.1:18100', remote: false, running: false }
	]
};

export const rulesFixture: Rule[] = [
	{
		name: 'office',
		listen: { host: '127.0.0.1', port: 18097 },
		forward: {
			way: [
				'socks5://demo:placeholder@hop-a.example:1080',
				{ lb: ['ssh://ops@bastion.example:22', 'ssh://ops@bastion-2.example:22'] }
			]
		}
	},
	{
		name: 'mirror',
		listen: { host: '127.0.0.1', port: 18098 },
		forward: { host: '10.0.0.5', port: 5432, way: ['ssh://ops@bastion.example:22'] }
	},
	{
		name: 'db-tunnel',
		listen: { host: '0.0.0.0', port: 18099, way: ['ssh://ops@edge.example:22'] },
		forward: { host: '127.0.0.1', port: 5432 }
	},
	{
		name: 'lab',
		disabled: true,
		listen: { host: '127.0.0.1', port: 18100, username: 'demo', password: 'placeholder' },
		forward: {}
	}
];

// Virtual endpoints, kept apart so the four-rule counts above stay valid: many entries share
// one exit channel; orphan forwards to a channel nobody listens on (allowed, dangling).
export const virtualRulesFixture: Rule[] = [
	{ name: 'shared-exit', listen: { host: '', port: 0, virtual: 'exit' }, forward: {} },
	{ name: 'lan-entry', listen: { host: '0.0.0.0', port: 18101 }, forward: { virtual: 'exit' } },
	{ name: 'orphan', listen: { host: '127.0.0.1', port: 18102 }, forward: { virtual: 'missing' } }
];

export const protocolRulesFixture: Rule[] = [
	{
		name: 'ss-only',
		listen: {
			host: '0.0.0.0',
			port: 18200,
			protocols: [{ type: 'ss', password: 'placeholder', cipher: 'chacha20-ietf-poly1305' }]
		},
		forward: {}
	},
	{
		name: 'mixed',
		listen: {
			host: '127.0.0.1',
			port: 18201,
			username: 'demo',
			password: 'placeholder',
			protocols: [
				{ type: 'http' },
				{ type: 'socks5', username: 'socks-user', password: 'socks-pass' }
			]
		},
		forward: {}
	}
];

const zero: Stats = {
	up: 0,
	down: 0,
	rate_up: 0,
	rate_down: 0,
	peak_rate_up: 0,
	peak_rate_down: 0,
	active: 0,
	total: 0,
	dials: 0,
	dial_failures: 0,
	latency_ms: 0,
	avg_latency_ms: 0
};

export const statsOf = (partial: Partial<Stats>): Stats => ({ ...zero, ...partial });

// office forwards through hop 2 (bastion, two URLs, dialed from this machine) into hop 1 (exit).
const OFFICE_EXIT_URL = 'socks5://demo:placeholder@hop-a.example:1080';
const BASTION_URL = 'ssh://ops@bastion.example:22';
const BASTION_2_URL = 'ssh://ops@bastion-2.example:22';
const EDGE_URL = 'ssh://ops@edge.example:22';

export const snapshotFixture: Snapshot = {
	since: '2026-09-19T08:00:00Z',
	rules: [
		{
			name: 'office',
			stats: statsOf({
				up: 5_242_880,
				down: 104_857_600,
				rate_up: 12_288,
				rate_down: 1_048_576,
				peak_rate_up: 65_536,
				peak_rate_down: 4_194_304,
				active: 3,
				total: 120,
				dials: 118,
				dial_failures: 2,
				latency_ms: 41.2,
				avg_latency_ms: 38.7,
				last_active: '2026-09-19T08:59:58Z'
			}),
			listen: [],
			forward: [
				{
					index: 0,
					parent_index: 1,
					stats: statsOf({
						up: 5_242_880,
						down: 104_857_600,
						rate_up: 12_288,
						rate_down: 1_048_576,
						peak_rate_up: 65_536,
						peak_rate_down: 4_194_304,
						active: 3,
						total: 120,
						dials: 118,
						dial_failures: 2,
						latency_ms: 41.2,
						avg_latency_ms: 38.7,
						last_active: '2026-09-19T08:59:58Z',
						last_up: '2026-09-19T08:59:57Z',
						last_down: '2026-09-19T08:59:58Z'
					}),
					urls: [
						{
							url: OFFICE_EXIT_URL,
							stats: statsOf({
								up: 5_242_880,
								down: 104_857_600,
								rate_up: 12_288,
								rate_down: 1_048_576,
								peak_rate_up: 65_536,
								peak_rate_down: 4_194_304,
								active: 3,
								total: 120,
								dials: 118,
								dial_failures: 2,
								latency_ms: 41.2,
								avg_latency_ms: 38.7,
								last_active: '2026-09-19T08:59:58Z',
								last_up: '2026-09-19T08:59:57Z',
								last_down: '2026-09-19T08:59:58Z'
							})
						}
					]
				},
				{
					index: 1,
					parent_index: -1,
					// The hop peak is the true server-side peak: below the sum of its URL peaks.
					stats: statsOf({
						up: 5_300_000,
						down: 105_000_000,
						rate_up: 12_500,
						rate_down: 1_050_000,
						peak_rate_up: 70_000,
						peak_rate_down: 4_300_000,
						active: 2,
						total: 9,
						dials: 9,
						dial_failures: 1,
						latency_ms: 18.4,
						avg_latency_ms: 20.1,
						last_active: '2026-09-19T08:59:58Z',
						last_up: '2026-09-19T08:59:57Z',
						last_down: '2026-09-19T08:59:58Z'
					}),
					urls: [
						{
							url: BASTION_URL,
							stats: statsOf({
								up: 4_000_000,
								down: 80_000_000,
								rate_up: 10_000,
								rate_down: 800_000,
								peak_rate_up: 60_000,
								peak_rate_down: 4_000_000,
								active: 1,
								total: 6,
								dials: 6,
								dial_failures: 0,
								latency_ms: 18.4,
								avg_latency_ms: 19.0,
								last_active: '2026-09-19T08:59:58Z',
								last_up: '2026-09-19T08:59:57Z',
								last_down: '2026-09-19T08:59:58Z'
							})
						},
						{
							url: BASTION_2_URL,
							stats: statsOf({
								up: 1_300_000,
								down: 25_000_000,
								rate_up: 2_500,
								rate_down: 250_000,
								peak_rate_up: 30_000,
								peak_rate_down: 2_000_000,
								active: 1,
								total: 3,
								dials: 3,
								dial_failures: 1,
								latency_ms: 25.0,
								avg_latency_ms: 22.3,
								last_active: '2026-09-19T08:45:00Z',
								last_up: '2026-09-19T08:44:59Z',
								last_down: '2026-09-19T08:45:00Z'
							})
						}
					]
				}
			],
			// Listed in first-seen order; the UI orders them by last_active, the never-active one last.
			targets: [
				{
					address: 'example.com:443',
					via: BASTION_URL,
					stats: statsOf({
						up: 3_000_000,
						down: 60_000_000,
						active: 1,
						total: 80,
						dials: 80,
						dial_failures: 2,
						latency_ms: 40.1,
						avg_latency_ms: 39.0,
						last_active: '2026-09-19T08:59:50Z'
					})
				},
				{
					address: 'cdn.example.net:443',
					via: BASTION_2_URL,
					stats: statsOf({
						up: 2_000_000,
						down: 40_000_000,
						active: 1,
						total: 30,
						dials: 30,
						latency_ms: 22.0,
						avg_latency_ms: 24.5,
						last_active: '2026-09-19T08:59:58Z'
					})
				},
				{
					address: '10.1.2.3:8080',
					via: '',
					stats: statsOf({
						up: 242_880,
						down: 4_857_600,
						active: 1,
						total: 10,
						dials: 10,
						latency_ms: 1.2,
						avg_latency_ms: 1.5
					})
				}
			],
			connections: [
				{
					id: 101,
					client: '127.0.0.1:18094',
					process: { pid: 4242, name: 'curl' },
					target: 'example.com:443',
					via: BASTION_URL,
					path: [
						{ index: 0, url: OFFICE_EXIT_URL, dialed: true },
						{ index: 1, url: BASTION_URL, dialed: true }
					],
					started: '2026-09-19T08:58:30Z',
					stats: statsOf({
						up: 12_288,
						down: 1_048_576,
						rate_up: 1_024,
						rate_down: 524_288,
						peak_rate_up: 4_096,
						peak_rate_down: 1_048_576,
						active: 1,
						total: 1,
						last_up: '2026-09-19T08:59:57Z',
						last_down: '2026-09-19T08:59:58Z'
					})
				},
				{
					id: 102,
					client: '127.0.0.1:18095',
					process: { pid: 777, name: 'Google Chrome Helper' },
					target: 'cdn.example.net:443',
					via: BASTION_2_URL,
					// A cached transport: nothing was dialed for this connection.
					path: [
						{ index: 0, url: '', dialed: false },
						{ index: 1, url: BASTION_2_URL, dialed: false }
					],
					started: '2026-09-19T08:59:10Z',
					stats: statsOf({
						up: 2_048,
						down: 65_536,
						rate_up: 512,
						rate_down: 16_384,
						peak_rate_up: 2_048,
						peak_rate_down: 65_536,
						active: 1,
						total: 1,
						last_up: '2026-09-19T08:59:50Z',
						last_down: '2026-09-19T08:59:55Z'
					})
				},
				{
					id: 103,
					client: '[::1]:52000',
					target: '10.1.2.3:8080',
					via: '',
					path: null,
					started: '2026-09-19T08:59:50Z',
					stats: statsOf({ active: 1, total: 1 })
				}
			],
			// Three older targets fell off the per-rule list; their bytes stay in the rule totals.
			targets_evicted: 3
		},
		{
			name: 'mirror',
			stats: statsOf({
				up: 1_048_576,
				down: 2_097_152,
				rate_up: 2_048,
				rate_down: 4_096,
				peak_rate_up: 8_192,
				peak_rate_down: 16_384,
				active: 1,
				total: 4,
				dials: 4,
				latency_ms: 12.5,
				avg_latency_ms: 11.9
			}),
			listen: [],
			forward: [
				{
					index: 0,
					parent_index: -1,
					stats: statsOf({
						up: 1_048_576,
						down: 2_097_152,
						rate_up: 2_048,
						rate_down: 4_096,
						peak_rate_up: 8_192,
						peak_rate_down: 16_384,
						active: 1,
						total: 4,
						dials: 4,
						latency_ms: 12.5,
						avg_latency_ms: 11.9,
						last_active: '2026-09-19T08:30:00Z',
						last_up: '2026-09-19T08:59:00Z',
						last_down: '2026-09-19T08:59:30Z'
					}),
					urls: [
						{
							url: BASTION_URL,
							stats: statsOf({
								up: 1_048_576,
								down: 2_097_152,
								rate_up: 2_048,
								rate_down: 4_096,
								peak_rate_up: 8_192,
								peak_rate_down: 16_384,
								active: 1,
								total: 4,
								dials: 4,
								latency_ms: 12.5,
								avg_latency_ms: 11.9,
								last_active: '2026-09-19T08:30:00Z',
								last_up: '2026-09-19T08:59:00Z',
								last_down: '2026-09-19T08:59:30Z'
							})
						}
					]
				}
			],
			targets: [
				{
					address: '10.0.0.5:5432',
					via: BASTION_URL,
					stats: statsOf({ up: 1_048_576, down: 2_097_152, active: 1, total: 4 })
				}
			],
			connections: [
				{
					id: 201,
					client: '127.0.0.1:60000',
					process: { pid: 9, name: 'psql' },
					target: '10.0.0.5:5432',
					via: BASTION_URL,
					path: [{ index: 0, url: BASTION_URL, dialed: true }],
					started: '2026-09-19T08:30:00Z',
					stats: statsOf({
						up: 1_048_576,
						down: 2_097_152,
						rate_up: 2_048,
						rate_down: 4_096,
						peak_rate_up: 8_192,
						peak_rate_down: 16_384,
						active: 1,
						total: 1,
						last_up: '2026-09-19T08:59:00Z',
						last_down: '2026-09-19T08:59:30Z'
					})
				}
			],
			targets_evicted: 0
		},
		{
			name: 'db-tunnel',
			stats: statsOf({ dials: 3, dial_failures: 3 }),
			listen: [
				{
					index: 0,
					parent_index: -1,
					stats: statsOf({ dials: 3, dial_failures: 3 }),
					urls: [{ url: EDGE_URL, stats: statsOf({ dials: 3, dial_failures: 3 }) }]
				}
			],
			forward: null,
			targets: null,
			connections: null,
			targets_evicted: 0
		}
	]
};

// The hosts page aggregates the hop URLs above; spelled out so tests assert numbers.
export const hostsTotals = {
	// bastion.example: office hop 2 URL + mirror hop 1 URL.
	bastionUp: '4.8 MB',
	bastionDown: '78.3 MB',
	bastionPeakDown: '3.8 MB/s',
	bastionConnections: '2 / 10',
	// (6 × 19.0 + 4 × 11.9) / 10
	bastionAvgLatency: '16.2 ms',
	order: ['hop-a.example', 'bastion.example', 'bastion-2.example', 'edge.example']
};

// Totals over snapshotFixture, spelled out so tests assert numbers rather than recompute them.
export const snapshotTotals = {
	active: 4,
	rateUp: '14.0 KB/s',
	rateDown: '1.0 MB/s',
	up: '6.0 MB',
	down: '102.0 MB'
};

export const webUIFixture: Address = { host: '127.0.0.1', port: 1088 };

// from_file is null as the Go server encodes an absent list.
export const noProxyFixture: NoProxy = {
	list: ['localhost', '127.0.0.1', '10.0.0.0/8'],
	from_env: ['NO_PROXY', 'no_proxy'],
	from_file: null
};

// Comments, blank lines and odd spacing must survive the round trip untouched.
export const rawFixture: RawConfig = {
	yaml: [
		'# JumpWay configuration',
		'web_ui:',
		'  host: 127.0.0.1',
		'  port: 1088',
		'',
		'rules:',
		'  - name: office    # main proxy',
		'    listen:',
		'      host: 127.0.0.1',
		'      port:   18097',
		'    forward:',
		'      way:',
		'        - socks5://demo:placeholder@hop-a.example:1080',
		'',
		'no_proxy:',
		'  list: [localhost, 127.0.0.1]',
		''
	].join('\n')
};
