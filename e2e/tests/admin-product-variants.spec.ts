import { test, expect } from '../fixtures/test.fixture'
import { AdminProductVariantsFeature } from '../features/admin-product-variants.feature'

/**
 * Admin E2E Tests: Product Variants Management
 * Tests creating products with variants (Size, Color options)
 */

test.describe('Admin - Product Variants', () => {
  test('should add a new product with product variants enabled', async ({ page }) => {
    const feature = new AdminProductVariantsFeature(page)

    // ARRANGE: Navigate to products page
    await feature.goto()
    await feature.waitForPage()

    const initialCount = await feature.getProductCount()

    // Open add product drawer
    await feature.openAddProductDrawer()

    // ACT: Fill basic product info
    const timestamp = Date.now()
    const productName = `Variant Product ${timestamp}`
    const productSlug = `variant-${timestamp}`

    await feature.fillBasicProductInfo({
      name: productName,
      slug: productSlug,
      brief: 'Test product with size and color variants',
      amount: '100.00',
      digitalType: 'file'
    })

    // Toggle Product Variants ON
    await feature.toggleProductVariants()

    // Add Option 1: Size with values s, m, l
    await feature.addOption('Size', ['s', 'm', 'l'])

    // Add Option 2: Color with values blue, white
    await feature.addOption('Color', ['blue', 'white'])

    // System should auto-generate 6 variants (s×blue, s×white, m×blue, m×white, l×blue, l×white)
    // Fill SKUs, quantities, and price surcharges for all 6 variants
    const variants = Array.from({ length: 6 }, (_, i) => ({
      sku: `SKU-${timestamp}-${i + 1}`,
      quantity: 10,
      priceSurcharge: 0
    }))
    await feature.fillVariantDetails(variants)

    // Save the product
    await feature.saveProduct()

    // Wait for save to complete
    await page.waitForTimeout(2000)

    // ASSERT: Product should be added to list
    await feature.goto()
    await feature.waitForPage()

    // Verify product count increased
    const newCount = await feature.getProductCount()
    expect(newCount).toBeGreaterThan(initialCount)

    // Verify product exists
    await feature.verifyProductExists(productName)
  })

  test('should verify variants persist after saving', async ({ page }) => {
    const feature = new AdminProductVariantsFeature(page)

    // ARRANGE: Navigate to products page
    await feature.goto()
    await feature.waitForPage()

    // Find a product with "Variant Product" in its name (created by first test)
    const variantProductRow = page.locator('[data-testid="product-row"]')
      .filter({ hasText: /Variant Product/i })

    const variantCount = await variantProductRow.count()
    if (variantCount === 0) {
      test.skip() // Skip if no variant product exists
    }

    // ACT: Open the variant product (not just any first product)
    await variantProductRow.first().click()
    await page.waitForTimeout(1000)

    // ASSERT: Verify variants section exists
    await feature.verifyVariantsExist()

    // Verify at least one option exists (Size or Color)
    const hasSizeOption = await page.getByText('Size', { exact: false }).count()
    const hasColorOption = await page.getByText('Color', { exact: false }).count()

    expect(hasSizeOption + hasColorOption).toBeGreaterThan(0)
  })
})
