# Feedoku Style Guide

A portable design system: **warm paper canvas, indigo ink, lavender accent, hairline translucent borders, glass panels, fully rounded controls, heavy tight headlines.** Built on Tailwind CSS v4 + shadcn/ui (new-york). Palette adapted from the YOU/ME design system (you.brainfogagency.com).

Everything below is copy-pasteable. To reuse in another project, follow [Quick start](#9-quick-start-for-a-new-project).

---

## 1. Principles

1. **Calm, reading-first.** Lots of whitespace, one accent colour, no decoration for its own sake.
2. **Soft surfaces, hairline edges.** Panels are translucent "glass" with a 1px low-contrast border and a large, soft shadow. Never hard black borders.
3. **Round everything interactive.** Buttons and inputs are pills (`rounded-full`); cards are `rounded-3xl`; big feature blocks `rounded-[2rem]`.
4. **Heavy headlines, quiet body.** Extra-bold, tightly tracked display type; regular muted body text.
5. **One accent.** Indigo-lavender (`--brand`) is used sparingly: eyebrow labels, key words in headlines, unread dots.
6. **Honest UI.** Features that don't exist yet are labelled "Coming soon", never implied.

---

## 2. Colour

All colours are CSS variables holding **full colour values** (hex/rgba, not HSL channels), mapped into Tailwind through `@theme`. Semantic names follow shadcn so shadcn components work unchanged.

| Token | Light | Dark | Use |
|---|---|---|---|
| `background` | `#f6f3ee` | `#0d0e11` | Page canvas (warm paper / near-black ink) |
| `foreground` | `#15171c` | `#f4f2ed` | Body text |
| `card` / `popover` | `#ffffff` | `#1a1b20` | Surfaces (used at 60–80% opacity for glass) |
| `primary` | `#2e2a4a` | `#dad5ee` | Primary buttons, logo mark, dark feature blocks |
| `primary-foreground` | `#ffffff` | `#15171c` | Text on primary |
| `secondary` / `muted` | `rgba(21,23,28,.05)` | `rgba(255,255,255,.05–.06)` | Quiet fills |
| `muted-foreground` | `#5a5e66` | `#a7a49c` | Secondary text |
| `accent` | `rgba(125,116,176,.16)` | `rgba(125,116,176,.26)` | Hover/selected wash (lavender) |
| `accent-foreground` | `#2e2a4a` | `#ece9f6` | Text on accent |
| `brand` | `#5b52a0` | `#c9c2ec` | The single accent colour (text, dots, icons) |
| `border` | `rgba(21,23,28,.12)` | `rgba(255,255,255,.12)` | Hairlines |
| `input` | `rgba(21,23,28,.16)` | `rgba(255,255,255,.16)` | Input borders |
| `ring` | `#5b52a0` | `#c9c2ec` | Focus ring |
| `destructive` | `#a11f16` | `#ffa69b` | Errors (`destructive-foreground`: `#fff` / `#15171c`) |

Status colours from the source system, if you need them: ok `#12602f`/`#8be0ae`, warn `#7a4600`/`#f6c877`, bad `#a11f16`/`#ffa69b`, info `#23408e`/`#b4c6ff` (light / dark).

**Rules**
- Text on `background`: `foreground` or `muted-foreground` only.
- Never use pure black/white or Tailwind greys (`gray-*`, `zinc-*`); use tokens.
- Use opacity modifiers for glass and washes (`bg-card/70`, `bg-accent/60`), not new colours.
- Dark mode is a first-class theme (class strategy), not an inversion: `primary` flips to light lavender.

---

## 3. Typography

| Role | Font | Notes |
|---|---|---|
| UI + headings | **Sora** (variable, 100–800) | via `next/font/google`, variable `--font-sora` |
| Code | **Geist Mono** | variable `--font-geist-mono` |
| Article body | Georgia / system serif | `.article-body` only (see §8) |

**Scale & weights**

| Element | Classes |
|---|---|
| Display / hero H1 | `text-5xl sm:text-6xl lg:text-7xl font-extrabold tracking-tighter leading-[1.02] text-balance` |
| Section H2 | `text-4xl sm:text-5xl font-extrabold tracking-tighter text-balance` |
| Card title | `text-xl font-bold tracking-tight` |
| Lead paragraph | `text-lg leading-relaxed text-muted-foreground text-pretty` |
| Body | `text-base leading-relaxed text-muted-foreground` (or `foreground` for primary reading) |
| Eyebrow | `text-sm font-semibold text-brand` |
| Caption / meta | `text-xs text-muted-foreground` |
| Wordmark | `text-[1.4rem] font-extrabold tracking-tighter` |

Headlines are always **extra-bold + `tracking-tighter`**. Body is never bold except for emphasis. Use `text-balance` on headings, `text-pretty` on paragraphs.

---

## 4. Shape, spacing, depth

**Radius** (base `--radius: 0.75rem`)

| Thing | Radius |
|---|---|
| Buttons, inputs, nav bar, badges, avatars | `rounded-full` |
| Cards, panels, FAQ items, dialogs | `rounded-3xl` (panels in dense app UI: `rounded-xl`) |
| Large feature / CTA blocks | `rounded-[2rem]` |
| Logo mark | `rounded-xl` |

**Depth**
- `--shadow-panel`: light `0 12px 40px rgba(40,36,60,.10)`, dark `0 12px 40px rgba(0,0,0,.45)`. Use as `shadow-[var(--shadow-panel)]`. Apply to floating surfaces (nav, hero preview, CTA, open FAQ), not to every card.
- Resting cards use only a hairline border; they gain `shadow-[var(--shadow-panel)]` on hover.

**Layout**
- Content width: `max-w-5xl` (landing), `max-w-3xl` (text-heavy), `max-w-2xl` for reading.
- Section rhythm: `px-4 py-24`. Hero: `pt-36 sm:pt-44 pb-16`.
- Gaps: grid `gap-4`, card padding `p-7`, stacks `space-y-3`.
- Mobile first; 16px side gutter minimum; no horizontal scroll.

---

## 5. Glass panel recipe

The signature surface. Always on top of the `.ambient` background.

```html
<div class="rounded-3xl border bg-card/70 backdrop-blur-xl shadow-[var(--shadow-panel)]">…</div>
```

- Border comes from the global `* { @apply border-border }` rule.
- Wrap the page (or app shell) in `class="ambient"` to get the lavender blobs behind the glass.
- Floating pill nav: `fixed inset-x-0 top-0 px-3 pt-3` → inner `mx-auto h-14 max-w-5xl rounded-full border bg-card/70 backdrop-blur-xl shadow-[var(--shadow-panel)]`.

---

## 6. Components

### Buttons (all fully rounded)
Defined once via `cva`; **use `asChild` + `Link`/`a` for navigation**.

| Variant | Look | Use |
|---|---|---|
| `default` | indigo `primary` fill | Primary action (one per view) |
| `outline` | hairline border, `bg-card/60` | Secondary action next to a primary |
| `secondary` | quiet fill | Tertiary |
| `ghost` | transparent, `accent` on hover | Toolbars, nav items, icon buttons |
| `link` | underlined text | Inline |
| `destructive` | red fill | Irreversible actions |

Sizes: `sm` h-8 · `default` h-10 · `lg` h-12 (hero CTAs) · `icon` 40×40 circle. Font weight `semibold`. Focus: 1px `ring`. Disabled: 50% opacity.

Pair CTAs as `Get started free →` (`lg`, default variant, trailing arrow icon) + `See how it works` (`lg`, outline).

### Inputs
`h-10 rounded-full border border-input bg-transparent px-4`, focus ring 1px `ring`. Labels sit above, `text-sm font-medium`.

### Badges
Pill, tiny. `default` = **Available/Live**, `secondary` = **Coming soon**. Add `rounded-full px-2.5`.

### Cards
`rounded-3xl border bg-card/70 p-7 backdrop-blur hover:shadow-[var(--shadow-panel)]`. Icon sits in a `size-11 rounded-2xl bg-accent text-accent-foreground` tile at top-left; badge top-right; title `mt-6`.

### Dark feature block
`rounded-[2rem] bg-primary p-8 text-primary-foreground sm:p-14`; muted copy via `opacity-70/80`, big step numbers `opacity-30`.

### Lists / selected rows
Selected: `bg-accent`. Hover: `hover:bg-accent/50`. Unread indicator: `size-1.5 rounded-full bg-brand`.

### FAQ
Native `<details>` in a `rounded-3xl border bg-card/70 px-6 py-5` card, chevron rotates with `group-open:rotate-180`.

### Tooltips for shortcuts
`<kbd class="rounded border bg-muted px-1.5 py-0.5 font-mono text-[11px]">`.

---

## 7. Logo

A heavy RSS-signal glyph (dot + two thick arcs) in a rounded-square tile, plus an extra-bold wordmark with a brand-coloured full stop.

```tsx
// components/logo.tsx
import { cn } from '@/lib/utils';

export function LogoGlyph({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" fill="none" aria-hidden className={className} strokeLinecap="round">
      <circle cx="9.5" cy="22.5" r="3.6" fill="currentColor" />
      <path d="M6.5 12.2a13.3 13.3 0 0 1 13.3 13.3" stroke="currentColor" strokeWidth="4.6" />
      <path d="M6.5 4.6A20.9 20.9 0 0 1 27.4 25.5" stroke="currentColor" strokeWidth="4.6" />
    </svg>
  );
}

export function Logo({ className, tone = 'default', markOnly }:
  { className?: string; tone?: 'default' | 'inverted'; markOnly?: boolean }) {
  const inverted = tone === 'inverted'; // use on a bg-primary surface
  return (
    <span className={cn('inline-flex items-center gap-2.5', className)}>
      <span className={cn('grid size-9 shrink-0 place-items-center rounded-xl shadow-sm',
        inverted ? 'bg-primary-foreground text-primary' : 'bg-primary text-primary-foreground')}>
        <LogoGlyph className="size-6" />
      </span>
      {!markOnly && (
        <span className="text-[1.4rem] leading-none font-extrabold tracking-tighter">
          Feedoku<span className={inverted ? 'opacity-60' : 'text-brand'}>.</span>
        </span>
      )}
    </span>
  );
}
```

Favicon / app icon (`app/icon.svg`, Next picks it up automatically):

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" fill="none">
  <rect width="32" height="32" rx="9" fill="#2e2a4a"/>
  <g transform="translate(3.2 3.2) scale(.8)" stroke-linecap="round">
    <circle cx="9.5" cy="22.5" r="3.6" fill="#fff"/>
    <path d="M6.5 12.2a13.3 13.3 0 0 1 13.3 13.3" stroke="#fff" stroke-width="4.6"/>
    <path d="M6.5 4.6A20.9 20.9 0 0 1 27.4 25.5" stroke="#c9c2ec" stroke-width="4.6"/>
  </g>
</svg>
```

Clear space: at least the height of the dot on all sides. Minimum mark size 20px. To rebrand in another project, swap the glyph paths and the wordmark text; keep tile, weight and tracking.

---

## 8. CSS to copy

### `globals.css` (tokens)

```css
@import 'tailwindcss';
@import 'tw-animate-css';

@custom-variant dark (&:is(.dark *));

@theme {
  --font-sans: var(--font-sora), ui-sans-serif, system-ui, sans-serif;
  --font-mono: var(--font-geist-mono), ui-monospace, SFMono-Regular, Menlo, monospace;

  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --color-brand: var(--brand);
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--chart-2);
  --color-chart-3: var(--chart-3);
  --color-chart-4: var(--chart-4);
  --color-chart-5: var(--chart-5);

  --radius-lg: var(--radius);
  --radius-md: calc(var(--radius) - 2px);
  --radius-xl: calc(var(--radius) + 4px);
  --radius-sm: calc(var(--radius) - 4px);
}

