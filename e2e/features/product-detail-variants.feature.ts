import { Page } from 'patchright'
import { expect } from 'patchright/test'

/**
 * Feature Object for Site Product Detail Variant Selector
 * Handles variant selection on product detail page (Size, Color options)
 */
export class ProductDetailVariantsFeature {
  constructor(private page: Page) {}

  async gotoProductsList() {
    // Navigate to root page where products are listed
    await this.page.goto('/')
    await this.page.waitForLoadState('networkidle')
  }

  async gotoProductBySlug(slug: string) {
    // Navigate directly to product detail page by slug
    await this.page.goto(`/products/${slug}`)
    await this.page.waitForLoadState('networkidle')
  }

  async clickFirstProduct() {
    const productLinks = this.page.locator('a[href^="/products/"]')
    await productLinks.first().waitFor({ state: 'visible', timeout: 10000 })
    await productLinks.first().click()
    await this.page.waitForLoadState('networkidle')
  }

  async clickProductWithVariants() {
    let pageNum = 1

    while (true) {
      // Use storefront product card selector
      const productCards = this.page.locator('[data-testid="product-card"]')
      await productCards.first().waitFor({ state: 'visible', timeout: 10000 })
      const count = await productCards.count()

      // Check all products on current page
      for (let i = 0; i < count; i++) {
        const productCard = productCards.nth(i)
        const cardText = await productCard.textContent()

        // Look specifically for "Variant Product" created by admin test
        // OR any product that shows variant count indicator
        const isVariantProduct = cardText?.includes('Variant Product')
        const hasVariantIndicator = cardText?.toLowerCase().includes('variant')

        if (isVariantProduct || hasVariantIndicator) {
          // This product has variants, click the first link (image or title)
          const productLink = productCard.locator('a').first()
          await productLink.waitFor({ state: 'visible', timeout: 5000 })
          await productLink.click()
          await this.page.waitForLoadState('networkidle')
          return // Successfully navigated to product with variants
        }
      }

      // No products with variants on this page, try next page
      const nextButton = this.page.locator('button:has-text("Next"), button:has-text("next"), button:has-text("NEXT")')
      const nextButtonExists = await nextButton.count() > 0

      if (nextButtonExists) {
        const isDisabled = await nextButton.first().isDisabled()
        if (!isDisabled) {
          await nextButton.first().click()
          await this.page.waitForLoadState('networkidle')
          pageNum++
          // Continue to next page
          continue
        }
      }

      // No next button or it's disabled, exhausted all pages
      throw new Error(`No products with variants found across all ${pageNum} pages. Make sure the admin test 'should add a new product with product variants enabled' runs first.`)
    }
  }

  async hasVariantSelector(): Promise<boolean> {
    // Check if product has variant selector by looking for option buttons
    const optionButtons = this.page.locator('button.brutal-btn')
    const count = await optionButtons.count()
    return count > 0
  }

  async verifyVariantSelectorDisplayed() {
    const optionButtons = this.page.locator('button.brutal-btn')
    await expect(optionButtons.first()).toBeVisible({ timeout: 5000 })
  }

  async verifyOptionLabelsExist() {
    // Check for Size or Color labels
    const labels = this.page.locator('label')
    const labelTexts = await labels.allTextContents()
    const hasOption = labelTexts.some(text =>
      text.toUpperCase().includes('SIZE') || text.toUpperCase().includes('COLOR')
    )
    expect(hasOption).toBe(true)
  }

  async clickOptionButton(index: number = 0) {
    const optionButtons = this.page.locator('button.brutal-btn')
    await optionButtons.nth(index).click()
    await this.page.waitForTimeout(500)
  }

  async getSelectedVariantInfo(): Promise<string> {
    const variantCard = this.page.locator('.brutal-card.bg-blue-50')
    const isVisible = await variantCard.isVisible().catch(() => false)
    if (!isVisible) {
      return ''
    }
    return await variantCard.textContent() || ''
  }

  async verifySelectedVariantCardDisplayed() {
    const variantCard = this.page.locator('.brutal-card.bg-blue-50')
    await expect(variantCard).toBeVisible({ timeout: 5000 })
  }

  async verifyAddToCartButtonExists() {
    const addToCartButton = this.page.locator('button', { hasText: /add to cart/i })
    await expect(addToCartButton.first()).toBeVisible({ timeout: 5000 })
  }

  async verifyPriceDisplayed() {
    // Price should contain currency symbols or decimal format
    const bodyText = await this.page.textContent('body') || ''
    const pricePatterns = [
      /\$\d+/,      // Dollar sign
      /€\d+/,       // Euro sign
      /£\d+/,       // Pound sign
      /\d+\.\d{2}/, // Decimal price
    ]

    const hasPrice = pricePatterns.some(pattern => pattern.test(bodyText))
    expect(hasPrice).toBe(true)
  }

  async isAddToCartEnabled(): Promise<boolean> {
    const addToCartButton = this.page.locator('button', { hasText: /add to cart/i }).first()
    return await addToCartButton.isEnabled()
  }

  async isOutOfStock(): Promise<boolean> {
    const bodyText = await this.page.textContent('body') || ''
    return bodyText.includes('Out of Stock') ||
           bodyText.includes('out of stock') ||
           bodyText.includes('Not Available')
  }

  async getOptionButtonCount(): Promise<number> {
    const optionButtons = this.page.locator('button.brutal-btn')
    return await optionButtons.count()
  }
}
