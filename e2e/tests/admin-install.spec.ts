import { test, expect } from '../fixtures/test.fixture'
import { installStatus, testDatabaseConnection } from '../utils/api'
import { IsolatedServer, startIsolatedServer } from '../utils/server'

/**
 * Admin E2E Tests: Install Wizard
 *
 * These run against servers of their own: the suite's shared server is
 * installed before the first test, and the wizard only exists before that.
 * They also stop and start, so the tests in each block are serial.
 *
 * The domain is filled in by hand: the wizard derives it from the address the
 * browser used, and a bare IP address is not a valid store domain.
 */

const NEW_ADMIN = { email: 'owner@example.com', password: 'test1234' }

/** Nothing pins the database for these: the choice must be the wizard's. */
const UNPINNED = { MYCART_DB_DRIVER: '', MYCART_DB_DSN: '' }

test.describe.serial('Admin - Install wizard (fresh store)', () => {
  let server: IsolatedServer

  test.beforeAll(async () => {
    server = await startIsolatedServer(UNPINNED)
  })

  test.afterAll(async () => {
    await server?.stop()
  })

  test('should send an uninstalled store to the wizard', async ({ page }) => {
    // ACT: the storefront is not served before the store exists
    await page.goto(`${server.baseURL}/`)

    // ASSERT
    await expect(page).toHaveURL(`${server.baseURL}/_/install`, { timeout: 15000 })
  })

  test('should offer the database engine and start on SQLite', async ({ adminInstall }) => {
    // ARRANGE & ACT
    await adminInstall.goto(server.baseURL)
    await adminInstall.verifyPageLoaded()

    // ASSERT: SQLite by default, PostgreSQL fields only once it is chosen
    await expect(adminInstall.driverSelect).toHaveValue('sqlite')
    await expect(adminInstall.hostInput).toHaveCount(0)

    await adminInstall.selectDriver('postgres')
    await expect(adminInstall.hostInput).toBeVisible()
    await expect(adminInstall.databaseNameInput).toBeVisible()
  })

  test('should install the store and let the new admin sign in', async ({ page, adminInstall }) => {
    // ARRANGE
    await adminInstall.goto(server.baseURL)
    await adminInstall.fillAccount({ ...NEW_ADMIN, domain: `localhost:${server.port}` })

    // ACT
    await adminInstall.install()

    // ASSERT: the wizard hands over to the sign-in page...
    await adminInstall.verifyRedirectToSignIn()
    await adminInstall.signInAs(NEW_ADMIN)
    await expect(page).toHaveURL(/\/_\/products$/)

    // ...the server reports the installation...
    const status = await installStatus(server.baseURL)
    expect(status.installed).toBe(true)
    expect(status.database.driver).toBe('sqlite')
    expect(status.database.locked).toBe(false)

    // ...and the storefront is served at last.
    await page.goto(`${server.baseURL}/`)
    await expect(page.getByText(/no products found/i)).toBeVisible({ timeout: 15000 })
  })
})

test.describe.serial('Admin - Install wizard (PostgreSQL selected)', () => {
  let server: IsolatedServer

  test.beforeAll(async () => {
    server = await startIsolatedServer(UNPINNED)
  })

  test.afterAll(async () => {
    await server?.stop()
  })

  test('should not install into PostgreSQL before the connection is tested', async ({ adminInstall }) => {
    // ARRANGE
    await adminInstall.goto(server.baseURL)
    await adminInstall.selectDriver('postgres')
    await adminInstall.fillPostgresConnection({
      host: '127.0.0.1',
      port: '5432',
      database: 'mycart',
      user: 'mycart',
      password: 'secret'
    })

    // ACT & ASSERT: the wizard will not write anywhere it has not reached
    await adminInstall.verifyInstallBlocked()
  })

  test('should report an unreachable server and stay uninstallable', async ({ adminInstall }) => {
    // ARRANGE: port 1 has no listener, so the connection is refused at once
    await adminInstall.goto(server.baseURL)
    await adminInstall.selectDriver('postgres')
    await adminInstall.fillPostgresConnection({
      host: '127.0.0.1',
      port: '1',
      database: 'mycart',
      user: 'mycart',
      password: 'secret'
    })

    // ACT
    await adminInstall.testConnection()

    // ASSERT: an explanation the operator can act on, and no installation
    await adminInstall.verifyDatabaseError(/not reachable/)
    await adminInstall.verifyInstallBlocked()
    expect((await installStatus(server.baseURL)).installed).toBe(false)
  })

  test('should install into PostgreSQL when a server is offered', async ({ adminInstall }) => {
    const dsn = process.env.E2E_POSTGRES_DSN
    if (!dsn) {
      test.skip(true, 'set E2E_POSTGRES_DSN to an empty PostgreSQL database to run this test')
      return
    }

    // The database has to be empty: the wizard refuses one that holds a cart.
    const probe = await testDatabaseConnection(server.baseURL, { driver: 'postgres', dsn })
    if (probe.result?.has_existing_schema) {
      test.skip(true, 'E2E_POSTGRES_DSN already holds an installed cart; drop the database to run this test')
      return
    }

    // ARRANGE
    await adminInstall.goto(server.baseURL)
    await adminInstall.selectDriver('postgres')
    await adminInstall.useConnectionString(dsn)
    await adminInstall.testConnection()
    await adminInstall.verifyInstallAllowed()
    await adminInstall.fillAccount({ ...NEW_ADMIN, domain: `localhost:${server.port}` })

    // ACT
    await adminInstall.install()
    await adminInstall.verifyRedirectToSignIn()

    // ASSERT: the running process now serves from PostgreSQL
    const status = await installStatus(server.baseURL)
    expect(status.installed).toBe(true)
    expect(status.database.driver).toBe('postgres')
    expect(status.database.source).toBe('wizard')

    // ...without ever handing the password back to the browser
    const password = new URL(dsn).password
    if (password) {
      expect(status.database.dsn).not.toContain(password)
    }
  })
})

test.describe.serial('Admin - Install wizard (database fixed by the operator)', () => {
  let server: IsolatedServer

  test.beforeAll(async () => {
    // A database named in the environment is pinned: the wizard must not offer
    // to put the cart anywhere else.
    server = await startIsolatedServer({ MYCART_DB_DSN: './pinned.db' })
  })

  test.afterAll(async () => {
    await server?.stop()
  })

  test('should show the pinned database instead of the engine selection', async ({ adminInstall }) => {
    // ARRANGE & ACT
    await adminInstall.goto(server.baseURL)

    // ASSERT
    await adminInstall.verifyDatabaseChoiceLocked()
    await expect(adminInstall.lockedNotice).toContainText('./pinned.db')
    // Locked, but installable: the operator has already decided where.
    await adminInstall.verifyInstallAllowed()
  })

  test('should install into the pinned database', async ({ page, adminInstall }) => {
    // ARRANGE
    await adminInstall.goto(server.baseURL)
    await adminInstall.fillAccount({ ...NEW_ADMIN, domain: `localhost:${server.port}` })

    // ACT
    await adminInstall.install()
    await adminInstall.verifyRedirectToSignIn()
    await adminInstall.signInAs(NEW_ADMIN)

    // ASSERT
    await expect(page).toHaveURL(/\/_\/products$/)
    const status = await installStatus(server.baseURL)
    expect(status.installed).toBe(true)
    expect(status.database.source).toBe('env')
  })
})
