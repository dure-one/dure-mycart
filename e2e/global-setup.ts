import { FullConfig } from 'patchright'

import { ADMIN_EMAIL, ADMIN_PASSWORD, AdminApi, apiURL, saveSession, signIn, waitForServer } from './utils/api'

/**
 * Playwright Global Setup
 *
 * Runs once before all tests to:
 * 1. Wait for the server the config started
 * 2. Install the store (if not already installed)
 * 3. Establish the admin session the tests share
 * 4. Seed test data (10+ products)
 *
 * The server itself is started by playwright.config.ts, which cleans the
 * database directory first so every run begins on a fresh store.
 */
async function globalSetup(config: FullConfig) {
  const baseURL = config.projects[0].use.baseURL || 'http://localhost:8080'

  console.log('⏳ Waiting for server to be ready...')
  await waitForServer(baseURL)
  console.log('  ✓ Server is ready')

  console.log('📦 Checking installation...')
  await installStore(baseURL)

  console.log('🔑 Establishing the session the tests share...')
  // Sign-in, installation and payment share a rate limit of ten requests a
  // minute per address, and the whole suite runs from one address, so the
  // tests reuse one session instead of each establishing its own. It has to
  // be a real sign-in: a token left by an earlier run names a database the
  // server no longer has.
  saveSession(baseURL, await signIn(baseURL))
  console.log('  ✓ Session established')

  console.log('🌱 Seeding test data...')
  await seedTestData(baseURL)

  console.log('✅ Global setup complete!')
}

/**
 * The store domain the installation records.
 *
 * Payment redirects are built from it, so it has to name the server under
 * test — a store that believes it lives at localhost:8080 sends the buyer
 * somewhere else when the suite runs on another port.
 */
function storeDomain(baseURL: string): string {
  const { hostname, port } = new URL(baseURL)
  const host = hostname === '127.0.0.1' ? 'localhost' : hostname
  return port ? `${host}:${port}` : host
}

/**
 * Install store via API (matches browser behavior from HAR log)
 */
async function installStore(baseURL: string) {
  const statusResponse = await fetch(apiURL(baseURL, '/api/install/status'))
  const status = await statusResponse.json()

  if (status.result?.installed) {
    console.log('  ℹ Store already installed, skipping')
    return
  }

  const response = await fetch(apiURL(baseURL, '/api/install'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      email: ADMIN_EMAIL,
      password: ADMIN_PASSWORD,
      domain: storeDomain(baseURL)
    })
  })

  const text = await response.text()
  if (!response.ok) {
    throw new Error(`Installation failed: ${response.status} ${text}`)
  }

  const result = text ? JSON.parse(text) : {}
  if (!result.success) {
    throw new Error(`Installation failed: ${result.message}`)
  }

  console.log('  ✓ Store installed via API')
}

/**
 * Seed test data (ensure 10+ active products exist)
 */
async function seedTestData(baseURL: string) {
  const admin = await AdminApi.connect(baseURL)
  const productCount = 15

  const existingProducts = await admin.products(100)
  console.log(`  → Found ${existingProducts.length} existing products`)

  const allProductIds: string[] = []

  for (let i = 1; i <= productCount; i++) {
    const slug = `test-product-${i}`
    const existing = existingProducts.find((p) => p.slug === slug)

    if (existing) {
      allProductIds.push(existing.id)
      continue
    }

    const product = await admin.createProduct({
      name: `Test Product ${i}`,
      slug,
      brief: `This is test product ${i} for E2E testing`,
      description: `Detailed description for test product ${i}. This product is used for automated testing.`,
      amount: 999 + i * 100,
      quantity: 100,
      sku: `TEST-${String(i).padStart(3, '0')}`,
      digital: { type: 'file' }
    })
    allProductIds.push(product.id)
  }

  console.log(`  ✓ Ensured ${allProductIds.length}/${productCount} products exist`)

  // Enable all products, including the ones that were already there
  let enabledCount = 0
  for (const productId of allProductIds) {
    try {
      await admin.setActive(productId, true)
      enabledCount++
    } catch {
      // leave it disabled; the suite only needs ten active products
    }
  }

  console.log(`  ✓ Enabled ${enabledCount}/${allProductIds.length} products`)
}

export default globalSetup
