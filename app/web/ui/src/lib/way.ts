import { list, type Nullable, type WayHop, type WayNode } from './types';

const text = (value: unknown) => String(value ?? '');

// Fresh arrays: the editor mutates hops in place.
export function normalizeWay(way: Nullable<WayNode[]>): WayHop[] {
	return list(way).map((node) => ({
		lb: (typeof node === 'string'
			? node.split('|')
			: Array.isArray(node)
				? node
				: list(node?.lb)
		).map(text)
	}));
}

export function serializeWay(hops: readonly WayHop[]): WayHop[] {
	return hops
		.map((hop) => ({ lb: hop.lb.map((url) => url.trim()).filter(Boolean) }))
		.filter((hop) => hop.lb.length > 0);
}
