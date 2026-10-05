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
		xmpp_port: 0
	})

	let autoWebSocketURL = $derived(`wss://${settings.xmpp_server || extractDomain(settings.xmpp_jid)}/ws`)
	let autoBOSHURL = $derived(`https://${settings.xmpp_server || extractDomain(settings.xmpp_jid)}/http-bind/`)

	function extractDomain(jid: string): string {
		if (!jid) return ''
		const atIndex = jid.indexOf('@')
		return atIndex > 0 ? jid.substring(atIndex + 1) : ''
	}
	let saving = $state(false)
	let testing = $state(false)
	let testResult = $state<string | null>(null)
	let testSuccess = $state(false)
	let testAttempts = $state<Array<{mode: string, address: string, error?: string}>>([])

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

	function handleJidBlur() {
		if (settings.xmpp_jid && !settings.xmpp_server) {
			const domain = settings.xmpp_jid.split('@')[1]
			if (domain) {
				settings.xmpp_server = domain
			}
		}
	}

	async function handleTest() {
		testing = true
		testResult = null
		testAttempts = []
		try {
			const response = await testXMPPConnection(settings)
			// Check result.success (actual XMPP connection), not top-level success (HTTP status)
			testSuccess = response.result?.success ?? false
			testAttempts = response.result?.attempts ?? []

			if (testSuccess) {
				testResult = t('responder.connectionSuccessful')
			} else {
				testResult = response.result?.error || t('responder.connectionFailed')
			}
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
				onblur={handleJidBlur}
				placeholder="admin@dure.co"
				ico="at-symbol"
			/>

			<FormInput
				id="xmpp_password"
				title={t('responder.xmppPassword')}
				type="password"
				bind:value={settings.xmpp_password}
				ico="lock-closed"
			/>

			<hr class="my-4 border-gray-300" />
			<p class="text-xs text-gray-500 mb-3">Optional settings (auto-detected if not provided)</p>

			<div class="grid grid-cols-2 gap-4">
				<FormInput
					id="xmpp_server"
					title={t('responder.xmppServer')}
					bind:value={settings.xmpp_server}
					placeholder="Auto-filled from JID domain"
					ico="server"
				/>

				<FormInput
					id="xmpp_port"
					title={t('responder.xmppPort')}
					type="number"
					bind:value={settings.xmpp_port}
					placeholder="443 (direct) or 5222 (starttls)"
					ico="hashtag"
				/>
			</div>

			<div class="auto-detected-endpoints mt-4 p-3 bg-gray-50 rounded border border-gray-200">
				<p class="text-xs font-medium text-gray-700 mb-2">Auto-detected connection endpoints:</p>
				<div class="space-y-1 text-xs text-gray-600">
					<div><span class="font-mono">WebSocket:</span> {autoWebSocketURL}</div>
					<div><span class="font-mono">BOSH:</span> {autoBOSHURL}</div>
				</div>
			</div>

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

			{#if testAttempts.length > 0}
				<div class="test-attempts">
					<h4 class="text-sm font-medium mb-2">Connection Attempts:</h4>
					{#each testAttempts as attempt}
						<div class="attempt" class:failed={attempt.error}>
							<div class="flex items-center gap-2">
								<span class="font-mono text-xs uppercase px-2 py-1 rounded bg-gray-100">{attempt.mode}</span>
								<span class="text-sm text-gray-600">{attempt.address}</span>
								{#if attempt.error}
									<span class="text-red-600 text-sm ml-auto">✗ {attempt.error}</span>
								{:else}
									<span class="text-green-600 text-sm ml-auto">✓ Connected</span>
								{/if}
							</div>
						</div>
					{/each}
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

	.test-attempts {
		margin-top: 1rem;
		padding: 1rem;
		border-radius: 0.25rem;
		background: #f9fafb;
		border: 1px solid #e5e7eb;
	}

	.test-attempts .attempt {
		padding: 0.5rem;
		margin-bottom: 0.5rem;
		border-radius: 0.25rem;
	}

	.test-attempts .attempt:last-child {
		margin-bottom: 0;
	}

	.test-attempts .attempt.failed {
		background: #fef2f2;
	}
</style>
