import { Page } from 'patchright'

import { ADMIN_EMAIL, ADMIN_PASSWORD, sessionToken } from './api'

/**
 * Hands the suite's admin session to a page, so the admin panel opens without
 * the sign-in form.
 *
 * The admin SPA authenticates from the token cookie alone. Sign-in is rate
 * limited together with installation and payment, and a suite that filled in
 * the form once per admin test would spend the whole budget on it.
 */
export async function useAdminSession(page: Page, baseURL: string): Promise<void> {
  await page.context().addCookies([{ name: 'token', value: await sessionToken(baseURL), url: baseURL }])
}

/**
 * Signs the admin panel in when the browser landed on the sign-in page.
 *
 * Used where the sign-in form is the thing under test, or where the server
 * belongs to the test rather than to the suite: the admin SPA redirects to
 * `/_/signin` whenever the session cookie is missing.
 */
export async function signInIfNeeded(page: Page, email = ADMIN_EMAIL, password = ADMIN_PASSWORD): Promise<void> {
  if (!page.url().includes('/_/signin')) {
    return
  }

  await page.locator('input[name="email"], input[type="email"]').fill(email)
  await page.locator('input[name="password"], input[type="password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL('/_/**', { timeout: 10000 })
}
