<script lang="ts">
	import MessageList from './MessageList.svelte'
	import { createMessage } from '$lib/utils/responder'

	export let customerId: string
	export let messages: Array<any> = []
	export let contactAddress: string
	export let contactType: string

	let messageContent = ''
	let sending = false

	async function handleSend() {
		if (!messageContent.trim() || sending) return

		sending = true
		try {
			await createMessage({
				contact_address: contactAddress,
				contact_type: contactType,
				content: messageContent,
				direction: 'outbound',
				channel: contactType
			})
			messageContent = ''
			// ponytail: emit event for parent to refresh - full reload pattern if needed
		} catch (err) {
			console.error('Failed to send message:', err)
		} finally {
			sending = false
		}
	}
</script>

<div class="chat-panel">
	<div class="messages-container">
		<MessageList {messages} />
	</div>
	<div class="input-container">
		<textarea
			bind:value={messageContent}
			placeholder="Type a message... (read-only in mycart)"
			rows="3"
			disabled
		/>
		<button on:click={handleSend} disabled={!messageContent.trim() || sending || true}>
			{sending ? 'Sending...' : 'Send'}
		</button>
	</div>
	<div class="info-note">
		Messages are read-only in mycart admin. Use workflows for sending/replying.
	</div>
</div>

<style>
	.chat-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		background: white;
		border-radius: 0.5rem;
		overflow: hidden;
	}

	.messages-container {
		flex: 1;
		overflow-y: auto;
	}

	.input-container {
		display: flex;
		gap: 0.5rem;
		padding: 1rem;
		border-top: 1px solid #e5e7eb;
	}

	textarea {
		flex: 1;
		padding: 0.5rem;
		border: 1px solid #d1d5db;
		border-radius: 0.25rem;
		resize: none;
		opacity: 0.6;
	}

	button {
		padding: 0.5rem 1rem;
		background: #3b82f6;
		color: white;
		border: none;
		border-radius: 0.25rem;
		cursor: not-allowed;
		opacity: 0.5;
	}

	button:not(:disabled) {
		cursor: pointer;
		opacity: 1;
	}

	.info-note {
		padding: 0.5rem 1rem;
		background: #fef3c7;
		color: #92400e;
		font-size: 0.875rem;
		text-align: center;
	}
</style>
