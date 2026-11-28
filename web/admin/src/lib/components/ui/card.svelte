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

<div
  class={cn('rounded-lg border border-zinc-200 bg-white text-zinc-950 shadow-sm', className)}
  {...restProps}
>
  {@render children?.()}
</div>
