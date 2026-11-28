<script lang="ts">
  import { cn } from '$lib/utils/cn.js'
  import type { Snippet } from 'svelte'

  let {
    class: className,
    children,
    ...restProps
  }: {
    class?: string
    children?: Snippet
  } & Record<string, any> = $props()
</script>

<h3
  class={cn('text-2xl font-semibold leading-none tracking-tight', className)}
  {...restProps}
>
  {@render children?.()}
</h3>
