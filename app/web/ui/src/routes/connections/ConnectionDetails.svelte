<script lang="ts">
	import HelpTip from '../../lib/components/ui/HelpTip.svelte';
	import { formatConnectionPath, type RuleConnection } from '../../lib/connections';
	import { DASH, formatDateTime } from '../../lib/format';
	import { t } from '../../lib/i18n.svelte';

	// The item shows the connection's traffic, client, process and duration; this adds the two
	// verbose facts it leaves out.
	let { connection }: { connection: RuleConnection } = $props();

	const startedAt = $derived(Date.parse(connection.started));
</script>

<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1 text-[13px]" data-connection-facts>
	<dt class="text-fg-subtle">{t('started')}</dt>
	<dd class="font-mono text-fg tabular-nums" data-started>
		{#if Number.isFinite(startedAt)}
			<time datetime={new Date(startedAt).toISOString()}>{formatDateTime(connection.started)}</time>
		{:else}
			{DASH}
		{/if}
	</dd>
	<dt class="flex items-center gap-1 text-fg-subtle">
		{t('path')}
		<HelpTip concept={t('path')} text={t('help.path')} />
	</dt>
	<dd class="font-mono [overflow-wrap:anywhere] text-fg" data-path>
		{formatConnectionPath(connection)}
	</dd>
</dl>
