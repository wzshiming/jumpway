import rawLayouts from './data/proxy_layouts.json';
import type { MessageKey } from './i18n/en';

export type LayoutInputKind = 'span' | 'text' | 'password' | 'select' | 'file';
export type BuilderField =
	'username' | 'password' | 'host' | 'port' | 'identity' | 'encrypto' | 'command';

export interface LayoutInput {
	name?: BuilderField;
	kind: LayoutInputKind;
	label?: MessageKey;
	value?: string;
	option?: boolean;
	items?: string[];
	placeholder?: string;
}

export interface ProxyLayout {
	name: string;
	inputs: LayoutInput[];
}

export type BuilderValues = Partial<Record<BuilderField, string | undefined>>;

export const layouts = rawLayouts as ProxyLayout[];

const ALIASES: Record<string, string> = { ss: 'shadowsocks', cmd: 'command', nc: 'netcat' };

export function layoutFor(protocol: string): ProxyLayout | undefined {
	const name = protocol.toLowerCase();
	return layouts.find((layout) => layout.name === (ALIASES[name] ?? name));
}

export const fieldsOf = (layout: ProxyLayout): (LayoutInput & { name: BuilderField })[] =>
	layout.inputs.filter(
		(input): input is LayoutInput & { name: BuilderField } => input.kind !== 'span'
	);

export function initialValues(layout: ProxyLayout, values: BuilderValues = {}): BuilderValues {
	return Object.fromEntries(
		fieldsOf(layout).map((field) => [field.name, String(values[field.name] ?? field.value ?? '')])
	);
}

export interface BuiltURL {
	url: string;
	valid: boolean;
	hint: Extract<MessageKey, 'builderHint' | 'builderShadowsocksHint' | 'builderCommandHint'> | null;
}

const commandFieldOf = (layout: ProxyLayout) =>
	layout.inputs.find((field) => field.name === 'command');

export function buildURL(layout: ProxyLayout, values: BuilderValues): BuiltURL {
	const scheme = layout.inputs.find((field) => field.kind === 'span')?.value ?? '';
	const commandField = commandFieldOf(layout);
	if (commandField) {
		const command = (values.command || '').trim();
		const valid = Boolean(command || commandField.option);
		return { url: scheme + command, valid, hint: valid ? null : 'builderCommandHint' };
	}
	const enc = encodeURIComponent;
	const shadowsocks = layout.name === 'shadowsocks';
	const username = values.username || '';
	const password = values.password || '';
	const encrypto = values.encrypto || '';
	const userinfo = shadowsocks
		? enc(encrypto) + ':' + enc(password) + '@'
		: username || password
			? enc(username) + (password ? ':' + enc(password) : '') + '@'
			: '';
	let host = (values.host || '').trim();
	if (host.includes(':') && !(host.startsWith('[') && host.endsWith(']'))) host = '[' + host + ']';
	const port = values.port || layout.inputs.find((field) => field.name === 'port')?.value || '';
	let url = scheme + userinfo + host + ':' + port;
	if (layout.name === 'ssh' && values.identity) url += '?identity_file=' + enc(values.identity);
	const validPort = /^\d+$/.test(port) && Number(port) >= 1 && Number(port) <= 65535;
	const valid = Boolean(host) && validPort && !(shadowsocks && (!encrypto || !password));
	return {
		url,
		valid,
		hint: valid ? null : shadowsocks ? 'builderShadowsocksHint' : 'builderHint'
	};
}

const decode = (value: string) => {
	try {
		return decodeURIComponent(value);
	} catch {
		return value;
	}
};

export function parseURL(value: string): { layout: ProxyLayout; values: BuilderValues } | null {
	const colon = value.indexOf(':');
	const opaque = colon > 0 ? layoutFor(value.slice(0, colon)) : undefined;
	if (opaque && commandFieldOf(opaque)) {
		return { layout: opaque, values: { command: value.slice(colon + 1) } };
	}
	let url: URL;
	try {
		url = new URL(value);
	} catch {
		return null;
	}
	const layout = layoutFor(url.protocol.slice(0, -1));
	if (!layout) return null;
	return {
		layout,
		values: {
			username: decode(url.username),
			password: decode(url.password),
			host: url.hostname,
			port: url.port || undefined,
			identity: url.searchParams.get('identity_file') ?? '',
			encrypto: decode(url.username)
		}
	};
}
