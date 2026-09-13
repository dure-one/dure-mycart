<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    title: string
    actions?: Snippet
  }

  let { title, actions }: Props = $props()
</script>

<div class="mb-5 flex items-center justify-between gap-4">
  <h1>{title}</h1>
  {#if actions}
    <div class="flex flex-none items-center gap-2">
      {@render actions()}
    </div>
  {/if}
</div>
