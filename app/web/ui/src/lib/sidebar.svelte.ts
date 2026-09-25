export const SIDEBAR_STORAGE_KEY = 'jumpway.sidebarCollapsed';
export const SIDEBAR_ID = 'sidebar';

let collapsed = $state(false);

function readStored(): boolean {
	try {
		return localStorage.getItem(SIDEBAR_STORAGE_KEY) === 'true';
	} catch {
		return false;
	}
}

// The desktop sidebar's width: the labelled column or an icon rail. The drawer never collapses.
export const sidebar = {
	get collapsed() {
		return collapsed;
	},
	set(next: boolean) {
		collapsed = next;
		try {
			localStorage.setItem(SIDEBAR_STORAGE_KEY, String(next));
		} catch {
			// Storage may be denied; the choice still applies for this page.
		}
	},
	toggle() {
		sidebar.set(!collapsed);
	},
	init() {
		collapsed = readStored();
	}
};
