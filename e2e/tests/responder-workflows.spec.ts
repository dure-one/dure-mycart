import { test, expect } from '../fixtures/test.fixture'

/**
 * Responder E2E Tests: Workflows and Settings
 *
 * Tests workflow CRUD, XMPP settings, and crontab configuration.
 */

test.describe('Responder - Workflows', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/_/responder/workflows')
		await page.waitForLoadState('networkidle')
	})

	test('view workflows page', async ({ page }) => {
		await expect(page.locator('h1')).toContainText('Workflows')
		await expect(page.locator('.btn-primary')).toContainText('New Workflow')
	})

	test('create workflow with mermaid diagram', async ({ page }) => {
		// Click new workflow button
		await page.locator('button:has-text("New Workflow")').click()

		// Fill form
		await page.locator('#name').fill('Test Workflow')
		await page.locator('#description').fill('E2E test workflow')

		// Mermaid content pre-filled, verify it exists
		const content = await page.locator('#content').inputValue()
		expect(content).toContain('mermaid')
		expect(content).toContain('graph')

		// Save
		await page.locator('button:has-text("Save")').click()
		await page.waitForTimeout(500)

		// Should redirect/reload to workflows list
		// Verify workflow created
		await expect(page.locator('.workflow-card')).toContainText('Test Workflow')
	})

	test('edit workflow', async ({ page }) => {
		// Check if workflows exist
		const firstCard = page.locator('.workflow-card').first()
		const hasWorkflows = await firstCard.isVisible().catch(() => false)

		if (hasWorkflows) {
			await firstCard.locator('button:has-text("Edit")').click()

			// Update name
			const nameInput = page.locator('#name')
			await nameInput.fill('Updated Workflow Name')

			await page.locator('button:has-text("Save")').click()
			await page.waitForTimeout(500)

			await expect(page.locator('.workflow-card')).toContainText('Updated Workflow Name')
		} else {
			test.skip() // No workflows to edit
		}
	})

	test('delete workflow', async ({ page }) => {
		const firstCard = page.locator('.workflow-card').first()
		const hasWorkflows = await firstCard.isVisible().catch(() => false)

		if (hasWorkflows) {
			// Setup confirm dialog handler
			page.on('dialog', (dialog) => dialog.accept())

			await firstCard.locator('button:has-text("Delete")').click()
			await page.waitForTimeout(500)

			// Page should reload
			// ponytail: smoke test - detailed verification needs workflow count tracking
		} else {
			test.skip()
		}
	})
})

test.describe('Responder - XMPP Settings', () => {
	test('configure XMPP connection', async ({ page }) => {
		await page.goto('/_/settings/responder')
		await page.waitForLoadState('networkidle')

		await expect(page.locator('h1')).toContainText('Responder Settings')

		// Fill XMPP config
		await page.locator('#jid').fill('bot@example.com')
		await page.locator('#password').fill('testpassword')
		await page.locator('#server').fill('example.com')
		await page.locator('#port').fill('5222')

		// Save
		await page.locator('button:has-text("Save Settings")').click()

		// Wait for save (alert or success message)
		await page.waitForTimeout(500)

		// ponytail: smoke test - detailed verification needs backend state check
	})

	test('test XMPP connection', async ({ page }) => {
		await page.goto('/_/settings/responder')

		// Test connection button should be visible
		const testButton = page.locator('button:has-text("Test Connection")')
		await expect(testButton).toBeVisible()

		// Click test (will fail without real XMPP server)
		await testButton.click()
		await page.waitForTimeout(1000)

		// Result should appear
		const result = page.locator('.test-result')
		await expect(result).toBeVisible()
	})
})

test.describe('Responder - Crontab Settings', () => {
	test('view crontab jobs', async ({ page }) => {
		await page.goto('/_/settings/crontab')
		await page.waitForLoadState('networkidle')

		await expect(page.locator('h1')).toContainText('Crontab Jobs')

		// Jobs table should exist (may be empty or populated)
		const jobsPanel = page.locator('.jobs-panel')
		await expect(jobsPanel).toBeVisible()
	})

	test('update job interval', async ({ page }) => {
		await page.goto('/_/settings/crontab')

		// Check if jobs exist
		const firstRow = page.locator('.table-row').first()
		const hasJobs = await firstRow.isVisible().catch(() => false)

		if (hasJobs) {
			const intervalSelect = firstRow.locator('select')
			await intervalSelect.selectOption('15min')
			await page.waitForTimeout(500)

			// Page should reload with updated interval
			// ponytail: smoke test - verification needs state persistence check
		} else {
			test.skip() // No jobs configured
		}
	})

	test('toggle job enabled status', async ({ page }) => {
		await page.goto('/_/settings/crontab')

		const firstRow = page.locator('.table-row').first()
		const hasJobs = await firstRow.isVisible().catch(() => false)

		if (hasJobs) {
			const toggle = firstRow.locator('.toggle input')
			const wasChecked = await toggle.isChecked()

			await toggle.click()
			await page.waitForTimeout(500)

			// State should flip
			const nowChecked = await toggle.isChecked()
			expect(nowChecked).toBe(!wasChecked)
		} else {
			test.skip()
		}
	})
})
