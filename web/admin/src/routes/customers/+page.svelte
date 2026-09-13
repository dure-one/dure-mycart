<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import Drawer from '$lib/components/Drawer.svelte'
  import CustomerView from '$lib/components/customer/View.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import { Badge, Chip, ChipGroup, FormInput, PageHeader, PageState } from '$lib/components'
  import { accountState, formatDate } from '$lib/utils'
  import { loadData } from '$lib/utils/apiHelpers'
  import { formatCurrencyWithTruncation } from '$lib/utils/currency'
  import { createDelayedReset } from '$lib/utils/delayedReset'
  import { DEFAULT_PAGE_SIZE } from '$lib/constants/pagination'
  import { DRAWER_CLOSE_DELAY_MS, SEARCH_DEBOUNCE_MS } from '$lib/constants/ui'
  import type { CustomerSummary } from '$lib/types/models'
  import { paymentSettingsStore } from '$lib/stores/payment'
  import { translate, locale } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)
  let currentLocale = $derived($locale)
  let paymentSettings = $derived($paymentSettingsStore)

  interface CustomersResponse {
    customers: CustomerSummary[]
    total: number
    page: number
    limit: number
  }

  let customers = $state<CustomerSummary[]>([])
  let loading = $state(true)
  let currentPage = $state(1)
  let limit = $state(DEFAULT_PAGE_SIZE)
  let total = $state(0)

  let search = $state('')
  let registeredOnly = $state(false)

  let drawerOpen = $state(false)
  let drawerCustomer = $state<CustomerSummary | null>(null)

  // Typing filters as it goes, but not on every keystroke: each one would be a
  // query, and the answer to the first would usually arrive after the answer to
  // the last.
  let searchTimer: ReturnType<typeof setTimeout> | null = null

  onMount(async () => {
    await loadCustomers(1)
  })

  onDestroy(() => {
    if (searchTimer !== null) clearTimeout(searchTimer)
  })

  async function loadCustomers(page = currentPage) {
    loading = true
    currentPage = page

    const params = new URLSearchParams({ page: String(page), limit: String(limit) })
    const term = search.trim()
    if (term) params.set('search', term)
    if (registeredOnly) params.set('registered', 'true')

    const result = await loadData<CustomersResponse>(
      `/api/_/customers?${params.toString()}`,
      t('customers.failedToLoad')
    )
    if (result) {
      customers = result.customers || []
      total = result.total || 0

      // The drawer shows a row of this list, so hand it the reloaded one
      // rather than letting it keep a copy that would drift from what the
      // table shows. Deleting the customer removes the row; the drawer closes
      // itself in that case, and keeps the last known row until it does.
      if (drawerCustomer) {
        drawerCustomer = customers.find((row) => row.email === drawerCustomer?.email) ?? drawerCustomer
      }
    }
    loading = false
  }

  function onSearchInput() {
    if (searchTimer !== null) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
      searchTimer = null
      loadCustomers(1)
    }, SEARCH_DEBOUNCE_MS)
  }

  function setRegisteredOnly(value: boolean) {
    if (registeredOnly === value) return
    registeredOnly = value
    loadCustomers(1)
  }

  function openCustomer(customer: CustomerSummary) {
    drawerCustomer = customer
    drawerOpen = true
  }

  const resetDrawer = createDelayedReset(DRAWER_CLOSE_DELAY_MS)

  function closeDrawer() {
    if (drawerOpen) {
      drawerOpen = false
      resetDrawer(() => {
        drawerCustomer = null
      })
    }
  }
</script>

<Main>
  <PageHeader title={t('customers.title')} />

  {#if loading}
    <PageState kind="loading" />
  {:else}
    <div class="mb-5 flex flex-wrap items-center gap-4">
      <div class="w-full sm:w-72">
        <FormInput
          id="customer-search"
          ico="user-group"
          title={t('customers.searchLabel')}
          bind:value={search}
          oninput={onSearchInput}
        />
      </div>
      <ChipGroup>
        <Chip active={!registeredOnly} onclick={() => setRegisteredOnly(false)}>
          {t('customers.all')}
        </Chip>
        <Chip active={registeredOnly} onclick={() => setRegisteredOnly(true)}>
          {t('customers.registeredOnly')}
        </Chip>
      </ChipGroup>
    </div>

    {#if customers.length === 0}
      <PageState kind="empty" message={t('customers.noCustomers')} />
    {:else}
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{t('customers.email')}</th>
              <th>{t('customers.name')}</th>
              <!-- One column carries both facts the operator needs about a row,
                   because they are one question in practice: is there an account
                   here, and is it open? "Guest" is the row where neither answer
                   applies. The badge comes from `accountState`, shared with the
                   drawer, so the row and the drawer word it the same way. -->
              <th class="w-40">{t('customers.account')}</th>
              <th class="w-32 text-right">{t('customers.purchases')}</th>
              <th class="w-40 text-right">{t('customers.spent')}</th>
              <th class="w-48">{t('customers.lastOrder')}</th>
            </tr>
          </thead>
          <tbody>
            {#each customers as customer (customer.email)}
              {@const account = accountState(customer)}

              <tr class="cursor-pointer hover:bg-gray-50" onclick={() => openCustomer(customer)}>
                <td>{customer.email}</td>
                <td>{customer.name || '-'}</td>
                <td>
                  <Badge variant={account.variant}>{t(account.labelKey)}</Badge>
                </td>
                <td class="text-right">{customer.purchases}</td>
                <td class="text-right">
                  {formatCurrencyWithTruncation(
                    customer.spent,
                    customer.currency || 'USD',
                    'admin',
                    paymentSettings?.truncation,
                    currentLocale,
                    paymentSettings?.number_format,
                    paymentSettings?.symbol_display?.admin
                  )}
                </td>
                <td>
                  {#if customer.last_order}
                    {formatDate(customer.last_order)}
                  {:else}
                    <span class="text-gray-400">-</span>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      {#if total > 0}
        <Pagination {currentPage} totalPages={Math.ceil(total / limit)} onPageChange={loadCustomers} />
      {/if}
    {/if}
  {/if}
</Main>

{#if drawerOpen}
  <Drawer isOpen={drawerOpen} onclose={closeDrawer} maxWidth="710px">
    {#if drawerCustomer}
      <!-- Keyed on the address so that opening another customer builds the
           drawer again: the password issued for the previous one, and the
           status it held, must not carry over. -->
      {#key drawerCustomer.email}
        <CustomerView customer={drawerCustomer} onclose={closeDrawer} onchanged={() => loadCustomers()} />
      {/key}
    {/if}
  </Drawer>
{/if}
