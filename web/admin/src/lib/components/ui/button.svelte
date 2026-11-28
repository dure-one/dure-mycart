<script lang="ts">
  import { Button as ButtonPrimitive } from 'bits-ui'
  import { type VariantProps, tv } from 'tailwind-variants'
  import { cn } from '$lib/utils/cn.js'
  import type { Snippet } from 'svelte'

  const buttonVariants = tv({
    base: 'inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
    variants: {
      variant: {
        default: 'bg-zinc-900 text-zinc-50 hover:bg-zinc-900/90',
        destructive: 'bg-red-500 text-zinc-50 hover:bg-red-500/90',
        outline: 'border border-zinc-200 bg-white hover:bg-zinc-100 hover:text-zinc-900',
        secondary: 'bg-zinc-100 text-zinc-900 hover:bg-zinc-100/80',
        ghost: 'hover:bg-zinc-100 hover:text-zinc-900',
        link: 'text-zinc-900 underline-offset-4 hover:underline'
      },
      size: {
        default: 'h-10 px-4 py-2',
        sm: 'h-9 rounded-md px-3',
        lg: 'h-11 rounded-md px-8',
        icon: 'h-10 w-10'
      }
    },
    defaultVariants: {
      variant: 'default',
      size: 'default'
    }
  })

  type Variant = VariantProps<typeof buttonVariants>['variant']
  type Size = VariantProps<typeof buttonVariants>['size']

  let {
    class: className,
    variant = 'default',
    size = 'default',
    children,
    ...restProps
  }: ButtonPrimitive.Props & {
    variant?: Variant
    size?: Size
    children?: Snippet
  } = $props()
</script>

<ButtonPrimitive.Root
  class={cn(buttonVariants({ variant, size, className }))}
  type="button"
  {...restProps}
>
  {@render children?.()}
</ButtonPrimitive.Root>
