<script lang="ts">
	import { onMount, onDestroy } from 'svelte'
	import { Editor, getSchema } from '@tiptap/core'
	import StarterKit from '@tiptap/starter-kit'
	import { Markdown } from '@tiptap/markdown'
	import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
	import { common, createLowlight } from 'lowlight'
	import { marked } from 'marked'

	interface Props {
		content: string
	}

	let { content }: Props = $props()
	let element: HTMLDivElement
	let editor: Editor | null = null

	const lowlight = createLowlight(common)

	const extensions = [
		StarterKit.configure({
			codeBlock: false
		}),
		CodeBlockLowlight.configure({
			lowlight,
			defaultLanguage: 'javascript'
		}),
		Markdown.configure({
			html: true,
			transformCopiedText: true,
			transformPastedText: true
		})
	]

	onMount(() => {
		// Parse markdown to HTML using marked (synchronous)
		const html = marked.parse(content || '') as string

		editor = new Editor({
			element: element,
			extensions: extensions,
			content: html,
			editable: false,
			editorProps: {
				attributes: {
					class: 'prose prose-sm max-w-none focus:outline-none'
				}
			}
		})
	})

	onDestroy(() => {
		editor?.destroy()
	})

	$effect(() => {
		if (editor && content) {
			const html = marked.parse(content) as string
			editor?.commands.setContent(html)
		}
	})
</script>

<div bind:this={element} class="markdown-viewer"></div>

<style>
	:global(.markdown-viewer .tiptap) {
		padding: 1rem;
		min-height: 200px;
	}

	:global(.markdown-viewer pre) {
		background: #1e293b;
		color: #e2e8f0;
		font-family: 'Courier New', monospace;
		padding: 0.75rem 1rem;
		border-radius: 0.5rem;
		overflow-x: auto;
		margin: 1rem 0;
	}

	:global(.markdown-viewer pre code) {
		background: none;
		color: inherit;
		font-size: 0.875rem;
		padding: 0;
	}

	:global(.markdown-viewer code) {
		background: #f1f5f9;
		padding: 0.125rem 0.375rem;
		border-radius: 0.25rem;
		font-size: 0.875em;
		color: #e11d48;
	}

	:global(.markdown-viewer h1) {
		font-size: 1.875rem;
		font-weight: 700;
		margin-top: 2rem;
		margin-bottom: 1rem;
	}

	:global(.markdown-viewer h2) {
		font-size: 1.5rem;
		font-weight: 600;
		margin-top: 1.5rem;
		margin-bottom: 0.75rem;
	}

	:global(.markdown-viewer h3) {
		font-size: 1.25rem;
		font-weight: 600;
		margin-top: 1.25rem;
		margin-bottom: 0.5rem;
	}

	:global(.markdown-viewer p) {
		margin-bottom: 1rem;
		line-height: 1.75;
	}

	:global(.markdown-viewer ul, .markdown-viewer ol) {
		margin-left: 1.5rem;
		margin-bottom: 1rem;
	}

	:global(.markdown-viewer li) {
		margin-bottom: 0.25rem;
	}

	/* Syntax highlighting - highlight.js classes */
	:global(.markdown-viewer .hljs-comment),
	:global(.markdown-viewer .hljs-quote) {
		color: #64748b;
		font-style: italic;
	}

	:global(.markdown-viewer .hljs-keyword),
	:global(.markdown-viewer .hljs-selector-tag),
	:global(.markdown-viewer .hljs-literal) {
		color: #c084fc;
		font-weight: 600;
	}

	:global(.markdown-viewer .hljs-string),
	:global(.markdown-viewer .hljs-regexp) {
		color: #86efac;
	}

	:global(.markdown-viewer .hljs-function),
	:global(.markdown-viewer .hljs-title),
	:global(.markdown-viewer .hljs-class .hljs-title) {
		color: #60a5fa;
	}

	:global(.markdown-viewer .hljs-number) {
		color: #fbbf24;
	}

	:global(.markdown-viewer .hljs-built_in),
	:global(.markdown-viewer .hljs-builtin-name) {
		color: #fb923c;
	}

	:global(.markdown-viewer .hljs-params) {
		color: #94a3b8;
	}

	:global(.markdown-viewer .hljs-attr),
	:global(.markdown-viewer .hljs-attribute) {
		color: #fbbf24;
	}

	:global(.markdown-viewer .hljs-variable) {
		color: #e2e8f0;
	}

	:global(.markdown-viewer .hljs-operator) {
		color: #c084fc;
	}
</style>
