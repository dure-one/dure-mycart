<script lang="ts">
	import { onMount } from 'svelte'
	import Main from '$lib/layouts/Main.svelte'
	import { PageHeader, PageState, Section, FormToggle } from '$lib/components'
	import { loadCrontabJobs, updateCrontabJob, checkCrontabStatus, installCrontab, uninstallCrontab } from '$lib/utils/responder'
	import { translate } from '$lib/i18n'
	import { showMessage } from '$lib/utils/message'

	let t = $derived($translate)

	let jobs = $state<any[]>([])
	let loading = $state(true)
	let serviceInstalled = $state(false)
	let serviceLoading = $state(false)

	const intervals = ['5min', '15min', '1hr', '6hr', 'daily']

	onMount(async () => {
		await loadJobs()
		await checkStatus()
	})

	async function loadJobs() {
		loading = true
		try {
			const response = await loadCrontabJobs()
			if (response.success && response.result) {
				jobs = response.result.jobs || []
			}
		} catch (err) {
			console.error('Failed to load jobs:', err)
		} finally {
			loading = false
		}
	}

	async function checkStatus() {
		try {
			const response = await checkCrontabStatus()
			if (response.success && response.result) {
				serviceInstalled = response.result.installed || false
			}
		} catch (err) {
			console.error('Failed to check crontab status:', err)
		}
	}

	async function handleServiceToggle() {
		serviceLoading = true
		try {
			if (serviceInstalled) {
				const response = await uninstallCrontab()
				if (response.success) {
					serviceInstalled = false
					showMessage(t('crontab.serviceDisabled'), 'connextSuccess')
				} else {
					showMessage(t('crontab.failedToDisable'), 'connextError')
					await checkStatus() // Re-fetch actual state on failure
				}
			} else {
				const response = await installCrontab()
				if (response.success) {
					serviceInstalled = true
					showMessage(t('crontab.serviceEnabled'), 'connextSuccess')
				} else {
					showMessage(t('crontab.failedToEnable'), 'connextError')
					await checkStatus() // Re-fetch actual state on failure
				}
			}
		} catch (err) {
			console.error('Failed to toggle crontab service:', err)
			showMessage(t('crontab.operationFailed'), 'connextError')
			await checkStatus() // Re-fetch actual state on error
		} finally {
			serviceLoading = false
		}
	}

	async function handleToggle(job: any) {
		try {
			await updateCrontabJob(job.id, { enabled: !job.enabled })
			await loadJobs()
		} catch (err) {
			console.error('Failed to toggle job:', err)
		}
	}

	async function handleIntervalChange(job: any, event: Event) {
		const target = event.target as HTMLSelectElement
		try {
			await updateCrontabJob(job.id, { interval: target.value })
			await loadJobs()
		} catch (err) {
			console.error('Failed to update interval:', err)
		}
	}

	function formatTimestamp(ts: number | null) {
		if (!ts) return t('crontab.never')
		return new Date(ts * 1000).toLocaleString()
	}
</script>

<Main>
	<PageHeader title={t('crontab.title')} />

	<Section>
		<p class="text-sm text-gray-600 mb-4">{t('crontab.description')}</p>

		<div class="mb-6 p-4 border rounded-lg bg-gray-50">
			<FormToggle
				id="crontab-service"
				title={t('crontab.useService')}
				value={serviceInstalled}
				onchange={handleServiceToggle}
				disabled={serviceLoading}
			/>
			<p class="text-xs text-gray-500 mt-2">
				{serviceInstalled ? t('crontab.serviceEnabledDesc') : t('crontab.serviceDisabledDesc')}
			</p>
		</div>

		{#if loading}
			<div class="loading-state">{t('crontab.loadingJobs')}</div>
		{:else if jobs.length === 0}
			<div class="empty-state">{t('crontab.noJobs')}</div>
		{:else}
			<div class="jobs-table">
				<div class="table-header">
					<div class="col-name">{t('crontab.job')}</div>
					<div class="col-interval">{t('crontab.interval')}</div>
					<div class="col-last-run">{t('crontab.lastRun')}</div>
					<div class="col-next-run">{t('crontab.nextRun')}</div>
					<div class="col-enabled">{t('crontab.enabled')}</div>
				</div>

				{#each jobs as job}
					<div class="table-row">
						<div class="col-name">
							<div class="job-name">{job.name || job.job_type}</div>
							<div class="job-type">{job.job_type}</div>
						</div>
						<div class="col-interval">
							<select
								value={job.interval}
								onchange={(e) => handleIntervalChange(job, e)}
								disabled={!job.enabled}
							>
								{#each intervals as interval}
									<option value={interval}>{interval}</option>
								{/each}
							</select>
						</div>
						<div class="col-last-run">{formatTimestamp(job.last_run)}</div>
						<div class="col-next-run">{formatTimestamp(job.next_run)}</div>
						<div class="col-enabled">
							<FormToggle
								id="job-{job.id}-enabled"
								value={job.enabled}
								onchange={() => handleToggle(job)}
							/>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Section>
</Main>

<style>
	.loading-state,
	.empty-state {
		text-align: center;
		padding: 2rem;
		color: #9ca3af;
	}

	.jobs-table {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.table-header,
	.table-row {
		display: grid;
		grid-template-columns: 2fr 1fr 1.5fr 1.5fr 100px;
		gap: 1rem;
		padding: 0.75rem;
		align-items: center;
	}

	.table-header {
		font-weight: 600;
		font-size: 0.875rem;
		color: #6b7280;
		border-bottom: 2px solid #e5e7eb;
	}

	.table-row {
		border-bottom: 1px solid #e5e7eb;
	}

	.job-name {
		font-weight: 500;
	}

	.job-type {
		font-size: 0.75rem;
		color: #9ca3af;
	}

	select {
		padding: 0.375rem;
		border: 1px solid #d1d5db;
		border-radius: 0.25rem;
		font-size: 0.875rem;
	}

	select:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
