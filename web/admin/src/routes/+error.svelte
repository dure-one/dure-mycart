<script lang="ts">
  import Blank from '$lib/layouts/Blank.svelte'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)
</script>

<Blank>
  <div class="content-center">
    <div class="header">
      <h1>404</h1>
      <p>{t('common.pageNotFound')}</p>
    </div>
  </div>
</Blank>
