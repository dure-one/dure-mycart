<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    active?: boolean
    disabled?: boolean
    onclick?: (event: MouseEvent) => void
    class?: string
    children?: Snippet
  }

  let { active = false, disabled = false, onclick, class: className = '', children }: Props = $props()

  let activeClass = $derived(
    active ? 'bg-green-200 text-green-900' : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
  )
</script>

<button
  type="button"
  class="cursor-pointer rounded p-2 text-sm {activeClass} {className} disabled:cursor-not-allowed disabled:opacity-50"
  {disabled}
  {onclick}
>
  {#if children}
    {@render children()}
  {/if}
</button>
