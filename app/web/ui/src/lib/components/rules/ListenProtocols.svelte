<script lang="ts">
	import { t } from '../../i18n.svelte';
	import {
		DEFAULT_TUN_ADDRESSES,
		DEFAULT_TUN_MTU,
		PROTOCOLS,
		tunAvailable,
		type DraftErrors,
		type ProtocolDraft,
		type ProtocolType,
		type RuleDraft
	} from '../../ruleEditor';
	import { fieldsOf, layoutFor } from '../../urlBuilder';
	import Field from '../ui/Field.svelte';
	import HelpTip from '../ui/HelpTip.svelte';

	interface Props {
		listen: RuleDraft['listen'];
		errors: DraftErrors;
	}

	let { listen = $bindable(), errors = $bindable() }: Props = $props();

	const ciphers =
		fieldsOf(layoutFor('shadowsocks')!).find((input) => input.name === 'encrypto')?.items ?? [];

	const ss = $derived(listen.protocols.find((row) => row.type === 'ss')!);
	// The Shadowsocks password error sits on the field it actually reads; the shared field also
	// answers for a shared password no username covers.
	const sharedPasswordError = $derived(
		(ss.custom ? undefined : errors.ssPassword) ?? errors.listenPassword
	);
	const rowError = (type: ProtocolType, name: 'username' | 'password') =>
		errors.protocolErrors?.[type]?.[name];
	const fieldError = (row: ProtocolDraft, name: 'username' | 'password') =>
		(row.type === 'ss' && name === 'password' && row.custom ? errors.ssPassword : undefined) ??
		rowError(row.type, name);
	const clearFieldError = (row: ProtocolDraft, name: 'username' | 'password') => {
		if (row.type === 'ss' && name === 'password') errors.ssPassword = undefined;
		const entry = errors.protocolErrors?.[row.type];
		if (entry) entry[name] = undefined;
	};
	const tunAllowed = $derived(tunAvailable(listen));
	// The TUN box also points at its own error and at why it is off.
	const boxDescribedBy = (row: ProtocolDraft) =>
		[
			errors.protocols && 'protocols-error',
			row.type === 'tun' && errors.tun && 'protocol-tun-error',
			row.type === 'tun' && !tunAllowed && 'protocol-tun-reason'
		]
			.filter(Boolean)
			.join(' ') || undefined;
</script>

