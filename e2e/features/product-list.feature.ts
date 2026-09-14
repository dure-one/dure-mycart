import { Page } from 'patchright'
import { expect } from 'patchright/test'

/**
 * Feature Object for Product List Page (Storefront)
 * Handles product browsing and add-to-cart from list
 */
export class ProductListFeature {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/')
  }

  async waitForProducts() {
    await this.page.waitForSelector('[data-testid="product-card"]', { timeout: 10000 })
  }

  async verifyPageLoaded() {
    await expect(this.page).toHaveURL('/')
    await this.waitForProducts()
  }

  async getProductCount() {
    const products = await this.page.locator('[data-testid="product-card"]').count()
    return products
  }

  async getProductByIndex(index: number) {
    return this.page.locator('[data-testid="product-card"]').nth(index)
  }

  async addToCartByIndex(index: number) {
    const product = await this.getProductByIndex(index)
    // Look for the green add button (bg-green-500)
    const addButton = product.locator('button.bg-green-500').first()
    await addButton.waitFor({ state: 'visible', timeout: 5000 })
    await addButton.click()
    // Wait for button to change to red (verifies item was added)
    const removeButton = product.locator('button.bg-red-500').first()
    await removeButton.waitFor({ state: 'visible', timeout: 5000 })
  }

  async verifyInCart(index: number) {
    const product = await this.getProductByIndex(index)
    // Look for the red remove button (bg-red-500) within the product card
    const removeButton = product.locator('button.bg-red-500').first()
    await expect(removeButton).toBeVisible({ timeout: 5000 })
  }

  async getProductName(index: number) {
    const product = await this.getProductByIndex(index)
    const name = await product.locator('h3').textContent()
    return name?.trim() || ''
  }
  /**
   * One product card, found by name. Products created for a single test are
   * looked up by name because their position in the list is not fixed.
   */
  cardByName(name: string) {
    return this.page.locator('[data-testid="product-card"]').filter({ hasText: name }).first()
  }

  async waitForProduct(name: string) {
    await this.cardByName(name).waitFor({ state: 'visible', timeout: 10000 })
  }

  async addToCartByName(name: string) {
    const card = this.cardByName(name)
    await card.waitFor({ state: 'visible', timeout: 10000 })
    // Look for the green add button (bg-green-500)
    await card.locator('button.bg-green-500').first().click()
    // Wait for button to change to red (verifies item was added)
    await card.locator('button.bg-red-500').first().waitFor({ state: 'visible', timeout: 5000 })
  }

  async verifyInCartByName(name: string) {
    const removeButton = this.cardByName(name).locator('button.bg-red-500').first()
    await expect(removeButton).toBeVisible({ timeout: 5000 })
  }
}
