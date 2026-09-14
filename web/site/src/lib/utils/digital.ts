/**
 * Utility for reading how a product is delivered.
 */

import type { Digital } from '$lib/types/models'

/**
 * chargesStock reports whether a product can run out.
 *
 * A digital file cannot: the shop stores one file and serves the same bytes to
 * every buyer, so the count on such a product is not stock, and the panel's
 * form leaves it at zero. Reading it as stock answers "quantity unavailable"
 * to every order of the thing the shop sells most.
 *
 * This is the storefront's copy of the server's rule in
 * `internal/queries/cart.go`, and it is a copy on purpose: the two answer the
 * same question at the two moments it is asked — here, what a page may offer
 * to put in the cart; there, what an order may hold. A page that offers what
 * the checkout then refuses is worse than one that offers nothing, and a page
 * that hides what the checkout would accept loses the sale.
 */
export function chargesStock(item: { digital?: Digital }): boolean {
  return item.digital?.type !== 'file'
}
