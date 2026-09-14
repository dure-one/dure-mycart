<script lang="ts">
  import { onMount } from 'svelte'
  import { Badge, DetailList, DrawerFooter, DrawerHeader, FormButton, PageState } from '$lib/components'
  import { accountState, apiUpdate, formatDate, confirmAction, paymentVariant, showMessage } from '$lib/utils'
  import { deleteData, handleApiCall, loadData } from '$lib/utils/apiHelpers'
  import { formatCurrencyWithTruncation } from '$lib/utils/currency'
  import type { Cart, CustomerSummary } from '$lib/types/models'
  import { paymentSettingsStore } from '$lib/stores/payment'
  import { translate, locale } from '$lib/i18n'

  interface Props {
    customer: CustomerSummary
    onclose?: () => void
    /** Called after any change, so the list behind the drawer can reload. */
    onchanged?: () => void
  }

  let { customer, onclose, onchanged }: Props = $props()

  // Reactive translation function
  let t = $derived($translate)
  let currentLocale = $derived($locale)
  let paymentSettings = $derived($paymentSettingsStore)

  let carts = $state<Cart[]>([])
  let loading = $state(true)
  let busy = $state(false)

  // The row the list handed down. It is reloaded after every change, so the
  // drawer shows what the server says rather than a copy that could disagree
  // with the table the operator is looking at.
  //
  // The badge below reads the account state from the same helper the list's
  // column does, so the row and the drawer cannot word it differently.
  let account = $derived(accountState(customer))

  // A password is issued exactly once and never written down anywhere, so it
  // lives here until the drawer closes — which is the whole point: this is the
  // one moment the operator can read it.
  let issuedPassword = $state('')

  onMount(async () => {
    const result = await loadData<{ carts: Cart[] }>(
      `/api/_/customers/carts?email=${encodeURIComponent(customer.email)}`,
      t('customers.failedToLoadCarts')
    )
    carts = result?.carts || []
    loading = false
  })

  function money(amount: number, currency: string): string {
    return formatCurrencyWithTruncation(
      amount,
      currency || 'USD',
      'admin',
      paymentSettings?.truncation,
      currentLocale,
      paymentSettings?.number_format,
      paymentSettings?.symbol_display?.admin
    )
  }

  async function toggleActive() {
    busy = true
    const result = await handleApiCall(
      () => apiUpdate<CustomerSummary>(`/api/_/customers/${customer.id}/active`, {}),
      customer.active ? t('customers.blockedMessage') : t('customers.unblockedMessage'),
      t('customers.blockFailed')
    )
    busy = false

    if (result) {
      onchanged?.()
    }
  }

  async function resetPassword() {
    if (!confirmAction(t('customers.resetPasswordConfirm'))) return

    busy = true
    const result = await handleApiCall(
      () => apiUpdate<{ password: string }>(`/api/_/customers/${customer.id}/password`, {}),
      t('customers.passwordIssued'),
      t('customers.resetPasswordFailed')
    )
    busy = false

    if (result) {
      issuedPassword = result.password
      onchanged?.()
    }
  }

  async function deleteAccount() {
    if (!confirmAction(t('customers.deleteConfirm'))) return

    busy = true
    // The outcome comes from the response's success flag rather than from a
    // payload, because this endpoint deletes and returns nothing: read as a
    // result, a completed delete is indistinguishable from a failed one, and
    // the drawer would stay open over the row it just removed.
    const deleted = await deleteData(
      `/api/_/customers/${customer.id}`,
      t('customers.deletedMessage'),
      t('customers.deleteFailed')
    )
    busy = false

    if (deleted) {
      onchanged?.()
      onclose?.()
    }
  }

  async function copyPassword() {
    try {
      await navigator.clipboard.writeText(issuedPassword)
      showMessage(t('customers.copied'))
    } catch {
      // The clipboard is unavailable (an insecure origin, or a browser that
      // refuses the call). The password is on screen and can be selected by
      // hand, which is the fallback rather than an error worth interrupting for.
      showMessage(t('customers.copyFailed'), 'connextWarning')
    }
  }
</script>

<div>
  <DrawerHeader title={t('customers.accountDetails')}>
    {#snippet actions()}
      {#if customer.registered}
        <FormButton
          variant={customer.active ? 'danger' : 'secondary'}
          size="sm"
          name={customer.active ? t('customers.block') : t('customers.unblock')}
          disabled={busy}
          onclick={toggleActive}
        />
        <FormButton
          variant="secondary"
          size="sm"
          name={t('customers.resetPassword')}
          disabled={busy}
          onclick={resetPassword}
        />
      {/if}
    {/snippet}
  </DrawerHeader>

  <div class="flow-root">
    <dl class="-my-3 mt-2 divide-y divide-gray-100 text-sm">
      <DetailList name={t('customers.email')}>
        <a href="mailto:{customer.email}" class="a-link">{customer.email}</a>
      </DetailList>

      {#if customer.name}
        <DetailList name={t('customers.name')}>{customer.name}</DetailList>
      {/if}

      <DetailList name={t('customers.account')}>
        <Badge variant={account.variant}>{t(account.labelKey)}</Badge>
      </DetailList>

      {#if customer.registered && customer.created}
        <DetailList name={t('customers.accountCreated')}>{formatDate(customer.created)}</DetailList>
      {/if}

      <DetailList name={t('customers.purchases')}>{customer.purchases}</DetailList>

      <DetailList name={t('customers.spent')}>
        {money(customer.spent, customer.currency)}
      </DetailList>

      {#if customer.last_order}
        <DetailList name={t('customers.lastOrder')}>{formatDate(customer.last_order)}</DetailList>
      {/if}

      <DetailList name={t('customers.ordersSection')} fullWidth={true}>
        {#if loading}
          <PageState kind="loading" />
        {:else if carts.length === 0}
          <PageState kind="empty" message={t('customers.noOrders')} />
        {:else}
          <div class="table-wrap">
            <table class="table-plain">
              <thead>
                <tr>
                  <th>{t('customers.date')}</th>
                  <th>{t('customers.amount')}</th>
                  <th>{t('customers.status')}</th>
                  <th>{t('customers.payment')}</th>
                </tr>
              </thead>
              <tbody>
                {#each carts as cart (cart.id)}
                  <tr>
                    <td>{formatDate(cart.created)}</td>
                    <td>{money(cart.amount_total, cart.currency)}</td>
                    <td>
                      <Badge variant={paymentVariant(cart.payment_status)}>
                        {cart.payment_status || '-'}
                      </Badge>
                    </td>
                    <td>{cart.payment_system || '-'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </DetailList>
    </dl>
  </div>

  {#if !customer.registered}
    <div class="notice notice-info mt-5">
      {t('customers.guestHint')}
    </div>
  {/if}

  {#if issuedPassword}
    <div class="notice notice-warning mt-5">
      <p class="text-sm font-medium">{t('customers.passwordIssued')}</p>
      <div class="mt-2 flex items-center gap-3">
        <code class="grow rounded bg-white px-3 py-2 font-mono text-sm">{issuedPassword}</code>
        <FormButton variant="secondary" size="sm" name={t('customers.copy')} onclick={copyPassword} />
      </div>
      <p class="mt-2 text-xs">{t('customers.passwordIssuedHint')}</p>
    </div>
  {/if}

  <DrawerFooter
    {onclose}
    ondelete={customer.registered ? deleteAccount : undefined}
    deleteLabel={t('customers.deleteAccount')}
  />
</div>
