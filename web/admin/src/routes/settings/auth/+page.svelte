<script lang="ts">
  import { onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import { FormButton, FormInput, PageHeader, PageState, Section } from '$lib/components'
  import { loadSettings, saveSettings } from '$lib/utils/settingsHelpers'
  import { saveData } from '$lib/utils/apiHelpers'
  import { validators, validateFields } from '$lib/utils/validation'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  interface AuthSettings {
    email: string
  }

  interface PasswordData {
    old: string
    new: string
  }

  let formData = $state<AuthSettings>({
    email: ''
  })
  let passwordData = $state<PasswordData>({
    old: '',
    new: ''
  })
  let formErrors = $state<Record<string, string>>({})
  let passwordErrors = $state<Record<string, string>>({})
  let loading = $state(true)

  onMount(async () => {
    formData = await loadSettings<AuthSettings>('auth', formData)
    loading = false
  })

  async function handleSubmit() {
    formErrors = validateFields(formData, [{ field: 'email', ...validators.email(t('settings.validEmailRequired')) }])

    if (Object.keys(formErrors).length > 0) {
      return
    }

    await saveSettings('auth', formData)
  }

  async function handlePasswordSubmit() {
    passwordErrors = validateFields(passwordData, [
      { field: 'old', ...validators.required(t('settings.oldPasswordRequired')) },
      { field: 'new', ...validators.minLength(6, t('settings.newPasswordMinLength')) }
    ])

    if (Object.keys(passwordErrors).length > 0) {
      return
    }

    const result = await saveData<PasswordData>(
      '/api/_/settings/password',
      passwordData,
      true,
      t('settings.passwordUpdated'),
      t('settings.failedToUpdatePassword')
    )

    if (result !== null) {
      passwordData = { old: '', new: '' }
    }
  }
</script>

<Main>
  <PageHeader title={t('settings.authSettings')} />

  {#if loading}
    <PageState kind="loading" />
  {:else}
    <Section title={t('settings.email')}>
      <form onsubmit={(e) => { e.preventDefault(); handleSubmit() }} class="max-w-2xl space-y-4">
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
    </Section>

    <Section title={t('settings.changePassword')}>
      <form onsubmit={(e) => { e.preventDefault(); handlePasswordSubmit() }} class="max-w-2xl space-y-4">
        <FormInput
          id="old_password"
          type="password"
          title={t('settings.oldPassword')}
          bind:value={passwordData.old}
          error={passwordErrors.old}
          ico="finger-print"
        />
        <FormInput
          id="new_password"
          type="password"
          title={t('settings.newPassword')}
          bind:value={passwordData.new}
          error={passwordErrors.new}
          ico="finger-print"
        />
        <div class="pt-4">
          <FormButton type="submit" name={t('settings.updatePassword')} variant="primary" />
        </div>
      </form>
    </Section>
  {/if}
</Main>
