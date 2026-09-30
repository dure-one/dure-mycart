<script lang="ts">
	export let message: {
		content: string
		direction: 'inbound' | 'outbound'
		channel: string
		delivery_status?: string
		created: number
	}

	const isInbound = message.direction === 'inbound'
	const channelIcon = message.channel === 'sms' ? '📱' : '💬'
</script>

<div class="message-bubble {isInbound ? 'inbound' : 'outbound'}">
	<div class="message-header">
		<span class="channel-icon">{channelIcon}</span>
		<span class="timestamp">{new Date(message.created * 1000).toLocaleString()}</span>
	</div>
	<div class="message-content">{message.content}</div>
	{#if message.delivery_status}
		<div class="delivery-status">{message.delivery_status}</div>
	{/if}
</div>

<style>
	.message-bubble {
		max-width: 70%;
		padding: 0.75rem;
		margin: 0.5rem 0;
		border-radius: 0.5rem;
	}

	.inbound {
		background: #f3f4f6;
		margin-right: auto;
	}

	.outbound {
		background: #3b82f6;
		color: white;
		margin-left: auto;
	}

	.message-header {
		display: flex;
		gap: 0.5rem;
		font-size: 0.75rem;
		opacity: 0.7;
		margin-bottom: 0.25rem;
	}

	.message-content {
		word-wrap: break-word;
	}

	.delivery-status {
		font-size: 0.7rem;
		margin-top: 0.25rem;
		opacity: 0.6;
	}
</style>
