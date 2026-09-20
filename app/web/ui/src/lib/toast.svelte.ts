export type ToastKind = 'success' | 'info' | 'warning' | 'error';

export interface Toast {
	id: number;
	kind: ToastKind;
	message: string;
}

export const SUCCESS_TOAST_MS = 6_000;

let list = $state<Toast[]>([]);
const timers = new Map<number, ReturnType<typeof setTimeout>>();
let sequence = 0;

function dismiss(id: number) {
	const timer = timers.get(id);
	if (timer !== undefined) clearTimeout(timer);
	timers.delete(id);
	list = list.filter((toast) => toast.id !== id);
}

function push(kind: ToastKind, message: string, ttl: number | null): number {
	const id = ++sequence;
	list = [...list, { id, kind, message }];
	if (ttl !== null)
		timers.set(
			id,
			setTimeout(() => dismiss(id), ttl)
		);
	return id;
}

export const toasts = {
	get list() {
		return list;
	},
	success: (message: string) => push('success', message, SUCCESS_TOAST_MS),
	info: (message: string) => push('info', message, SUCCESS_TOAST_MS),
	warning: (message: string) => push('warning', message, null),
	error: (message: string) => push('error', message, null),
	dismiss,
	clear() {
		for (const id of Array.from(timers.keys())) dismiss(id);
		list = [];
	}
};
