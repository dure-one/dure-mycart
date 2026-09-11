<script lang="ts">
  import { goto } from '$app/navigation'
  import { base } from '$app/paths'
  import { onDestroy, onMount } from 'svelte'
  import { browser } from '$app/environment'
  import Blank from '$lib/layouts/Blank.svelte'
  import FormInput from '$lib/components/form/Input.svelte'
  import FormButton from '$lib/components/form/Button.svelte'
  import { apiPost } from '$lib/utils/api'
  import { showMessage } from '$lib/utils/message'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  let email = $state('')
  let password = $state('')
  let domain = $state('')
  let dbType = $state('sqlite')
  let databaseUrl = $state('')
  let sqlitePath = $state('./lc_base/data.db')
  let emailError = $state('')
  let passwordError = $state('')
  let domainError = $state('')
  let dbError = $state('')

  let redirectTimer: ReturnType<typeof setTimeout> | undefined
  onDestroy(() => {
    if (redirectTimer !== undefined) clearTimeout(redirectTimer)
  })

  function validateEmail(value: string) {
    if (!value) {
      return t('install.emailRequired')
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
      return t('install.emailInvalid')
    }
    return ''
  }

  function validatePassword(value: string) {
    if (!value) {
      return t('install.passwordRequired')
    }
    if (value.length < 6) {
      return t('install.passwordMinLength')
    }
    if (value.length > 72) {
      return t('install.passwordMaxLength')
    }
    return ''
  }

  function validateDomain(value: string) {
    if (!value) {
      return t('install.domainRequired')
    }
    // Basic domain validation
    const domainRegex = /^([a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}$/i
    if (!domainRegex.test(value) && value !== 'localhost' && !value.startsWith('localhost:')) {
      return t('install.domainInvalid')
    }
    return ''
  }

  function validateDatabase() {
    if (dbType === 'postgres') {
      if (!databaseUrl) {
        return t('install.databaseUrlRequired') || 'PostgreSQL connection string required'
      }
      if (!databaseUrl.startsWith('postgres://') && !databaseUrl.startsWith('postgresql://')) {
        return t('install.databaseUrlInvalid') || 'Invalid PostgreSQL connection string'
      }
    } else if (dbType === 'sqlite') {
      if (!sqlitePath) {
        return t('install.sqlitePathRequired') || 'SQLite database path required'
      }
    }
    return ''
  }

  async function handleSubmit(event?: Event) {
    event?.preventDefault()

    emailError = validateEmail(email)
    passwordError = validatePassword(password)
    domainError = validateDomain(domain)
    dbError = validateDatabase()

    if (emailError || passwordError || domainError || dbError) {
      return
    }

    try {
      const payload = {
        email,
        password,
        domain,
        dbType,
        databaseUrl: dbType === 'postgres' ? databaseUrl : '',
        sqlitePath: dbType === 'sqlite' ? sqlitePath : ''
      }
      const res = await apiPost(`/api/install`, payload)
      if (res?.success) {
        showMessage(t('install.installedSuccessfully'), 'connextSuccess')
        // Redirect to signin page after successful installation
        redirectTimer = setTimeout(() => {
          goto(`${base}/signin`)
        }, 1000)
      } else {
        showMessage(res?.result || res?.message || t('install.installationFailed'), 'connextError')
      }
    } catch (error) {
      showMessage(t('install.networkError'), 'connextError')
    }
  }

  onMount(() => {
    // Set default domain from current location if in browser
    if (browser) {
      const url = new URL(window.location.href)
      domain = url.origin.replace(/^https?:\/\//, '')
    }
  })
</script>

<Blank>
  <div class="mx-auto max-w-screen-xl px-4 py-16 sm:px-6 lg:px-8">
    <div class="mx-auto max-w-lg text-center">
      <h1 class="text-2xl font-bold sm:text-3xl">🛒 {t('install.title')} myCart</h1>
      <p class="mt-4 text-gray-600">{t('install.configureCart')}</p>
    </div>
    <form onsubmit={(e) => handleSubmit(e)} class="mx-auto mt-8 mb-0 max-w-md space-y-4">
      <FormInput id="email" type="email" title={t('install.email')} ico="at-symbol" error={emailError} bind:value={email} />
      <FormInput
        id="password"
        type="password"
        title={t('install.password')}
        ico="finger-print"
        error={passwordError}
        bind:value={password}
      />
      <FormInput
        id="domain"
        type="text"
        title={t('install.domain')}
        ico="glob-alt"
        error={domainError}
        bind:value={domain}
        placeholder="example.com"
      />

      <!-- Database Selection -->
      <div>
        <label for="dbType" class="block text-sm font-medium text-gray-700 mb-2">
          {t('install.databaseType') || 'Database Type'}
        </label>
        <select
          id="dbType"
          bind:value={dbType}
          class="w-full rounded-lg border-gray-200 p-4 text-sm shadow-sm"
        >
          <option value="sqlite">SQLite (Default)</option>
          <option value="postgres">PostgreSQL</option>
        </select>
      </div>

      {#if dbType === 'postgres'}
        <FormInput
          id="databaseUrl"
          type="text"
          title={t('install.postgresConnection') || 'PostgreSQL Connection String'}
          ico="database"
          error={dbError}
          bind:value={databaseUrl}
          placeholder="postgresql://user:password@host:5432/database"
        />
      {:else}
        <FormInput
          id="sqlitePath"
          type="text"
          title={t('install.sqlitePath') || 'SQLite Database Path'}
          ico="folder"
          error={dbError}
          bind:value={sqlitePath}
          placeholder="./lc_base/data.db"
        />
      {/if}

      <FormButton type="submit" name={t('install.installButton')} color="green" ico="arrow-right" />
    </form>
  </div>
</Blank>
