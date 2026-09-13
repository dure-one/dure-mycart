<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { apiPost } from '$lib/utils/api'
  import { cabinetAvailable } from '$lib/utils/cabinet'
  import { handleNavigation } from '$lib/utils/navigation'
  import CabinetUnavailable from '$lib/components/CabinetUnavailable.svelte'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  let email = $state('')
  let name = $state('')
  let password = $state('')
  let error = $state('')
  let busy = $state(false)

  // null until the server has answered, so that a shop without a cabinet never
  // draws a registration form it would refuse to accept.
  let available = $state<boolean | null>(null)

  onMount(async () => {
    available = await cabinetAvailable()
  })

  // The same rules the API enforces, checked here so the buyer reads them in
  // their own language. The server stays the authority: whatever this misses
  // comes back as its own message, and nothing is created on the strength of a
  // check that only ran in the browser.
  const MIN_PASSWORD = 8

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    error = ''

    if (!email.includes('@')) {
      error = t('account.invalidEmail')
      return
    }
    if (password.length < MIN_PASSWORD) {
      error = t('account.shortPassword')
      return
    }

    busy = true
    const res = await apiPost('/api/customer/signup', { email, password, name })
    busy = false

    if (res.success) {
      // Not signed in: the account exists, and the buyer is sent to the sign-in
      // form so the address they typed is the one they sign in with.
      goto('/signin?created=1')
      return
    }

    error = res.status === 404 ? t('account.unavailable') : res.message || t('account.failed')
  }
</script>

<div class="min-h-screen bg-white px-4 py-12 sm:px-6 lg:px-8">
  <div class="mx-auto max-w-md">
    {#if available === false}
      <CabinetUnavailable title={t('account.signUpTitle')} />
    {:else if available === null}
      <div class="brutal-card p-12 text-center">
        <p class="text-xl font-black tracking-wider text-black uppercase">{t('common.loading')}</p>
      </div>
    {:else}
      <div class="brutal-card p-8 sm:p-12">
        <h1 class="mb-2 text-3xl font-black tracking-tighter text-black uppercase sm:text-4xl">
          {t('account.signUpTitle')}
        </h1>
        <p class="mb-8 text-lg tracking-wide text-black">{t('account.signUpSubtitle')}</p>

        {#if error}
          <p class="mb-6 border-4 border-black bg-red-300 p-4 text-base font-black tracking-wide text-black uppercase">
            {error}
          </p>
        {/if}

        <form onsubmit={handleSubmit} class="space-y-6">
          <div>
            <label for="email" class="mb-2 block text-sm font-black tracking-wider text-black uppercase">
              {t('account.email')}
            </label>
            <input
              type="email"
              id="email"
              bind:value={email}
              required
              autocomplete="email"
              class="w-full border-4 border-black bg-white px-4 py-3 text-lg font-black tracking-wider text-black uppercase focus:ring-4 focus:ring-yellow-300 focus:outline-none"
              placeholder={t('account.emailPlaceholder')}
            />
          </div>

          <div>
            <label for="name" class="mb-2 block text-sm font-black tracking-wider text-black uppercase">
              {t('account.name')}
            </label>
            <input
              type="text"
              id="name"
              bind:value={name}
              autocomplete="name"
              maxlength="100"
              class="w-full border-4 border-black bg-white px-4 py-3 text-lg font-black tracking-wider text-black uppercase focus:ring-4 focus:ring-yellow-300 focus:outline-none"
              placeholder={t('account.namePlaceholder')}
            />
          </div>

          <div>
            <label for="password" class="mb-2 block text-sm font-black tracking-wider text-black uppercase">
              {t('account.password')}
            </label>
            <input
              type="password"
              id="password"
              bind:value={password}
              required
              autocomplete="new-password"
              minlength={MIN_PASSWORD}
              class="w-full border-4 border-black bg-white px-4 py-3 text-lg font-black tracking-wider text-black focus:ring-4 focus:ring-yellow-300 focus:outline-none"
            />
            <p class="mt-2 text-sm tracking-wide text-gray-700">{t('account.passwordHint')}</p>
          </div>

          <button
            type="submit"
            disabled={busy}
            class="w-full cursor-pointer border-4 border-black bg-yellow-300 px-8 py-4 text-xl font-black tracking-wider text-black uppercase transition-all duration-200 enabled:hover:-translate-x-1 enabled:hover:-translate-y-1 enabled:hover:shadow-[14px_14px_0px_0px_rgba(0,0,0,1)] disabled:cursor-not-allowed disabled:opacity-50"
          >
            {busy ? t('common.loading') : t('account.signUp')}
          </button>
        </form>

        <p class="mt-8 text-center text-base font-black tracking-wide text-black uppercase">
          {t('account.haveAccount')}
          <a
            href="/signin"
            onclick={(e) => handleNavigation(e, '/signin')}
            class="ml-1 underline decoration-yellow-300 decoration-4 underline-offset-4 hover:decoration-black"
          >
            {t('account.signIn')}
          </a>
        </p>
      </div>
    {/if}
  </div>
</div>
