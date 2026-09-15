<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/state'
  import { goto } from '$app/navigation'
  import { apiPost } from '$lib/utils/api'
  import { cabinetAvailable } from '$lib/utils/cabinet'
  import { customerErrorKey } from '$lib/utils/customerErrors'
  import { handleNavigation } from '$lib/utils/navigation'
  import CabinetUnavailable from '$lib/components/CabinetUnavailable.svelte'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  let email = $state('')
  let password = $state('')
  let error = $state('')
  let busy = $state(false)

  // null until the server has answered. The form is not drawn before then: a
  // shop whose cabinet is off must not show a sign-in form even for a moment,
  // and hiding it in the settings alone would leave the page itself working.
  let available = $state<boolean | null>(null)

  onMount(async () => {
    available = await cabinetAvailable()
  })

  // Set by the sign-up form. Sign-up deliberately does not sign the new account
  // in, so the buyer types their address twice — which is the point of it, and
  // the reason this says what just happened instead of silently showing a form.
  let justCreated = $derived(page.url.searchParams.get('created') === '1')

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    error = ''
    busy = true

    const res = await apiPost('/api/customer/signin', { email, password })
    busy = false

    if (res.success) {
      goto('/account')
      return
    }

    // 404 is the shop answering that it has no cabinet at all — the one failure
    // a buyer can do nothing about, and worth saying differently from a wrong
    // password.
    error = res.status === 404 ? t('account.unavailable') : t(customerErrorKey(res.message))
  }
</script>

<div class="min-h-screen bg-white px-4 py-12 sm:px-6 lg:px-8">
  <div class="mx-auto max-w-md">
    {#if available === false}
      <CabinetUnavailable title={t('account.signInTitle')} />
    {:else if available === null}
      <div class="brutal-card p-12 text-center">
        <p class="text-xl font-black tracking-wider text-black uppercase">{t('common.loading')}</p>
      </div>
    {:else}
      <div class="brutal-card p-8 sm:p-12">
        <h1 class="mb-2 text-3xl font-black tracking-tighter text-black uppercase sm:text-4xl">
          {t('account.signInTitle')}
        </h1>
        <p class="mb-8 text-lg tracking-wide text-black">{t('account.signInSubtitle')}</p>

        {#if justCreated}
          <p
            class="mb-6 border-4 border-black bg-green-300 p-4 text-base font-black tracking-wide text-black uppercase"
          >
            {t('account.created')}
          </p>
        {/if}

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
            <label for="password" class="mb-2 block text-sm font-black tracking-wider text-black uppercase">
              {t('account.password')}
            </label>
            <input
              type="password"
              id="password"
              bind:value={password}
              required
              autocomplete="current-password"
              class="w-full border-4 border-black bg-white px-4 py-3 text-lg font-black tracking-wider text-black focus:ring-4 focus:ring-yellow-300 focus:outline-none"
            />
          </div>

          <button
            type="submit"
            disabled={busy}
            class="w-full cursor-pointer border-4 border-black bg-yellow-300 px-8 py-4 text-xl font-black tracking-wider text-black uppercase transition-all duration-200 enabled:hover:-translate-x-1 enabled:hover:-translate-y-1 enabled:hover:shadow-[14px_14px_0px_0px_rgba(0,0,0,1)] disabled:cursor-not-allowed disabled:opacity-50"
          >
            {busy ? t('common.loading') : t('account.signIn')}
          </button>
        </form>

        <p class="mt-8 text-center text-base font-black tracking-wide text-black uppercase">
          {t('account.noAccount')}
          <a
            href="/signup"
            onclick={(e) => handleNavigation(e, '/signup')}
            class="ml-1 underline decoration-yellow-300 decoration-4 underline-offset-4 hover:decoration-black"
          >
            {t('account.signUp')}
          </a>
        </p>
      </div>
    {/if}
  </div>
</div>
