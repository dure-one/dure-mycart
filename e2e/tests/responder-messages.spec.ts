import { test, expect } from '../fixtures/test.fixture'

/**
 * Responder E2E Tests: Messages
 *
 * Tests message viewing and contact linking functionality.
 * Requires XMPP worker to be configured for full test coverage.
 */

test.describe('Responder - Messages', () => {
	test.beforeEach(async ({ page, context }) => {
		// Navigate to messages page
		await page.goto('/_/responder/messages')
		await page.waitForLoadState('networkidle')
	})

	test('view message threads list', async ({ page }) => {
		// Check page loaded
		await expect(page.locator('h1')).toContainText('Messages')

		// Check thread list exists (may be empty)
		const threadList = page.locator('.thread-list')
		await expect(threadList).toBeVisible()
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
		const channelFilter = page.locator('select').first()
		await expect(channelFilter).toBeVisible()

		// Select SMS channel
		await channelFilter.selectOption('sms')
		await page.waitForTimeout(300)

		// Verify filter applied (threads reload or filter client-side)
		// ponytail: basic smoke test - detailed assertions need test data
	})

	test('search conversations', async ({ page }) => {
		const searchInput = page.locator('input[placeholder*="Search"]')
		await expect(searchInput).toBeVisible()

		await searchInput.fill('test')
		await page.waitForTimeout(300)

		// Verify search applied
		// ponytail: smoke test - full verification needs test data
	})
})
