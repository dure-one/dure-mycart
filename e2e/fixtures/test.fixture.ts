import { test as base } from 'patchright/test'
import { ProductListFeature } from '../features/product-list.feature'
import { ProductDetailFeature } from '../features/product-detail.feature'
import { CartFeature } from '../features/cart.feature'
import { CheckoutFeature } from '../features/checkout.feature'
import { AdminProductsFeature } from '../features/admin-products.feature'
import { AdminCartsFeature } from '../features/admin-carts.feature'
import { AdminInstallFeature } from '../features/admin-install.feature'
import { AdminApi } from '../utils/api'
import { useAdminSession } from '../utils/admin-page'

type Fixtures = {
  productList: ProductListFeature
  productDetail: ProductDetailFeature
  cart: CartFeature
  checkout: CheckoutFeature
  adminProducts: AdminProductsFeature
  adminCarts: AdminCartsFeature
  adminInstall: AdminInstallFeature
  /** Admin session for setting up test data; cleans up the products it creates. */
  adminApi: AdminApi
}

export const test = base.extend<Fixtures>({
  productList: async ({ page }, use) => {
    const productList = new ProductListFeature(page)
    await use(productList)
  },
  productDetail: async ({ page }, use) => {
    const productDetail = new ProductDetailFeature(page)
    await use(productDetail)
  },
  cart: async ({ page }, use) => {
    const cart = new CartFeature(page)
    await use(cart)
  },
  checkout: async ({ page }, use) => {
    const checkout = new CheckoutFeature(page)
    await use(checkout)
  },
  adminProducts: async ({ page, baseURL }, use) => {
    const adminProducts = new AdminProductsFeature(page, baseURL ?? '')
    await use(adminProducts)
  },
  adminCarts: async ({ page, baseURL }, use) => {
    // Admin pages are behind the sign-in page; the session global setup
    // established spares every test the login detour and its rate limit.
    await useAdminSession(page, baseURL ?? '')
    await use(new AdminCartsFeature(page))
  },
  adminInstall: async ({ page }, use) => {
    const adminInstall = new AdminInstallFeature(page)
    await use(adminInstall)
  },
  adminApi: async ({ baseURL }, use) => {
    const adminApi = await AdminApi.connect(baseURL ?? 'http://localhost:8080')
    await use(adminApi)
    await adminApi.dispose()
  },
})

export { expect } from 'patchright/test'
