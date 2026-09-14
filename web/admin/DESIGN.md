# Admin design system

The admin panel is one product, so it renders as one product. This file is the
contract every page and component is held to; `web/AGENTS.md` points here.

It exists because the panel grew four page-header patterns, three copies of the
same chip row, hand-rolled toggles, and debug leftovers. Anything below is
therefore **normative**: a page that diverges is a bug, not a variation.

---

## 1. The one accent: green

Green is the panel's only accent. It marks actions, states and focus.

| Role | Class |
|------|-------|
| Primary action (Save, Add, Install, Sign in) | `bg-green-600 hover:bg-green-700 text-white` |
| Primary action, hovered/active shade | `bg-green-500` |
| Focus ring on inputs and interactive shells | `border-green-600 ring-1 ring-green-600` |
| Toggle on, active chip, active nav item | `bg-green-200 text-green-900` (chip) / `bg-green-500` (toggle) |
| Success text (paid, free price) | `text-green-600` |
| Destructive action | `text-red-700` inline, `bg-red-600` filled |

Blue is **not** an accent, in any shade: `blue-50` through `blue-900` appear
nowhere in `src/`. The sidebar active state, the logo, every focus ring and
every in-content link move to green. Red stays for danger, yellow for
warnings, gray for neutrals — those are semantics, not accents.

| Role | Class |
|------|-------|
| In-content link (`.a-link`, or `ActionLink variant="default"`) | `text-green-700 hover:text-green-900` |
| Table-cell / description-list value link | `class="a-link"` |

## 2. Typography

`assets/main.css` owns the scale. Pages must not re-style headings.

| Element | Rendered | Used for |
|---------|----------|----------|
| `h1` | `text-2xl font-bold text-gray-900 sm:text-3xl` | page title, drawer title |
| `h2` | `text-lg font-bold text-gray-900 sm:text-xl` | section heading |
| `h3` | `text-base font-semibold text-gray-900` | sub-section / group label |
| body | `text-sm text-gray-800` | everything else |
| muted | `text-gray-500` | hints, empty states |

So `<h2 class="mb-4 text-xl font-bold">`, `<h3 class="text-sm font-medium
text-gray-700">` and `<h3 class="mb-3 text-lg font-semibold">` are all
violations — delete the utility classes and let `main.css` do it.

## 3. Page shell

Every page inside `Main` is exactly this shape:

```svelte
<Main>
  <PageHeader title={t('products.title')}>
    {#snippet actions()}
      <FormButton variant="secondary" ico="arrow-path" name={t('products.csv.import')} onclick={openCsv} />
      <FormButton variant="primary" ico="plus" name={t('products.addProduct')} onclick={openAdd} />
    {/snippet}
  </PageHeader>

  {#if loading}
    <PageState kind="loading" />
  {:else if items.length === 0}
    <PageState kind="empty" message={t('products.noProducts')} />
  {:else}
    <div class="table-wrap">
      <table>…</table>
    </div>
    <Pagination … />
  {/if}
</Main>
```

* The `h1` lives in `PageHeader` and is **always rendered** — the e2e suite waits
  for `h1` on products, carts and pages, and a loading state must not hide it.
* Loading, empty and error states are `PageState`. The literal
  `<div class="py-8 text-center">` is gone. No debug text in a state message.
* Sections within a page use `Section` (`<hr class="mt-5" />` + heading +
  spacing), never a hand-rolled `<hr>` + `<div class="mt-5">` pair.
* **Page padding belongs to the shell.** `Main`'s content container carries
  `px-5 pt-5 pb-5`; a page never adds its own outer padding or margin. The
  bottom half of that is what keeps the last element — a chip row, a Save
  button — off the sticky footer bar, so a page that ends flush against the
  footer is a shell bug, not a page bug.
* **Every page that fetches on mount gates its body on `loading`**, whatever it
  fetched: settings pages included, not just the list pages. The `PageHeader`
  stays above the gate (so `h1` is present from first paint); everything the
  fetch fills in sits inside `{:else}`.

Sign-in, install and the error page are the three exceptions: they render inside
`Blank` and use `<div class="content-center"><div class="header">…</div></div>`
— the CSS class, never a copy of its utility list.

## 4. Component inventory

Import **only** from the barrel: `import { PageHeader, FormButton } from '$lib/components'`.

