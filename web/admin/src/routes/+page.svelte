<script lang="ts">
  import { goto } from '$app/navigation'
  import { resolve } from '$app/paths'
  import { onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import { PageState } from '$lib/components'

  onMount(() => {
    goto(resolve('/products'))
  })
</script>

<Main>
  <PageState kind="loading" />
</Main>
