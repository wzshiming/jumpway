export interface ConfirmOptions {
	title?: string;
	message: string;
	confirmLabel?: string;
	cancelLabel?: string;
	danger?: boolean;
}

export type ConfirmHost = (options: ConfirmOptions) => Promise<boolean>;

let host: ConfirmHost | null = null;

// Until the dialog component registers, window.confirm keeps every flow working.
export const confirmService = {
	register(handler: ConfirmHost): () => void {
		host = handler;
		return () => {
			if (host === handler) host = null;
		};
	}
};

export function confirm(options: ConfirmOptions): Promise<boolean> {
	return host ? host(options) : Promise.resolve(window.confirm(options.message));
}
