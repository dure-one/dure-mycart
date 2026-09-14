<script lang="ts">
  import type { Snippet } from 'svelte'

  interface Props {
    variant?: 'default' | 'danger'
    name?: string
    disabled?: boolean
    class?: string
    onclick?: (event: MouseEvent) => void
    children?: Snippet
  }

  let {
    variant = 'default',
    name = undefined,
    disabled = false,
    class: className = '',
    onclick,
    children
  }: Props = $props()

  const VARIANT_CLASSES: Record<string, string> = {
    default: 'text-green-700 hover:text-green-900',
    danger: 'text-red-700 hover:text-red-800'
  }

  let variantClass = $derived(VARIANT_CLASSES[variant] ?? VARIANT_CLASSES.default)
</script>

<button
  type="button"
  class="cursor-pointer text-sm font-medium {variantClass} {className} disabled:cursor-not-allowed disabled:opacity-50"
  {disabled}
  {onclick}
>
  {#if children}
    {@render children()}
  {:else}
    {name}
  {/if}
</button>
