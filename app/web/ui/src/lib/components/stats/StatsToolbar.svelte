<script lang="ts">
	import IconExternalLink from '~icons/lucide/external-link';
	import IconRotateCcw from '~icons/lucide/rotate-ccw';
	import { errorMessage } from '../../api';
	import { busy } from '../../busy.svelte';
	import { confirm } from '../../confirm';
	import { DASH, formatDateTime } from '../../format';
	import { t } from '../../i18n.svelte';
	import { resetStats, stats } from '../../stats.svelte';
	import { toasts } from '../../toast.svelte';
	import Button from '../ui/Button.svelte';

	// Since, Prometheus and Reset for the three statistics pages.
	async function reset() {
		if (busy.value) return;
		const ok = await confirm({ message: t('resetStatsConfirm'), danger: true });
		if (!ok) return;
		try {
			await resetStats();
			toasts.success(t('statsReset'));
		} catch (reason) {
			toasts.error(errorMessage(reason));
		}
	}
</script>

<p class="text-sm text-fg-muted tabular-nums" data-since>
	{stats.data ? t('statsSince', { time: formatDateTime(stats.data.since) }) : DASH}
</p>
<a href="/metrics" class="btn btn-secondary">
	{t('prometheus')}
	<IconExternalLink class="size-3.5 text-fg-muted" aria-hidden="true" />
</a>
<Button variant="secondary" disabled={busy.value} onclick={() => void reset()}>
	<IconRotateCcw class="size-4" aria-hidden="true" />
	{t('resetStats')}
</Button>
