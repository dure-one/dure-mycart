import { Page } from 'patchright'
import { expect } from 'patchright/test'

/**
 * Feature Object for the checkout half of the storefront cart page.
 *
 * A cart of free items is paid through the built-in dummy provider, which is
 * the only payment flow the suite can finish without a payment account: the
 * order is written, marked paid and shown on the success page. A paid cart
 * cannot leave the cart page at all while no payment system is configured.
 */
export class CheckoutFeature {
  constructor(private page: Page) {}

  get emailInput() {
    return this.page.locator('#email')
  }

  get submitButton() {
    return this.page.locator('form button[type="submit"]')
  }

  async goto() {
    await this.page.goto('/cart')
  }

  async waitForCart() {
    // Either the cart has items or it is empty: both are settled pages.
    const items = this.page.locator('[data-testid="cart-item"]')
    const empty = this.page.getByText(/cart is empty/i)
    await expect(items.or(empty).first()).toBeVisible({ timeout: 10000 })
  }

  async verifyPageLoaded() {
    await expect(this.page).toHaveURL('/cart')
    await this.waitForCart()
  }

  async verifyCheckoutRequiresEmail() {
    await expect(this.submitButton).toBeDisabled()
    await expect(this.submitButton).toHaveText(/get for free/i)
    await this.emailInput.fill('buyer@example.com')
    await expect(this.submitButton).toBeEnabled()
  }

  /** Fills in the buyer address, pays and waits for the success page. */
  async completeCheckout(email: string) {
    await this.emailInput.fill(email)
    await expect(this.submitButton).toBeEnabled()
    await this.submitButton.click()
    await this.page.waitForURL(/\/cart\/payment\/success/, { timeout: 20000 })
    await this.waitForSuccessPage()
  }

  async waitForSuccessPage() {
    await expect(this.page.getByRole('heading', { name: /payment successful/i })).toBeVisible({ timeout: 15000 })
  }

  /** Asserts the success page describes the order that was just placed. */
  async verifyOrder(productName: string) {
    await expect(this.page.getByRole('heading', { name: /order details/i })).toBeVisible()
    await expect(this.page.getByRole('link', { name: productName })).toBeVisible()
    await expect(this.page.getByText(/thank you for your purchase/i)).toBeVisible()
  }

  /** Submits a cart the server rejects and waits for the review dialog. */
  async submitExpectingValidationError() {
    await this.emailInput.fill('buyer@example.com')
    await this.submitButton.click()
  }

  async verifyValidationError(expected: RegExp) {
    const dialog = this.page.getByRole('dialog').filter({ hasText: /some items need attention/i })
    await expect(dialog).toBeVisible({ timeout: 10000 })
    await expect(dialog).toContainText(expected)
    // The checkout did not go through.
    await expect(this.page).toHaveURL('/cart')
  }

  async verifyItemNeedsDeletion(productName: string) {
    const item = this.page.locator('[data-testid="cart-item"]').filter({ hasText: productName }).first()
    await expect(item).toHaveClass(/needs-deletion/)
  }

  /** A paid cart with no payment system configured: nothing to do but tell the buyer. */
  async verifyNoPaymentSystems() {
    await expect(this.page.getByText(/no payment systems available/i)).toBeVisible({ timeout: 10000 })
    await expect(this.emailInput).toBeHidden()
    await expect(this.submitButton).toHaveCount(0)
  }
}
