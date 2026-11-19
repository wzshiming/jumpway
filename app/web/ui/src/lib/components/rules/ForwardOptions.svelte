<script lang="ts">
	import { t } from '../../i18n.svelte';
	import type { DraftErrors, RuleDraft } from '../../ruleEditor';
	import Field from '../ui/Field.svelte';

	interface Props {
		forward: RuleDraft['forward'];
		errors: DraftErrors;
	}

	let { forward = $bindable(), errors = $bindable() }: Props = $props();

	// DNS mode udp ignores the server and idle; they keep their drafts but are neither sent nor checked.
	const dnsUDP = $derived(forward.dns.mode === 'udp');
</script>

<fieldset class="mt-5">
	<legend class="field-label">{t('udp')}</legend>
	<div class="grid items-start gap-4 sm:grid-cols-3">
		<Field id="udp-mode" label={t('udpMode')} hint={t('udpModeHint')}>
			{#snippet children({ describedBy })}
				<select
					id="udp-mode"
					class="input"
					bind:value={forward.udp.mode}
					aria-describedby={describedBy}
				>
					<option value="">{t('optionDefault')}</option>
					<option value="proxy">{t('udpMode.proxy')}</option>
					<option value="direct">{t('udpMode.direct')}</option>
					<option value="block">{t('udpMode.block')}</option>
				</select>
			{/snippet}
		</Field>
		<Field id="udp-nat" label={t('udpNat')} hint={t('udpNatHint')}>
			{#snippet children({ describedBy })}
				<select
					id="udp-nat"
					class="input"
					bind:value={forward.udp.nat}
					aria-describedby={describedBy}
				>
					<option value="">{t('udpNat.auto')}</option>
					<option value="full-cone">{t('udpNat.fullCone')}</option>
					<option value="symmetric">{t('udpNat.symmetric')}</option>
				</select>
			{/snippet}
		</Field>
		<Field
			id="udp-idle"
			label={t('udpIdle')}
			hint={t('udpIdleHint')}
			error={errors.udpIdle ? t(errors.udpIdle) : null}
		>
			{#snippet children({ describedBy, invalid })}
				<input
					id="udp-idle"
					class="input font-mono text-[13px]"
					type="text"
					inputmode="numeric"
					placeholder="60"
					bind:value={forward.udp.idle}
					oninput={() => (errors.udpIdle = undefined)}
					aria-invalid={invalid || undefined}
					aria-describedby={describedBy}
				/>
			{/snippet}
		</Field>
	</div>
</fieldset>
<fieldset class="mt-5">
	<legend class="field-label">{t('dns')}</legend>
	<div class="grid items-start gap-4 sm:grid-cols-3">
		<Field id="dns-mode" label={t('dnsMode')} hint={t('dnsModeHint')}>
			{#snippet children({ describedBy })}
				<select
					id="dns-mode"
					class="input"
					bind:value={forward.dns.mode}
					onchange={() => {
						errors.dnsServer = undefined;
						errors.dnsIdle = undefined;
					}}
					aria-describedby={describedBy}
				>
					<option value="">{t('dnsMode.default')}</option>
					<option value="tcp">{t('dnsMode.tcp')}</option>
					<option value="udp">{t('dnsMode.udp')}</option>
				</select>
			{/snippet}
		</Field>
		<Field
			id="dns-server"
			label={t('dnsServer')}
			hint={t('dnsServerHint')}
			error={errors.dnsServer ? t(errors.dnsServer) : null}
		>
			{#snippet children({ describedBy, invalid })}
				<input
					id="dns-server"
					class="input font-mono text-[13px]"
					type="text"
					placeholder={t('dnsServerPlaceholder')}
					bind:value={forward.dns.server}
					oninput={() => (errors.dnsServer = undefined)}
					disabled={dnsUDP}
					aria-invalid={invalid || undefined}
					aria-describedby={describedBy}
					autocapitalize="off"
					spellcheck="false"
				/>
			{/snippet}
		</Field>
		<Field
			id="dns-idle"
			label={t('dnsIdle')}
			hint={t('dnsIdleHint')}
			error={errors.dnsIdle ? t(errors.dnsIdle) : null}
		>
			{#snippet children({ describedBy, invalid })}
				<input
					id="dns-idle"
					class="input font-mono text-[13px]"
					type="text"
					inputmode="numeric"
					placeholder="10"
					bind:value={forward.dns.idle}
					oninput={() => (errors.dnsIdle = undefined)}
					disabled={dnsUDP}
					aria-invalid={invalid || undefined}
					aria-describedby={describedBy}
				/>
			{/snippet}
		</Field>
	</div>
</fieldset>
