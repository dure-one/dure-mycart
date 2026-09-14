/**
 * The translation key of the message to show for a failed cabinet request.
 *
 * A failed sign-in and a taken address arrive as one English sentence each and
 * no error code, so the sentence is the only thing that tells them apart — and
 * it is the server's own answer to the question of which failure happened, not
 * a rule made up here. Every other answer (a validation message, a dropped
 * connection) reads as the generic failure rather than reaching the buyer in
 * English.
 */
const CUSTOMER_ERROR_KEYS: Record<string, string> = {
  'wrong email or password': 'account.wrongCredentials',
  'this email is already registered': 'account.emailTaken'
}

export function customerErrorKey(message: string | undefined): string {
  return (message && CUSTOMER_ERROR_KEYS[message]) || 'account.requestFailed'
}
