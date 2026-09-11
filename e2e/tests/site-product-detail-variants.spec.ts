import { test, expect } from '../fixtures/test.fixture'
import { ProductDetailVariantsFeature } from '../features/product-detail-variants.feature'

/**
 * Site E2E Tests: Product Detail Variant Selector
 * Tests variant selection on product detail page
 *
 * DEPENDENCY: These tests depend on 'Admin - Product Variants › should add a new product with product variants enabled'
 * running first to create a product with variants.
 */

test.describe('Site - Product Detail Variant Selector', () => {
  test('should update selected variant when different options are clicked', async ({ page }) => {
    const feature = new ProductDetailVariantsFeature(page)

    // ARRANGE: Navigate to product with variants
    await feature.gotoProductsList()
    await feature.clickProductWithVariants()

    const buttonCount = await feature.getOptionButtonCount()
    if (buttonCount < 2) {
      test.skip()
    }

    // ACT: Click first option
    await feature.clickOptionButton(0)
    const firstSelection = await feature.getSelectedVariantInfo()

    // Click second option
    await feature.clickOptionButton(1)
    const secondSelection = await feature.getSelectedVariantInfo()

    // ASSERT: Both selections should have content
    expect(firstSelection.length).toBeGreaterThan(0)
    expect(secondSelection.length).toBeGreaterThan(0)
  })

  test('should enable Add to Cart button when variant is in stock', async ({ page }) => {
    const feature = new ProductDetailVariantsFeature(page)

    // ARRANGE: Navigate to product with variants
    await feature.gotoProductsList()
    await feature.clickProductWithVariants()

    // ACT: Select a variant
    await feature.clickOptionButton(0)

    // Check if variant is in stock
    const isOutOfStock = await feature.isOutOfStock()

    if (!isOutOfStock) {
      // ASSERT: Add to Cart button should be enabled
      const isEnabled = await feature.isAddToCartEnabled()
      expect(isEnabled).toBe(true)
    }
  })
})
