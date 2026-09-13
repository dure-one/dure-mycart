<script lang="ts">
  import SvgIcon from '../SvgIcon.svelte'
  import { apiPost } from '$lib/utils/api'

  interface Props {
    /**
     * Endpoint to post to. Takes precedence over the product endpoint built
     * from `productId` and `section`, so a page that uploads something other
     * than a product file — the shop's own logo, say — can use this component
     * instead of growing a second copy of the drag-and-drop shell.
     */
    url?: string
    section?: string
    accept?: string
    productId?: string
    /** A mark is one file, so those callers turn the multi-select off. */
    multiple?: boolean
    onadded?: (res: any) => void
  }

  let {
    url = undefined,
    section = undefined,
    accept = undefined,
    productId = undefined,
    multiple = true,
    onadded
  }: Props = $props()

  // Unique per instance: two uploads on one page would otherwise share an id,
  // and the second label would point at the first input.
  const fieldId = $props.id()
  const endpoint = $derived(url ?? `/api/_/products/${productId}/${section}`)

  let fileInput: HTMLInputElement | undefined = $state()
  let isDragging = $state(false)

  const onChange = async () => {
    if (!fileInput?.files) return

    for (const file of fileInput.files) {
      const formData = new FormData()
      formData.append('document', file)
      const res = await apiPost(endpoint, formData)
      onadded?.(res)
    }
  }

  const dragover = (event: DragEvent) => {
    event.preventDefault()
    isDragging = true
  }

  const dragleave = (event: DragEvent) => {
    event.preventDefault()
    isDragging = false
  }

  const drop = (event: DragEvent) => {
    event.preventDefault()
    if (fileInput && event.dataTransfer?.files) {
      fileInput.files = event.dataTransfer.files
      onChange()
    }
    isDragging = false
  }

</script>

<div
  class="upload bg-gray-200 focus-within:ring-2 focus-within:ring-green-600 {isDragging ? 'bg-green-300' : ''}"
  role="presentation"
  ondragover={dragover}
  ondragleave={dragleave}
  ondrop={drop}
>
  <input
    type="file"
    {multiple}
    name="fields[assetsFieldHandle][]"
    id={fieldId}
    onchange={onChange}
    bind:this={fileInput}
    {accept}
  />
  <label for={fieldId}>
    <SvgIcon name="plus" className="h-5 w-5" stroke="currentColor" />
  </label>
</div>

<style>
  @reference "tailwindcss";

  :global(.upload) {
    @apply grid h-16 cursor-pointer place-content-center rounded-lg;
  }

  :global(.upload input) {
    @apply absolute h-px w-px overflow-hidden opacity-0;
  }

  :global(.upload label) {
    @apply block cursor-pointer border-0 p-0 shadow-none;
  }
</style>
