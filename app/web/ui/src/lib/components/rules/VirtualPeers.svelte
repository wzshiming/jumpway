<script lang="ts">
	import { t } from '../../i18n.svelte';
	import { ruleRoute } from '../../routes';
	import { isVirtualChannel, virtualPeers, type VirtualSide } from '../../rule';
	import type { Rule } from '../../types';

	// Peer links describe configuration, not runtime health.
	interface Props {
		side: VirtualSide;
		channel: string;
		rules: readonly Rule[] | null;
		self?: string;
	}

	let { side, channel, rules, self = '' }: Props = $props();

	const name = $derived(channel.trim());
	const peers = $derived(
		rules && isVirtualChannel(name) ? virtualPeers(rules, side, name, self) : null
	);
</script>

{#if peers?.length}
	<p
		class="mt-1 font-sans text-xs break-normal [overflow-wrap:anywhere] text-fg-muted"
		data-virtual-peers
	>
		{t(side === 'listen' ? 'virtualIncoming' : 'virtualOutgoing')}
		{#each peers as peer, index (peer.name)}
			{index ? ', ' : ''}<a
				href={ruleRoute(peer.name)}
				class="relative z-10 font-mono text-fg hover:text-accent">{peer.name}</a
			>
		{/each}
	</p>
{:else if peers && side === 'forward'}
	<p
		class="mt-1 font-sans text-xs break-normal [overflow-wrap:anywhere] text-warning"
		data-virtual-peers
	>
		{t('virtualNoListener')}
	</p>
{/if}
