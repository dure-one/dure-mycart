/**
 * How the panel describes a customer's account.
 */

import type { CustomerSummary } from '$lib/types/models'

/**
 * The account state of a customer, as a badge the caller translates.
 *
 * The list and the drawer answer one question — is there an account here, and
 * is it open — and they are written from one place so they answer it in one
 * vocabulary. A buyer who checked out without an account is a guest, and
 * "blocked" describes only an account that exists; splitting the answer into
 * "Registration" and "Status" gave the drawer two rows and two words for the
 * column the list calls "Account".
 */
export function accountState(customer: CustomerSummary): {
  labelKey: string
  variant: 'success' | 'danger' | 'neutral'
} {
  if (!customer.registered) {
    return { labelKey: 'customers.guest', variant: 'neutral' }
  }
  return customer.active
    ? { labelKey: 'customers.activeYes', variant: 'success' }
    : { labelKey: 'customers.activeNo', variant: 'danger' }
}
