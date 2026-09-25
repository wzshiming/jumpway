import { text } from './format';

export type SortDirection = 'ascending' | 'descending';

export interface ListSort<K extends string> {
	key: K;
	direction: SortDirection;
}

// Case-insensitive subsequence match over any of the values: "1809" hits "127.0.0.1:18094".
export function fuzzyMatch(query: string, values: readonly unknown[], language: string): boolean {
	const needle = text(query).trim().toLocaleLowerCase(language);
	if (!needle) return true;
	return values.some((value) => {
		let position = 0;
		const haystack = text(value).toLocaleLowerCase(language);
		for (const character of needle) {
			position = haystack.indexOf(character, position);
			if (position < 0) return false;
			position++;
		}
		return true;
	});
}
