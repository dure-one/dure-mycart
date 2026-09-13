<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import CabinetUnavailable from '$lib/components/CabinetUnavailable.svelte'
  import { apiGet, apiPost } from '$lib/utils/api'
  import { formatCurrency } from '$lib/utils/currency'
  import { handleNavigation } from '$lib/utils/navigation'
  import { translate, locale } from '$lib/i18n'
  import type { Customer, CustomerPurchase } from '$lib/types/models'

  // Reactive translation function
  let t = $derived($translate)

  let customer = $state<Customer | null>(null)
  let purchases = $state<CustomerPurchase[]>([])
  let loading = $state(true)
  let error = $state('')
  // The shop has no cabinet at all, which is not a failure of this page: every
  // endpoint of it answers 404 there. The page renders nothing but the notice
  // in that case — no order list, no sign-out button, no link onwards.
  let unavailable = $state(false)
  let copied = $state('')
  let signingOut = $state(false)
  let copiedTimer: ReturnType<typeof setTimeout> | undefined

  onMount(async () => {
    const me = await apiGet<Customer>('/api/customer/me')

    // Not signed in is the ordinary way in here: the page has no other way of
    // learning who is asking, so the answer is the sign-in form rather than an
    // error to read.
    if (me.status === 401) {
      goto('/signin')
      return
    }

    if (!me.success || !me.result) {
      loading = false
      unavailable = me.status === 404
      error = unavailable ? '' : me.message || t('account.failed')
      return
    }

    customer = me.result

    const res = await apiGet<{ purchases: CustomerPurchase[] }>('/api/customer/purchases')
    loading = false

    if (!res.success) {
      error = res.message || t('account.failed')
      return
    }

    purchases = res.result?.purchases ?? []
  })

  async function signOut() {
    signingOut = true
    await apiPost('/api/customer/signout')
    goto('/')
  }

  async function copy(code: string) {
    try {
      await navigator.clipboard.writeText(code)
      copied = code
      clearTimeout(copiedTimer)
      copiedTimer = setTimeout(() => (copied = ''), 2000)
    } catch {
      // A browser that refuses the clipboard — an insecure origin, a denied
      // permission — leaves the key on screen to be selected by hand, which is
      // where it already is. Nothing is worth saying about it.
    }
  }

  function formatDate(unixSeconds: number): string {
    return new Date(unixSeconds * 1000).toLocaleDateString($locale, {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    })
  }

  // Leaving the page inside the two seconds would otherwise fire the reset
  // against a component that is gone.
  onDestroy(() => clearTimeout(copiedTimer))
</script>

