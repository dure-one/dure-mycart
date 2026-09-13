<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    gap?: string
    class?: string
    children?: Snippet
  }

  let { gap = 'gap-2', class: className = '', children }: Props = $props()
</script>

<div class="flex flex-wrap items-center {gap} {className}">
  {#if children}
    {@render children()}
  {/if}
</div>
