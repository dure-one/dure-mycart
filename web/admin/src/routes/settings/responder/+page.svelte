<script lang="ts">
	import { loadResponderSettings, saveResponderSettings, testXMPPConnection } from '$lib/utils/responder'
	import { onMount } from 'svelte'

	let settings = {
		xmpp_jid: '',
		xmpp_password: '',
		xmpp_server: '',
		xmpp_port: 5222
	}
	let saving = false
	let testing = false
	let testResult: string | null = null

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
			alert('Settings saved successfully')
		} catch (err) {
			console.error('Failed to save settings:', err)
			alert('Failed to save settings')
		} finally {
			saving = false
		}
	}

	async function handleTest() {
		testing = true
		testResult = null
		try {
			const response = await testXMPPConnection()
			testResult = response.success ? 'Connection successful!' : 'Connection failed'
		} catch (err) {
			console.error('Connection test failed:', err)
			testResult = 'Connection test failed'
		} finally {
			testing = false
		}
	}
</script>

<div class="settings-page">
	<div class="page-header">
		<h1>Responder Settings</h1>
	</div>

	<div class="settings-panel">
		<h2>XMPP Configuration</h2>
		<p class="description">Configure XMPP connection for message synchronization via MAM</p>

		<form on:submit|preventDefault={handleSave}>
			<div class="form-group">
				<label for="jid">XMPP JID</label>
				<input
					id="jid"
					type="text"
					bind:value={settings.xmpp_jid}
					placeholder="bot@example.com"
					required
				/>
			</div>

			<div class="form-group">
				<label for="password">Password</label>
				<input
					id="password"
					type="password"
					bind:value={settings.xmpp_password}
					required
				/>
			</div>

			<div class="form-row">
				<div class="form-group">
					<label for="server">Server</label>
					<input
						id="server"
						type="text"
						bind:value={settings.xmpp_server}
						placeholder="example.com"
						required
					/>
				</div>

				<div class="form-group">
					<label for="port">Port</label>
					<input
						id="port"
						type="number"
						bind:value={settings.xmpp_port}
						min="1"
						max="65535"
						required
					/>
				</div>
			</div>

			<div class="form-actions">
				<button type="button" class="btn-secondary" on:click={handleTest} disabled={testing}>
					{testing ? 'Testing...' : 'Test Connection'}
				</button>
				<button type="submit" class="btn-primary" disabled={saving}>
					{saving ? 'Saving...' : 'Save Settings'}
				</button>
			</div>

			{#if testResult}
				<div class="test-result" class:success={testResult.includes('successful')}>
					{testResult}
				</div>
			{/if}
		</form>
	</div>
</div>

<style>
	.settings-page {
		padding: 1.5rem;
		max-width: 800px;
	}

	.page-header h1 {
		margin: 0 0 1.5rem;
		font-size: 1.5rem;
	}

	.settings-panel {
		background: white;
		border-radius: 0.5rem;
		padding: 1.5rem;
	}

	.settings-panel h2 {
		margin: 0 0 0.5rem;
		font-size: 1.25rem;
	}

	.description {
		color: #6b7280;
		font-size: 0.875rem;
		margin-bottom: 1.5rem;
	}

	.form-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
	}

	.form-group {
		margin-bottom: 1rem;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.25rem;
		font-weight: 500;
		font-size: 0.875rem;
	}

	.form-group input {
		width: 100%;
		padding: 0.5rem;
		border: 1px solid #d1d5db;
		border-radius: 0.25rem;
	}

	.form-actions {
		display: flex;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	.btn-primary,
	.btn-secondary {
		padding: 0.5rem 1rem;
		border: none;
		border-radius: 0.25rem;
		cursor: pointer;
		font-size: 0.875rem;
	}

	.btn-primary {
		background: #3b82f6;
		color: white;
	}

	.btn-secondary {
		background: #f3f4f6;
		color: #374151;
	}

	.btn-primary:disabled,
	.btn-secondary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

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
