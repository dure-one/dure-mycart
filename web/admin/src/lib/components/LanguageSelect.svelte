<script lang="ts">
  import SvgIcon from './SvgIcon.svelte'
  import { locale, availableLocales, translate, type Locale } from '$lib/i18n'

  interface Props {
    className?: string
  }

  let { className = '' }: Props = $props()

  // Reactive translation function
  let t = $derived($translate)
  let currentLocale = $derived($locale)
  let locales = $derived($availableLocales)

  function switchLocale(event: Event) {
    locale.set((event.currentTarget as HTMLSelectElement).value as Locale)
  }
</script>

<!-- Used from the shell's status bar and from the sign-in and install forms, so
     the language is reachable on every page, signed in or not. Each language is
     listed under its own name, which reads the same whichever language is
     active.

     The `field` class opts the select out of the global `select:not(.field)`
     rule in main.css, which would otherwise draw it as a bordered white input
     and override the transparent look the shell's controls share. -->
<label
  class="relative inline-flex cursor-pointer items-center gap-1.5 rounded px-3 py-2 text-gray-500 transition-colors hover:bg-gray-200 hover:text-gray-700 {className}"
>
  <SvgIcon name="glob-alt" stroke="currentColor" className="h-5 w-5 shrink-0" />
  <select
    class="field cursor-pointer appearance-none bg-transparent py-0 ps-0 pe-5 text-sm focus:outline-none"
    aria-label={t('settings.language')}
    value={currentLocale}
    onchange={switchLocale}
  >
    {#each locales as loc}
      <option value={loc.code}>{loc.name}</option>
    {/each}
  </select>
  <svg
    class="pointer-events-none absolute end-3 h-4 w-4"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.5"
    stroke-linecap="round"
    aria-hidden="true"
  >
    <path d="M6 9l6 6 6-6" />
  </svg>
</label>
