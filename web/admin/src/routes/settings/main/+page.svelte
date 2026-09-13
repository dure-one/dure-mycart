<script lang="ts">
  import { onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import { Chip, ChipGroup, FormButton, FormInput, PageHeader, PageState, Section } from '$lib/components'
  import { loadSettings, saveSettings } from '$lib/utils/settingsHelpers'
  import { validators, validateFields } from '$lib/utils/validation'
  import { translate, locale, availableLocales, type Locale } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)
  let currentLocale = $derived($locale)
  let locales = $derived($availableLocales)

  interface MainSettings {
    site_name: string
    domain: string
    email: string
  }

  let formData = $state<MainSettings>({
    site_name: '',
    domain: '',
    email: ''
  })
  let formErrors = $state<Record<string, string>>({})
  let loading = $state(true)

  onMount(async () => {
    const loaded = await loadSettings<MainSettings>('main', formData)
    if (loaded) {
      formData = loaded
    }
    loading = false
  })

  async function handleSubmit() {
    formErrors = validateFields(formData, [
      { field: 'site_name', ...validators.minLength(6, t('settings.siteNameMinLength')) },
      { field: 'domain', ...validators.required(t('settings.domainRequired')) },
      { field: 'email', ...validators.email(t('settings.validEmailRequired')) }
    ])

    if (Object.keys(formErrors).length > 0) {
      return
    }

    await saveSettings('main', formData)
  }

  function switchLocale(newLocale: Locale) {
    locale.set(newLocale)
  }
</script>

<Main>
  <PageHeader title={t('settings.mainSettings')} />

  {#if loading}
    <PageState kind="loading" />
  {:else}
    <form onsubmit={(e) => { e.preventDefault(); handleSubmit() }} class="max-w-2xl space-y-4">
      <FormInput
        id="site_name"
        title={t('settings.siteName')}
        bind:value={formData.site_name}
        error={formErrors.site_name}
        ico="home"
      />
      <FormInput
        id="domain"
        title={t('settings.domain')}
        bind:value={formData.domain}
        error={formErrors.domain}
        ico="link"
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

    <Section title={t('settings.language')}>
      <ChipGroup>
        {#each locales as loc}
          <Chip active={currentLocale === loc.code} onclick={() => switchLocale(loc.code)}>{loc.name}</Chip>
        {/each}
      </ChipGroup>
    </Section>
  {/if}
</Main>
