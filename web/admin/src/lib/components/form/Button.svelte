<script lang="ts">
  import type { Snippet } from 'svelte'
  import SvgIcon from '../SvgIcon.svelte'
  import { DEFAULT_BUTTON_NAME } from '$lib/constants/ui'

  interface Props {
    variant?: 'primary' | 'secondary' | 'danger'
    size?: 'sm' | 'md' | 'lg'
    name?: string
    ico?: string
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
    onclick?: (event: MouseEvent) => void
    class?: string
    children?: Snippet
  }

  let {
    variant = 'primary',
    size = 'md',
    name = DEFAULT_BUTTON_NAME,
    ico = undefined,
    type = 'button',
    disabled = false,
    onclick,
    class: className = '',
    children
  }: Props = $props()

  const VARIANT_CLASSES: Record<string, string> = {
    primary: 'bg-green-600 hover:bg-green-700 text-white',
    secondary: 'bg-gray-200 hover:bg-gray-300 text-gray-800 border border-gray-300',
    danger: 'bg-red-600 hover:bg-red-700 text-white'
  }

  const SIZE_CLASSES: Record<string, string> = {
    sm: 'px-3 py-1.5 text-sm',
    md: 'px-8 py-2 text-sm',
    lg: 'px-10 py-3 text-base'
  }

  let variantClasses = $derived(VARIANT_CLASSES[variant] ?? VARIANT_CLASSES.primary)
  let sizeClasses = $derived(SIZE_CLASSES[size])
  let icoClasses = $derived(ico ? 'focus:outline-none focus:ring' : '')
</script>

<button
  class="group relative inline-flex cursor-pointer items-center overflow-hidden rounded font-medium {variantClasses} {sizeClasses} {icoClasses} {className} disabled:opacity-50 disabled:cursor-not-allowed"
  {type}
  {disabled}
  onclick={onclick}
>
  {#if ico}
    <SvgIcon
      name={ico}
      stroke="currentColor"
      className="h-4 w-4 absolute -start-full transition-all group-hover:start-4"
    />
  {/if}
  {#if children}
    {@render children()}
  {:else}
    <span class="{ico ? 'transition-all group-hover:ms-2 group-hover:-me-2' : ''}">
      {name}
    </span>
  {/if}
</button>
