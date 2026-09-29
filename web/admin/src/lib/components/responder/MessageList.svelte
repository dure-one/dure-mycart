<script lang="ts">
	import MessageBubble from './MessageBubble.svelte'

	export let messages: Array<{
		id: string
		content: string
		direction: 'inbound' | 'outbound'
		channel: string
		delivery_status?: string
		created: number
	}> = []
</script>

<div class="message-list">
	{#if messages.length === 0}
		<div class="empty-state">No messages</div>
	{:else}
		{#each messages as message (message.id)}
			<MessageBubble {message} />
		{/each}
	{/if}
</div>

<style>
	.message-list {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow-y: auto;
		padding: 1rem;
	}

	.empty-state {
		text-align: center;
		padding: 2rem;
		color: #9ca3af;
	}
</style>
