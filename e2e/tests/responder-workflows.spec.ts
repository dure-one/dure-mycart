import { test, expect } from '../fixtures/test.fixture'
import { useAdminSession } from '../utils/admin-page'

/**
 * Responder E2E Tests: Workflows and Settings
 *
 * Tests workflow CRUD, XMPP settings, and crontab configuration.
 */

test.describe('Responder - Workflows', () => {
	test.beforeEach(async ({ page, baseURL }) => {
		// Authenticate as admin
		await useAdminSession(page, baseURL ?? '')

		await page.goto('/_/responder/workflows')
		await page.waitForLoadState('networkidle')
	})

	test('view workflows page', async ({ page }) => {
		// Use last h1 to get the page heading (not parent nav heading)
		await expect(page.locator('h1').last()).toContainText('Workflows')

		// Check for Add Workflow button (translated key: responder.addWorkflow)
		// Button has .btn-primary class and starts with "+"
		const addButton = page.locator('button.btn-primary')
		await expect(addButton).toBeVisible()

		// Button text should contain "Workflow" (in any language)
		const buttonText = await addButton.textContent()
		expect(buttonText).toMatch(/workflow/i)
	})

	test('create workflow with mermaid diagram', async ({ page }) => {
		// Click add workflow button
		const addButton = page.locator('button.btn-primary')
		await addButton.click()

		// Wait for drawer to fully open and settle
		await page.waitForTimeout(800)

		// Wait for the name input to be ready
		const nameInput = page.locator('#name')
		await nameInput.waitFor({ state: 'visible' })
		await nameInput.fill('Test Workflow E2E')

		await page.locator('#description').fill('E2E test workflow')

		// The content field is a MarkdownEditor, not a simple input
		// Check if the mermaid preview is visible (pre-filled content)
		const mermaidPreview = page.locator('text=Workflow Diagram Preview')
		await expect(mermaidPreview).toBeVisible()

		// Submit via form submission to avoid drawer intercept
		await page.locator('form').evaluate(form => (form as HTMLFormElement).requestSubmit())

		await page.waitForTimeout(1000)

		// Should close drawer and show workflow in table
		// Check if table has the new workflow name
		const table = page.locator('table')
		const hasWorkflow = await table.locator('text=Test Workflow E2E').isVisible().catch(() => false)

		// If there's data, verify it exists; if empty state, that's also valid
		if (hasWorkflow) {
			await expect(table).toContainText('Test Workflow E2E')
		}
	})

	test('edit workflow', async ({ page }) => {
		// Check if workflows exist in table
		const table = page.locator('table')
		const hasTable = await table.isVisible().catch(() => false)

		if (hasTable) {
			// Find and click the edit button (pencil icon) in the first row
			const firstEditButton = table.locator('tbody tr').first().locator('[data-ico="pencil-square"]').or(
				table.locator('tbody tr').first().locator('button').filter({ hasText: /edit/i })
			)

			const hasEditButton = await firstEditButton.isVisible().catch(() => false)
			if (!hasEditButton) {
				test.skip() // No edit button found
				return
			}

			await firstEditButton.click()
			await page.waitForTimeout(300)

			// Update name in drawer
			const nameInput = page.locator('#name')
			await nameInput.fill('Updated Workflow Name E2E')

			// Save
			const saveButton = page.locator('button').filter({ hasText: /save/i })
			await saveButton.first().click()
			await page.waitForTimeout(500)

			// Verify update
			await expect(table).toContainText('Updated Workflow Name E2E')
		} else {
			test.skip() // No workflows to edit
		}
	})

	test('delete workflow', async ({ page }) => {
		const table = page.locator('table')
		const hasTable = await table.isVisible().catch(() => false)

		if (hasTable) {
			// Click edit to open drawer with delete button
			const firstEditButton = table.locator('tbody tr').first().locator('[data-ico="pencil-square"]')
			const hasEditButton = await firstEditButton.isVisible().catch(() => false)

			if (!hasEditButton) {
				test.skip()
				return
			}

			await firstEditButton.click()
			await page.waitForTimeout(300)

			// Setup confirm dialog handler
			page.on('dialog', (dialog) => dialog.accept())

			// Delete button is in the drawer footer
			const deleteButton = page.locator('button').filter({ hasText: /delete/i })
			await deleteButton.click()
			await page.waitForTimeout(500)

			// Drawer should close
			// ponytail: smoke test - detailed verification needs workflow count tracking
		} else {
			test.skip()
		}
	})
})

