<script lang="ts">
	import { onMount } from 'svelte'
	import Main from '$lib/layouts/Main.svelte'
	import { PageHeader } from '$lib/components'
	import ChatPanel from '$lib/components/responder/ChatPanel.svelte'
	import LinkContactModal from '$lib/components/responder/LinkContactModal.svelte'
	import { translate } from '$lib/i18n'
	import { loadMessageThreads, loadMessagesByCustomer } from '$lib/utils/responder'

	let t = $derived($translate)

	let threads = $state<any[]>([])
	let loading = $state(true)
	let selectedThread = $state<any | null>(null)
	let messages = $state<any[]>([])
	let showLinkModal = $state(false)
	let channelFilter = $state('')
	let searchQuery = $state('')

	onMount(async () => {
		await loadThreads()
	})

	async function loadThreads() {
		loading = true
		try {
			const params = new URLSearchParams()
			if (channelFilter) params.set('channel', channelFilter)

			const response = await loadMessageThreads(params.toString())
			if (response.success && response.result) {
				threads = response.result.threads || []
			}
		} catch (err) {
			console.error('Failed to load threads:', err)
		} finally {
			loading = false
		}
	}

	async function selectThread(thread: any) {
		selectedThread = thread
		try {
			const response = await loadMessagesByCustomer(thread.customer_id)
			if (response.success && response.result) {
				messages = response.result.messages || []
			}
		} catch (err) {
			console.error('Failed to load messages:', err)
		}
	}

	async function handleChannelChange(event: Event) {
		const target = event.target as HTMLSelectElement
		channelFilter = target.value
		await loadThreads()
	}

	let filteredThreads = $derived(
		threads.filter((thread) => {
			if (!searchQuery) return true
			const query = searchQuery.toLowerCase()
			return (
				thread.customer_name?.toLowerCase().includes(query) ||
				thread.last_message?.toLowerCase().includes(query) ||
				thread.contact_address?.toLowerCase().includes(query)
			)
		})
	)

	function handleLinkContact() {
		showLinkModal = true
	}

	async function handleLinkConfirm() {
		showLinkModal = false
		await loadThreads()
		selectedThread = null
	}
</script>

<Main>
	<PageHeader title={t('responder.messages')} />

	<div class="messages-container">
		<div class="thread-sidebar">
			<div class="filters">
				<input
					type="text"
					placeholder={t('responder.searchConversations')}
					bind:value={searchQuery}
					class="search-input"
				/>
				<select value={channelFilter} onchange={handleChannelChange} class="channel-filter">
					<option value="">{t('responder.allChannels')}</option>
					<option value="xmpp">XMPP</option>
					<option value="sms">SMS</option>
				</select>
			</div>

			{#if loading}
				<div class="loading-state">{t('common.loading')}</div>
			{:else if filteredThreads.length === 0}
				<div class="empty-state">{t('responder.noThreads')}</div>
			{:else}
				<div class="thread-list">
					{#each filteredThreads as thread}
						<div
							class="thread-item"
							class:active={selectedThread?.customer_id === thread.customer_id}
							onclick={() => selectThread(thread)}
							role="button"
							tabindex="0"
							onkeydown={(e) => e.key === 'Enter' && selectThread(thread)}
						>
							<div class="thread-header">
								<span class="customer-name">{thread.customer_name || thread.contact_address}</span>
								<span class="channel-badge">{thread.channel}</span>
							</div>
							<div class="thread-preview">{thread.last_message || t('responder.noMessages')}</div>
							<div class="thread-meta">
								<span class="timestamp">{new Date(thread.last_message_at).toLocaleString()}</span>
								{#if thread.unread_count > 0}
									<span class="unread-badge">{thread.unread_count}</span>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="chat-container">
			{#if selectedThread}
				<ChatPanel
					{messages}
					currentCustomer={selectedThread.customer_name || selectedThread.contact_address}
					onLinkContact={handleLinkContact}
				/>
			{:else}
				<div class="no-selection">{t('responder.selectThread')}</div>
			{/if}
		</div>
	</div>

	{#if showLinkModal && selectedThread}
		<LinkContactModal
			contactId={selectedThread.contact_id}
			currentCustomerId={selectedThread.customer_id}
			onConfirm={handleLinkConfirm}
			onCancel={() => (showLinkModal = false)}
		/>
	{/if}
</Main>

<style>
	.messages-container {
		display: grid;
		grid-template-columns: 350px 1fr;
		gap: 1rem;
		height: calc(100vh - 200px);
		min-height: 600px;
	}

	.thread-sidebar {
		display: flex;
		flex-direction: column;
		border-right: 1px solid #e5e7eb;
		overflow: hidden;
	}

	.filters {
		padding: 1rem;
		border-bottom: 1px solid #e5e7eb;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.search-input,
	.channel-filter {
		padding: 0.5rem;
		border: 1px solid #d1d5db;
		border-radius: 0.25rem;
		font-size: 0.875rem;
	}

	.thread-list {
		flex: 1;
		overflow-y: auto;
	}

	.thread-item {
		padding: 1rem;
		border-bottom: 1px solid #f3f4f6;
		cursor: pointer;
		transition: background-color 0.15s;
	}

	.thread-item:hover {
		background-color: #f9fafb;
	}

	.thread-item.active {
		background-color: #eff6ff;
		border-left: 3px solid #3b82f6;
	}

	.thread-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.25rem;
	}

	.customer-name {
		font-weight: 600;
		font-size: 0.875rem;
	}

	.channel-badge {
		font-size: 0.75rem;
		padding: 0.125rem 0.5rem;
		background-color: #e5e7eb;
		border-radius: 0.25rem;
		text-transform: uppercase;
	}

	.thread-preview {
		font-size: 0.875rem;
		color: #6b7280;
		margin-bottom: 0.25rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.thread-meta {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 0.75rem;
		color: #9ca3af;
	}

	.unread-badge {
		background-color: #3b82f6;
		color: white;
		padding: 0.125rem 0.375rem;
		border-radius: 0.75rem;
		font-weight: 600;
	}

	.chat-container {
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.no-selection {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: #9ca3af;
		font-size: 1rem;
	}

	.loading-state,
	.empty-state {
		padding: 2rem;
		text-align: center;
		color: #6b7280;
	}
</style>
