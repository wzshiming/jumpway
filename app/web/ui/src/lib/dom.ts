// A DOM id derived from an arbitrary key; "_hex_" escapes keep it selector-safe and injective.
export const elementId = (prefix: string, key: string): string =>
	prefix +
	'-' +
	key.replace(/[^A-Za-z0-9-]/gu, (char) => '_' + char.codePointAt(0)!.toString(16) + '_');
