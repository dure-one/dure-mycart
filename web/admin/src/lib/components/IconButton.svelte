<script lang="ts">
  import SvgIcon from './SvgIcon.svelte'

  interface Props {
    ico: string
    label: string
    variant?: 'default' | 'danger'
    /**
     * Toggle state, for a button that stays on — a toolbar action over a
     * selection. Omit it for a plain action: the button then carries no
     * `aria-pressed`, because there is no state for one to report.
     */
    active?: boolean
    disabled?: boolean
    svgClass?: string
    class?: string
    onclick?: (event: MouseEvent) => void
  }

  let {
    ico,
    label,
    variant = 'default',
    active = undefined,
    disabled = false,
    svgClass = 'h-5 w-5',
    class: className = '',
    onclick
  }: Props = $props()

  const VARIANT_CLASSES: Record<string, string> = {
    default: 'text-gray-500 hover:bg-gray-200 hover:text-gray-700',
    danger: 'text-red-700 hover:bg-red-100 hover:text-red-800'
  }

  // An active toggle wears the chip's on state rather than a filled accent:
  // several toolbar buttons can be on at once, and a row of solid green
  // reads as a row of primary actions.
  let stateClass = $derived(
    active ? 'bg-green-200 text-green-900' : (VARIANT_CLASSES[variant] ?? VARIANT_CLASSES.default)
  )
</script>

<button
  type="button"
  aria-label={label}
  aria-pressed={active === undefined ? undefined : active}
  title={label}
  class="inline-flex cursor-pointer items-center justify-center rounded p-1.5 {stateClass} {className} disabled:cursor-not-allowed disabled:opacity-50"
  {disabled}
  {onclick}
>
  <SvgIcon name={ico} stroke="currentColor" className={svgClass} />
</button>
