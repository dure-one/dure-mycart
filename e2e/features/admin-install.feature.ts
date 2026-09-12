import { Page } from 'patchright'
import { expect } from 'patchright/test'

export interface Account {
  email: string
  password: string
  domain?: string
}

/**
 * Feature Object for the Admin Install Wizard
 *
 * Only exists on a server that has not been installed yet, which is why the
 * tests that use it start a server of their own. The domain is filled in
 * explicitly: the wizard defaults it to the address the browser used, and a
 * bare IP address is not a valid store domain.
 */
export class AdminInstallFeature {
  constructor(private page: Page) {}

  get emailInput() {
    return this.page.locator('#email')
  }

  get passwordInput() {
    return this.page.locator('#password')
  }

  get domainInput() {
    return this.page.locator('#domain')
  }

  get driverSelect() {
    return this.page.locator('#database-driver')
  }

  get lockedNotice() {
    return this.page.getByText(/database is fixed by the server configuration/i)
  }

  get hostInput() {
    return this.page.locator('#pg-host')
  }

  get portInput() {
    return this.page.locator('#pg-port')
  }

  get databaseNameInput() {
    return this.page.locator('#pg-database')
  }

  get userInput() {
    return this.page.locator('#pg-user')
  }

  get databasePasswordInput() {
    return this.page.locator('#pg-password')
  }

  get dsnInput() {
    return this.page.locator('#database-dsn')
  }

  get dsnToggle() {
    return this.page.locator('label[for="toggle_database-dsn-mode"]')
  }

  get testConnectionButton() {
    return this.page.getByRole('button', { name: /test connection|testing/i })
  }

  get installButton() {
    return this.page.locator('form button[type="submit"]')
  }

  get databaseError() {
    return this.page.locator('fieldset span.text-red-500')
  }

  async goto(baseURL?: string) {
    await this.page.goto(baseURL ? `${baseURL}/_/install` : '/_/install')
    await this.page.waitForSelector('h1', { timeout: 10000 })
  }

  async verifyPageLoaded() {
    await expect(this.page).toHaveURL(/\/_\/install$/)
    await expect(this.driverSelect).toBeVisible({ timeout: 10000 })
  }

  async fillAccount({ email, password, domain }: Account) {
    await this.emailInput.fill(email)
    await this.passwordInput.fill(password)
    if (domain !== undefined) {
      await this.domainInput.fill(domain)
    }
  }

  async selectDriver(driver: 'sqlite' | 'postgres') {
    await this.driverSelect.selectOption(driver)
  }

  async fillPostgresConnection(connection: {
    host?: string
    port?: string
    database?: string
    user?: string
    password?: string
  }) {
    if (connection.host !== undefined) await this.hostInput.fill(connection.host)
    if (connection.port !== undefined) await this.portInput.fill(connection.port)
    if (connection.database !== undefined) await this.databaseNameInput.fill(connection.database)
    if (connection.user !== undefined) await this.userInput.fill(connection.user)
    if (connection.password !== undefined) await this.databasePasswordInput.fill(connection.password)
  }

  /** Switches the PostgreSQL fields to a single connection string. */
  async useConnectionString(dsn: string) {
    await this.dsnToggle.click()
    await this.dsnInput.fill(dsn)
  }

  async testConnection() {
    await this.testConnectionButton.click()
    await expect(this.testConnectionButton).toBeEnabled({ timeout: 15000 })
  }

  async install() {
    await this.installButton.click()
  }

  async verifyInstallBlocked() {
    await expect(this.installButton).toBeDisabled()
  }

  async verifyInstallAllowed() {
    await expect(this.installButton).toBeEnabled()
  }

  async verifyDatabaseError(expected: RegExp) {
    await expect(this.databaseError).toBeVisible({ timeout: 15000 })
    await expect(this.databaseError).toContainText(expected)
  }

  async verifyDatabaseChoiceLocked() {
    await expect(this.lockedNotice).toBeVisible({ timeout: 10000 })
    await expect(this.driverSelect).toHaveCount(0)
  }

  /** Waits for the redirect the wizard does after a successful installation. */
  async verifyRedirectToSignIn() {
    await this.page.waitForURL(/\/_\/signin$/, { timeout: 15000 })
  }

  /** Signs in with the account the wizard just created. */
  async signInAs({ email, password }: Account) {
    await expect(this.page.locator('#email')).toBeVisible({ timeout: 10000 })
    await this.page.locator('#email').fill(email)
    await this.page.locator('#password').fill(password)
    await this.page.locator('form button[type="submit"]').click()
    await this.page.waitForURL(/\/_\/products$/, { timeout: 15000 })
  }
}
