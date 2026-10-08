<script lang="ts">
	import { onMount } from 'svelte'
	import Main from '$lib/layouts/Main.svelte'
	import { Drawer, DrawerHeader, DrawerFooter, PageHeader, PageState, IconButton } from '$lib/components'
	import MessageList from '$lib/components/responder/MessageList.svelte'
	import { translate } from '$lib/i18n'
	import { loadMessageThreads, loadCustomerMessages, markMessageRead } from '$lib/utils/responder'
	import { formatDate } from '$lib/utils'

	let t = $derived($translate)

	let threads = $state<any[]>([])
	let loading = $state(true)
	let drawerOpen = $state(false)
	let selectedThread = $state<any | null>(null)
	let messages = $state<any[]>([])
	let channelFilter = $state('')

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

	async function openThread(thread: any) {
		selectedThread = thread
		try {
			const response = await loadCustomerMessages(thread.customer_id)
			if (response.success && response.result) {
				messages = response.result.messages || []
			}
		} catch (err) {
			console.error('Failed to load messages:', err)
		}
		drawerOpen = true
	}

	function closeDrawer() {
		drawerOpen = false
		setTimeout(() => {
			selectedThread = null
			messages = []
		}, 300)
	}

	async function handleChannelChange(event: Event) {
		const target = event.target as HTMLSelectElement
		channelFilter = target.value
		await loadThreads()
	}
</script>

<Main>
	<PageHeader title={t('responder.messages')}>
		{#snippet actions()}
			<select value={channelFilter} onchange={handleChannelChange} class="select">
				<option value="">{t('responder.allChannels')}</option>
				<option value="xmpp">XMPP</option>
				<option value="sms">SMS</option>
			</select>
		{/snippet}
	</PageHeader>

	{#if loading}
		<PageState kind="loading" />
	{:else if threads.length === 0}
		<PageState kind="empty" message={t('responder.noThreads')} />
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>{t('customers.name')}</th>
						<th class="w-32">{t('responder.allChannels')}</th>
						<th>{t('responder.lastRun')}</th>
						<th class="w-48">{t('common.created')}</th>
						<th class="w-24"></th>
					</tr>
				</thead>
				<tbody>
					{#each threads as thread (thread.customer_id)}
						<tr>
							<td>
								<div class="font-medium">{thread.customer_name || thread.customer_email}</div>
								{#if thread.contacts && thread.contacts.length > 0}
									<div class="text-xs text-gray-500">
										{#if thread.contacts[0].type === 'xmpp'}JID: {/if}{thread.contacts[0].address}
									</div>
								{/if}
							</td>
							<td>
								{#if thread.contacts && thread.contacts.length > 0}
									<span class="badge">{thread.contacts[0].type}</span>
								{:else}
									-
								{/if}
							</td>
							<td>
								<div class="text-sm text-gray-600 truncate max-w-md">
									{thread.last_message_preview || '-'}
								</div>
							</td>
							<td>
								{#if thread.last_message_at}
									{formatDate(thread.last_message_at)}
								{:else}
									-
								{/if}
							</td>
							<td>
								<div class="flex items-center gap-2">
									<IconButton ico="chat" label={t('common.edit')} onclick={() => openThread(thread)} />
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</Main>

{#if drawerOpen && selectedThread}
	<Drawer isOpen={drawerOpen} onclose={closeDrawer} maxWidth="710px">
		<DrawerHeader title={selectedThread.customer_name || selectedThread.customer_email} />

		<div class="p-4">
			<div class="mb-4 text-sm text-gray-600">
				<div><strong>Customer:</strong> {selectedThread.customer_email}</div>
				{#if selectedThread.contacts && selectedThread.contacts.length > 0}
					<div>
						<strong>Contacts:</strong> {selectedThread.contacts.map(c => c.address).join(', ')}
					</div>
				{/if}
			</div>

			<div class="message-list-container">
				<MessageList {messages} />
			</div>
		</div>

		<DrawerFooter onclose={closeDrawer} />
	</Drawer>
{/if}

<style>
	.select {
		padding: 0.5rem 2rem 0.5rem 0.75rem;
		border: 1px solid #d1d5db;
		border-radius: 0.375rem;
		font-size: 0.875rem;
		background-color: white;
	}

	.badge {
		display: inline-block;
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
		font-weight: 500;
		border-radius: 0.25rem;
		background-color: #e5e7eb;
		text-transform: uppercase;
	}

	.message-list-container {
		max-height: 500px;
		overflow-y: auto;
		border: 1px solid #e5e7eb;
		border-radius: 0.375rem;
		padding: 1rem;
	}
</style>
