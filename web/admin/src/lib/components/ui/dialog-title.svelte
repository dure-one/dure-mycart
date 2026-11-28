<script lang="ts">
  import { Dialog as DialogPrimitive } from 'bits-ui'
  import { cn } from '$lib/utils/cn.js'
  import type { Snippet } from 'svelte'

  let {
    class: className,
    children,
    ...restProps
  }: DialogPrimitive.TitleProps & {
    children?: Snippet
  } = $props()
</script>

<DialogPrimitive.Title
  class={cn('text-lg font-semibold leading-none tracking-tight', className)}
  {...restProps}
>
  {@render children?.()}
</DialogPrimitive.Title>
