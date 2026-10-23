<script lang="ts" module>
  import { tv, type VariantProps } from 'tailwind-variants';

  export const badgeVariants = tv({
    base: 'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold whitespace-nowrap',
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground',
        secondary: 'bg-secondary text-secondary-foreground',
        accent: 'bg-accent text-accent-foreground',
        destructive: 'bg-destructive/15 text-destructive',
        outline: 'border border-input text-muted-foreground',
      },
    },
    defaultVariants: { variant: 'secondary' },
  });

  export type BadgeProps = VariantProps<typeof badgeVariants>;
</script>

<script lang="ts">
  import type { Snippet } from 'svelte';
  import { cn } from '#lib/utils.ts';

  let {
    variant,
    class: className,
    children,
  }: {
    variant?: BadgeProps['variant'];
    class?: string;
    children?: Snippet;
  } = $props();
</script>

<span class={cn(badgeVariants({ variant }), className)}>
  {@render children?.()}
</span>
