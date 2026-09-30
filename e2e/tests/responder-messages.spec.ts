import { test, expect } from '../fixtures/test.fixture'
import { useAdminSession } from '../utils/admin-page'

/**
 * Responder E2E Tests: Messages
 *
 * Tests message viewing and contact linking functionality.
 * Requires XMPP worker to be configured for full test coverage.
 */

test.describe('Responder - Messages', () => {
	test.beforeEach(async ({ page, baseURL }) => {
		// Authenticate as admin
		await useAdminSession(page, baseURL ?? '')

		// Navigate to messages page
		await page.goto('/_/responder/messages')
		await page.waitForLoadState('networkidle')
	})

	test('view message threads list', async ({ page }) => {
		// Check page loaded - use last h1 to get the page heading (not parent nav heading)
		await expect(page.locator('h1').last()).toContainText('Messages')

		// Page should load without errors
		// Check for channel filter (always present)
		const channelFilter = page.locator('select.select')
		await expect(channelFilter).toBeVisible()

		// Verify we can interact with the page (no crashes)
		const options = await channelFilter.locator('option').count()
		expect(options).toBeGreaterThan(0)
	})

	test('select thread and view messages', async ({ page }) => {
		// Check if threads exist
		const firstThread = page.locator('.thread-item').first()
		const hasThreads = await firstThread.isVisible().catch(() => false)

		if (hasThreads) {
			await firstThread.click()
			await page.waitForTimeout(500)

			// Chat panel should be visible
			const chatPanel = page.locator('.chat-panel')
			await expect(chatPanel).toBeVisible()
		} else {
			// Skip if no threads - needs XMPP messages
			test.skip()
		}
	})

	test('link contact to different customer', async ({ page }) => {
		// ponytail: outline test - full implementation needs customer data
		// 1. Select a thread with a contact
		// 2. Click "Link Contact" button
		// 3. Enter target customer ID in modal
		// 4. Confirm linking
		// 5. Verify threads combined under target customer

		test.skip() // Requires test data setup
	})

	test('filter threads by channel', async ({ page }) => {
		// Channel filter is in the PageHeader actions snippet
		const channelFilter = page.locator('select.select')
		await expect(channelFilter).toBeVisible()

		// Check that filter has expected options
		const options = await channelFilter.locator('option').allTextContents()
		expect(options.some(opt => opt.includes('XMPP') || opt.includes('SMS'))).toBeTruthy()

		// Select a channel if options exist
		const optionValues = await channelFilter.locator('option').evaluateAll(
			elements => elements.map(el => (el as HTMLOptionElement).value)
		)
		if (optionValues.includes('sms')) {
			await channelFilter.selectOption('sms')
			await page.waitForTimeout(300)
		} else if (optionValues.includes('xmpp')) {
			await channelFilter.selectOption('xmpp')
			await page.waitForTimeout(300)
		}

		// Verify filter applied (page should reload or update)
		// ponytail: basic smoke test - detailed assertions need test data
	})

	test('search conversations', async ({ page }) => {
		// Search functionality not implemented in current UI
		// The messages page has channel filter but no search input
		test.skip()
	})
})
