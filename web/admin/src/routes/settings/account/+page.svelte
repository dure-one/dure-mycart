<script lang="ts">
  import { onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import { FormButton, FormInput, FormToggle, PageHeader, PageState, Section } from '$lib/components'
  import { loadSettings, saveSettings } from '$lib/utils/settingsHelpers'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  interface AccountSettings {
    enabled: boolean
    expire_hours: number
  }

  // Stored in hours, edited in days: "how long does a buyer stay signed in" is
  // a question an operator answers in days, and the setting is only ever read
  // back by the session issuer.
  const HOURS_PER_DAY = 24
  const DEFAULT_DAYS = 30
  const MAX_DAYS = 365

  let formData = $state<AccountSettings>({ enabled: false, expire_hours: DEFAULT_DAYS * HOURS_PER_DAY })
  let sessionDays = $state(String(DEFAULT_DAYS))
  let formErrors = $state<Record<string, string>>({})
  let loading = $state(true)

  onMount(async () => {
    formData = await loadSettings<AccountSettings>('account', formData)
    const days = Math.round((formData.expire_hours || DEFAULT_DAYS * HOURS_PER_DAY) / HOURS_PER_DAY)
    sessionDays = String(Math.min(Math.max(days, 1), MAX_DAYS))
    loading = false
  })

  async function handleSubmit() {
    const days = Number(sessionDays)
    formErrors = {}

    if (!Number.isInteger(days) || days < 1 || days > MAX_DAYS) {
      formErrors.sessionDays = t('settings.sessionLifetimeInvalid')
      return
    }

    const saved = await saveSettings('account', {
      enabled: formData.enabled,
      expire_hours: days * HOURS_PER_DAY
    })

    if (saved) {
      formData.expire_hours = days * HOURS_PER_DAY
    }
  }
</script>

<Main>
  <PageHeader title={t('settings.customerSettings')} />

  {#if loading}
    <PageState kind="loading" />
  {:else}
    <Section title={t('settings.customerCabinet')}>
      <form onsubmit={(e) => { e.preventDefault(); handleSubmit() }} class="max-w-2xl space-y-6">
        <div class="flex items-center justify-between gap-4">
          <div>
            <h3>{t('settings.customerCabinetEnabled')}</h3>
            <p class="text-sm text-gray-500">{t('settings.customerCabinetEnabledDesc')}</p>
          </div>
          <FormToggle id="customer-cabinet" bind:value={formData.enabled} />
        </div>

        <div>
          <FormInput
            id="session_days"
            type="number"
            min="1"
            max={MAX_DAYS}
            title={t('settings.sessionLifetime')}
            ico="user"
            bind:value={sessionDays}
            error={formErrors.sessionDays}
          />
          <p class="mt-2 text-sm text-gray-500">{t('settings.sessionLifetimeDesc')}</p>
        </div>

        <div class="pt-4">
          <FormButton type="submit" name={t('common.save')} variant="primary" />
        </div>
      </form>
    </Section>
  {/if}
</Main>
