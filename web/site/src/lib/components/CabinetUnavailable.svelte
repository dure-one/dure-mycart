<script lang="ts">
  import { translate } from '$lib/i18n'
  import { handleNavigation } from '$lib/utils/navigation'

  // Reactive translation function
  let t = $derived($translate)

  interface Props {
    title: string
  }

  let { title }: Props = $props()
</script>

<div class="brutal-card bg-yellow-300 p-8 sm:p-12">
  <h1 class="mb-4 text-3xl font-black tracking-tighter text-black uppercase sm:text-4xl">{title}</h1>
  <p class="mb-8 text-lg tracking-wide text-black">{t('account.unavailable')}</p>
  <a href="/" onclick={(e) => handleNavigation(e, '/')} class="brutal-btn inline-block bg-white px-8 py-3">
    {t('account.goToShop')}
  </a>
</div>
