<script lang="ts">
  export interface SelectOption {
    value: string
    label: string
  }

  interface Props {
    value: string
    options: SelectOption[]
    label: string
    onChange: (value: string) => void
    className?: string
  }

  let { value, options, label, onChange, className = '' }: Props = $props()

  const uid = $props.id()
  const listboxId = `${uid}-listbox`
  const optionId = (index: number) => `${listboxId}-option-${index}`

  let open = $state(false)
  let activeIndex = $state(-1)
  let root = $state<HTMLDivElement | null>(null)
  let trigger = $state<HTMLButtonElement | null>(null)

  let selectedIndex = $derived(options.findIndex((option) => option.value === value))
  let selectedLabel = $derived(options[selectedIndex]?.label ?? value)

  // Opens on the current choice, like a native dropdown does; ArrowUp opens on
  // the last one.
  function openList(atEnd = false) {
    activeIndex = atEnd ? options.length - 1 : Math.max(selectedIndex, 0)
    open = true
  }

  function closeList(refocus = false) {
    open = false
    if (refocus) trigger?.focus()
  }

  function choose(index: number) {
    const option = options[index]
    if (!option) return
    onChange(option.value)
    closeList(true)
  }

  function move(step: number) {
    if (options.length === 0) return
    activeIndex = (activeIndex + step + options.length) % options.length
  }

  // Focus stays on the trigger, so every key is handled there and the active
  // option is exposed through aria-activedescendant. The options carry
  // tabindex="-1" only to keep them out of the tab order — they are activated
  // through the trigger, never focused on their own.
  function handleKeydown(event: KeyboardEvent) {
    if (!open) {
      if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
        event.preventDefault()
        openList()
      } else if (event.key === 'ArrowUp') {
        event.preventDefault()
        openList(true)
      }
      return
    }

    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        move(1)
        break
      case 'ArrowUp':
        event.preventDefault()
        move(-1)
        break
      case 'Home':
        event.preventDefault()
        activeIndex = 0
        break
      case 'End':
        event.preventDefault()
        activeIndex = options.length - 1
        break
      case 'Enter':
      case ' ':
        event.preventDefault()
        choose(activeIndex)
        break
      case 'Escape':
        event.preventDefault()
        closeList(true)
        break
      case 'Tab':
        closeList()
        break
    }
  }

  function handleWindowClick(event: MouseEvent) {
    if (open && root && !root.contains(event.target as Node)) {
      closeList()
    }
  }
</script>

<svelte:window onclick={handleWindowClick} />

<!-- A native <select> cannot be used here: the browser draws its opened list and
     takes no styling for it, so the site's look would break the moment the list
     opens. This is the ARIA select-only combobox instead — focus never leaves the
     trigger, the arrow keys move the active option, and the list is ordinary
     markup that can carry the same black border, offset shadow and yellow
     highlight as the rest of the site. -->
<div class="relative {className}" bind:this={root}>
  <button
    type="button"
    bind:this={trigger}
    role="combobox"
    class="flex w-full cursor-pointer items-center justify-between gap-4 border-4 border-black bg-white py-3 pr-4 pl-6 text-sm font-black tracking-wider text-black uppercase transition-all duration-200 hover:-translate-x-1 hover:-translate-y-1 hover:shadow-[12px_12px_0px_0px_rgba(0,0,0,1)] focus:ring-4 focus:ring-black focus:outline-none"
    aria-label={label}
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-controls={open ? listboxId : undefined}
    aria-activedescendant={open && activeIndex >= 0 ? optionId(activeIndex) : undefined}
    onclick={() => (open ? closeList() : openList())}
    onkeydown={handleKeydown}
  >
    <span>{selectedLabel}</span>
    <svg
      class="h-4 w-4 shrink-0 transition-transform duration-200 {open ? 'rotate-180' : ''}"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <use href="/assets/img/sprite.svg#chevron-down" />
    </svg>
  </button>

  {#if open}
    <div
      id={listboxId}
      role="listbox"
      tabindex="-1"
      aria-label={label}
      class="absolute right-0 z-50 mt-2 max-h-[70vh] min-w-full overflow-y-auto border-4 border-black bg-white shadow-[8px_8px_0px_0px_rgba(0,0,0,1)]"
    >
      {#each options as option, index (option.value)}
        <button
          type="button"
          id={optionId(index)}
          role="option"
          tabindex="-1"
          aria-selected={option.value === value}
          class="flex w-full cursor-pointer items-center justify-between gap-4 px-6 py-3 text-start text-sm font-black tracking-wider uppercase {index ===
            activeIndex || option.value === value
            ? 'bg-yellow-300'
            : 'bg-white'}"
          onclick={() => choose(index)}
          onmouseenter={() => (activeIndex = index)}
        >
          <span>{option.label}</span>
          {#if option.value === value}
            <svg class="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
              <use href="/assets/img/sprite.svg#check" />
            </svg>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>
