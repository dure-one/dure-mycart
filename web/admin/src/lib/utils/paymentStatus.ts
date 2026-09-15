/**
 * How the panel colours a payment.
 *
 * The statuses are the ones the payment providers report (see pkg/litepay): a
 * payment is created, is awaiting payment, and then succeeds, fails, is
 * cancelled, or is still being processed. Anything else is a status this panel
 * does not know, and is shown without a colour rather than guessed at.
 *
 * The cart list, the cart drawer and the customer drawer all read the same
 * column, so they read it here instead of each spelling the mapping out.
 */
export function paymentVariant(status: string): 'success' | 'warning' | 'danger' | 'neutral' {
  switch (status) {
    case 'paid':
      return 'success'
    case 'canceled':
    case 'failed':
      return 'danger'
    case 'new':
    case 'processed':
    case 'unpaid':
      return 'warning'
    default:
      return 'neutral'
  }
}
