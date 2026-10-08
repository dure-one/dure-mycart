<script lang="ts">
  import { onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import { FormButton, FormInput, FormToggle } from '$lib/components'
  import { loadSettings, saveSettings } from '$lib/utils/settingsHelpers'
  import { validators, validateFields } from '$lib/utils/validation'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  interface SellerInfoSettings {
    enabled: boolean
    business_name: string
    representative: string
    customer_service: string
    business_reg_number: string
    business_address: string
    ecommerce_license: string
    email: string
  }

  let formData = $state<SellerInfoSettings>({
    enabled: false,
    business_name: '',
    representative: '',
    customer_service: '',
    business_reg_number: '',
    business_address: '',
    ecommerce_license: '',
    email: ''
  })
  let formErrors = $state<Record<string, string>>({})
  let loading = $state(true)

  onMount(async () => {
    const loaded = await loadSettings<SellerInfoSettings>('sellerinfo', formData)
    if (loaded) {
      formData = loaded
    }
    loading = false
  })

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    formErrors = validateFields(formData, [
      { field: 'email', ...validators.email(t('settings.validEmailRequired')) },
      { field: 'business_name', ...validators.maxLength(100, t('settings.businessNameTooLong')) },
      { field: 'representative', ...validators.maxLength(50, t('settings.representativeTooLong')) },
      { field: 'customer_service', ...validators.maxLength(50, t('settings.customerServiceTooLong')) },
      { field: 'business_reg_number', ...validators.maxLength(50, t('settings.businessRegNumberTooLong')) },
      { field: 'business_address', ...validators.maxLength(200, t('settings.businessAddressTooLong')) },
      { field: 'ecommerce_license', ...validators.maxLength(50, t('settings.ecommerceLicenseTooLong')) }
    ])

    if (Object.keys(formErrors).length > 0) {
      return
    }

    await saveSettings('sellerinfo', formData)
  }
</script>

<Main>
  <h1 class="mb-5">{t('settings.sellerInfoSettings')}</h1>

  {#if loading}
    <div class="py-8 text-center">{t('common.loading')}</div>
  {:else}
    <form onsubmit={handleSubmit} class="max-w-2xl space-y-4">
      <FormToggle
        id="sellerinfo-enabled"
        label={t('settings.enableSellerInfo')}
        bind:value={formData.enabled}
      />

      <FormInput
        id="business_name"
        title={t('settings.businessName')}
        bind:value={formData.business_name}
        error={formErrors.business_name}
        ico="building-storefront"
      />

      <FormInput
        id="representative"
        title={t('settings.representative')}
        bind:value={formData.representative}
        error={formErrors.representative}
        ico="user"
      />

      <FormInput
        id="customer_service"
        title={t('settings.customerService')}
        bind:value={formData.customer_service}
        error={formErrors.customer_service}
        ico="phone"
      />

      <FormInput
        id="business_reg_number"
        title={t('settings.businessRegNumber')}
        bind:value={formData.business_reg_number}
        error={formErrors.business_reg_number}
        ico="identification"
      />

      <FormInput
        id="business_address"
        title={t('settings.businessAddress')}
        bind:value={formData.business_address}
        error={formErrors.business_address}
        ico="map-pin"
      />

      <FormInput
        id="ecommerce_license"
        title={t('settings.ecommerceLicense')}
        bind:value={formData.ecommerce_license}
        error={formErrors.ecommerce_license}
        ico="document-text"
      />

      <FormInput
        id="email"
        type="email"
        title={t('settings.email')}
        bind:value={formData.email}
        error={formErrors.email}
        ico="at-symbol"
      />

      <div class="pt-4">
        <FormButton type="submit" name={t('common.save')} variant="primary" />
      </div>
    </form>
  {/if}
</Main>
