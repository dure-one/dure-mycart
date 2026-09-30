<script lang="ts">
	import { render } from '@crafter/mermaid'

	interface Props {
		content: string
	}

	let { content }: Props = $props()

	function extractMermaidCode(markdown: string): string {
		const match = markdown.match(/```mermaid\n([\s\S]*?)```/)
		return match ? match[1].trim() : markdown.trim()
	}

	let mermaidCode = $derived(extractMermaidCode(content || ''))
	let svg = $state('')
	let error = $state<string | null>(null)

	$effect(() => {
		if (!mermaidCode) {
			svg = ''
			error = 'No diagram content'
			return
		}

		try {
			svg = render(mermaidCode)
			error = null
		} catch (err) {
			console.error('Failed to render mermaid diagram:', err)
			error = err instanceof Error ? err.message : 'Failed to render diagram'
			svg = ''
		}
	})
</script>

<div class="mermaid-viewer">
	{#if error}
		<div class="error-message">
			<p class="text-sm text-red-600">{error}</p>
			<details class="mt-2">
				<summary class="text-xs text-gray-600 cursor-pointer">Show code</summary>
				<pre class="code-block mt-2"><code>{mermaidCode}</code></pre>
			</details>
		</div>
	{:else if svg}
		<div class="diagram-container">
			{@html svg}
		</div>
	{/if}
</div>

<style>
	.mermaid-viewer {
		width: 100%;
	}

	.error-message {
		padding: 1rem;
		background: #fef2f2;
		border: 1px solid #fecaca;
		border-radius: 0.375rem;
	}

	.diagram-container {
		width: 100%;
		overflow-x: auto;
		padding: 1rem;
		background: #ffffff;
		border: 1px solid #e5e7eb;
		border-radius: 0.375rem;
	}

	.diagram-container :global(svg) {
		max-width: 100%;
		height: auto;
	}

	.code-block {
		background: #1e293b;
		color: #e2e8f0;
		padding: 0.75rem;
		border-radius: 0.25rem;
		overflow-x: auto;
		font-family: 'Courier New', monospace;
		font-size: 0.75rem;
		line-height: 1.5;
		margin: 0;
	}
</style>
