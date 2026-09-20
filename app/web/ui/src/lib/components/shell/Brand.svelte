<script lang="ts">
	import iconBlack from '$brand/icon_black.png';
	import iconWhite from '$brand/icon_white.png';
	import { tooltip } from '../../actions/tooltip.svelte';
	import { t } from '../../i18n.svelte';
	import { theme } from '../../theme.svelte';

	// compact: the bitmap alone, named for assistive technology and the tooltip.
	interface Props {
		compact?: boolean;
	}

	let { compact = false }: Props = $props();
</script>

<a
	href="#/"
	class="inline-flex items-center gap-2 rounded-md text-fg"
	aria-label={compact ? t('appName') : undefined}
	use:tooltip={compact ? t('appName') : null}
>
	<img
		src={theme.resolved === 'dark' ? iconWhite : iconBlack}
		alt=""
		width="24"
		height="24"
		class="size-6"
	/>
	{#if !compact}
		<span class="text-[15px] font-semibold">{t('appName')}</span>
	{/if}
</a>
