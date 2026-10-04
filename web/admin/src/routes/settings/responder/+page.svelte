<script lang="ts">
	import { onMount } from 'svelte'
	import Main from '$lib/layouts/Main.svelte'
	import { PageHeader, FormButton, FormInput, FormSelect, FormToggle, Section } from '$lib/components'
	import { loadResponderSettings, saveResponderSettings, testXMPPConnection } from '$lib/utils/responder'
	import { translate } from '$lib/i18n'

	let t = $derived($translate)

	let settings = $state({
		enabled: false,
		xmpp_jid: '',
		xmpp_password: '',
		xmpp_server: '',
		xmpp_port: 0,
		xmpp_connect_addr: '',
		xmpp_connection_mode: 'auto',
		xmpp_websocket_url: '',
		xmpp_bosh_url: ''
	})
	let saving = $state(false)
	let testing = $state(false)
	let testResult = $state<string | null>(null)
	let testSuccess = $state(false)

	onMount(async () => {
		const response = await loadResponderSettings()
		if (response.success && response.result) {
			settings = { ...settings, ...response.result }
		}
	})

	async function handleSave() {
		saving = true
		testResult = null
		try {
			await saveResponderSettings(settings)
			alert(t('responder.settingsSaved'))
		} catch (err) {
			console.error('Failed to save settings:', err)
			alert(t('responder.failedToSave'))
		} finally {
			saving = false
		}
	}

	async function handleTest() {
		testing = true
		testResult = null
		try {
			const response = await testXMPPConnection(settings)
			// Check result.success (actual XMPP connection), not top-level success (HTTP status)
			testSuccess = response.result?.success ?? false
			testResult = testSuccess ? t('responder.connectionSuccessful') : (response.result?.error || t('responder.connectionFailed'))
		} catch (err) {
			console.error('Connection test failed:', err)
			testSuccess = false
			testResult = t('responder.connectionFailed')
		} finally {
			testing = false
		}
	}
</script>

<Main>
	<PageHeader title={t('menu.responder')} />

	<Section>
		<h2 class="text-lg font-medium mb-1">{t('responder.xmppConfiguration')}</h2>
		<p class="text-sm text-gray-600 mb-4">{t('responder.xmppConfigDesc')}</p>

		<form onsubmit={(e) => { e.preventDefault(); handleSave() }} class="max-w-2xl space-y-4">
			<FormToggle
				id="xmpp-service-enabled"
				title={t('responder.useXmppService')}
				bind:value={settings.enabled}
			/>

			<FormInput
				id="xmpp_jid"
				title={t('responder.xmppJid')}
				bind:value={settings.xmpp_jid}
				placeholder="bot@example.com"
				ico="at-symbol"
			/>

			<FormInput
				id="xmpp_password"
				title={t('responder.xmppPassword')}
				type="password"
				bind:value={settings.xmpp_password}
				ico="lock-closed"
			/>

			<FormSelect
				id="xmpp_connection_mode"
				title="Connection Mode"
				bind:value={settings.xmpp_connection_mode}
			>
				<option value="auto">Auto (detect from settings)</option>
				<option value="direct-tls">Direct TLS + ALPN (XEP-0368, port 443)</option>
				<option value="starttls">STARTTLS (traditional, port 5222)</option>
				<option value="websocket">WebSocket (RFC 7395)</option>
				<option value="bosh">BOSH / HTTP Binding (XEP-0206)</option>
			</FormSelect>

			{#if settings.xmpp_connection_mode === 'websocket'}
				<FormInput
					id="xmpp_websocket_url"
					title="WebSocket URL"
					bind:value={settings.xmpp_websocket_url}
					placeholder="wss://example.com/xmpp-websocket"
					ico="link"
				/>
			{:else if settings.xmpp_connection_mode === 'bosh'}
				<FormInput
					id="xmpp_bosh_url"
					title="BOSH Endpoint URL"
					bind:value={settings.xmpp_bosh_url}
					placeholder="https://example.com/http-bind"
					ico="link"
				/>
			{:else}
				<div class="grid grid-cols-2 gap-4">
					<FormInput
						id="xmpp_server"
						title={t('responder.xmppServer')}
						bind:value={settings.xmpp_server}
						placeholder="Optional: defaults to JID domain"
						ico="server"
					/>

					<FormInput
						id="xmpp_port"
						title={t('responder.xmppPort')}
						type="number"
						bind:value={settings.xmpp_port}
						placeholder="Optional: defaults to 443"
						ico="hashtag"
					/>
				</div>

				<FormInput
					id="xmpp_connect_addr"
					title={t('responder.xmppConnectAddr')}
					bind:value={settings.xmpp_connect_addr}
					placeholder="Optional: override connection address"
					ico="link"
				/>
			{/if}

			<div class="flex gap-2 pt-4">
				<FormButton
					name={testing ? t('responder.testing') : t('responder.testConnection')}
					variant="secondary"
					disabled={testing}
					onclick={handleTest}
					type="button"
				/>
				<FormButton
					name={saving ? t('common.save') + '...' : t('common.save')}
					variant="primary"
					disabled={saving}
					type="submit"
				/>
			</div>

			{#if testResult}
				<div class="test-result" class:success={testSuccess}>
					{testResult}
				</div>
			{/if}
		</form>
	</Section>
</Main>

<style>
	.test-result {
		margin-top: 1rem;
		padding: 0.75rem;
		border-radius: 0.25rem;
		background: #fee2e2;
		color: #991b1b;
	}

	.test-result.success {
		background: #d1fae5;
		color: #065f46;
	}
</style>
