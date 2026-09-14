<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import FormInput from '../form/Input.svelte'
  import FormToggle from '../form/Toggle.svelte'
  import { loadPaymentSettings, savePaymentSettings, togglePaymentActive } from '$lib/composables/usePaymentSettings'
  import { systemStore } from '$lib/stores/system'
  import { MIN_COINBASE_API_KEY_LENGTH, ERROR_MESSAGES } from '$lib/constants/validation'
  import type { CoinbaseSettings } from '$lib/types/models'
  import { translate } from '$lib/i18n'
  import { DrawerFooter, DrawerHeader } from '$lib/components'

  // Reactive translation function
  let t = $derived($translate)

  interface Props {
    onclose?: () => void
  }

  let { onclose }: Props = $props()

  let settings = $state<CoinbaseSettings>({
    active: false,
    api_key: ''
  })
  let formErrors = $state<Record<string, string>>({})
  let unsubscribe: (() => void) | null = null

  onMount(async () => {
    settings = await loadPaymentSettings<CoinbaseSettings>('coinbase', settings)

    unsubscribe = systemStore.subscribe((store) => {
      if (store.payments?.coinbase !== undefined) {
        settings.active = store.payments.coinbase
      }
    })
  })

  onDestroy(() => {
    unsubscribe?.()
  })

  async function handleSubmit() {
    formErrors = {}

    if (!settings.api_key || settings.api_key.length < MIN_COINBASE_API_KEY_LENGTH) {
      formErrors.api_key = ERROR_MESSAGES.COINBASE_API_KEY_TOO_SHORT
      return
    }

    await savePaymentSettings('coinbase', settings)
  }

  async function handleToggleActive() {
    const previousValue = settings.active
    const success = await togglePaymentActive('coinbase', settings.active)

    if (!success) {
      settings.active = previousValue
    }
  }

  function close() {
    onclose?.()
  }
</script>

<div>
  <DrawerHeader title="Coinbase Commerce">
    {#snippet actions()}
      <FormToggle
        id="coinbase-active"
        bind:value={settings.active}
        disabled={Object.keys(formErrors).length > 0}
        onchange={handleToggleActive}
      />
    {/snippet}
  </DrawerHeader>

  <form onsubmit={(e) => { e.preventDefault(); handleSubmit() }}>
    <div class="flow-root">
      <dl class="mx-auto -my-3 mt-2 mb-0 space-y-4 text-sm">
        <FormInput
          id="api_key"
          type="text"
          title={t('payment.apiKey')}
          bind:value={settings.api_key}
          error={formErrors.api_key}
          ico="key"
        />
      </dl>
    </div>

    <DrawerFooter onclose={close} submitLabel={t('common.save')} />
  </form>
</div>
