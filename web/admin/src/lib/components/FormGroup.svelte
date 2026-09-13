<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    label: string
    children?: Snippet
  }

  let { label, children }: Props = $props()
</script>

<hr />
<h3 class="mb-3">{label}</h3>
{#if children}
  {@render children()}
{/if}
