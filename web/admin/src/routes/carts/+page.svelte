<script lang="ts">
  import { onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import Drawer from '$lib/components/Drawer.svelte'
  import CartView from '$lib/components/cart/View.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import { PageHeader, PageState, Badge, IconButton } from '$lib/components'
  import { loadData, handleApiCall } from '$lib/utils/apiHelpers'
  import { apiPost } from '$lib/utils'
  import { formatCurrency, formatCurrencyWithTruncation } from '$lib/utils/currency'
  import { costFormat, formatDate } from '$lib/utils'
  import { STRIPE_DASHBOARD_URL } from '$lib/utils/constants'
  import type { Cart } from '$lib/types/models'
  import { paymentSettingsStore } from '$lib/stores/payment'
  import { translate, locale } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)
  let currentLocale = $derived($locale)
  let paymentSettings = $derived($paymentSettingsStore)

  interface DrawerCart {
    cart: Cart
  }

  interface CartsResponse {
    carts: Cart[]
    total: number
    page: number
    limit: number
  }

  import { DEFAULT_PAGE_SIZE } from '$lib/constants/pagination'
  import { DRAWER_CLOSE_DELAY_MS } from '$lib/constants/ui'
  import { createDelayedReset } from '$lib/utils/delayedReset'

  let carts = $state<Cart[]>([])
  let loading = $state(true)
  let drawerOpen = $state(false)
  let drawerCart = $state<DrawerCart | null>(null)
  let currentPage = $state(1)
  let limit = $state(DEFAULT_PAGE_SIZE)
  let total = $state(0)

  onMount(async () => {
    await loadCarts()
  })

  async function loadCarts(page = currentPage) {
    loading = true
    currentPage = page
    const result = await loadData<CartsResponse>(
      `/api/_/carts?page=${page}&limit=${limit}`,
      t('carts.failedToLoad')
    )
    if (result) {
      carts = result.carts || []
      total = result.total || 0
    }
    loading = false
  }

  function handlePageChange(page: number) {
    loadCarts(page)
  }

  function openView(cart: Cart) {
    drawerCart = { cart }
    drawerOpen = true
  }

  const resetDrawer = createDelayedReset(DRAWER_CLOSE_DELAY_MS)

  function closeDrawer() {
    if (drawerOpen) {
      drawerOpen = false
      resetDrawer(() => {
        drawerCart = null
      })
    }
  }

  async function sendMail(cartId: string, event: Event) {
    event.stopPropagation()
    await handleApiCall(
      () => apiPost(`/api/_/carts/${cartId}/mail`, {}),
      t('carts.mailSentSuccessfully'),
      t('carts.failedToSendMailMessage')
    )
  }
</script>

<Main>
  <PageHeader title={t('carts.title')} />

  {#if loading}
    <PageState kind="loading" />
  {:else if carts.length === 0}
    <PageState kind="empty" message={t('carts.noCarts')} />
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{t('carts.email')}</th>
            <th>{t('carts.priceColumn')}</th>
            <th>{t('carts.statusColumn')}</th>
            <th>{t('carts.paymentColumn')}</th>
            <th class="w-48">{t('common.created')}</th>
            <th class="w-48">{t('common.updated')}</th>
            <th class="w-12"></th>
          </tr>
        </thead>
        <tbody>
          {#each carts as cart, index (cart.id)}
            <tr
              class:bg-green-50={cart.payment_status === 'paid'}
              class="cursor-pointer hover:bg-gray-50"
              onclick={() => openView(cart)}
            >
              <td>{cart.email || '-'}</td>
              <td>
                {formatCurrencyWithTruncation(
                  cart.amount_total,
                  cart.currency || 'USD',
                  'admin',
                  paymentSettings?.truncation,
                  currentLocale,
                  paymentSettings?.number_format,
                  paymentSettings?.symbol_display?.admin
                )}
              </td>
              <td>
                <Badge
                  variant={cart.payment_status === 'paid'
                    ? 'success'
                    : cart.payment_status === 'pending'
                      ? 'warning'
                      : cart.payment_status === 'failed'
                        ? 'danger'
                        : 'neutral'}
                >
                  {cart.payment_status || '-'}
                </Badge>
              </td>
              <td>{cart.payment_system || '-'}</td>
              <td>{formatDate(cart.created)}</td>
              <td>
                {#if cart.updated}
                  {formatDate(cart.updated)}
                {/if}
              </td>
              <td onclick={(e) => e.stopPropagation()}>
                <IconButton
                  ico="envelope"
                  label={t('carts.sendMail')}
                  disabled={cart.payment_status !== 'paid'}
                  onclick={(e) => sendMail(cart.id, e)}
                />
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    {#if total > 0}
      <Pagination
        currentPage={currentPage}
        totalPages={Math.ceil(total / limit)}
        onPageChange={handlePageChange}
      />
    {/if}
  {/if}
</Main>

{#if drawerOpen}
  <Drawer isOpen={drawerOpen} onclose={closeDrawer} maxWidth="710px">
    {#if drawerCart}
      <CartView drawer={drawerCart} onclose={closeDrawer} />
    {/if}
  </Drawer>
{/if}