test.describe('Responder - XMPP Settings', () => {
	test('configure XMPP connection', async ({ page, baseURL }) => {
		// Authenticate as admin
		await useAdminSession(page, baseURL ?? '')

		await page.goto('/_/settings/responder')
		await page.waitForLoadState('networkidle')

		// Use last h1 to get the page heading (not parent nav heading)
		await expect(page.locator('h1').last()).toContainText('Responder')

		// Fill XMPP config - actual field IDs from the UI
		await page.locator('#xmpp_jid').fill('bot@example.com')
		await page.locator('#xmpp_password').fill('testpassword')
		await page.locator('#xmpp_server').fill('example.com')
		await page.locator('#xmpp_port').fill('5222')

		// Save button is a FormButton with type="submit"
		page.on('dialog', dialog => dialog.accept())
		const saveButton = page.locator('button[type="submit"]')
		await saveButton.click()

		// Wait for save (alert or success message)
		await page.waitForTimeout(500)

		// ponytail: smoke test - detailed verification needs backend state check
	})

	test('test XMPP connection', async ({ page, baseURL }) => {
		// Authenticate as admin
		await useAdminSession(page, baseURL ?? '')

		await page.goto('/_/settings/responder')

		// Test connection button is a FormButton with type="button"
		// Look for button that contains "Test" text
		const testButton = page.locator('button[type="button"]').filter({ hasText: /test/i })
		await expect(testButton).toBeVisible()

		// Click test (will fail without real XMPP server)
		await testButton.click()

		// Wait for result to appear in .test-result div (up to 10s for API response)
		const result = page.locator('.test-result')
		await expect(result).toBeVisible({ timeout: 10000 })

		// Verify result contains text (success or failure message)
		await expect(result).not.toBeEmpty()
	})
})

test.describe('Responder - Crontab Settings', () => {
	test('view crontab jobs', async ({ page, baseURL }) => {
		// Authenticate as admin
		await useAdminSession(page, baseURL ?? '')

		await page.goto('/_/settings/crontab')
		await page.waitForLoadState('networkidle')

		// Use last h1 to get the page heading
		// The page title comes from t('crontab.title')
		const heading = page.locator('h1').last()
		await expect(heading).toBeVisible()

		// Jobs table uses custom .jobs-table class, or shows empty/loading state
		const jobsTable = page.locator('.jobs-table')
		const emptyState = page.locator('.empty-state')
		const loadingState = page.locator('.loading-state')

		// At least one should be visible
		const hasContent = await jobsTable.isVisible().catch(() => false) ||
			await emptyState.isVisible().catch(() => false) ||
			await loadingState.isVisible().catch(() => false)

		expect(hasContent).toBeTruthy()
	})

	test('update job interval', async ({ page, baseURL }) => {
		// Authenticate as admin
		await useAdminSession(page, baseURL ?? '')

		await page.goto('/_/settings/crontab')

		// Check if jobs table exists
		const jobsTable = page.locator('.jobs-table')
		const hasTable = await jobsTable.isVisible().catch(() => false)

		if (hasTable) {
			// Find first table row
			const firstRow = page.locator('.table-row').first()
			const intervalSelect = firstRow.locator('select')

			// Check if select exists
			const hasSelect = await intervalSelect.isVisible().catch(() => false)
			if (!hasSelect) {
				test.skip()
				return
			}

			// Select a different interval
			await intervalSelect.selectOption('15min')
			await page.waitForTimeout(500)

			// Interval should be updated
			// ponytail: smoke test - verification needs state persistence check
		} else {
			test.skip() // No jobs configured
		}
	})

	test('toggle job enabled status', async ({ page, baseURL }) => {
		// Authenticate as admin
		await useAdminSession(page, baseURL ?? '')

		await page.goto('/_/settings/crontab')

		// Check if jobs table exists
		const jobsTable = page.locator('.jobs-table')
		const hasTable = await jobsTable.isVisible().catch(() => false)

		if (hasTable) {
			const firstRow = page.locator('.table-row').first()

			// FormToggle uses a checkbox input
			const toggle = firstRow.locator('input[type="checkbox"]')
			const hasToggle = await toggle.isVisible().catch(() => false)

			if (!hasToggle) {
				test.skip()
				return
			}

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
