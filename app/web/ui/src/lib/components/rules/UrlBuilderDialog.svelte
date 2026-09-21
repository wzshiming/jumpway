<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { closeModal, openModal } from '../../dialog';
	import { t } from '../../i18n.svelte';
	import {
		buildURL,
		fieldsOf,
		initialValues,
		layouts,
		parseURL,
		type BuilderValues,
		type ProxyLayout
	} from '../../urlBuilder';
	import Button from '../ui/Button.svelte';
	import Field from '../ui/Field.svelte';

	const TITLE_ID = 'url-builder-title';
	const HINT_ID = 'url-builder-hint';
	const PREVIEW_ID = 'url-builder-preview';

	let dialog: HTMLDialogElement;
	let layout = $state<ProxyLayout>(layouts[0]);
	let values = $state<BuilderValues>({});
	let active = $state(false);
	let pending: ((value: string | null) => void) | null = null;
	let opener: Element | null = null;

	const fields = $derived(fieldsOf(layout));
	const built = $derived(buildURL(layout, values));

	// Resolves with the chosen URL, or null on Cancel, Escape and unmount; one question at a time.
	export function open(initial: string): Promise<string | null> {
		if (pending) return Promise.resolve(null);
		const parsed = parseURL(initial);
		layout = parsed?.layout ?? layouts[0];
		values = initialValues(layout, parsed?.values ?? {});
		opener = document.activeElement;
		active = true;
		return new Promise<string | null>((resolve) => {
			pending = resolve;
			void tick().then(() => {
				if (pending !== resolve) return;
				openModal(dialog);
				dialog.querySelector<HTMLElement>('[data-builder-fields] :is(input, select)')?.focus();
			});
		});
	}

	function settle(value: string | null) {
		const resolve = pending;
		pending = null;
		active = false;
		if (dialog?.open) closeModal(dialog);
		if (opener instanceof HTMLElement) opener.focus();
		opener = null;
		resolve?.(value);
	}

	// As before: another protocol starts from that layout's defaults.
	function selectLayout(name: string) {
		const next = layouts.find((entry) => entry.name === name);
		if (!next) return;
		layout = next;
		values = initialValues(next);
	}

	onMount(() => () => settle(null));
</script>

<dialog
	bind:this={dialog}
	class="m-auto w-[min(440px,calc(100vw-2rem))] rounded-lg border border-line bg-surface p-0 text-fg shadow-xl"
	aria-labelledby={TITLE_ID}
	onclose={() => {
		if (pending && !dialog.open) settle(null);
	}}
>
	{#if active}
		<!-- Enter submits only through the enabled "Use URL" button; every other control is type=button. -->
		<form
			class="space-y-3 p-5"
			novalidate
			autocomplete="off"
			onsubmit={(event) => {
				event.preventDefault();
				if (built.valid) settle(built.url);
			}}
		>
			<h2 id={TITLE_ID} class="text-base font-semibold">{t('builderTitle')}</h2>
			<Field id="url-builder-protocol" label={t('protocol')}>
				{#snippet children()}
					<select
						id="url-builder-protocol"
						class="input"
						value={layout.name}
						onchange={(event) => selectLayout(event.currentTarget.value)}
					>
						{#each layouts as entry (entry.name)}
							<option value={entry.name}>{entry.name}</option>
						{/each}
					</select>
				{/snippet}
			</Field>
			<div class="space-y-3" data-builder-fields>
				{#each fields as field (layout.name + '/' + field.name)}
					{@const id = 'url-builder-' + field.name}
					<Field
						{id}
						label={t(field.label ?? `field.${field.name}`)}
						optional={field.option === true}
					>
						{#snippet children()}
							{#if field.kind === 'select'}
								<select {id} class="input" bind:value={values[field.name]}>
									{#each field.items ?? [] as item (item)}
										<option value={item}>{item}</option>
									{/each}
								</select>
							{:else}
								<input
									{id}
									class="input font-mono text-[13px]"
									type={field.kind === 'password' ? 'password' : 'text'}
									bind:value={values[field.name]}
									placeholder={field.placeholder ??
										(field.name === 'port'
											? field.value
											: field.kind === 'file'
												? '~/.ssh/id_ed25519'
												: undefined)}
									inputmode={field.name === 'port' ? 'numeric' : undefined}
									autocomplete={field.kind === 'password' ? 'new-password' : 'off'}
									autocapitalize="none"
									spellcheck="false"
								/>
							{/if}
						{/snippet}
					</Field>
				{/each}
			</div>
			<div>
				<p id="{PREVIEW_ID}-label" class="field-label">{t('preview')}</p>
				<output
					id={PREVIEW_ID}
					aria-labelledby="{PREVIEW_ID}-label"
					class="block min-h-8 rounded-md bg-surface-2 px-2.5 py-1.5 font-mono text-[13px] break-all"
				>
					{built.url}
				</output>
			</div>
			<p id={HINT_ID} role="status" class="min-h-4 text-xs text-fg-muted">
				{built.hint ? t(built.hint) : ''}
			</p>
			<div class="flex justify-end gap-2 pt-1">
				<Button variant="secondary" onclick={() => settle(null)}>{t('cancel')}</Button>
				<Button variant="primary" type="submit" disabled={!built.valid}>{t('useURL')}</Button>
			</div>
		</form>
	{/if}
</dialog>
