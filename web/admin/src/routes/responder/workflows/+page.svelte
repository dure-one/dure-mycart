<script lang="ts">
	import { onMount } from 'svelte'
	import Main from '$lib/layouts/Main.svelte'
	import { Drawer, DrawerHeader, DrawerFooter, PageHeader, PageState, IconButton, FormInput, FormTextarea, FormGroup } from '$lib/components'
	import { translate } from '$lib/i18n'
	import { loadWorkflows, saveWorkflow, deleteWorkflow } from '$lib/utils/responder'
	import { formatDate, confirmDelete, showMessage } from '$lib/utils'
	import { DRAWER_CLOSE_DELAY_MS } from '$lib/constants/ui'
	import { createDelayedReset } from '$lib/utils/delayedReset'

	let t = $derived($translate)

	let workflows = $state<any[]>([])
	let loading = $state(true)
	let drawerOpen = $state(false)
	let drawerMode = $state<'add' | 'edit'>('add')
	let drawerWorkflow = $state<any | null>(null)

	let formData = $state({
		name: '',
		description: '',
		content: '```mermaid\ngraph TD\n  A[Start] --> B[Process]\n  B --> C[End]\n```',
		enabled: true,
		tags: '',
		version: '1.0',
		author: 'admin'
	})

	let formErrors = $state<Record<string, string>>({})

	onMount(async () => {
		await loadWorkflows()
	})

	async function loadWorkflows() {
		loading = true
		try {
			const response = await loadWorkflows()
			if (response.success && response.result) {
				workflows = response.result.workflows || []
			}
		} catch (err) {
			console.error('Failed to load workflows:', err)
		} finally {
			loading = false
		}
	}

	function openAdd() {
		formData = {
			name: '',
			description: '',
			content: '```mermaid\ngraph TD\n  A[Start] --> B[Process]\n  B --> C[End]\n```',
			enabled: true,
			tags: '',
			version: '1.0',
			author: 'admin'
		}
		formErrors = {}
		drawerWorkflow = null
		drawerMode = 'add'
		drawerOpen = true
	}

	function openEdit(workflow: any) {
		formData = {
			name: workflow.name || '',
			description: workflow.description || '',
			content: workflow.content || '',
			enabled: workflow.enabled !== undefined ? workflow.enabled : true,
			tags: workflow.tags || '',
			version: workflow.version || '1.0',
			author: workflow.author || 'admin'
		}
		drawerWorkflow = workflow
		formErrors = {}
		drawerMode = 'edit'
		drawerOpen = true
	}

	const resetDrawer = createDelayedReset(DRAWER_CLOSE_DELAY_MS)

	function closeDrawer() {
		if (drawerOpen) {
			drawerOpen = false
			resetDrawer(() => {
				drawerWorkflow = null
				drawerMode = 'add'
			})
		}
	}

	async function handleSubmit() {
		formErrors = {}

		if (!formData.name || formData.name.length < 3) {
			formErrors.name = 'Name must be at least 3 characters'
			return
		}

		const workflowData: any = {
			name: formData.name,
			description: formData.description,
			content: formData.content,
			enabled: formData.enabled,
			tags: formData.tags,
			version: formData.version,
			author: formData.author
		}

		if (drawerMode === 'edit' && drawerWorkflow) {
			workflowData.id = drawerWorkflow.id
		}

		try {
			const response = await saveWorkflow(workflowData)
			if (response.success) {
				showMessage(t('responder.workflowSaved'), 'connextSuccess')
				await loadWorkflows()
				closeDrawer()
			} else {
				showMessage(t('responder.failedToSaveWorkflow'), 'connextError')
			}
		} catch (err) {
			console.error('Failed to save workflow:', err)
			showMessage(t('responder.failedToSaveWorkflow'), 'connextError')
		}
	}

	async function handleDelete(workflow: any) {
		if (!confirmDelete('workflow', workflow.name)) {
			return
		}

		try {
			const response = await deleteWorkflow(workflow.id)
			if (response.success) {
				showMessage(t('responder.workflowDeleted'), 'connextSuccess')
				await loadWorkflows()
				closeDrawer()
			} else {
				showMessage(t('responder.failedToDeleteWorkflow'), 'connextError')
			}
		} catch (err) {
			console.error('Failed to delete workflow:', err)
			showMessage(t('responder.failedToDeleteWorkflow'), 'connextError')
		}
	}

	function deleteFromFooter() {
		if (drawerWorkflow) {
			handleDelete(drawerWorkflow)
		}
	}
</script>

