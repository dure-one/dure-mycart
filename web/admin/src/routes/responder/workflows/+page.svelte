<script lang="ts">
	import { saveWorkflow, deleteWorkflow } from '$lib/utils/responder'
	import type { PageData } from './$types'

	export let data: PageData

	let editing: any = null
	let showEditor = false

	function newWorkflow() {
		editing = {
			name: '',
			description: '',
			content: '```mermaid\ngraph TD\n  A[Start] --> B[Process]\n  B --> C[End]\n```',
			enabled: false,
			tags: '[]',
			version: '1.0',
			author: 'admin'
		}
		showEditor = true
	}

	function editWorkflow(workflow: any) {
		editing = { ...workflow }
		showEditor = true
	}

	async function handleSave() {
		if (!editing) return
		try {
			await saveWorkflow(editing)
			showEditor = false
			editing = null
			// ponytail: reload page - full refresh pattern
			window.location.reload()
		} catch (err) {
			console.error('Failed to save workflow:', err)
		}
	}

	async function handleDelete(id: string) {
		if (!confirm('Delete this workflow?')) return
		try {
			await deleteWorkflow(id)
			window.location.reload()
		} catch (err) {
			console.error('Failed to delete workflow:', err)
		}
	}

	function cancel() {
		showEditor = false
		editing = null
	}
</script>

<div class="workflows-page">
	<div class="page-header">
		<h1>Workflows</h1>
		<button class="btn-primary" on:click={newWorkflow}>+ New Workflow</button>
	</div>

	{#if showEditor && editing}
		<div class="editor-panel">
			<div class="editor-header">
				<h2>{editing.id ? 'Edit' : 'Create'} Workflow</h2>
				<div class="actions">
					<button class="btn-secondary" on:click={cancel}>Cancel</button>
					<button class="btn-primary" on:click={handleSave}>Save</button>
				</div>
			</div>

			<div class="editor-form">
				<div class="form-row">
					<div class="form-group">
						<label for="name">Name</label>
						<input id="name" type="text" bind:value={editing.name} required />
					</div>
					<div class="form-group">
						<label for="enabled">
							<input id="enabled" type="checkbox" bind:checked={editing.enabled} />
							Enabled
						</label>
					</div>
				</div>

				<div class="form-group">
					<label for="description">Description</label>
					<input id="description" type="text" bind:value={editing.description} />
				</div>

				<div class="form-group">
					<label for="content">Content (Mermaid Markdown)</label>
					<textarea
						id="content"
						bind:value={editing.content}
						rows="15"
						placeholder="Enter mermaid diagram markdown..."
					/>
					<small>Documentation-only mermaid diagrams. Not executable in mycart.</small>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label for="version">Version</label>
						<input id="version" type="text" bind:value={editing.version} />
					</div>
					<div class="form-group">
						<label for="author">Author</label>
						<input id="author" type="text" bind:value={editing.author} />
					</div>
					<div class="form-group">
						<label for="tags">Tags (JSON)</label>
						<input id="tags" type="text" bind:value={editing.tags} />
					</div>
				</div>
			</div>
		</div>
	{:else}
		<div class="workflows-list">
			{#if data.workflows.length === 0}
				<div class="empty-state">
					<p>No workflows yet</p>
					<button class="btn-primary" on:click={newWorkflow}>Create First Workflow</button>
				</div>
			{:else}
				<div class="workflow-cards">
					{#each data.workflows as workflow}
						<div class="workflow-card">
							<div class="card-header">
								<h3>{workflow.name}</h3>
								<span class="status" class:enabled={workflow.enabled}>
									{workflow.enabled ? 'Enabled' : 'Disabled'}
								</span>
							</div>
							{#if workflow.description}
								<p class="description">{workflow.description}</p>
							{/if}
							<div class="card-meta">
								<span>v{workflow.version}</span>
								<span>by {workflow.author}</span>
							</div>
							<div class="card-actions">
								<button class="btn-secondary" on:click={() => editWorkflow(workflow)}>Edit</button>
								<button class="btn-danger" on:click={() => handleDelete(workflow.id)}>Delete</button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.workflows-page {
		padding: 1.5rem;
	}

	.page-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	.page-header h1 {
		margin: 0;
		font-size: 1.5rem;
	}

	.btn-primary,
	.btn-secondary,
	.btn-danger {
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

	.btn-danger {
		background: #ef4444;
		color: white;
	}

	.editor-panel {
		background: white;
		border-radius: 0.5rem;
		padding: 1.5rem;
	}

	.editor-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	.editor-header h2 {
		margin: 0;
	}

	.editor-header .actions {
		display: flex;
		gap: 0.5rem;
	}

	.editor-form {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.form-row {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 1rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
	}

	.form-group label {
		margin-bottom: 0.25rem;
		font-weight: 500;
		font-size: 0.875rem;
	}

	.form-group input[type='text'],
	.form-group textarea {
		padding: 0.5rem;
		border: 1px solid #d1d5db;
		border-radius: 0.25rem;
	}

	.form-group textarea {
		font-family: monospace;
		resize: vertical;
	}

	.form-group small {
		margin-top: 0.25rem;
		color: #6b7280;
		font-size: 0.75rem;
	}

	.workflows-list {
		background: white;
		border-radius: 0.5rem;
		padding: 1.5rem;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
		color: #6b7280;
	}

	.workflow-cards {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
		gap: 1rem;
	}

	.workflow-card {
		border: 1px solid #e5e7eb;
		border-radius: 0.5rem;
		padding: 1rem;
	}

	.card-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.5rem;
	}

	.card-header h3 {
		margin: 0;
		font-size: 1.125rem;
	}

	.status {
		padding: 0.25rem 0.5rem;
		border-radius: 0.25rem;
		font-size: 0.75rem;
		background: #f3f4f6;
		color: #6b7280;
	}

	.status.enabled {
		background: #d1fae5;
		color: #065f46;
	}

	.description {
		color: #6b7280;
		font-size: 0.875rem;
		margin-bottom: 0.5rem;
	}

	.card-meta {
		display: flex;
		gap: 1rem;
		font-size: 0.75rem;
		color: #9ca3af;
		margin-bottom: 1rem;
	}

	.card-actions {
		display: flex;
		gap: 0.5rem;
	}
</style>
