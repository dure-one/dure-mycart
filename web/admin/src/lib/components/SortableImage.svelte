<script lang="ts">
  import SvgIcon from './SvgIcon.svelte'

  interface ImageItem {
    id: string
    name: string
    ext: string
    position?: number
  }

  interface Props {
    images: ImageItem[]
    onReorder: (newOrder: ImageItem[]) => void
    onDelete: (index: number) => void
  }

  let { images, onReorder, onDelete }: Props = $props()

  let items = $state<ImageItem[]>([...images])
  let draggedIndex = $state<number | null>(null)
  let dragOverIndex = $state<number | null>(null)

  // Sync items when images prop changes
  $effect(() => {
    items = [...images]
  })

  function handleDragStart(event: DragEvent, index: number) {
    draggedIndex = index
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('text/html', String(index))
    }
  }

  function handleDragOver(event: DragEvent, index: number) {
    event.preventDefault()
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move'
    }
    dragOverIndex = index
  }

  function handleDragLeave() {
    dragOverIndex = null
  }

  function handleDrop(event: DragEvent, dropIndex: number) {
    event.preventDefault()

    if (draggedIndex === null || draggedIndex === dropIndex) {
      draggedIndex = null
      dragOverIndex = null
      return
    }

    const newItems = [...items]
    const [draggedItem] = newItems.splice(draggedIndex, 1)
    newItems.splice(dropIndex, 0, draggedItem)

    items = newItems
    onReorder(newItems)

    draggedIndex = null
    dragOverIndex = null
  }

  function handleDragEnd() {
    draggedIndex = null
    dragOverIndex = null
  }

  function handleDelete(index: number) {
    onDelete(index)
  }
</script>

<div class="flex flex-wrap gap-3">
  {#each items as image, index (image.id)}
    <div
      draggable="true"
      ondragstart={(e) => handleDragStart(e, index)}
      ondragover={(e) => handleDragOver(e, index)}
      ondragleave={handleDragLeave}
      ondrop={(e) => handleDrop(e, index)}
      ondragend={handleDragEnd}
      class="group relative rounded-lg border-2 border-gray-200 bg-white p-2 transition-all cursor-grab active:cursor-grabbing {draggedIndex === index
        ? 'opacity-50'
        : ''} {dragOverIndex === index ? 'border-blue-500' : ''}"
    >
      <!-- Image Preview -->
      <div class="relative">
        <img
          src="/uploads/{image.name}_sm.{image.ext}"
          alt="Product image {index + 1}"
          class="h-24 w-24 rounded object-cover"
        />
        {#if index === 0}
          <div
            class="absolute -top-1 -right-1 rounded-full bg-blue-500 px-2 py-0.5 text-xs font-medium text-white shadow-sm"
          >
            Rep
          </div>
        {/if}

        <!-- Delete Button (shows on hover) -->
        <button
          type="button"
          onclick={() => handleDelete(index)}
          class="absolute -top-2 -left-2 rounded-full bg-red-600 p-1.5 text-white opacity-0 group-hover:opacity-100 transition-opacity shadow-md hover:bg-red-700"
          aria-label="Delete image"
        >
          <SvgIcon name="trash" className="h-3 w-3" stroke="currentColor" />
        </button>
      </div>
    </div>
  {/each}
</div>
