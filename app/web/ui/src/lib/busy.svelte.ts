let depth = $state(0);

export const busy = {
	get value() {
		return depth > 0;
	},
	async run<T>(task: () => Promise<T>): Promise<T> {
		depth++;
		try {
			return await task();
		} finally {
			depth--;
		}
	}
};