@layer base {
  :root {
    color-scheme: light;
    --background: #f6f3ee;
    --foreground: #15171c;
    --card: #ffffff;
    --card-foreground: #15171c;
    --popover: #ffffff;
    --popover-foreground: #15171c;
    --primary: #2e2a4a;
    --primary-foreground: #ffffff;
    --secondary: rgba(21, 23, 28, 0.05);
    --secondary-foreground: #2e2a4a;
    --muted: rgba(21, 23, 28, 0.05);
    --muted-foreground: #5a5e66;
    --accent: rgba(125, 116, 176, 0.16);
    --accent-foreground: #2e2a4a;
    --destructive: #a11f16;
    --destructive-foreground: #ffffff;
    --border: rgba(21, 23, 28, 0.12);
    --input: rgba(21, 23, 28, 0.16);
    --ring: #5b52a0;
    --brand: #5b52a0;
    --blob-1: #ece9f6;
    --blob-2: rgba(218, 213, 238, 0.55);
    --shadow-panel: 0 12px 40px rgba(40, 36, 60, 0.1);
    --radius: 0.75rem;
    --chart-1: #5b52a0; --chart-2: #15795a; --chart-3: #ff5a1f; --chart-4: #1e4fa3; --chart-5: #a02457;
  }

  .dark {
    color-scheme: dark;
    --background: #0d0e11;
    --foreground: #f4f2ed;
    --card: #1a1b20;
    --card-foreground: #f4f2ed;
    --popover: #1a1b20;
    --popover-foreground: #f4f2ed;
    --primary: #dad5ee;
    --primary-foreground: #15171c;
    --secondary: rgba(255, 255, 255, 0.06);
    --secondary-foreground: #ece9f6;
    --muted: rgba(255, 255, 255, 0.05);
    --muted-foreground: #a7a49c;
    --accent: rgba(125, 116, 176, 0.26);
    --accent-foreground: #ece9f6;
    --destructive: #ffa69b;
    --destructive-foreground: #15171c;
    --border: rgba(255, 255, 255, 0.12);
    --input: rgba(255, 255, 255, 0.16);
    --ring: #c9c2ec;
    --brand: #c9c2ec;
    --blob-1: rgba(125, 116, 176, 0.26);
    --blob-2: rgba(218, 213, 238, 0.12);
    --shadow-panel: 0 12px 40px rgba(0, 0, 0, 0.45);
    --chart-1: #c9c2ec; --chart-2: #3dd994; --chart-3: #ffc2aa; --chart-4: #b5caee; --chart-5: #ffe1ed;
  }
}

