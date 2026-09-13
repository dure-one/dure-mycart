<script lang="ts">
  import SvgIcon from './SvgIcon.svelte'

  interface Props {
    ico: string
    label: string
    variant?: 'default' | 'danger'
    disabled?: boolean
    svgClass?: string
    class?: string
    onclick?: (event: MouseEvent) => void
  }

  let {
    ico,
    label,
    variant = 'default',
    disabled = false,
    svgClass = 'h-5 w-5',
    class: className = '',
    onclick
  }: Props = $props()

  const VARIANT_CLASSES: Record<string, string> = {
    default: 'text-gray-500 hover:bg-gray-200 hover:text-gray-700',
    danger: 'text-red-700 hover:bg-red-100 hover:text-red-800'
  }

  let variantClass = $derived(VARIANT_CLASSES[variant] ?? VARIANT_CLASSES.default)
</script>

<button
  type="button"
  aria-label={label}
  title={label}
  class="inline-flex cursor-pointer items-center justify-center rounded p-1.5 {variantClass} {className} disabled:cursor-not-allowed disabled:opacity-50"
  {disabled}
  {onclick}
>
  <SvgIcon name={ico} stroke="currentColor" className={svgClass} />
</button>
