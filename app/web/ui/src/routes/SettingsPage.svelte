<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { shortcut } from '../lib/actions/shortcut';
	import { ApiError, configsApi, errorMessage, isAborted } from '../lib/api';
	import { busy } from '../lib/busy.svelte';
	import Banner from '../lib/components/ui/Banner.svelte';
	import Button from '../lib/components/ui/Button.svelte';
	import Field from '../lib/components/ui/Field.svelte';
	import FormActions from '../lib/components/ui/FormActions.svelte';
	import PageHeader from '../lib/components/ui/PageHeader.svelte';
	import Skeleton from '../lib/components/ui/Skeleton.svelte';
	import { t } from '../lib/i18n.svelte';
	import { moved } from '../lib/moved.svelte';
	import { router } from '../lib/router.svelte';
	import {
		parsePort,
		reachableAddress,
		readLines,
		writeFailure,
		type WriteFailure
	} from '../lib/settings';
	import { status } from '../lib/status.svelte';
	import { toasts } from '../lib/toast.svelte';
	import { list, type Address, type NoProxy } from '../lib/types';

	// Two resources, two forms: saving one never touches the other's draft or dirty state.

	interface AddressDraft {
		host: string;
		port: string;
	}

	interface BypassDraft {
		list: string;
		from_env: string;
		from_file: string;
	}

	let address = $state<AddressDraft>({ host: '', port: '' });
	let addressBaseline = $state('');
	let bypass = $state<BypassDraft>({ list: '', from_env: '', from_file: '' });
	let bypassBaseline = $state('');
	let ready = $state(false);
	let loading = $state(false);
	let loadError = $state.raw<unknown>(null);
	let portInvalid = $state(false);
	let savingAddress = $state(false);
	let savingBypass = $state(false);
	let addressFailure = $state<WriteFailure | null>(null);
	let bypassFailure = $state<WriteFailure | null>(null);
	let addressForm = $state<HTMLFormElement | null>(null);
	let bypassForm = $state<HTMLFormElement | null>(null);
	let controller: AbortController | null = null;

	const addressSnapshot = (draft: AddressDraft) =>
		JSON.stringify({ host: draft.host.trim(), port: draft.port.trim() });
	const readBypass = (draft: BypassDraft): NoProxy => ({
		list: readLines(draft.list),
		from_env: readLines(draft.from_env),
		from_file: readLines(draft.from_file)
	});
	const bypassSnapshot = (draft: BypassDraft) => JSON.stringify(readBypass(draft));

	const addressDirty = $derived(ready && addressSnapshot(address) !== addressBaseline);
	const bypassDirty = $derived(ready && bypassSnapshot(bypass) !== bypassBaseline);
	const dirty = $derived(addressDirty || bypassDirty);

	function adopt(web: Address, noProxy: NoProxy) {
		address = { host: web.host ?? '', port: String(web.port ?? 0) };
		addressBaseline = addressSnapshot(address);
		bypass = {
			list: list(noProxy.list).join('\n'),
			from_env: list(noProxy.from_env).join('\n'),
			from_file: list(noProxy.from_file).join('\n')
		};
		bypassBaseline = bypassSnapshot(bypass);
		ready = true;
	}

	function load() {
		controller?.abort();
		const own = new AbortController();
		controller = own;
		loading = true;
		loadError = null;
		Promise.all([configsApi.getWebUI(own.signal), configsApi.getNoProxy(own.signal)])
			.then(
				([web, noProxy]) => {
					if (controller === own) adopt(web, noProxy);
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

	async function saveAddress() {
		if (!ready || busy.value || loading) return;
		const port = parsePort(address.port);
		if (port === null) {
			portInvalid = true;
			await tick();
			addressForm?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus();
			return;
		}
		addressFailure = null;
		const body: Address = { host: address.host.trim(), port };
		const candidate = reachableAddress(body, location.hostname);
		moved.beginWrite();
		let applied = true;
		savingAddress = true;
		try {
			await busy.run(() => configsApi.updateWebUI(body));
		} catch (reason) {
			savingAddress = false;
			if (reason instanceof ApiError && reason.unreachable) {
				if (!moved.uncertain(candidate)) addressFailure = writeFailure(reason);
				return;
			}
			if (!(reason instanceof ApiError && reason.saved)) {
				addressFailure = writeFailure(reason);
				return;
			}
			applied = false;
			toasts.warning(errorMessage(reason));
		}
		savingAddress = false;
		addressBaseline = addressSnapshot(address);
		if (applied) toasts.success(t('saved'));
		await moved.afterWrite(candidate, applied);
	}

	async function saveBypass() {
		if (!ready || busy.value || loading) return;
		bypassFailure = null;
		const body = readBypass(bypass);
		let applied = true;
		savingBypass = true;
		try {
			await busy.run(() => configsApi.updateNoProxy(body));
		} catch (reason) {
			savingBypass = false;
			if (!(reason instanceof ApiError && reason.saved)) {
				bypassFailure = writeFailure(reason);
				return;
			}
			applied = false;
			toasts.warning(errorMessage(reason));
		}
		savingBypass = false;
		bypassBaseline = bypassSnapshot(bypass);
		if (applied) toasts.success(t('saved'));
		void status.refresh();
	}

	// Ctrl/Cmd+S saves the form owning the focus, else the only dirty one; ambiguity saves nothing.
	function saveFocused() {
		const active = document.activeElement;
		const forms = [addressForm, bypassForm];
		const focused = forms.find((form) => form && active && form.contains(active));
		const target =
			focused ?? (addressDirty !== bypassDirty ? (addressDirty ? addressForm : bypassForm) : null);
		target?.requestSubmit();
	}
</script>

<div use:shortcut={{ key: 's', handler: saveFocused }}>
	<PageHeader title={t('page.settings')} description={null} />

	{#if loadError}
		<Banner kind="error" title={errorMessage(loadError)} message={null}>
			<Button variant="secondary" onclick={load}>{t('retry')}</Button>
		</Banner>
	{:else if ready}
		<form
			bind:this={addressForm}
			novalidate
			autocomplete="off"
			aria-labelledby="web-ui-heading"
			aria-busy={busy.value}
			onsubmit={(event) => {
				event.preventDefault();
				void saveAddress();
			}}
		>
			<fieldset class="band pt-0" disabled={busy.value}>
				<legend id="web-ui-heading" class="band-title">{t('webUI')}</legend>
				{#if addressFailure}
					<Banner kind="error" title={addressFailure.title} message={addressFailure.message} />
				{/if}
				<div class="grid gap-4 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
					<Field id="web-ui-host" label={t('host')} hint={t('webUIHint')}>
						{#snippet children({ describedBy })}
							<input
								id="web-ui-host"
								class="input font-mono text-[13px]"
								type="text"
								bind:value={address.host}
								placeholder="127.0.0.1"
								aria-describedby={describedBy}
								spellcheck="false"
							/>
						{/snippet}
					</Field>
					<Field id="web-ui-port" label={t('port')} error={portInvalid ? t('invalidPort') : null}>
						{#snippet children({ describedBy, invalid })}
							<input
								id="web-ui-port"
								class="input font-mono text-[13px]"
								type="text"
								inputmode="numeric"
								bind:value={address.port}
								oninput={() => (portInvalid = false)}
								required
								aria-invalid={invalid || undefined}
								aria-describedby={describedBy}
							/>
						{/snippet}
					</Field>
				</div>
				<FormActions dirty={addressDirty} disabled={busy.value} saving={savingAddress} />
			</fieldset>
		</form>

		<form
			bind:this={bypassForm}
			novalidate
			autocomplete="off"
			aria-labelledby="no-proxy-heading"
			aria-busy={busy.value}
			onsubmit={(event) => {
				event.preventDefault();
				void saveBypass();
			}}
		>
			<fieldset class="band" disabled={busy.value}>
				<legend id="no-proxy-heading" class="band-title">{t('noProxy')}</legend>
				{#if bypassFailure}
					<Banner kind="error" title={bypassFailure.title} message={bypassFailure.message} />
				{/if}
				<div class="grid gap-4 md:grid-cols-3">
					<Field id="no-proxy-list" label={t('hostsCIDRs')} hint={t('listHint')}>
						{#snippet children({ describedBy })}
							<textarea
								id="no-proxy-list"
								class="textarea font-mono text-[13px]"
								rows="8"
								bind:value={bypass.list}
								aria-describedby={describedBy}
								spellcheck="false"></textarea>
						{/snippet}
					</Field>
					<Field id="no-proxy-env" label={t('fromEnv')} hint={t('envHint')}>
						{#snippet children({ describedBy })}
							<textarea
								id="no-proxy-env"
								class="textarea font-mono text-[13px]"
								rows="8"
								bind:value={bypass.from_env}
								aria-describedby={describedBy}
								spellcheck="false"></textarea>
						{/snippet}
					</Field>
					<Field id="no-proxy-files" label={t('fromFiles')} hint={t('filesHint')}>
						{#snippet children({ describedBy })}
							<textarea
								id="no-proxy-files"
								class="textarea font-mono text-[13px]"
								rows="8"
								bind:value={bypass.from_file}
								aria-describedby={describedBy}
								spellcheck="false"></textarea>
						{/snippet}
					</Field>
				</div>
				<FormActions dirty={bypassDirty} disabled={busy.value} saving={savingBypass} />
			</fieldset>
		</form>
	{:else}
		{#snippet fieldSkeleton(height: string)}
			<div class="min-w-0">
				<Skeleton class="mb-1.5 h-3 w-20" />
				<Skeleton class="w-full rounded-md {height}" />
			</div>
		{/snippet}
		<div aria-busy="true">
			<span class="sr-only">{t('loading')}</span>
			<div class="band pt-0">
				<Skeleton class="mb-3 h-5 w-20" />
				<div class="grid gap-4 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
					{@render fieldSkeleton('h-8')}
					{@render fieldSkeleton('h-8')}
				</div>
			</div>
			<div class="band">
				<Skeleton class="mb-3 h-5 w-20" />
				<div class="grid gap-4 md:grid-cols-3">
					{@render fieldSkeleton('h-44')}
					{@render fieldSkeleton('h-44')}
					{@render fieldSkeleton('h-44')}
				</div>
			</div>
		</div>
	{/if}
</div>
