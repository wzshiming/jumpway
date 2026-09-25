<script lang="ts">
	import { tick } from 'svelte';
	import IconArrowDown from '~icons/lucide/arrow-down';
	import IconArrowUp from '~icons/lucide/arrow-up';
	import IconPlus from '~icons/lucide/plus';
	import IconTrash from '~icons/lucide/trash-2';
	import IconWand from '~icons/lucide/wand-sparkles';
	import IconX from '~icons/lucide/x';
	import { t, type MessageKey } from '../../i18n.svelte';
	import { hopTitle, type HopRoles } from '../../rule';
	import { newHop, newUrl, type DraftErrors, type HopDraft, type UrlDraft } from '../../ruleEditor';
	import Button from '../ui/Button.svelte';
	import IconButton from '../ui/IconButton.svelte';

	interface Props {
		hops: HopDraft[];
		// Per-URL errors are read from `urls`, keyed by UrlDraft.id, and cleared as the row is edited.
		errors: DraftErrors;
		idPrefix: string;
		roles: HopRoles;
		hint: MessageKey;
		// Opens the URL builder for the current text; resolves with the chosen URL or null.
		build: (current: string) => Promise<string | null>;
	}

	let { hops = $bindable(), errors = $bindable(), idPrefix, roles, hint, build }: Props = $props();

	const hintId = $derived(idPrefix + '-hint');
	const addHopId = $derived(idPrefix + '-add-hop');
	const hopId = (hop: HopDraft) => `${idPrefix}-hop-${hop.id}`;
	const urlId = (url: UrlDraft) => `${idPrefix}-url-${url.id}`;
	const errorOf = (url: UrlDraft): MessageKey | undefined => errors.urls?.[url.id];
	const clearError = (url: UrlDraft) => {
		if (errors.urls) delete errors.urls[url.id];
	};

	async function focus(id: string) {
		await tick();
		document.getElementById(id)?.focus();
	}

	function addHop() {
		const hop = newHop();
		hops.push(hop);
		void focus(urlId(hop.urls[0]));
	}

	function removeHop(index: number) {
		hops.splice(index, 1);
		void focus(addHopId);
	}

	function move(index: number, offset: -1 | 1) {
		const target = index + offset;
		[hops[index], hops[target]] = [hops[target], hops[index]];
		const hop = hops[target];
		const again = offset < 0 ? target > 0 : target < hops.length - 1;
		void focus(`${hopId(hop)}-${offset < 0 === again ? 'up' : 'down'}`);
	}

	function addUrl(hop: HopDraft) {
		const url = newUrl();
		hop.urls.push(url);
		void focus(urlId(url));
	}

	function removeUrl(hop: HopDraft, index: number) {
		hop.urls.splice(index, 1);
		const next = hop.urls[Math.min(index, hop.urls.length - 1)];
		void focus(next ? urlId(next) : `${hopId(hop)}-add-url`);
	}

	async function openBuilder(url: UrlDraft) {
		const result = await build(url.value);
		if (result === null) return;
		url.value = result;
		clearError(url);
		void focus(urlId(url));
	}
</script>

<div class="space-y-3">
	<p id={hintId} class="text-xs text-fg-muted">{t(hint)}</p>
	{#if hops.length}
		<ol class="space-y-3" aria-describedby={hintId}>
			{#each hops as hop, index (hop.id)}
				<li
					id={hopId(hop)}
					class="rounded-lg border border-line bg-surface p-3"
					aria-labelledby="{hopId(hop)}-title"
				>
					<div class="flex flex-wrap items-center justify-between gap-2">
						<h4 id="{hopId(hop)}-title" class="text-sm font-medium">
							{hopTitle(index, hops.length, roles)}
						</h4>
						<div class="flex items-center gap-0.5">
							<IconButton
								id="{hopId(hop)}-up"
								label={t('up')}
								disabled={index === 0}
								onclick={() => move(index, -1)}
							>
								<IconArrowUp class="size-4" aria-hidden="true" />
							</IconButton>
							<IconButton
								id="{hopId(hop)}-down"
								label={t('down')}
								disabled={index === hops.length - 1}
								onclick={() => move(index, 1)}
							>
								<IconArrowDown class="size-4" aria-hidden="true" />
							</IconButton>
							<IconButton label={t('deleteHop')} onclick={() => removeHop(index)}>
								<IconTrash class="size-4" aria-hidden="true" />
							</IconButton>
						</div>
					</div>
					<div class="mt-2 space-y-2">
						{#each hop.urls as url, position (url.id)}
							{@const error = errorOf(url)}
							<div>
								<div class="flex items-center gap-1.5">
									<label for={urlId(url)} class="sr-only">
										{t('proxyURL')}
										{t('urlNumber', { number: position + 1 })}
									</label>
									<input
										id={urlId(url)}
										class="input font-mono text-[13px]"
										type="text"
										bind:value={url.value}
										oninput={() => clearError(url)}
										placeholder="socks5://user:pass@host:1080"
										aria-invalid={error ? true : undefined}
										aria-describedby={error ? `${urlId(url)}-error` : undefined}
										spellcheck="false"
										autocomplete="off"
										autocapitalize="none"
									/>
									<IconButton label={t('build')} onclick={() => void openBuilder(url)}>
										<IconWand class="size-4" aria-hidden="true" />
									</IconButton>
									<IconButton label={t('remove')} onclick={() => removeUrl(hop, position)}>
										<IconX class="size-4" aria-hidden="true" />
									</IconButton>
								</div>
								{#if error}
									<p id="{urlId(url)}-error" class="mt-1 text-xs text-danger">{t(error)}</p>
								{/if}
							</div>
						{/each}
						<Button id="{hopId(hop)}-add-url" variant="ghost" onclick={() => addUrl(hop)}>
							<IconPlus class="size-4" aria-hidden="true" />
							{t('addURL')}
						</Button>
					</div>
				</li>
			{/each}
		</ol>
	{/if}
	<Button id={addHopId} variant="secondary" onclick={addHop}>
		<IconPlus class="size-4" aria-hidden="true" />
		{t('addHop')}
	</Button>
</div>
