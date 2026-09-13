<script lang="ts">
  import SvgIcon from './SvgIcon.svelte'

  interface Props {
    variant?: 'success' | 'warning' | 'danger' | 'neutral'
    ico?: string
    svgClass?: string
    children?: import('svelte').Snippet
  }

  let {
    variant = 'neutral',
    ico = undefined,
    svgClass = '-ms-0.5 me-5.5 h-5 w-5',
    children
  }: Props = $props()

  const VARIANT_CLASSES: Record<string, string> = {
    success: 'bg-green-100 text-green-800',
    warning: 'bg-yellow-100 text-yellow-800',
    danger: 'bg-red-100 text-red-800',
    neutral: 'bg-gray-100 text-gray-700'
  }

  let variantClass = $derived(VARIANT_CLASSES[variant] ?? VARIANT_CLASSES.neutral)
</script>

<span class="badge {variantClass}">
  {#if ico}
    <SvgIcon name={ico} stroke="currentColor" className={ico ? `-ms-0.5 me-1.5 ${svgClass}` : 'h-0 w-0'} />
  {/if}
  <p>
    {#if children}
      {@render children()}
    {/if}
  </p>
</span>

<style>
  @reference "tailwindcss";

  :global(.badge) {
    @apply inline-flex items-center justify-center rounded-full px-2.5 py-0.5;
  }

  :global(.badge p) {
    @apply text-xs whitespace-nowrap;
  }
</style>
