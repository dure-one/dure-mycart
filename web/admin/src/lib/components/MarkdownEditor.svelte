<script lang="ts">
	import { onMount, onDestroy } from 'svelte'
	import { Editor } from '@tiptap/core'
	import StarterKit from '@tiptap/starter-kit'
	import { Markdown } from '@tiptap/markdown'
	import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
	import { common, createLowlight } from 'lowlight'
	import { marked } from 'marked'

	interface Props {
		value: string
		onchange?: (content: string) => void
		placeholder?: string
	}

	let { value = '', onchange, placeholder = 'Enter markdown content...' }: Props = $props()
	let element: HTMLDivElement
	let editor: Editor | null = null
	let viewMode = $state<'wysiwyg' | 'source'>('wysiwyg')
	let sourceContent = $state(value)

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
		const html = marked.parse(value || '') as string

		editor = new Editor({
			element: element,
			extensions: extensions,
			content: html,
			editable: true,
			editorProps: {
				attributes: {
					class: 'prose prose-sm max-w-none focus:outline-none'
				}
			},
			onUpdate: ({ editor }) => {
				// Get markdown from editor using getMarkdown method
				if (viewMode === 'wysiwyg' && typeof (editor as any).getMarkdown === 'function') {
					const markdown = (editor as any).getMarkdown()
					onchange?.(markdown)
				}
			}
		})
	})

	onDestroy(() => {
		editor?.destroy()
	})

	$effect(() => {
		if (editor && value && viewMode === 'wysiwyg') {
			const currentMarkdown = typeof (editor as any).getMarkdown === 'function'
				? (editor as any).getMarkdown()
				: ''

			if (value !== currentMarkdown) {
				const html = marked.parse(value || '') as string
				editor.commands.setContent(html)
			}
		}
	})

	function toggleMode(mode: 'wysiwyg' | 'source') {
		if (mode === 'source' && editor) {
			// Switching to source mode - get markdown from editor
			if (typeof (editor as any).getMarkdown === 'function') {
				sourceContent = (editor as any).getMarkdown()
			} else {
				// Fallback: get HTML and keep it
				sourceContent = editor.getHTML()
			}

			// Destroy editor when switching to source mode
			editor.destroy()
			editor = null
		} else if (mode === 'wysiwyg') {
			// Switching to WYSIWYG mode - need to recreate editor
			viewMode = mode

			// Wait for DOM to update, then create editor
			setTimeout(() => {
				if (element && !editor) {
					const html = marked.parse(sourceContent || '') as string

					editor = new Editor({
						element: element,
						extensions: extensions,
						content: html,
						editable: true,
						editorProps: {
							attributes: {
								class: 'prose prose-sm max-w-none focus:outline-none'
							}
						},
						onUpdate: ({ editor }) => {
							if (viewMode === 'wysiwyg' && typeof (editor as any).getMarkdown === 'function') {
								const markdown = (editor as any).getMarkdown()
								onchange?.(markdown)
							}
						}
					})
				}
			}, 0)
			return
		}
		viewMode = mode
	}

	function handleSourceChange(e: Event) {
		const target = e.target as HTMLTextAreaElement
		sourceContent = target.value
		onchange?.(sourceContent)
	}

	// Sync external value changes with sourceContent
	$effect(() => {
		if (value && value !== sourceContent) {
			sourceContent = value
		}
	})
</script>

