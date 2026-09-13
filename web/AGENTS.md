# web/AGENTS.md

Two independent SvelteKit apps compiled to static bundles and embedded
into the Go binary via `web/embed.go`.

| App | URL base | Audience |
|-----|----------|----------|
| `admin/` | `/_/` | Store operator |
| `site/` | `/` | Customer-facing storefront |

Both apps ship separately (no monorepo package sharing) to keep bundle
sizes minimal. Shared utilities are **copied**, not imported — see
`src/lib/utils/sanitize.ts` mirrored in both.

## Conventions

### Runes-only Svelte 5

- `$state`, `$derived`, `$effect`, `$props`, `$bindable`.
- No `export let`, no legacy `$:` reactive blocks.
- Child content: `{@render children?.()}`, not `<slot />`.

### Security

- **All `{@html}` of user-authored content MUST be wrapped in
  `sanitizeHTML(...)` from `$lib/utils/sanitize.ts`.** The helper uses
  `isomorphic-dompurify` and works in SSR too.
- SVG sprite (`SvgSprite.svelte`) is the only trusted `{@html}` source —
  it renders a compile-time generated string baked into the bundle.

### Effects & timers

- Every `setTimeout` set inside an effect or event handler must be
  tracked and cleared in `onDestroy`. The canonical pattern is in
  `web/admin/src/lib/components/Drawer.svelte`.

### Data flow

- All API calls go through `$lib/utils/api.ts` helpers (`apiGet`,
  `apiPost`, …), which handle JSON parsing and unified error shapes.
- Do not `console.error` in committed code — set a user-visible error
  state instead.

### Styling

- TailwindCSS v4. The entry is `admin/src/assets/app.css`, which imports
  `main.css` (typography, form shells, links, notices) and `table.css`
  (tables). Shared appearance lives in those classes, not in repeated
  utility strings.
- No inline CSS-in-JS. Prefer utility classes; use `<style>` blocks for
  one-off animations that Tailwind cannot express.
- **Working in `admin/`? Read [`admin/DESIGN.md`](admin/DESIGN.md) first.**
  It is the normative contract for the panel's single accent, typography
  scale, page shell, component inventory and the frozen Playwright
  selectors. A page or element that departs from it is a bug, not a
  variation.

### Build

```bash
cd web/admin && bun install && bun run build
cd web/site  && bun install && bun run build
```

Both produce static bundles into `build/` that `web/embed.go` ships
inside the Go binary.
