<script lang="ts">
  import { type VariantProps, tv } from 'tailwind-variants'
  import { cn } from '$lib/utils/cn.js'
  import type { Snippet } from 'svelte'

  const badgeVariants = tv({
    base: 'inline-flex items-center rounded-full border border-zinc-200 px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-zinc-950 focus:ring-offset-2',
    variants: {
      variant: {
        default: 'border-transparent bg-zinc-900 text-zinc-50 hover:bg-zinc-900/80',
        secondary: 'border-transparent bg-zinc-100 text-zinc-900 hover:bg-zinc-100/80',
        destructive: 'border-transparent bg-red-500 text-zinc-50 hover:bg-red-500/80',
        outline: 'text-zinc-950'
      }
    },
    defaultVariants: {
      variant: 'default'
    }
  })

  type Variant = VariantProps<typeof badgeVariants>['variant']

  let {
    class: className,
    variant = 'default',
    children,
    ...restProps
  }: {
    class?: string
    variant?: Variant
    children?: Snippet
  } & Record<string, any> = $props()
</script>

<div class={cn(badgeVariants({ variant }), className)} {...restProps}>
  {@render children?.()}
</div>