<Main>
	<PageHeader title={t('responder.workflows')}>
		{#snippet actions()}
			<button class="btn-primary" onclick={openAdd}>
				+ {t('responder.addWorkflow')}
			</button>
		{/snippet}
	</PageHeader>

	{#if loading}
		<PageState kind="loading" />
	{:else if workflows.length === 0}
		<PageState kind="empty" message={t('responder.noWorkflows')} />
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th>{t('responder.workflowName')}</th>
						<th class="w-64">{t('responder.workflowDescription')}</th>
						<th class="w-24">{t('responder.workflowEnabled')}</th>
						<th class="w-48">{t('common.updated')}</th>
						<th class="w-24"></th>
					</tr>
				</thead>
				<tbody>
					{#each workflows as workflow (workflow.id)}
						<tr class:opacity-30={!workflow.enabled}>
							<td>
								<div class="font-medium">{workflow.name}</div>
								{#if workflow.version}
									<div class="text-xs text-gray-500">v{workflow.version}</div>
								{/if}
							</td>
							<td>
								<div class="text-sm text-gray-600 truncate max-w-xs">
									{workflow.description || '-'}
								</div>
							</td>
							<td>
								{#if workflow.enabled}
									<span class="badge badge-success">Enabled</span>
								{:else}
									<span class="badge badge-gray">Disabled</span>
								{/if}
							</td>
							<td>
								{#if workflow.updated}
									{formatDate(workflow.updated)}
								{:else if workflow.created}
									{formatDate(workflow.created)}
								{:else}
									-
								{/if}
							</td>
							<td>
								<div class="flex items-center gap-2">
									<IconButton ico="pencil-square" label={t('common.edit')} onclick={() => openEdit(workflow)} />
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</Main>

{#if drawerOpen}
	<Drawer isOpen={drawerOpen} onclose={closeDrawer} maxWidth="710px">
		<DrawerHeader title={drawerMode === 'add' ? t('responder.addWorkflow') : t('responder.editWorkflow')} />

		<form onsubmit={(e) => { e.preventDefault(); handleSubmit() }}>
			<div class="flow-root">
				<dl class="mx-auto -my-3 mt-4 mb-0 space-y-4 text-sm">
					<FormInput
						id="name"
						title={t('responder.workflowName')}
						bind:value={formData.name}
						error={formErrors.name}
						ico="pencil"
					/>

					<FormInput
						id="description"
						title={t('responder.workflowDescription')}
						bind:value={formData.description}
						ico="at-symbol"
					/>

					<div class="flex items-center gap-2">
						<input
							id="enabled"
							type="checkbox"
							bind:checked={formData.enabled}
							class="h-4 w-4"
						/>
						<label for="enabled" class="text-sm font-medium">
							{t('responder.workflowEnabled')}
						</label>
					</div>

					<FormGroup label={t('responder.workflowContent')}>
						<textarea
							bind:value={formData.content}
							rows="15"
							class="w-full p-2 border border-gray-300 rounded font-mono text-sm"
							placeholder="Enter mermaid diagram markdown..."
						/>
						<div class="text-xs text-gray-500 mt-1">
							Documentation-only mermaid diagrams. Not executable in mycart.
						</div>
					</FormGroup>

					<div class="grid grid-cols-3 gap-3">
						<FormInput
							id="version"
							title="Version"
							bind:value={formData.version}
							ico="hashtag"
						/>
						<FormInput
							id="author"
							title="Author"
							bind:value={formData.author}
							ico="user"
						/>
						<FormInput
							id="tags"
							title={t('responder.workflowTags')}
							bind:value={formData.tags}
							ico="tag"
						/>
					</div>
				</dl>
			</div>

			<DrawerFooter
				onclose={closeDrawer}
				submitLabel={drawerMode === 'add' ? t('common.add') : t('common.save')}
				ondelete={drawerMode === 'edit' && drawerWorkflow ? deleteFromFooter : undefined}
				deleteLabel={t('common.delete')}
			/>
		</form>
	</Drawer>
{/if}

<style>
	.btn-primary {
		padding: 0.5rem 1rem;
		background: #3b82f6;
		color: white;
		border: none;
		border-radius: 0.375rem;
		font-size: 0.875rem;
		cursor: pointer;
	}

	.badge {
		display: inline-block;
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
		font-weight: 500;
		border-radius: 0.25rem;
	}

	.badge-success {
		background-color: #d1fae5;
		color: #065f46;
	}

	.badge-gray {
		background-color: #e5e7eb;
		color: #6b7280;
	}
</style>
