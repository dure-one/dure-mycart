<script lang="ts">
	import { useSortable } from '@dnd-kit/sortable'
	import { CSS } from '@dnd-kit/utilities'

	interface Image {
		id: string
		name: string
		ext: string
		orig_name: string
	}

	interface Props {
		image: Image
	}

	let { image }: Props = $props()

	const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
		id: image.id
	})

	const style = $derived(
		CSS.Transform.toString(transform) + (transition ? `; transition: ${transition}` : '')
	)
</script>

<div
	bind:this={setNodeRef}
	{style}
	class="relative group"
	class:opacity-50={isDragging}
	data-sortable-image
	data-image-id={image.id}
>
	<img
		src="/uploads/{image.name}_sm.{image.ext}"
		alt={image.orig_name}
		class="w-full h-auto rounded border border-gray-200"
	/>
	<div
		class="absolute top-2 right-2 cursor-grab active:cursor-grabbing bg-white/90 rounded p-1.5 shadow-sm opacity-0 group-hover:opacity-100 transition-opacity"
		{...attributes}
		{...listeners}
	>
		<!-- Simple drag handle using SVG -->
		<svg
			class="w-4 h-4 text-gray-600"
			fill="none"
			stroke="currentColor"
			viewBox="0 0 24 24"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				stroke-width="2"
				d="M4 8h16M4 16h16"
			/>
		</svg>
	</div>
</div>