| Component | Replaces | Contract |
|-----------|----------|----------|
| `PageHeader` | 4 header patterns | `title`, snippet `actions` |
| `PageState` | `py-8 text-center` blocks | `kind: 'loading'\|'empty'\|'error'`, `message?` |
| `Section` | `<hr>` + `<h2>` + spacing | `title`, children |
| `ChipGroup` / `Chip` | language, letters, providers, symbol rows | `Chip` renders a real `<button>`: `active`, `onclick`, children |
| `IconButton` | 19 `<SvgIcon role="button">` | `ico`, `label` (required → `aria-label` and tooltip), `variant: 'default'\|'danger'`, `active?: boolean` (a toggle that stays on: chip's `bg-green-200 text-green-900` plus `aria-pressed`), `disabled`, `onclick` |
| `ActionLink` | `cursor-pointer text-red-700` spans | real `<button>`, `variant: 'default'\|'danger'`, `onclick` |
| `DrawerHeader` | drawer title markup | `title`, snippet `actions` |
| `DrawerFooter` | drawer action row | `onclose`, `submitLabel`, `ondelete?`, `deleteLabel?` |
| `FormGroup` | `<hr>` + `<p class="font-semibold">` | `label`, children |
| `Badge` | ad-hoc status colours | `variant: 'success'\|'warning'\|'danger'\|'neutral'`, `ico?`, children |

Form primitives stay `FormButton`, `FormInput`, `FormSelect`, `FormTextarea`,
`FormToggle`, `FormUpload`. `form/Checkbox.svelte` is deleted — `FormToggle`
covers the boolean input and nothing imported the checkbox.

`FormButton` has one variant axis:

* `variant: 'primary' | 'secondary' | 'danger'` (default `primary`).
* The legacy `color` prop (`color="green"`, `color="gray"`, …) is **removed**;
  `color="green"` → `variant="primary"`, `color="gray"` → `variant="secondary"`,
  `color="red"` → `variant="danger"`.

## 5. Elements

**Tables.** The global element styles in `assets/table.css` are the only table
styling. Never put utility classes on `<table>`, `<tr>`, `<th>` or `<td>` that
the global rules already set — the row's cursor, hover and active shades come
from there, and so does its minimum height: a row is at least as tall as one
carrying an icon-only control, so a table whose rows hold nothing but text and
a badge keeps the rhythm of one with a column of buttons. Wrap every table in
`<div class="table-wrap">`. A table inside a drawer that must not offer row
hover/cursor uses `<table class="table-plain">`.

**Chips.** One look: `rounded p-2`, `bg-green-200 text-green-900` when active,
`bg-gray-200 text-gray-700 hover:bg-gray-300` when not. Spacing comes from
`ChipGroup`'s `gap`, never from `ml-5` or `index > 0` arithmetic.

A chip group that **owns its container** — the two choices in a settings
drawer, where there is no list around it — fills that container instead of
hugging its left edge: add `flex-1 py-3 text-center` to each chip so the
options become equal halves, and let the group take the width. A chip group
that is a *list of items* (the letters of `Mail letters`, the payment
providers) keeps its natural width.

**Icon-only actions.** Always `IconButton`, never a bare `SvgIcon` with
`role="button"` — icon-only controls are inaccessible without a label, and the
label doubles as the tooltip.

**Forms.** One submit shape:

```svelte
<form onsubmit={(e) => { e.preventDefault(); handleSubmit() }}>
```

Never `onsubmit={handleSubmit}` with a `preventDefault()` inside, and never a
click handler on a submit button. Field errors render as
`<span class="error text-red-500">{error}</span>`.

**Drawers.** Width is `710px` for form/detail drawers and `725px` for
letter/provider drawers. Title is `DrawerHeader`, actions are `DrawerFooter`.

**Setting rows.** A heading and its description with the control that
belongs to them — a toggle, a button — are one row:
`flex items-center justify-between gap-4`, text block on the left in a plain
`<div>`, control on the right. `items-center` alone is not enough: the
control then follows the text instead of sitting at the far edge, and the
right half of the drawer goes empty. `FormToggle` is a bare control with its
own `<label>`, not a row — it never belongs in a wrapper band of its own.

**Links.** A link inside content is green, via the `.a-link` class in
`main.css` (the anchor counterpart of `ActionLink`), never an ad-hoc
`text-blue-600 hover:underline`. Image and thumbnail anchors keep their own
wrapper classes and are the exception.

**Notices.** One shape, three tones, all from `main.css`:

| Tone | Class | Use |
|------|-------|-----|
| Neutral | `notice notice-info` | information the reader may ignore |
| Attention | `notice notice-warning` | something to check before saving |
| Failure | `notice notice-danger` | the section cannot work as configured |

The tone carries meaning — never recolour a notice for looks alone, and never
reintroduce a one-off `rounded border …-50 …-200` banner.

## 6. Frozen selectors

The Playwright suite in `e2e/` drives this UI. These must survive any refactor:

| Selector | Owner |
|----------|-------|
| `[data-testid="product-row"]` | products table row |
| `#drawer_content` | `Drawer` content panel |
| `button.bg-green-600` containing "Add Product" | products primary action |
| `getByRole('textbox', { name: 'Name' \| 'URL' \| 'Brief' \| 'Amount' })` | label text of `FormInput` stays the accessible name |
| `getByRole('combobox', { name: 'Digital type' })` | `FormSelect` accessible name |
| `#email`, `#password`, `#domain`, `#database-driver`, `#pg-*`, `#database-dsn` | install field ids |
| `label[for="toggle_database-dsn-mode"]` | `FormToggle` keeps a `<label for>` |
| `fieldset span.text-red-500` | install keeps the literal `text-red-500` class on that error |
| `form button[type="submit"]` | submit button inside the form |
| `h1` visible | `PageHeader` always renders the title |

## 7. No leftovers

* No `console.log` / `console.error` / debug markers (`[FIX v2.0]`, `[PAGE …]`,
  `=== PRODUCT SAVE DEBUG ===`) in committed code.
* No user-facing English string in a component. All copy goes through
  `t('…')`, with the key added to **all three** of `locales/en.json`,
  `locales/ko.json`, `locales/zh.json`.
* No `blue-*` class anywhere (see §1). The audit greps are
  `grep -rn 'blue-' src/ --include='*.svelte' --include='*.css'` → empty and
  `grep -rn 'class="rounded border' src/ --include='*.svelte'` → empty.
* No global element selector that fights a page. `main.css` styles `table`,
  `h1`–`h3`, `img` and inputs; it must not style bare `header`, `label` or
  `div`.