<fieldset class="mt-4">
	<legend class="field-label">{t('protocols')}</legend>
	<div class="flex flex-wrap gap-x-5 gap-y-2">
		{#each listen.protocols as row, index (row.type)}
			{@const tun = row.type === 'tun'}
			<span class="inline-flex items-center gap-1">
				<label class="inline-flex h-8 items-center gap-2 text-sm">
					<input
						id="protocol-{row.type}"
						type="checkbox"
						class="size-4 accent-accent"
						bind:checked={row.enabled}
						disabled={tun && !tunAllowed}
						onchange={() => {
							errors.protocols = undefined;
							if (tun) errors.tun = undefined;
						}}
						aria-invalid={errors.protocols || (tun && errors.tun) ? true : undefined}
						aria-describedby={boxDescribedBy(row)}
					/>
					{PROTOCOLS[index].label}
				</label>
				{#if tun}
					<HelpTip concept={t('tunDevice')} text={t('help.tun')} />
				{/if}
			</span>
		{/each}
	</div>
	{#if !tunAllowed}
		<p id="protocol-tun-reason" class="mt-1 text-xs text-fg-muted">
			{t(listen.kind === 'virtual' ? 'tunNotWithVirtual' : 'tunNotWithWay')}
		</p>
	{/if}
	{#if errors.protocols}
		<p id="protocols-error" class="mt-1 text-xs text-danger">{t(errors.protocols)}</p>
	{/if}
</fieldset>
<div class="mt-4 grid gap-4 sm:grid-cols-2">
	<Field
		id="listen-username"
		label={t('username')}
		optional
		hint={t('credentialsHint')}
		error={errors.listenUsername ? t(errors.listenUsername) : null}
	>
		{#snippet children({ describedBy, invalid })}
			<input
				id="listen-username"
				class="input"
				type="text"
				bind:value={listen.username}
				oninput={() => (errors.listenUsername = undefined)}
				aria-invalid={invalid || undefined}
				aria-describedby={describedBy}
				spellcheck="false"
			/>
		{/snippet}
	</Field>
	<Field
		id="listen-password"
		label={t('password')}
		optional
		error={sharedPasswordError ? t(sharedPasswordError) : null}
	>
		{#snippet children({ describedBy, invalid })}
			<input
				id="listen-password"
				class="input"
				type="password"
				bind:value={listen.password}
				oninput={() => {
					errors.ssPassword = undefined;
					errors.listenPassword = undefined;
				}}
				aria-invalid={invalid || undefined}
				aria-describedby={describedBy}
				autocomplete="new-password"
			/>
		{/snippet}
	</Field>
</div>
{#each listen.protocols as row, index (row.type)}
	{#if row.enabled && (row.type !== 'tun' || tunAllowed)}
		<fieldset class="mt-4" aria-labelledby="protocol-{row.type}-heading">
			<legend
				class="flex w-full items-center justify-between gap-3 border-b border-line pb-2 text-sm font-medium"
			>
				<span id="protocol-{row.type}-heading">{PROTOCOLS[index].label}</span>
				{#if PROTOCOLS[index].fields.length}
					<label class="inline-flex h-8 shrink-0 items-center gap-2 font-normal">
						<input
							id="protocol-{row.type}-custom"
							type="checkbox"
							class="size-4 accent-accent"
							bind:checked={row.custom}
						/>
						{t('customCredentials')}
					</label>
				{/if}
			</legend>
			{#if row.type === 'tun'}
				<p class="mt-3 text-xs text-fg-muted">{t('tunHint')}</p>
				<div class="mt-3 grid gap-4 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
					<Field
						id="protocol-tun-address"
						label={t('tunAddress')}
						optional
						hint={t('tunAddressHint')}
						error={errors.tunAddress ? t(errors.tunAddress) : null}
					>
						{#snippet children({ describedBy, invalid })}
							<input
								id="protocol-tun-address"
								class="input font-mono text-[13px]"
								type="text"
								bind:value={row.address}
								oninput={() => (errors.tunAddress = undefined)}
								placeholder={DEFAULT_TUN_ADDRESSES.join(', ')}
								aria-invalid={invalid || undefined}
								aria-describedby={describedBy}
								spellcheck="false"
							/>
						{/snippet}
					</Field>
					<Field
						id="protocol-tun-mtu"
						label={t('tunMTU')}
						optional
						hint={t('tunMTUHint')}
						error={errors.tunMTU ? t(errors.tunMTU) : null}
					>
						{#snippet children({ describedBy, invalid })}
							<input
								id="protocol-tun-mtu"
								class="input font-mono text-[13px]"
								type="text"
								inputmode="numeric"
								bind:value={row.mtu}
								oninput={() => (errors.tunMTU = undefined)}
								placeholder={String(DEFAULT_TUN_MTU)}
								aria-invalid={invalid || undefined}
								aria-describedby={describedBy}
							/>
						{/snippet}
					</Field>
				</div>
				{#if errors.tun}
					<p id="protocol-tun-error" class="mt-3 text-xs text-danger">
						{t(errors.tun, { name: errors.tunOwner ?? '' })}
					</p>
				{/if}
			{:else}
				<div class="grid gap-4 sm:grid-cols-2" class:mt-3={row.custom || row.type === 'ss'}>
					{#if row.type === 'ss'}
						<Field
							id="protocol-ss-cipher"
							label={t('cipher')}
							error={errors.ssCipher ? t(errors.ssCipher) : null}
						>
							{#snippet children({ describedBy, invalid })}
								<select
									id="protocol-ss-cipher"
									class="input"
									bind:value={row.cipher}
									onchange={() => (errors.ssCipher = undefined)}
									aria-invalid={invalid || undefined}
									aria-describedby={describedBy}
								>
									{#if !row.cipher}
										<option value=""></option>
									{:else if !ciphers.includes(row.cipher)}
										<option value={row.cipher}>{row.cipher}</option>
									{/if}
									{#each ciphers as item (item)}
										<option value={item}>{item}</option>
									{/each}
								</select>
							{/snippet}
						</Field>
					{/if}
					{#if row.custom}
						{#each PROTOCOLS[index].fields as name (name)}
							{@const error = fieldError(row, name)}
							<Field
								id="protocol-{row.type}-{name}"
								label={t(name)}
								error={error ? t(error) : null}
							>
								{#snippet children({ describedBy, invalid })}
									<input
										id="protocol-{row.type}-{name}"
										class="input"
										type={name === 'password' ? 'password' : 'text'}
										bind:value={row[name]}
										oninput={() => clearFieldError(row, name)}
										aria-invalid={invalid || undefined}
										aria-describedby={describedBy}
										autocomplete={name === 'password' ? 'new-password' : undefined}
										spellcheck="false"
									/>
								{/snippet}
							</Field>
						{/each}
					{/if}
				</div>
			{/if}
		</fieldset>
	{/if}
{/each}
