import { apiGet } from './api'

/**
 * Asks the server whether this shop has a customer cabinet at all.
 *
 * The switch travels with the public settings, but a cabinet page may not act
 * on that copy: the storefront keeps the settings for up to five minutes to
 * paint the first frame, and an operator who has just turned the cabinet off
 * must not keep being offered a sign-in form until it expires. The API is the
 * authority — while the cabinet is off, every one of its endpoints answers 404
 * — and /me is the cheapest of them to ask.
 *
 * Only an explicit 404 turns the cabinet off. A request that failed for some
 * other reason leaves it on, because the server is still the one that decides
 * what it will serve, and a buyer whose connection dropped should not be told
 * the shop has no cabinet.
 */
export async function cabinetAvailable(): Promise<boolean> {
  const res = await apiGet('/api/customer/me')
  return res.status !== 404
}
