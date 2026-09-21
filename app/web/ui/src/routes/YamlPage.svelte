<script lang="ts">
	import { onMount } from 'svelte';
	import { shortcut } from '../lib/actions/shortcut';
	import { ApiError, configsApi, errorMessage, isAborted } from '../lib/api';
	import { busy } from '../lib/busy.svelte';
	import Banner from '../lib/components/ui/Banner.svelte';
	import Button from '../lib/components/ui/Button.svelte';
	import FormActions from '../lib/components/ui/FormActions.svelte';
	import PageHeader from '../lib/components/ui/PageHeader.svelte';
	import { confirm } from '../lib/confirm';
	import { t } from '../lib/i18n.svelte';
	import { moved } from '../lib/moved.svelte';
	import { router } from '../lib/router.svelte';
	import { writeFailure, type WriteFailure } from '../lib/settings';
	import { toasts } from '../lib/toast.svelte';

	// config.yaml as text: nothing is parsed here, so comments and spacing round-trip untouched.

	let text = $state('');
	let baseline = $state('');
	let ready = $state(false);
	let loading = $state(false);
	let saving = $state(false);
	let loadError = $state.raw<unknown>(null);
	let failure = $state<WriteFailure | null>(null);
	let form = $state<HTMLFormElement | null>(null);
	let controller: AbortController | null = null;

	const dirty = $derived(ready && text !== baseline);

	function adopt(yaml: string) {
		text = yaml;
		baseline = yaml;
		ready = true;
	}

	function load() {
		controller?.abort();
		const own = new AbortController();
		controller = own;
		loading = true;
		loadError = null;
		configsApi
			.getRaw(own.signal)
			.then(
				(raw) => {
					if (controller === own) adopt(raw.yaml);
				},
				(reason: unknown) => {
					if (controller === own && !isAborted(reason)) loadError = reason;
				}
			)
			.finally(() => {
				if (controller !== own) return;
				controller = null;
				loading = false;
			});
	}

	onMount(() => {
		load();
		const unregister = router.registerLeaveGuard(() => dirty);
		return () => {
			unregister();
			controller?.abort();
			controller = null;
		};
	});

	$effect(() => {
		if (!dirty) return;
		const warn = (event: BeforeUnloadEvent) => {
			event.preventDefault();
			event.returnValue = '';
		};
		window.addEventListener('beforeunload', warn);
		return () => window.removeEventListener('beforeunload', warn);
	});

	async function save() {
		if (!ready || busy.value || loading) return;
		failure = null;
		const yaml = text;
		moved.beginWrite();
		let applied = true;
		saving = true;
		try {
			await busy.run(() => configsApi.updateRaw({ yaml }));
		} catch (reason) {
			if (!(reason instanceof ApiError && reason.saved)) {
				failure = writeFailure(reason);
				saving = false;
				return;
			}
			applied = false;
			toasts.warning(errorMessage(reason));
		}
		saving = false;
		// On disk from here on; whatever the re-read does, the text is not unsaved.
		baseline = yaml;
		if (applied) toasts.success(t('saved'));
		if (await moved.afterWrite('', applied)) return;
		try {
			const raw = await busy.run(() => configsApi.getRaw());
			if (text === yaml) adopt(raw.yaml);
		} catch (reason) {
			failure = { title: t('requestFailed'), message: errorMessage(reason) };
		}
	}

	async function reload() {
		if (!ready || busy.value || loading) return;
		if (dirty) {
			const ok = await confirm({
				message: t('confirmReload'),
				confirmLabel: t('reload'),
				danger: true
			});
			if (!ok || busy.value) return;
		}
		failure = null;
		try {
			const raw = await busy.run(() => configsApi.getRaw());
			adopt(raw.yaml);
			toasts.success(t('reloaded'));
		} catch (reason) {
			failure = { title: t('requestFailed'), message: errorMessage(reason) };
		}
	}
</script>

<PageHeader title={t('page.yaml')} description={null} />

{#if loadError}
	<Banner kind="error" title={errorMessage(loadError)} message={null}>
		<Button variant="secondary" onclick={load}>{t('retry')}</Button>
	</Banner>
{:else if ready}
	<form
		bind:this={form}
		novalidate
		autocomplete="off"
		aria-label={t('yaml')}
		aria-busy={busy.value}
		onsubmit={(event) => {
			event.preventDefault();
			void save();
		}}
		use:shortcut={{ key: 's', handler: () => form?.requestSubmit() }}
	>
		{#if failure}
			<Banner kind="error" title={failure.title} message={failure.message} />
		{/if}
		<fieldset class="min-w-0" disabled={busy.value}>
			<label for="yaml-source" class="field-label">{t('yamlSource')}</label>
			<textarea
				id="yaml-source"
				class="textarea min-h-[60vh] resize-y font-mono text-[13px] leading-5 whitespace-pre"
				bind:value={text}
				spellcheck="false"
				autocapitalize="off"
				wrap="off"></textarea>
		</fieldset>
		<FormActions {dirty} disabled={busy.value} {saving} sticky>
			{#snippet secondary()}
				<Button variant="secondary" disabled={busy.value} onclick={() => void reload()}>
					{t('reload')}
				</Button>
			{/snippet}
		</FormActions>
	</form>
{:else}
	<p class="text-sm text-fg-muted" aria-busy="true">{t('loading')}</p>
{/if}
