import { Page } from 'patchright'
import { expect } from 'patchright/test'

/**
 * Feature Object for the Admin Cart (order) List
 * Handles viewing orders placed on the storefront
 */
export class AdminCartsFeature {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/_/carts')
  }

  async waitForPage() {
    await this.page.waitForSelector('h1', { timeout: 10000 })
  }

  async verifyPageLoaded() {
    await expect(this.page).toHaveURL('/_/carts')
    await this.waitForPage()
  }

  /** The panel that slides in with the details of one order. */
  get drawer() {
    return this.page.locator('#drawer_content')
  }

  /** The list row of one order, found by the buyer's address. */
  rowFor(email: string) {
    return this.page.locator('tbody tr').filter({ hasText: email }).first()
  }

  async verifyOrder(email: string, expected: { status: string; paymentSystem: string }) {
    const row = this.rowFor(email)
    await expect(row).toBeVisible({ timeout: 10000 })
    await expect(row).toContainText(expected.status)
    await expect(row).toContainText(expected.paymentSystem)
  }

  async openOrder(email: string) {
    await this.rowFor(email).click()
    await expect(this.drawer).toBeVisible({ timeout: 10000 })
  }

  async verifyOrderDetails(expected: string) {
    await expect(this.drawer).toContainText(expected, { timeout: 10000 })
  }
}
