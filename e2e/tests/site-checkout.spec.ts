import { test } from '../fixtures/test.fixture'
import { AdminApi, ProductInput, Product } from '../utils/api'

/**
 * Site E2E Tests: Checkout and Orders
 *
 * Free items are the only carts that can be paid for here: the built-in dummy
 * provider is refused for anything with a total above 0, and the test store
 * has no payment account configured. Everything else about the checkout is
 * still covered — the order that is written, the success page, the admin view
 * of the order and the refusal when an item is no longer available.
 *
 * Paying is rate limited together with sign-in and installation (ten requests
 * a minute per address), so the four tests here that pay, and nothing else,
 * have to fit in that budget alongside the suite's one sign-in.
 */

/** Creates a product nothing else depends on; it is deleted after the test. */
async function createProduct(
  adminApi: AdminApi,
  name: string,
  overrides: Partial<ProductInput> = {}
): Promise<Product> {
  const token = Math.random().toString(36).slice(2, 8)
  return adminApi.createProduct({
    name: `${name} ${token}`,
    slug: `e2e-${token}`,
    brief: 'Created by the e2e checkout suite',
    description: 'Created by the e2e checkout suite.',
    amount: 0,
    quantity: 10,
    sku: `E2E-${token}`,
    digital: { type: 'file' },
    ...overrides
  })
}

test.describe('Site - Checkout', () => {
  test('should complete checkout of a free item', async ({ adminApi, productList, checkout }) => {
    // ARRANGE: a free item is in the cart
    const product = await createProduct(adminApi, 'E2E Free Item')
    await productList.goto()
    await productList.waitForProducts()
    await productList.addToCartByName(product.name)

    // ACT: check out with an email address
    await checkout.goto()
    await checkout.verifyPageLoaded()
    await checkout.completeCheckout('checkout-buyer@example.com')

    // ASSERT: the success page describes the order
    await checkout.verifyOrder(product.name)
  })

  test('should empty the cart once the order is placed', async ({ adminApi, productList, cart, checkout }) => {
    // ARRANGE
    const product = await createProduct(adminApi, 'E2E Empty Cart Item')
    await productList.goto()
    await productList.waitForProducts()
    await productList.addToCartByName(product.name)

    // ACT
    await checkout.goto()
    await checkout.verifyPageLoaded()
    await checkout.completeCheckout('checkout-buyer@example.com')

    // ASSERT: the cart is cleared when the success page loads
    await cart.goto()
    await cart.verifyCartIsEmpty()
  })

  test('should record the order in the admin panel', async ({ adminApi, productList, checkout, adminCarts }) => {
    // ARRANGE
    const product = await createProduct(adminApi, 'E2E Ordered Item')
    const buyer = `e2e-order-${Date.now()}@example.com`
    await productList.goto()
    await productList.waitForProducts()
    await productList.addToCartByName(product.name)

    // ACT
    await checkout.goto()
    await checkout.verifyPageLoaded()
    await checkout.completeCheckout(buyer)

    // ASSERT: the shop owner sees a paid order with the item in it
    await adminCarts.goto()
    await adminCarts.verifyPageLoaded()
    await adminCarts.verifyOrder(buyer, { status: 'paid', paymentSystem: 'dummy' })
    await adminCarts.openOrder(buyer)
    await adminCarts.verifyOrderDetails(product.name)
  })

  test('should refuse an item that sold out while it was in the cart', async ({ adminApi, productList, checkout }) => {
    // ARRANGE: the last unit is in the cart, then it is gone.
    //
    // The count is a licence-key count. A digital file is not stock — the shop
    // stores one file and serves the same bytes to every buyer, so quantity on
    // a file product is ignored and this test would be asserting a refusal the
    // shop is right not to make.
    const product = await createProduct(adminApi, 'E2E Sold Out Item', {
      quantity: 1,
      digital: { type: 'data' }
    })
    await productList.goto()
    await productList.waitForProducts()
    await productList.addToCartByName(product.name)
    await adminApi.updateProduct(product.id, { quantity: 0 })

    // ACT: checkout
    await checkout.goto()
    await checkout.verifyPageLoaded()
    await checkout.submitExpectingValidationError()

    // ASSERT: the order is not placed and the buyer is told why
    await checkout.verifyValidationError(/no longer available/i)
    await checkout.verifyItemNeedsDeletion(product.name)
  })

  test('should require an email before the order can be placed', async ({ adminApi, productList, checkout }) => {
    // ARRANGE
    const product = await createProduct(adminApi, 'E2E Email Required Item')
    await productList.goto()
    await productList.waitForProducts()
    await productList.addToCartByName(product.name)

    // ACT & ASSERT: the button is dead until an address is entered
    await checkout.goto()
    await checkout.verifyPageLoaded()
    await checkout.verifyCheckoutRequiresEmail()
  })

  test('should tell the buyer when no payment system is configured', async ({ adminApi, productList, checkout }) => {
    // ARRANGE: a paid item, and a store without a payment provider
    const product = await createProduct(adminApi, 'E2E Paid Item', { amount: 2500, quantity: 5 })
    await productList.goto()
    await productList.waitForProducts()
    await productList.addToCartByName(product.name)

    // ACT & ASSERT: there is nothing to pay with, and the page says so
    await checkout.goto()
    await checkout.verifyPageLoaded()
    await checkout.verifyNoPaymentSystems()
  })
})