<div class="min-h-screen bg-white px-4 py-12 sm:px-6 lg:px-8">
  <div class="mx-auto max-w-3xl">
    {#if loading}
      <div class="brutal-card p-12 text-center">
        <p class="text-2xl font-black tracking-wider text-black uppercase">{t('common.loading')}</p>
      </div>
    {:else if unavailable}
      <CabinetUnavailable title={t('header.account')} />
    {:else if error || !customer}
      <div class="brutal-card bg-red-300 p-8 sm:p-12">
        <h1 class="mb-4 text-3xl font-black tracking-tighter text-black uppercase sm:text-4xl">
          {t('error.errorTitle')}
        </h1>
        <p class="text-lg tracking-wide text-black">{error || t('account.failed')}</p>
      </div>
    {:else}
      <div class="brutal-card mb-8 bg-yellow-300 p-8">
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 class="text-3xl font-black tracking-tighter text-black uppercase sm:text-4xl">
              {t('header.account')}
            </h1>
            <p class="mt-2 text-base font-black tracking-wide text-black uppercase">
              {t('account.signedInAs')} <span class="font-normal normal-case">{customer.email}</span>
            </p>
          </div>
          <button
            type="button"
            onclick={signOut}
            disabled={signingOut}
            class="cursor-pointer border-4 border-black bg-white px-6 py-3 text-sm font-black tracking-wider text-black uppercase transition-all duration-200 enabled:hover:-translate-x-1 enabled:hover:-translate-y-1 enabled:hover:shadow-[12px_12px_0px_0px_rgba(0,0,0,1)] disabled:cursor-not-allowed disabled:opacity-50"
          >
            {t('account.signOut')}
          </button>
        </div>
      </div>

      {#if purchases.length === 0}
        <div class="brutal-card p-8 text-center sm:p-12">
          <h2 class="mb-4 text-2xl font-black tracking-tighter text-black uppercase">
            {t('account.emptyTitle')}
          </h2>
          <p class="mb-8 text-lg tracking-wide text-black">{t('account.emptyMessage')}</p>
          <a href="/" onclick={(e) => handleNavigation(e, '/')} class="brutal-btn inline-block bg-yellow-300 px-8 py-3">
            {t('account.goToShop')}
          </a>
        </div>
      {:else}
        <ul class="space-y-8">
          {#each purchases as purchase (purchase.id)}
            <li class="brutal-card p-6 sm:p-8">
              <div class="mb-6 flex flex-wrap items-center justify-between gap-4 border-b-4 border-black pb-4">
                <div>
                  <h2 class="text-xl font-black tracking-tighter text-black uppercase">
                    {t('account.order')}
                    {purchase.id}
                  </h2>
                  <p class="mt-1 text-base tracking-wide text-gray-700">{formatDate(purchase.created)}</p>
                </div>
                <p class="text-xl font-black text-black">
                  {formatCurrency(purchase.amount_total / 100, purchase.currency)}
                </p>
              </div>

              <ul class="space-y-4">
                <!--
                  Not keyed by product: one product can fill two lines of the
                  same order, one per variant, and a keyed block refuses the
                  repeat. The lines are loaded once and never reordered, so
                  position is identity enough.
                -->
                {#each purchase.items ?? [] as item}
                  <li class="border-4 border-black bg-white p-4 sm:p-6">
                    <div class="flex flex-wrap items-baseline justify-between gap-2">
                      <a
                        href={`/products/${item.slug}`}
                        class="cursor-pointer text-lg font-black tracking-tight text-black uppercase decoration-yellow-300 decoration-4 underline-offset-4 hover:underline"
                      >
                        {item.name}
                      </a>
                      <span class="text-sm font-black tracking-wider text-gray-700 uppercase">
                        {t('account.quantity')}
                        {item.quantity}
                      </span>
                    </div>

                    {#if item.files && item.files.length > 0}
                      <ul class="mt-4 space-y-3">
                        {#each item.files as file (file.id)}
                          <li class="flex flex-wrap items-center justify-between gap-3 border-t-2 border-black/20 pt-3">
                            <span class="text-base tracking-wide break-all text-black">{file.orig_name}</span>
                            <!--
                              data-sveltekit-reload keeps SvelteKit's router out of
                              this link: the href is not a page of the app, and the
                              browser has to handle the response itself for the
                              attachment to be saved instead of navigated to.
                            -->
                            <a
                              href={`/api/customer/purchases/${file.id}/download`}
                              data-sveltekit-reload
                              class="brutal-btn bg-green-500 px-6 py-2 text-sm text-white"
                            >
                              {t('account.download')}
                            </a>
                          </li>
                        {/each}
                      </ul>
                    {/if}

                    {#if item.codes && item.codes.length > 0}
                      <div class="mt-4 border-t-2 border-black/20 pt-4">
                        <h3 class="mb-3 text-sm font-black tracking-wider text-black uppercase">
                          {t('account.keys')}
                        </h3>
                        <ul class="space-y-2">
                          <!-- Not keyed by the value: two rows can hold the
                               same string, and a keyed block refuses the
                               repeat. -->
                          {#each item.codes as code}
                            <li
                              class="flex flex-wrap items-center justify-between gap-3 border-4 border-black bg-yellow-100 p-3"
                            >
                              <code class="text-base break-all text-black">{code}</code>
                              <button
                                type="button"
                                onclick={() => copy(code)}
                                class="cursor-pointer border-4 border-black bg-white px-4 py-1 text-xs font-black tracking-wider text-black uppercase hover:bg-yellow-300"
                              >
                                {copied === code ? t('account.copied') : t('account.copy')}
                              </button>
                            </li>
                          {/each}
                        </ul>
                      </div>
                    {/if}
                  </li>
                {/each}
              </ul>
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  </div>
</div>