<div class="markdown-editor-wrapper">
	<!-- Toolbar -->
	<div class="editor-toolbar">
		<button
			type="button"
			class="toolbar-btn"
			class:active={viewMode === 'wysiwyg'}
			onclick={() => toggleMode('wysiwyg')}
			title="WYSIWYG Mode"
		>
			<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
			</svg>
		</button>
		<button
			type="button"
			class="toolbar-btn"
			class:active={viewMode === 'source'}
			onclick={() => toggleMode('source')}
			title="Source Code Mode"
		>
			<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
			</svg>
		</button>
	</div>

	<!-- Editor or Source -->
	{#if viewMode === 'wysiwyg'}
		<div bind:this={element} class="markdown-editor"></div>
	{:else}
		<textarea
			bind:value={sourceContent}
			oninput={handleSourceChange}
			class="source-editor"
			placeholder={placeholder}
		></textarea>
	{/if}
</div>

<style>
	.markdown-editor-wrapper {
		border: 1px solid #d1d5db;
		border-radius: 0.375rem;
		background: #ffffff;
		overflow: hidden;
	}

	.editor-toolbar {
		display: flex;
		gap: 0.25rem;
		padding: 0.5rem;
		background: #f9fafb;
		border-bottom: 1px solid #e5e7eb;
	}

	.toolbar-btn {
		padding: 0.5rem;
		border: none;
		background: transparent;
		border-radius: 0.375rem;
		cursor: pointer;
		color: #6b7280;
		transition: all 0.2s;
	}

	.toolbar-btn:hover {
		background: #e5e7eb;
		color: #1f2937;
	}

	.toolbar-btn.active {
		background: #3b82f6;
		color: #ffffff;
	}

	.markdown-editor {
		min-height: 400px;
		background: #ffffff;
	}

	.source-editor {
		width: 100%;
		min-height: 400px;
		padding: 1rem;
		border: none;
		font-family: 'Courier New', monospace;
		font-size: 0.875rem;
		line-height: 1.5;
		resize: vertical;
		background: #1e293b;
		color: #e2e8f0;
	}

	.source-editor:focus {
		outline: none;
	}

	:global(.markdown-editor .tiptap) {
		padding: 1rem;
		min-height: 400px;
	}

	:global(.markdown-editor .tiptap:focus) {
		outline: none;
	}

	:global(.markdown-editor pre) {
		background: #1e293b;
		color: #e2e8f0;
		font-family: 'Courier New', monospace;
		padding: 0.75rem 1rem;
		border-radius: 0.5rem;
		overflow-x: auto;
		margin: 1rem 0;
	}

	:global(.markdown-editor pre code) {
		background: none;
		color: inherit;
		font-size: 0.875rem;
		padding: 0;
	}

	:global(.markdown-editor code) {
		background: #f1f5f9;
		padding: 0.125rem 0.375rem;
		border-radius: 0.25rem;
		font-size: 0.875em;
		color: #e11d48;
	}

	:global(.markdown-editor h1) {
		font-size: 1.875rem;
		font-weight: 700;
		margin-top: 2rem;
		margin-bottom: 1rem;
	}

	:global(.markdown-editor h2) {
		font-size: 1.5rem;
		font-weight: 600;
		margin-top: 1.5rem;
		margin-bottom: 0.75rem;
	}

	:global(.markdown-editor h3) {
		font-size: 1.25rem;
		font-weight: 600;
		margin-top: 1.25rem;
		margin-bottom: 0.5rem;
	}

	:global(.markdown-editor p) {
		margin-bottom: 1rem;
		line-height: 1.75;
	}

	:global(.markdown-editor ul, .markdown-editor ol) {
		margin-left: 1.5rem;
		margin-bottom: 1rem;
	}

	:global(.markdown-editor li) {
		margin-bottom: 0.25rem;
	}

	/* Syntax highlighting */
	:global(.markdown-editor .hljs-comment),
	:global(.markdown-editor .hljs-quote) {
		color: #64748b;
		font-style: italic;
	}

	:global(.markdown-editor .hljs-keyword),
	:global(.markdown-editor .hljs-selector-tag),
	:global(.markdown-editor .hljs-literal) {
		color: #c084fc;
		font-weight: 600;
	}

	:global(.markdown-editor .hljs-string),
	:global(.markdown-editor .hljs-regexp) {
		color: #86efac;
	}

	:global(.markdown-editor .hljs-function),
	:global(.markdown-editor .hljs-title),
	:global(.markdown-editor .hljs-class .hljs-title) {
		color: #60a5fa;
	}

	:global(.markdown-editor .hljs-number) {
		color: #fbbf24;
	}

	:global(.markdown-editor .hljs-built_in),
	:global(.markdown-editor .hljs-builtin-name) {
		color: #fb923c;
	}

	:global(.markdown-editor .hljs-params) {
		color: #94a3b8;
	}

	:global(.markdown-editor .hljs-attr),
	:global(.markdown-editor .hljs-attribute) {
		color: #fbbf24;
	}

	:global(.markdown-editor .hljs-variable) {
		color: #e2e8f0;
	}

	:global(.markdown-editor .hljs-operator) {
		color: #c084fc;
	}

	/* Placeholder */
	:global(.markdown-editor .tiptap p.is-editor-empty:first-child::before) {
		color: #9ca3af;
		content: attr(data-placeholder);
		float: left;
		height: 0;
		pointer-events: none;
	}
</style>
