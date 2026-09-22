<script lang="ts">
	import { onMount, tick } from 'svelte';
	import IconChevronRight from '~icons/lucide/chevron-right';
	import { shortcut } from '../lib/actions/shortcut';
	import { ApiError, configsApi, errorMessage, isAborted } from '../lib/api';
	import { busy } from '../lib/busy.svelte';
	import ChainFlow from '../lib/components/rules/ChainFlow.svelte';
	import HopEditor from '../lib/components/rules/HopEditor.svelte';
	import ListenProtocols from '../lib/components/rules/ListenProtocols.svelte';
	import UrlBuilderDialog from '../lib/components/rules/UrlBuilderDialog.svelte';
	import VirtualPeers from '../lib/components/rules/VirtualPeers.svelte';
	import Banner from '../lib/components/ui/Banner.svelte';
	import Button from '../lib/components/ui/Button.svelte';
	import Field from '../lib/components/ui/Field.svelte';
	import FormActions from '../lib/components/ui/FormActions.svelte';
	import PageHeader from '../lib/components/ui/PageHeader.svelte';
	import { confirm } from '../lib/confirm';
	import { t } from '../lib/i18n.svelte';
	import { router } from '../lib/router.svelte';
	import { HOME_ROUTE, ruleRoute } from '../lib/routes';
	import {
		chainSummary,
		EXIT_ROLES,
		hostPort,
		LISTEN_ROLES,
		virtualAddress,
		virtualChannels
	} from '../lib/rule';
	import {
		draftFrom,
		emptyDraft,
		readRule,
		snapshot,
		wayOf,
		type DraftErrors,
		type RuleDraft
	} from '../lib/ruleEditor';
	import { status } from '../lib/status.svelte';
	import { toasts } from '../lib/toast.svelte';
	import { list, type Rule } from '../lib/types';

	// null edits a new rule at #/new.
	interface Props {
		name: string | null;
	}

	let { name }: Props = $props();

	// The name the backend knows the rule by; set once a new rule is first saved so a retry is a PUT.
	// App remounts this page per route name, so only the initial prop matters.
	// svelte-ignore state_referenced_locally
	let savedName = $state<string | null>(name);
	// Bound by the form; `ready` gates it until the stored rule has been copied in.
	let draft = $state<RuleDraft>(emptyDraft());
	let ready = $state(false);
	let baseline = $state('');
	let loading = $state(false);
	let saving = $state(false);
	let loadError = $state.raw<unknown>(null);
	let saveError = $state<string | null>(null);
	let errors = $state<DraftErrors>({});
	let form = $state<HTMLFormElement | null>(null);
	let builder = $state<ReturnType<typeof UrlBuilderDialog>>();
	let building = false;
	let controller: AbortController | null = null;
	// All rules, for virtual peer links and channel suggestions; null while unknown.
	let peerRules = $state.raw<Rule[] | null>(null);
	let peerController: AbortController | null = null;

	const dirty = $derived(ready && snapshot(draft) !== baseline);
	const forwarding = $derived(draft.mode === 'forward');
	const listenVirtual = $derived(draft.listen.kind === 'virtual');
	const targetVirtual = $derived(forwarding && draft.target.kind === 'virtual');
	const title = $derived(savedName ?? t('newRule'));
	const listenLabel = $derived(
		listenVirtual
			? virtualAddress(draft.listen.virtual.trim())
			: hostPort(draft.listen.host.trim(), draft.listen.port.trim() || '0')
	);
	const targetLabel = $derived(
		targetVirtual
			? virtualAddress(draft.target.virtual.trim())
			: forwarding && draft.target.port.trim()
				? hostPort(draft.target.host.trim(), draft.target.port.trim())
				: ''
	);
	const listenWay = $derived(listenVirtual ? [] : wayOf(draft.listen.way));
	const forwardWay = $derived(targetVirtual ? [] : wayOf(draft.forward.way));
	const self = $derived(savedName ?? '');
	const listenChannels = $derived(virtualChannels(peerRules ?? [], 'listen', self));
	const targetChannels = $derived(virtualChannels(peerRules ?? [], 'forward', self));
	const notFound = $derived(
		loadError instanceof ApiError && loadError.status === 400 && /not found/.test(loadError.message)
	);

	function adopt(next: RuleDraft) {
		draft = next;
		baseline = snapshot(next);
		ready = true;
	}

	function loadPeers() {
		peerController?.abort();
		const own = new AbortController();
		peerController = own;
		configsApi.listRules(own.signal).then(
			(value) => {
				if (peerController === own) peerRules = list(value);
			},
			(reason: unknown) => {
				if (peerController === own && !isAborted(reason)) toasts.error(errorMessage(reason));
			}
		);
	}

	function load() {
		controller?.abort();
		controller = null;
		loadError = null;
		saveError = null;
		errors = {};
		loadPeers();
		if (savedName === null) {
			adopt(emptyDraft());
			return;
		}
		const own = new AbortController();
		controller = own;
		loading = true;
		configsApi
			.getRule(savedName, own.signal)
			.then(
				(rule) => {
					if (controller === own) adopt(draftFrom(rule));
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
			peerController?.abort();
			peerController = null;
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

	const markClean = () => {
		baseline = snapshot(draft);
	};

	async function save() {
		if (!ready || busy.value || loading || building) return;
		const result = readRule(draft);
		if (!result.ok) {
			errors = result.errors;
			await tick();
			form?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus();
			return;
		}
		errors = {};
		saveError = null;
		const rule = result.rule;
		const previous = savedName;
		let applied = true;
		saving = true;
		try {
			await busy.run(() =>
				previous === null ? configsApi.createRule(rule) : configsApi.updateRule(previous, rule)
			);
		} catch (reason) {
			if (!(reason instanceof ApiError && reason.saved)) {
				saveError = errorMessage(reason);
				saving = false;
				return;
			}
			// On disk but not applied: the rule exists under its new name, so a retry must not POST again.
			applied = false;
			toasts.warning(errorMessage(reason));
		}
		saving = false;
		markClean();
		savedName = rule.name;
		if (peerRules) peerRules = [...peerRules.filter((entry) => entry.name !== previous), rule];
		if (applied) toasts.success(t('saved'));
		void status.refresh();
		const hash = ruleRoute(rule.name);
		if (router.route.hash !== hash) await router.navigate(hash, { replace: true });
	}

	async function remove() {
		const current = savedName;
		if (!ready || current === null || busy.value || building) return;
		const ok = await confirm({
			message: t('confirmDeleteRule', { name: current }),
			confirmLabel: t('deleteRule'),
			danger: true
		});
		if (!ok) return;
		saveError = null;
		let applied = true;
		try {
			await busy.run(() => configsApi.deleteRule(current));
		} catch (reason) {
			if (!(reason instanceof ApiError && reason.saved)) {
				saveError = errorMessage(reason);
				return;
			}
			applied = false;
			toasts.warning(errorMessage(reason));
		}
		markClean();
		if (applied) toasts.success(t('deleted'));
		void status.refresh();
		await router.navigate(HOME_ROUTE, { replace: true });
	}

	async function openBuilder(current: string): Promise<string | null> {
		if (building || busy.value || !builder) return null;
		building = true;
		try {
			return await builder.open(current);
		} finally {
			building = false;
		}
	}
</script>

<nav aria-label={t('breadcrumb')} class="mb-2 flex items-center gap-1 text-sm text-fg-muted">
	<a href={HOME_ROUTE} class="hover:text-fg">{t('page.overview')}</a>
	<IconChevronRight class="size-3.5" aria-hidden="true" />
	<span aria-current="page" class="truncate text-fg">{title}</span>
</nav>
<PageHeader {title} description={null} />

{#if loadError}
	<Banner
		kind="error"
		title={notFound ? t('ruleNotFound', { name: savedName ?? '' }) : errorMessage(loadError)}
		message={null}
	>
		{#if !notFound}
			<Button variant="secondary" onclick={load}>{t('retry')}</Button>
		{/if}
		<a href={HOME_ROUTE} class="btn btn-ghost">{t('page.overview')}</a>
	</Banner>
{:else if ready}
	<form
		bind:this={form}
		novalidate
		autocomplete="off"
		aria-busy={busy.value}
		onsubmit={(event) => {
			event.preventDefault();
			void save();
		}}
		use:shortcut={{ key: 's', handler: () => form?.requestSubmit() }}
	>
		{#if saveError}
			<Banner kind="error" title={t('requestFailed')} message={saveError} />
		{/if}

		<fieldset class="band pt-0" disabled={busy.value}>
			<legend class="band-title">{t('general')}</legend>
			<div class="grid items-end gap-4 sm:grid-cols-[minmax(0,1fr)_auto_auto_auto]">
				<Field id="rule-name" label={t('rule')} error={errors.name ? t(errors.name) : null}>
					{#snippet children({ describedBy, invalid })}
						<input
							id="rule-name"
							class="input"
							type="text"
							bind:value={draft.name}
							oninput={() => (errors.name = undefined)}
							required
							aria-invalid={invalid || undefined}
							aria-describedby={describedBy}
							spellcheck="false"
						/>
					{/snippet}
				</Field>
				<label class="inline-flex h-8 items-center gap-2 text-sm">
					<input
						type="checkbox"
						role="switch"
						class="size-4 accent-accent"
						bind:checked={draft.enabled}
					/>
					{t('enabled')}
				</label>
				<fieldset>
					<legend class="field-label">{t('mode')}</legend>
					<div class="segment">
						<label>
							<input type="radio" name="mode" value="proxy" bind:group={draft.mode} />
							{t('proxy')}
						</label>
						<label>
							<input type="radio" name="mode" value="forward" bind:group={draft.mode} />
							{t('portForward')}
						</label>
					</div>
				</fieldset>
				<fieldset>
					<legend class="field-label">{t('endpointKind')}</legend>
					<div class="segment">
						<label>
							<input
								type="radio"
								name="listen-kind"
								value="address"
								bind:group={draft.listen.kind}
							/>
							{t('address')}
						</label>
						<label>
							<input
								type="radio"
								name="listen-kind"
								value="virtual"
								bind:group={draft.listen.kind}
							/>
							{t('virtual')}
						</label>
					</div>
				</fieldset>
			</div>
		</fieldset>

		<section class="band" aria-labelledby="chain-heading">
			<h2 id="chain-heading" class="band-title">{t('chain')}</h2>
			<ChainFlow {listenWay} {forwardWay} target={targetLabel} address={listenLabel} />
		</section>

		<fieldset class="band mt-5" disabled={busy.value}>
			<legend class="band-title">{t('listen')}</legend>
			{#if listenVirtual}
				<div>
					<Field
						id="listen-virtual"
						label={t('virtualChannel')}
						error={errors.listenVirtual ? t(errors.listenVirtual) : null}
					>
						{#snippet children({ describedBy, invalid })}
							<input
								id="listen-virtual"
								class="input font-mono text-[13px]"
								type="text"
								list="listen-virtual-list"
								bind:value={draft.listen.virtual}
								oninput={() => (errors.listenVirtual = undefined)}
								required
								aria-invalid={invalid || undefined}
								aria-describedby={describedBy}
								autocapitalize="off"
								spellcheck="false"
							/>
							<datalist id="listen-virtual-list">
								{#each listenChannels as channel (channel)}
									<option value={channel}></option>
								{/each}
							</datalist>
						{/snippet}
					</Field>
					<VirtualPeers side="listen" channel={draft.listen.virtual} rules={peerRules} {self} />
				</div>
			{:else}
				<div class="grid gap-4 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
					<Field id="listen-host" label={t('host')}>
						{#snippet children({ describedBy })}
							<input
								id="listen-host"
								class="input font-mono text-[13px]"
								type="text"
								bind:value={draft.listen.host}
								placeholder="127.0.0.1"
								aria-describedby={describedBy}
								spellcheck="false"
							/>
						{/snippet}
					</Field>
					<Field
						id="listen-port"
						label={t('port')}
						error={errors.listenPort ? t(errors.listenPort) : null}
					>
						{#snippet children({ describedBy, invalid })}
							<input
								id="listen-port"
								class="input font-mono text-[13px]"
								type="text"
								inputmode="numeric"
								bind:value={draft.listen.port}
								oninput={() => (errors.listenPort = undefined)}
								required
								aria-invalid={invalid || undefined}
								aria-describedby={describedBy}
							/>
						{/snippet}
					</Field>
				</div>
			{/if}
			{#if forwarding}
				<p class="mt-3 text-xs text-fg-muted">{t('credentialsProxyOnly')}</p>
			{:else}
				<ListenProtocols bind:listen={draft.listen} bind:errors />
			{/if}
			{#if !listenVirtual}
				<div class="mt-5">
					<h3 class="mb-2 text-sm font-medium">{t('listenThrough')}</h3>
					<HopEditor
						bind:hops={draft.listen.way}
						idPrefix="listen"
						roles={LISTEN_ROLES}
						hint="listenThroughHint"
						build={openBuilder}
					/>
				</div>
			{/if}
		</fieldset>

		<fieldset class="band mt-5" disabled={busy.value}>
			<legend class="band-title">{t('exit')}</legend>
			{#if forwarding}
				<fieldset>
					<legend class="field-label">{t('endpointKind')}</legend>
					<div class="segment">
						<label>
							<input
								type="radio"
								name="target-kind"
								value="address"
								bind:group={draft.target.kind}
							/>
							{t('address')}
						</label>
						<label>
							<input
								type="radio"
								name="target-kind"
								value="virtual"
								bind:group={draft.target.kind}
							/>
							{t('virtual')}
						</label>
					</div>
				</fieldset>
			{/if}
			{#if targetVirtual}
				<div class="mt-4">
					<Field
						id="target-virtual"
						label={t('virtualChannel')}
						error={errors.targetVirtual ? t(errors.targetVirtual) : null}
					>
						{#snippet children({ describedBy, invalid })}
							<input
								id="target-virtual"
								class="input font-mono text-[13px]"
								type="text"
								list="target-virtual-list"
								bind:value={draft.target.virtual}
								oninput={() => (errors.targetVirtual = undefined)}
								required
								aria-invalid={invalid || undefined}
								aria-describedby={describedBy}
								autocapitalize="off"
								spellcheck="false"
							/>
							<datalist id="target-virtual-list">
								{#each targetChannels as channel (channel)}
									<option value={channel}></option>
								{/each}
							</datalist>
						{/snippet}
					</Field>
					<VirtualPeers side="forward" channel={draft.target.virtual} rules={peerRules} {self} />
				</div>
			{:else if forwarding}
				<div class="mt-4 grid gap-4 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
					<Field id="target-host" label={t('targetHost')} hint={t('onExitNode')}>
						{#snippet children({ describedBy })}
							<input
								id="target-host"
								class="input font-mono text-[13px]"
								type="text"
								bind:value={draft.target.host}
								placeholder="127.0.0.1"
								aria-describedby={describedBy}
								spellcheck="false"
							/>
						{/snippet}
					</Field>
					<Field
						id="target-port"
						label={t('targetPort')}
						error={errors.targetPort ? t(errors.targetPort) : null}
					>
						{#snippet children({ describedBy, invalid })}
							<input
								id="target-port"
								class="input font-mono text-[13px]"
								type="text"
								inputmode="numeric"
								bind:value={draft.target.port}
								oninput={() => (errors.targetPort = undefined)}
								required
								aria-invalid={invalid || undefined}
								aria-describedby={describedBy}
							/>
						{/snippet}
					</Field>
				</div>
			{/if}
			{#if !targetVirtual}
				<div class:mt-5={forwarding}>
					<h3 class="mb-1 text-sm font-medium">{t('exitChain')}</h3>
					<p class="mb-2 text-xs text-fg-muted" data-chain-summary>
						{chainSummary(forwardWay.length, targetLabel)}
					</p>
					<HopEditor
						bind:hops={draft.forward.way}
						idPrefix="exit"
						roles={EXIT_ROLES}
						hint="hopsHint"
						build={openBuilder}
					/>
				</div>
			{/if}
		</fieldset>

		<FormActions {dirty} disabled={busy.value || loading} {saving} sticky>
			{#snippet secondary()}
				<Button
					variant="secondary"
					disabled={busy.value || loading}
					onclick={() => void router.navigate(HOME_ROUTE)}
				>
					{t('cancel')}
				</Button>
				{#if savedName !== null}
					<Button variant="danger" disabled={busy.value} onclick={() => void remove()}>
						{t('deleteRule')}
					</Button>
				{/if}
			{/snippet}
		</FormActions>
	</form>
	<UrlBuilderDialog bind:this={builder} />
{:else}
	<p class="text-sm text-fg-muted" aria-busy="true">{t('loading')}</p>
{/if}
