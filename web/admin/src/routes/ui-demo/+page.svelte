<script lang="ts">
  import Main from '$lib/layouts/Main.svelte'
  import PageHeader from '$lib/components/PageHeader.svelte'

  // Existing components
  import ExistingBadge from '$lib/components/Badge.svelte'
  import ExistingAlert from '$lib/components/Alert.svelte'

  // shadcn-svelte components
  import Button from '$lib/components/ui/button.svelte'
  import Badge from '$lib/components/ui/badge.svelte'
  import Card from '$lib/components/ui/card.svelte'
  import CardHeader from '$lib/components/ui/card-header.svelte'
  import CardTitle from '$lib/components/ui/card-title.svelte'
  import CardContent from '$lib/components/ui/card-content.svelte'
  import { Root as Dialog, Trigger as DialogTrigger } from '$lib/components/ui/dialog.svelte'
  import DialogHeaderComp from '$lib/components/ui/dialog-header.svelte'
  import DialogTitleComp from '$lib/components/ui/dialog-title.svelte'

  let dialogOpen = $state(false)
</script>

<Main>
  <PageHeader title="shadcn-svelte UI Components Demo" />

  <div class="space-y-12 p-6">
    <!-- Buttons Comparison -->
    <section>
      <h2 class="mb-6 text-2xl font-bold">Buttons</h2>

      <div class="grid gap-8 md:grid-cols-2">
        <!-- shadcn Buttons -->
        <div class="rounded-lg border border-zinc-200 bg-white p-6">
          <h3 class="mb-4 text-lg font-semibold text-zinc-900">shadcn-svelte Buttons</h3>
          <div class="flex flex-wrap gap-3">
            <Button variant="default">Default</Button>
            <Button variant="secondary">Secondary</Button>
            <Button variant="destructive">Destructive</Button>
            <Button variant="outline">Outline</Button>
            <Button variant="ghost">Ghost</Button>
            <Button variant="link">Link</Button>
          </div>
          <div class="mt-4 flex flex-wrap gap-3">
            <Button size="sm">Small</Button>
            <Button size="default">Default</Button>
            <Button size="lg">Large</Button>
          </div>
        </div>

        <!-- Existing Buttons -->
        <div class="rounded-lg border border-zinc-200 bg-white p-6">
          <h3 class="mb-4 text-lg font-semibold text-zinc-900">Existing Buttons (native)</h3>
          <div class="flex flex-wrap gap-3">
            <button class="rounded bg-blue-600 px-4 py-2 text-white hover:bg-blue-700">
              Primary
            </button>
            <button class="rounded border border-zinc-300 bg-white px-4 py-2 text-zinc-700 hover:bg-zinc-50">
              Secondary
            </button>
            <button class="rounded bg-red-600 px-4 py-2 text-white hover:bg-red-700">
              Danger
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- Badges Comparison -->
    <section>
      <h2 class="mb-6 text-2xl font-bold">Badges</h2>

      <div class="grid gap-8 md:grid-cols-2">
        <!-- shadcn Badges -->
        <div class="rounded-lg border border-zinc-200 bg-white p-6">
          <h3 class="mb-4 text-lg font-semibold text-zinc-900">shadcn-svelte Badges</h3>
          <div class="flex flex-wrap gap-3">
            <Badge variant="default">Default</Badge>
            <Badge variant="secondary">Secondary</Badge>
            <Badge variant="destructive">Destructive</Badge>
            <Badge variant="outline">Outline</Badge>
          </div>
        </div>

        <!-- Existing Badges -->
        <div class="rounded-lg border border-zinc-200 bg-white p-6">
          <h3 class="mb-4 text-lg font-semibold text-zinc-900">Existing Badges</h3>
          <div class="flex flex-wrap gap-3">
            <ExistingBadge color="blue">Blue</ExistingBadge>
            <ExistingBadge color="red">Red</ExistingBadge>
            <ExistingBadge color="green">Green</ExistingBadge>
            <ExistingBadge color="yellow">Yellow</ExistingBadge>
            <ExistingBadge color="gray">Gray</ExistingBadge>
          </div>
        </div>
      </div>
    </section>

    <!-- Cards Comparison -->
    <section>
      <h2 class="mb-6 text-2xl font-bold">Cards</h2>

      <div class="grid gap-8 md:grid-cols-2">
        <!-- shadcn Card -->
        <Card>
          <CardHeader>
            <CardTitle>shadcn-svelte Card</CardTitle>
          </CardHeader>
          <CardContent>
            <p class="text-sm text-zinc-600">
              This is a shadcn-svelte card component with pre-defined styling. It includes
              border, shadow, rounded corners, and padding.
            </p>
          </CardContent>
        </Card>

        <!-- Existing Card style -->
        <div class="rounded-lg border border-zinc-300 bg-white p-6 shadow-sm">
          <h3 class="mb-2 text-xl font-semibold">Existing Card Style</h3>
          <p class="text-sm text-zinc-600">
            This mimics the existing card pattern used in the admin panel. Manual
            Tailwind classes applied directly.
          </p>
        </div>
      </div>
    </section>

    <!-- Dialog/Modal -->
    <section>
      <h2 class="mb-6 text-2xl font-bold">Dialog (Modal)</h2>

      <div class="rounded-lg border border-zinc-200 bg-white p-6">
        <h3 class="mb-4 text-lg font-semibold text-zinc-900">shadcn-svelte Dialog</h3>
        <Dialog bind:open={dialogOpen}>
          <DialogTrigger>
            <Button>Open Dialog</Button>
          </DialogTrigger>
          <div>
            <DialogHeaderComp>
              <DialogTitleComp>Sample Dialog</DialogTitleComp>
            </DialogHeaderComp>
            <p class="text-sm text-zinc-600">
              This is a shadcn-svelte dialog component with overlay, animations, and
              accessibility features built-in using bits-ui primitives.
            </p>
            <div class="mt-4 flex justify-end">
              <Button variant="outline" onclick={() => (dialogOpen = false)}>Close</Button>
            </div>
          </div>
        </Dialog>
      </div>
    </section>

    <!-- Alert Comparison -->
    <section>
      <h2 class="mb-6 text-2xl font-bold">Alerts</h2>

      <div class="grid gap-8 md:grid-cols-2">
        <!-- Existing Alert -->
        <div>
          <h3 class="mb-4 text-lg font-semibold text-zinc-900">Existing Alert</h3>
          <ExistingAlert variant="info">
            This is the existing alert component from the codebase.
          </ExistingAlert>
        </div>

        <!-- shadcn Alternative (using Card) -->
        <div>
          <h3 class="mb-4 text-lg font-semibold text-zinc-900">shadcn Card Alternative</h3>
          <Card class="border-blue-200 bg-blue-50">
            <CardContent class="pt-6">
              <p class="text-sm text-blue-900">
                shadcn-svelte doesn't have an alert component by default, but cards can be styled
                similarly.
              </p>
            </CardContent>
          </Card>
        </div>
      </div>
    </section>

    <!-- Visual Assessment Notes -->
    <section class="rounded-lg border-2 border-dashed border-zinc-300 bg-zinc-50 p-6">
      <h2 class="mb-4 text-xl font-bold">Visual Assessment</h2>
      <div class="space-y-2 text-sm text-zinc-700">
        <p><strong>shadcn-svelte strengths:</strong></p>
        <ul class="ml-6 list-disc space-y-1">
          <li>Consistent design system with variants</li>
          <li>Built-in accessibility via bits-ui primitives</li>
          <li>Professional, modern aesthetic</li>
          <li>Type-safe component APIs</li>
        </ul>

        <p class="mt-4"><strong>Existing components:</strong></p>
        <ul class="ml-6 list-disc space-y-1">
          <li>More colorful and vibrant</li>
          <li>Custom styling per component</li>
          <li>Simpler, more direct implementation</li>
        </ul>

        <p class="mt-4"><strong>Integration notes:</strong></p>
        <ul class="ml-6 list-disc space-y-1">
          <li>shadcn uses zinc color palette vs existing blue/red/green accents</li>
          <li>shadcn is more subdued and professional</li>
          <li>Existing style is more vibrant and "startup-like"</li>
          <li>Both approaches work - depends on desired brand direction</li>
        </ul>
      </div>
    </section>
  </div>
</Main>
