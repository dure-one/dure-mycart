<script lang="ts">
  import { onMount } from 'svelte'
  import { DetailList, DrawerFooter, DrawerHeader, IconButton, PageState } from '$lib/components'
  import { formatDate } from '$lib/utils'
  import { formatCurrencyWithTruncation } from '$lib/utils/currency'
  import { loadData } from '$lib/utils/apiHelpers'
  import type { Product } from '$lib/types/models'
  import { translate, locale } from '$lib/i18n'
  import { paymentSettingsStore } from '$lib/stores/payment'
  import { sanitizeHTML } from '$lib/utils/sanitize'

  // Reactive translation function
  let t = $derived($translate)
  let currentLocale = $derived($locale)
  let paymentSettings = $derived($paymentSettingsStore)

  interface DrawerProduct {
    product: Product
    index: number
    currency?: string
  }

  interface Props {
    drawer: DrawerProduct
    updateActive?: ((index: number) => void) | undefined
    onclose?: () => void
  }

  let { drawer, updateActive, onclose }: Props = $props()

  let product = $state<Product | null>(null)
  let loading = $state(true)
  let lastProductId = $state<string | null>(null)

  async function loadProduct() {
    if (!drawer?.product?.id) return

    loading = true
    const result = await loadData<Product>(`/api/_/products/${drawer.product.id}`, t('products.failedToLoadProduct'))
    if (result) {
      product = result
      lastProductId = drawer.product.id
    }
    loading = false
  }

  onMount(async () => {
    await loadProduct()
  })

  // Reload product when drawer.product.id changes
  $effect(() => {
    if (drawer?.product?.id && drawer.product.id !== lastProductId) {
      loadProduct()
    }
  })

  function close() {
    onclose?.()
  }

  async function active() {
    if (updateActive && product) {
      await updateActive(drawer.index)
      // Update local product state reactively
      if (product) {
        product = { ...product, active: !product.active }
      }
    }
  }
</script>

