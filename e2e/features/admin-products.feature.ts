import { Page } from 'patchright'
import { expect } from 'patchright/test'

import { useAdminSession } from '../utils/admin-page'

/**
 * Feature Object for Admin Product Management
 * Handles product list viewing and product creation
 */
export class AdminProductsFeature {
  constructor(
    private page: Page,
    private baseURL: string
  ) {}

  async goto() {
    // The suite's session is handed over before navigating: the panel
    // redirects to `/_/signin` without it, and signing in there would spend
    // the server's rate limit once per test.
    await useAdminSession(this.page, this.baseURL)
    await this.page.goto('/_/products')
    // Wait for navigation to settle
    await this.page.waitForLoadState('networkidle')
  }

  async waitForPage() {
    // Wait for the page title (h1) to be visible, which indicates page has loaded
    await this.page.waitForSelector('h1', { timeout: 10000 })
    // Wait a bit for the page content to render
    await this.page.waitForTimeout(500)
  }

  async verifyPageLoaded() {
    await expect(this.page).toHaveURL('/_/products')
    await this.waitForPage()
  }

  async hasProducts() {
    const products = await this.page.locator('[data-testid="product-row"]').count()
    return products > 0
  }

  async getProductCount() {
    return await this.page.locator('[data-testid="product-row"]').count()
  }

  async openAddProductDrawer() {
    // Wait for page to be ready
    await this.waitForPage()
    // Find the green add button - it should be the only green button on the page
    const addButton = this.page.locator('button').filter({ hasText: /add product/i }).or(this.page.locator('button.bg-green-600'))
    await addButton.first().waitFor({ state: 'visible', timeout: 10000 })
    await addButton.first().click()
    // Wait for drawer to open by checking for the Name input field
    await this.page.getByRole('textbox', { name: 'Name' }).waitFor({ state: 'visible', timeout: 10000 })
  }

  async fillProductForm(data: {
    name: string
    slug: string
    brief?: string
    description?: string
    amount?: string
    digitalType?: 'file' | 'data' | 'api'
  }) {
    // Use accessible role-based selectors for better reliability
    await this.page.getByRole('textbox', { name: 'Name' }).fill(data.name)
    await this.page.getByRole('textbox', { name: 'URL' }).fill(data.slug)

    // Fill brief if provided
    if (data.brief) {
      await this.page.getByRole('textbox', { name: 'Brief' }).fill(data.brief)
    }

    // Fill description if provided
    if (data.description) {
      // Description is in a rich text editor (article > textbox)
      const descField = this.page.locator('article textbox, article [contenteditable="true"]').first()
      await descField.fill(data.description)
    }

    // Fill amount if provided
    if (data.amount) {
      await this.page.getByRole('textbox', { name: 'Amount' }).fill(data.amount)
    }

    // Select digital type if provided (required field)
    if (data.digitalType) {
      await this.page.getByRole('combobox', { name: 'Digital type' }).selectOption(data.digitalType)
    }
  }

  async saveProduct() {
    const saveButton = this.page.locator('button:has-text("SAVE"), button:has-text("Save"), button[type="submit"]')
    await saveButton.first().click()
    // Wait for drawer to close (Name field should disappear after successful save)
    await this.page.getByRole('textbox', { name: 'Name' }).waitFor({ state: 'hidden', timeout: 10000 })
  }

  async verifyProductExists(productName: string) {
    // Use partial text match since product name appears with description in same cell
    const product = this.page.getByText(productName, { exact: false })
    await expect(product).toBeVisible({ timeout: 10000 })
  }

  async closeDrawer() {
    // Try to close drawer by clicking close button or outside
    const closeButton = this.page.locator('button[aria-label*="close" i], button:has-text("×")')
    if (await closeButton.count() > 0) {
      await closeButton.first().click()
    }
  }
}
