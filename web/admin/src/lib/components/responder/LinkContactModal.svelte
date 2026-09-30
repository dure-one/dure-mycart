<script lang="ts">
	import { linkContact } from '$lib/utils/responder'

	export let contactId: string
	export let onClose: () => void
	export let onSuccess: () => void

	let searchQuery = ''
	let selectedCustomerId = ''
	let linking = false

	async function handleLink() {
		if (!selectedCustomerId || linking) return

		linking = true
		try {
			await linkContact(contactId, selectedCustomerId)
			onSuccess()
			onClose()
		} catch (err) {
			console.error('Failed to link contact:', err)
		} finally {
			linking = false
		}
	}
</script>

<div class="modal-overlay" on:click={onClose}>
	<div class="modal-content" on:click|stopPropagation>
		<div class="modal-header">
			<h3>Link Contact to Customer</h3>
			<button class="close-btn" on:click={onClose}>&times;</button>
		</div>

		<div class="modal-body">
			<div class="form-group">
				<label for="search">Search Customer (email or ID)</label>
				<input
					id="search"
					type="text"
					bind:value={searchQuery}
					placeholder="Enter customer email or ID"
				/>
			</div>

			<div class="form-group">
				<label for="customer-id">Customer ID</label>
				<input
					id="customer-id"
					type="text"
					bind:value={selectedCustomerId}
					placeholder="Paste customer ID"
				/>
				<small>Enter the ID of the customer to link this contact to</small>
			</div>
		</div>

		<div class="modal-footer">
			<button class="btn-secondary" on:click={onClose}>Cancel</button>
			<button class="btn-primary" on:click={handleLink} disabled={!selectedCustomerId || linking}>
				{linking ? 'Linking...' : 'Link Contact'}
			</button>
		</div>
	</div>
</div>

<style>
	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
	}

	.modal-content {
		background: white;
		border-radius: 0.5rem;
		max-width: 500px;
		width: 90%;
		box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1);
	}

	.modal-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem;
		border-bottom: 1px solid #e5e7eb;
	}

	.modal-header h3 {
		margin: 0;
		font-size: 1.125rem;
	}

	.close-btn {
		background: none;
		border: none;
		font-size: 1.5rem;
		cursor: pointer;
		color: #6b7280;
		padding: 0;
		width: 2rem;
		height: 2rem;
	}

	.modal-body {
		padding: 1rem;
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

	.form-group small {
		display: block;
		margin-top: 0.25rem;
		color: #6b7280;
		font-size: 0.75rem;
	}

	.modal-footer {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		padding: 1rem;
		border-top: 1px solid #e5e7eb;
	}

	.btn-secondary,
	.btn-primary {
		padding: 0.5rem 1rem;
		border: none;
		border-radius: 0.25rem;
		cursor: pointer;
		font-size: 0.875rem;
	}

	.btn-secondary {
		background: #f3f4f6;
		color: #374151;
	}

	.btn-primary {
		background: #3b82f6;
		color: white;
	}

	.btn-primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