<div>
  <DrawerHeader title={`${t('products.viewProduct')} ${product?.name || ''}`}>
    {#snippet actions()}
      {#if product}
        <IconButton ico={product.active ? 'eye' : 'eye-slash'} label={t('products.active')} onclick={active} />
      {/if}
    {/snippet}
  </DrawerHeader>

  {#if loading}
    <PageState kind="loading" />
  {:else if product}
    <div class="flow-root">
      <dl class="-my-3 mt-2 divide-y divide-gray-100 text-sm">
        <DetailList name={t('products.id')}>{product.id}</DetailList>
        <DetailList name={t('products.name')}>{product.name}</DetailList>
        <DetailList name={t('products.price')}>
          {#if !product.amount || parseFloat(String(product.amount)) === 0}
            <span class="font-bold text-green-600">{t('carts.free')}</span>
          {:else}
            {formatCurrencyWithTruncation(
              product.amount,
              drawer.currency || 'USD',
              'admin',
              paymentSettings?.truncation,
              currentLocale,
              paymentSettings?.number_format,
              paymentSettings?.symbol_display?.admin
            )}
          {/if}
        </DetailList>
        <DetailList name={t('products.slug')}>{product.slug}</DetailList>
        <DetailList name={t('products.metadata')}>
          {#each product.metadata || [] as data (data.key)}
            <div>{data.key}: {data.value}</div>
          {/each}
        </DetailList>
        <DetailList name={t('products.attributes')}>
          {#each product.attributes || [] as item (item)}
            <div>{item}</div>
          {/each}
        </DetailList>
        {#if product.has_variants && product.variants && product.variants.length > 0}
          <DetailList name={t('products.productVariants')}>
            <!-- Options summary -->
            <div class="mb-4 rounded-lg bg-gray-50 p-3">
              <h4 class="mb-2 text-sm font-medium text-gray-700">{t('products.options')}:</h4>
              {#each product.options || [] as option}
                <div class="mb-1 text-sm text-gray-600">
                  <strong class="text-gray-900">{option.name}:</strong> {option.values.map(v => v.value).join(', ')}
                </div>
              {/each}
            </div>

            <!-- Variants table (read-only) -->
            <div class="mb-2 text-sm font-medium text-gray-700">
              {t('products.generatedVariants')} ({product.variants.length})
            </div>
            <div class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-200 text-sm">
                <thead class="bg-gray-50">
                  <tr>
                    <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      {t('products.variant')}
                    </th>
                    <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      {t('products.sku')}
                    </th>
                    <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      {t('products.priceSurcharge')}
                    </th>
                    <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      {t('products.totalPrice')}
                    </th>
                    <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      {t('products.quantity')}
                    </th>
                    <th class="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                      {t('products.active')}
                    </th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 bg-white">
                  {#each product.variants as variant}
                    <tr class:opacity-50={!variant.active}>
                      <td class="whitespace-nowrap px-3 py-2 text-gray-900">
                        {Object.entries(variant.option_values).map(([key, value]) => `${key}: ${value}`).join(', ')}
                      </td>
                      <td class="whitespace-nowrap px-3 py-2 text-gray-700">
                        {variant.sku || '-'}
                      </td>
                      <td class="whitespace-nowrap px-3 py-2 text-gray-700">
                        {#if variant.price_surcharge === 0}
                          <span class="text-gray-400">-</span>
                        {:else}
                          {formatCurrencyWithTruncation(
                            variant.price_surcharge,
                            drawer.currency || 'USD',
                            'admin',
                            paymentSettings?.truncation,
                            currentLocale,
                            paymentSettings?.number_format,
                            paymentSettings?.symbol_display?.admin
                          )}
                        {/if}
                      </td>
                      <td class="whitespace-nowrap px-3 py-2 text-gray-700">
                        {#if (product.amount || 0) + variant.price_surcharge === 0}
                          <span class="font-bold text-green-600">{t('carts.free')}</span>
                        {:else}
                          {formatCurrencyWithTruncation(
                            (product.amount || 0) + variant.price_surcharge,
                            drawer.currency || 'USD',
                            'admin',
                            paymentSettings?.truncation,
                            currentLocale,
                            paymentSettings?.number_format,
                            paymentSettings?.symbol_display?.admin
                          )}
                        {/if}
                      </td>
                      <td class="whitespace-nowrap px-3 py-2 text-gray-700">
                        {variant.quantity}
                      </td>
                      <td class="whitespace-nowrap px-3 py-2">
                        <SvgIcon
                          name={variant.active ? 'eye' : 'eye-slash'}
                          className="h-4 w-4 {variant.active ? 'text-green-600' : 'text-gray-400'}"
                          stroke="currentColor"
                        />
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </DetailList>
        {/if}
        <DetailList name={t('common.created')}>{formatDate(product.created)}</DetailList>
        {#if product.updated}
          <DetailList name={t('common.updated')}>{formatDate(product.updated)}</DetailList>
        {/if}
        {#if product.images}
          <DetailList name={t('products.images')} grid={true}>
            {#each product.images as item (item.id)}
              <div>
                <a href="/uploads/{item.name}.{item.ext}" target="_blank" aria-label={t('carts.viewFullSizeImage')}>
                  <img
                    style="width: 100%; max-width: 150px"
                    src="/uploads/{item.name}_sm.{item.ext}"
                    alt="{product.name} - {item.name}"
                    loading="lazy"
                  />
                </a>
              </div>
            {/each}
          </DetailList>
        {/if}
        <DetailList name={t('products.briefShortDescription')}>{product.brief}</DetailList>

        <div class="tiptap pt-3">{@html sanitizeHTML(product.description)}</div>
      </dl>
    </div>
  {:else}
    <PageState kind="error" message={t('products.failedToLoadProduct')} />
  {/if}

  <DrawerFooter onclose={close} />
</div>
