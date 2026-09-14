<script lang="ts">
  import { FormInput, IconButton, PageState } from '$lib/components'
  import type { ProductVariant, ProductOption } from '$lib/types/models'
  import { translate, locale } from '$lib/i18n'
  import { formatCurrencyWithTruncation } from '$lib/utils/currency'
  import { paymentSettingsStore } from '$lib/stores/payment'

  let t = $derived($translate)
  let currentLocale = $derived($locale)
  let paymentSettings = $derived($paymentSettingsStore)

  interface Props {
    variants: ProductVariant[]
    options: ProductOption[]
    basePrice: number
    currency: string
    onUpdate: (variants: ProductVariant[]) => void
    disabled?: boolean
  }

  let { variants, options, basePrice, currency, onUpdate, disabled = false }: Props = $props()

  let localVariants = $state<ProductVariant[]>([...variants])
  let optionsSignature = $state('')

  // Create signature from options (names + all values)
  function createOptionsSignature(opts: ProductOption[]): string {
    return JSON.stringify(
      opts.map(o => ({
        name: o.name,
        values: o.values.map(v => v.value)
      }))
    )
  }

  // Sync when variant count OR options change
  $effect(() => {
    const currentSignature = createOptionsSignature(options)
    const countChanged = variants.length !== localVariants.length
    const optionsChanged = currentSignature !== optionsSignature

    if (countChanged || optionsChanged) {
      optionsSignature = currentSignature
      localVariants = [...variants]
    }
  })

  function updateVariant(index: number, field: keyof ProductVariant, value: any) {
    localVariants = localVariants.map((v, i) => i === index ? { ...v, [field]: value } : v)
    onUpdate(localVariants)
  }

  function updateVariantSKU(index: number, event: Event) {
    const target = event.target as HTMLInputElement
    updateVariant(index, 'sku', target.value)
  }

  function updateVariantQuantity(index: number, event: Event) {
    const target = event.target as HTMLInputElement
    const qty = parseInt(target.value) || 0
    updateVariant(index, 'quantity', qty)
  }

  function updateVariantPrice(index: number, event: Event) {
    const target = event.target as HTMLInputElement
    const price = parseInt(target.value) || 0
    updateVariant(index, 'price_surcharge', price)
  }

  function toggleVariantActive(index: number) {
    updateVariant(index, 'active', !localVariants[index].active)
  }

  function getVariantLabel(variant: ProductVariant): string {
    return Object.entries(variant.option_values)
      .map(([key, value]) => `${key}: ${value}`)
      .join(', ')
  }

  function getTotalPrice(variant: ProductVariant): number {
    return basePrice + variant.price_surcharge
  }
</script>

<div class="table-wrap">
  <table class="table-plain">
    <thead>
      <tr>
        <th>{t('products.variant')}</th>
        <th>{t('products.sku')}</th>
        <th>{t('products.priceSurcharge')}</th>
        <th>{t('products.totalPrice')}</th>
        <th>{t('products.quantity')}</th>
        <th>{t('products.active')}</th>
      </tr>
    </thead>
    <tbody>
      {#each localVariants as variant, index (index)}
        <tr class:bg-gray-50={!variant.active}>
          <td class="whitespace-nowrap">
            {getVariantLabel(variant)}
          </td>
          <td class="whitespace-nowrap">
            <FormInput
              id="variant-{index}-sku"
              label={t('products.sku')}
              type="text"
              value={variant.sku || ''}
              oninput={(e) => updateVariantSKU(index, e)}
              placeholder={t('products.sku')}
              {disabled}
              compact
            />
          </td>
          <td class="whitespace-nowrap">
            <FormInput
              id="variant-{index}-price"
              label={t('products.priceSurcharge')}
              type="number"
              value={String(variant.price_surcharge)}
              oninput={(e) => updateVariantPrice(index, e)}
              placeholder="0"
              {disabled}
              compact
            />
          </td>
          <td class="whitespace-nowrap text-gray-700">
            {#if getTotalPrice(variant) === 0}
              <span class="font-bold text-green-600">{t('carts.free')}</span>
            {:else}
              {formatCurrencyWithTruncation(
                getTotalPrice(variant),
                currency || 'USD',
                'admin',
                paymentSettings?.truncation,
                currentLocale,
                paymentSettings?.number_format,
                paymentSettings?.symbol_display?.admin
              )}
            {/if}
          </td>
          <td class="whitespace-nowrap">
            <FormInput
              id="variant-{index}-quantity"
              label={t('products.quantity')}
              type="number"
              value={String(variant.quantity)}
              oninput={(e) => updateVariantQuantity(index, e)}
              placeholder="0"
              {disabled}
              min="0"
              compact
            />
          </td>
          <td class="whitespace-nowrap">
            <IconButton
              ico={variant.active ? 'eye' : 'eye-slash'}
              label={t('products.toggleActive')}
              svgClass="h-5 w-5 {variant.active ? 'text-green-600' : 'text-gray-400'}"
              onclick={() => toggleVariantActive(index)}
              {disabled}
            />
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

{#if localVariants.length === 0}
  <PageState kind="empty" message={t('products.noVariants')} />
{/if}

{#if localVariants.length > 100}
  <div class="mt-2 text-sm text-red-600">
    {t('products.tooManyVariants')}
  </div>
{/if}
