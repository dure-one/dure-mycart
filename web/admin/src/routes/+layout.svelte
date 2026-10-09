<script lang="ts">
  import { onMount } from 'svelte'
  import '../assets/app.css'
  import { Toaster } from 'svelte-sonner'
  import Alert from '$lib/components/Alert.svelte'
  import SvgSprite from '$lib/components/SvgSprite.svelte'
  import { mainSettingsStore } from '$lib/stores/main'
  import { loadData } from '$lib/utils/apiHelpers'
  import type { MainSettings } from '$lib/types/models'

  let { children } = $props()

  onMount(async () => {
    const settings = await loadData<MainSettings>('/api/settings/main', '')
    if (settings) {
      mainSettingsStore.set(settings)
    }
  })
</script>

<SvgSprite />
<Toaster position="bottom-right" />
<Alert />
{@render children?.()}
