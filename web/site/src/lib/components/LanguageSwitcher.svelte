<script lang="ts">
  import Select from './Select.svelte'
  import { locale, availableLocales, translate, type Locale } from '$lib/i18n'

  let t = $derived($translate)
  let currentLocale = $derived($locale)

  // Each language is listed under its own name, so the list reads the same
  // whichever language is currently active.
  let options = $derived($availableLocales.map((loc) => ({ value: loc.code, label: loc.name })))
</script>

<Select value={currentLocale} {options} label={t('header.language')} onChange={(code) => locale.set(code as Locale)} />