@layer base {
  * { @apply border-border; }
  body { @apply bg-background text-foreground; }
}

@layer components {
  /* Soft lavender blobs behind glass panels. Add class="ambient" to a page wrapper. */
  .ambient::before {
    content: '';
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    background:
      radial-gradient(40rem 32rem at 8% -6%, var(--blob-1), transparent 70%),
      radial-gradient(34rem 28rem at 100% 100%, var(--blob-2), transparent 70%);
  }
}
```

### Long-form reading (`.article-body`)

For sanitised HTML (articles, docs, blog posts). Serif body, sans headings.

```css
@layer components {
  .article-body { font-family: ui-serif, Georgia, Cambria, 'Times New Roman', serif; font-size: 1.0625rem; line-height: 1.75; overflow-wrap: anywhere; }
  .article-body > * + * { margin-top: 1.1em; }
  .article-body h1, .article-body h2, .article-body h3, .article-body h4 { font-family: var(--font-sans); font-weight: 600; line-height: 1.3; margin-top: 1.8em; }
  .article-body h1 { font-size: 1.5em; } .article-body h2 { font-size: 1.3em; } .article-body h3 { font-size: 1.15em; }
  .article-body a { text-decoration: underline; text-underline-offset: 3px; }
  .article-body img, .article-body video { max-width: 100%; height: auto; border-radius: var(--radius-md); }
  .article-body ul { list-style: disc; padding-left: 1.5em; } .article-body ol { list-style: decimal; padding-left: 1.5em; }
  .article-body blockquote { border-left: 3px solid var(--color-border); padding-left: 1em; color: var(--color-muted-foreground); }
  .article-body pre { overflow-x: auto; border-radius: var(--radius-md); background: var(--color-muted); padding: 1em; font-family: var(--font-mono); font-size: .85em; line-height: 1.6; }
  .article-body code:not(pre code) { border-radius: 4px; background: var(--color-muted); padding: .1em .35em; font-family: var(--font-mono); font-size: .88em; }
  .article-body table { display: block; overflow-x: auto; }
  .article-body th, .article-body td { border: 1px solid var(--color-border); padding: .4em .7em; }
}
```

### Button (`components/ui/button.tsx` variants)

```ts
const buttonVariants = cva(
  'inline-flex cursor-pointer items-center justify-center gap-2 rounded-full text-sm font-semibold whitespace-nowrap transition-colors focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-hidden disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground shadow-sm hover:bg-primary/90',
        destructive: 'bg-destructive text-destructive-foreground shadow-xs hover:bg-destructive/90',
        outline: 'border border-input bg-card/60 shadow-xs hover:bg-accent hover:text-accent-foreground',
        secondary: 'bg-secondary text-secondary-foreground shadow-xs hover:bg-secondary/80',
        ghost: 'hover:bg-accent hover:text-accent-foreground',
        link: 'text-primary underline-offset-4 hover:underline',
      },
      size: { default: 'h-10 px-5 py-2', sm: 'h-8 px-4 text-xs', lg: 'h-12 px-8 text-base', icon: 'size-10' },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  }
);
```

---

## 9. Quick start for a new project

1. **Stack:** Next.js (or any Tailwind v4 app), `tailwindcss@4`, `tw-animate-css`, `class-variance-authority`, `clsx`, `tailwind-merge`, `lucide-react`.
2. **shadcn:** `bunx shadcn@latest init` (style `new-york`, base colour any, CSS variables **on**), then add the components you need (`button input badge dialog sheet tooltip skeleton scroll-area separator dropdown-menu`).
3. **Replace** the generated token blocks in `globals.css` with §8. Remove any `hsl(var(--x))` wrappers: tokens here hold full colours.
4. **Fonts** (`app/fonts.ts`):
   ```ts
   import { Sora } from 'next/font/google';
   export const sora = Sora({ subsets: ['latin'], variable: '--font-sora', display: 'swap' });
   ```
   Put `sora.variable` on `<html>` and use `font-sans antialiased` on `<body>`.
5. **Theme:** `next-themes` with `attribute="class"`, `defaultTheme="system"`, `enableSystem`.
6. **Buttons/inputs:** apply the §8 button variants and the pill input; delete any `rounded-md` overrides.
7. **Logo + icon:** drop in §7; adjust glyph/wordmark for the new product.
8. **Page shell:** wrap pages in `className="ambient"`, content in glass panels (§5).

## 10. Do / Don't

| Do | Don't |
|---|---|
| One primary button per view | Two filled buttons side by side |
| `rounded-full` controls, `rounded-3xl` cards | Mix `rounded-md` into this system |
| Tokens (`bg-card/70`, `text-muted-foreground`) | Hard-coded hex, `gray-*`, `black/white` |
| `font-extrabold tracking-tighter` headlines | Bold body text or loose-tracked headings |
| Accent (`text-brand`) on one phrase per section | Accent-coloured paragraphs or multiple accent hues |
| Hairline borders + soft large shadows | Dark borders, small harsh shadows |
| Label unfinished features "Coming soon" | Show mock features as live |
| Visible focus ring, `sr-only` labels on icon buttons | Icon-only buttons without `aria-label` |

## 11. Accessibility

- Body text contrast ≥ 4.5:1 on `background`/`card` in both themes (`muted-foreground` is the lowest-contrast text allowed for body copy; use `foreground` for anything essential).
- Every interactive element is keyboard reachable with a visible `ring`.
- Icon-only buttons need `aria-label`; decorative icons `aria-hidden`.
- Don't rely on colour alone: unread = dot **and** bold title; saved = icon **and** label.
- Respect `prefers-reduced-motion` (entrance animations come from `tw-animate-css` classes like `animate-in fade-in slide-in-from-bottom-4 duration-700`; keep them subtle and one-shot).

## 12. Source of truth in this repo

| What | File |
|---|---|
| Tokens, ambient, article typography | `www/src/app/globals.css` |
| Buttons / inputs | `www/src/components/ui/button.tsx`, `input.tsx` |
| Logo | `www/src/components/logo.tsx`, `www/src/app/icon.svg` |
| Landing sections (reference layouts) | `www/src/app/(landing)/_components/` |
| App shell (reference dense UI) | `www/src/app/app/` |
