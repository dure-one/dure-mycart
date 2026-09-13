<script lang="ts">
  import { goto } from '$app/navigation'
  import { base } from '$app/paths'
  import { onMount } from 'svelte'
  import Blank from '$lib/layouts/Blank.svelte'
  import FormInput from '$lib/components/form/Input.svelte'
  import FormButton from '$lib/components/form/Button.svelte'
  import { apiPost, apiGet } from '$lib/utils/api'
  import { showMessage } from '$lib/utils/message'
  import LanguageSelect from '$lib/components/LanguageSelect.svelte'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  let email = $state('')
  let password = $state('')
  let emailError = $state('')
  let passwordError = $state('')

  function validateEmail(value: string) {
    if (!value) {
      return t('auth.emailRequired')
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
      return t('auth.emailInvalid')
    }
    return ''
  }

  function validatePassword(value: string) {
    if (!value) {
      return t('auth.passwordRequired')
    }
    if (value.length < 6) {
      return t('auth.passwordMinLength')
    }
    return ''
  }

  async function handleSubmit() {
    emailError = validateEmail(email)
    passwordError = validatePassword(password)

    if (emailError || passwordError) {
      return
    }

    try {
      const res = await apiPost(`/api/sign/in`, { email, password })
      if (res?.success) {
        goto(`${base}/products`)
      } else {
        showMessage(res?.result || res?.message || t('auth.loginFailed'), 'connextError')
      }
    } catch (error) {
      showMessage(t('auth.networkError'), 'connextError')
    }
  }

  onMount(async () => {
    // Check if already authenticated
    try {
      const checkRes = await apiGet('/api/_/version')
      if (checkRes?.success) {
        goto(`${base}/products`)
      }
    } catch (error) {
      // Not authenticated, stay on signin page
    }
  })
</script>

<Blank>
  <div class="content-center">
    <div class="header">
      <h1>👨‍🎨 {t('auth.adminSignIn')}</h1>
    </div>
    <form
      onsubmit={(e) => {
        e.preventDefault()
        handleSubmit()
      }}
      class="mx-auto mt-8 mb-0 max-w-md space-y-4"
    >
      <FormInput
        id="email"
        type="email"
        title={t('auth.email')}
        ico="at-symbol"
        error={emailError}
        bind:value={email}
      />
      <FormInput
        id="password"
        type="password"
        title={t('auth.password')}
        ico="finger-print"
        error={passwordError}
        bind:value={password}
      />
      <!-- The picker shares the row with the submit action: someone who lands
           here in a language they cannot read has to be able to change it
           before signing in, and it is found where the eye already is. -->
      <div class="flex items-center justify-between gap-4">
        <FormButton type="submit" name={t('auth.login')} variant="primary" ico="arrow-right" />
        <LanguageSelect />
      </div>
    </form>
  </div>
</Blank>
