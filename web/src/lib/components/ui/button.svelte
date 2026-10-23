<script lang="ts" module>
  import { tv, type VariantProps } from 'tailwind-variants';

  export const buttonVariants = tv({
    base: 'inline-flex shrink-0 items-center justify-center gap-2 rounded-full font-semibold whitespace-nowrap transition-colors focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50',
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        outline: 'border border-input bg-card/60 hover:bg-accent',
        secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/70',
        ghost: 'hover:bg-accent',
        destructive:
          'bg-destructive text-destructive-foreground hover:bg-destructive/90',
      },
      size: {
        sm: 'h-8 px-3.5 text-sm',
        default: 'h-10 px-5 text-sm',
        lg: 'h-12 px-7 text-base',
        icon: 'size-10',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  });

  export type ButtonProps = VariantProps<typeof buttonVariants>;
</script>

<script lang="ts">
  import type { Snippet } from 'svelte';
  import type {
    HTMLAnchorAttributes,
    HTMLButtonAttributes,
  } from 'svelte/elements';
  import { cn } from '#lib/utils.ts';

  type Props = HTMLButtonAttributes &
    Pick<HTMLAnchorAttributes, 'target' | 'rel'> & {
    variant?: ButtonProps['variant'];
    size?: ButtonProps['size'];
    /** Renders an anchor instead of a button. */
    href?: string;
    children?: Snippet;
  };

  let {
    variant,
    size,
    href,
    class: className,
    children,
    ...rest
  }: Props = $props();
</script>

{#if href}
  <a
    {href}
    class={cn(buttonVariants({ variant, size }), className)}
    {...rest as HTMLAnchorAttributes}
  >
    {@render children?.()}
  </a>
{:else}
  <button class={cn(buttonVariants({ variant, size }), className)} {...rest}>
    {@render children?.()}
  </button>
{/if}
