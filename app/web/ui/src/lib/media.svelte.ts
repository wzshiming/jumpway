export const DESKTOP_QUERY = '(min-width: 768px)';

export interface MediaQueryState {
	readonly matches: boolean;
	start(): () => void;
}

// Without matchMedia (jsdom) the fallback layout applies.
export function mediaQuery(query: string, fallback: boolean): MediaQueryState {
	let matches = $state(fallback);
	return {
		get matches() {
			return matches;
		},
		start() {
			const media = typeof matchMedia === 'function' ? matchMedia(query) : null;
			if (!media) return () => {};
			matches = media.matches;
			const onChange = (event: { matches: boolean }) => {
				matches = event.matches;
			};
			media.addEventListener('change', onChange);
			return () => media.removeEventListener('change', onChange);
		}
	};
}
