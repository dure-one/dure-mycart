<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    title: string
    children?: Snippet
  }

  let { title, children }: Props = $props()
</script>

<hr class="mt-5" />

<div class="mt-5">
  <h2 class="mb-5">{title}</h2>
  {#if children}
    {@render children()}
  {/if}
</div>
