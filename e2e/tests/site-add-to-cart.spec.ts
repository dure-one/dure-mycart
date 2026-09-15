import { test, expect } from '../fixtures/test.fixture'

/**
 * Site E2E Tests: Add to Cart Functionality
 * Tests adding products from both list and detail pages
 */

test.describe('Site - Add to Cart', () => {
  test('should add product to cart from product list page', async ({ productList, cart }) => {
    // ARRANGE: Navigate to product list
    await productList.goto()
    await productList.waitForProducts()

    // Get product name before adding to cart
    const productName = await productList.getProductName(0)

    // ACT: Add first product to cart
    await productList.addToCartByIndex(0)

    // ASSERT: Product should show as in cart
    await productList.verifyInCart(0)

    // Verify in cart page
    await cart.goto()
    await cart.verifyCartHasItems(1)
    const cartItemName = await cart.getItemNameByIndex(0)
    expect(cartItemName).toBe(productName)
  })

  test('should add product to cart from product detail page', async ({ page, productList, productDetail, cart }) => {
    // ARRANGE: Get a product slug from list page
    await productList.goto()
    await productList.waitForProducts()

    // Click on first product to go to detail page
    const firstProduct = await productList.getProductByIndex(0)
    await firstProduct.locator('a').first().click()
    await page.waitForURL(/\/products\/.*/)

    // Get current URL to extract slug
    const url = page.url()
    const slug = url.split('/products/')[1]

    // Get product name from detail page
    const productName = await productDetail.getProductName()

    // ACT: Add to cart from detail page
    await productDetail.addToCart()

    // ASSERT: Verify shows as in cart on detail page
    await productDetail.verifyInCart()

    // Verify in cart page
    await cart.goto()
    await cart.verifyCartHasItems(1)
    const cartItemName = await cart.getItemNameByIndex(0)
    expect(cartItemName).toBe(productName)
  })

  test('should offer no quantity for a download, and one where the count is stock', async ({
    page,
    adminApi,
    productList,
    cart
  }) => {
    // ARRANGE: two products that differ only in what the shop delivers.
    //
    // A download is one copy. The shop stores a single file and serves the same
    // bytes to every buyer, so a second copy is that file charged twice — and
    // the count beside it in the panel is left at zero, which is why the page
    // must not read it as stock. A licence key is drawn from a pile the
    // operator counts, and a buyer may take several at once.
    const token = Math.random().toString(36).slice(2, 8)
    const download = await adminApi.createProduct({
      name: `E2E Download ${token}`,
      slug: `e2e-download-${token}`,
      brief: 'One file, one copy',
      amount: 0,
      quantity: 0,
      sku: `E2E-DL-${token}`,
      digital: { type: 'file' }
    })
    const keys = await adminApi.createProduct({
      name: `E2E Licence Keys ${token}`,
      slug: `e2e-licence-keys-${token}`,
      brief: 'Ten keys, sold by the one',
      amount: 0,
      quantity: 10,
      sku: `E2E-KEYS-${token}`,
      digital: { type: 'data' }
    })

    await productList.goto()
    await productList.waitForProducts()

    // ASSERT: the download offers no number to choose; the keys do
    await expect(productList.cardByName(download.name)).toBeVisible()
    await expect(productList.cardByName(download.name).locator('input[aria-label="Quantity"]')).toHaveCount(0)
    await expect(productList.cardByName(keys.name).locator('input[aria-label="Quantity"]')).toHaveCount(1)

    // ACT: the download goes in the cart
    await productList.addToCartByName(download.name)

    // ASSERT: the line is one copy, and there is nothing to step
    await cart.goto()
    await cart.verifyCartHasItems(1)
    expect(await cart.getItemNameByIndex(0)).toBe(download.name)
    await expect(page.locator('[data-testid="cart-item"] input[aria-label="Quantity"]')).toHaveCount(0)
  })

  test('should add multiple products to cart', async ({ productList, cart }) => {
    // ARRANGE
    await productList.goto()
    await productList.waitForProducts()

    // ACT: Add first two products
    await productList.addToCartByIndex(0)
    await productList.addToCartByIndex(1)

    // ASSERT: Both should be in cart
    await productList.verifyInCart(0)
    await productList.verifyInCart(1)

    // Verify cart has 2 items
    await cart.goto()
    await cart.verifyCartHasItems(2)
  })
})
