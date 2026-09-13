<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { Editor } from '@tiptap/core'
  import StarterKit from '@tiptap/starter-kit'
  import Placeholder from '@tiptap/extension-placeholder'
  import IconButton from './IconButton.svelte'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  interface Props {
    modelValue?: string
    placeholder?: string
    id?: string
    onupdateModelValue?: (value: string) => void
  }

  let { modelValue = $bindable(''), placeholder = '', id = undefined, onupdateModelValue }: Props = $props()

  interface EditorAction {
    /** The sprite icon and the `editor.<name>` translation key. */
    name: string
    icon: string
    /** ProseMirror shortcut notation, spelled for the reader in the tooltip. */
    shortcut?: string
    run: (editor: Editor) => void
    /** Only the toggling actions carry one — the button then reports `aria-pressed`. */
    isActive?: (editor: Editor) => boolean
    /** Omitted where the action is always available. */
    isEnabled?: (editor: Editor) => boolean
  }

  // Reading order: history, marks, block types, lists, then the pair that
  // inserts or clears. Each group is a divider in the toolbar.
  const ACTION_GROUPS: EditorAction[][] = [
    [
      {
        name: 'undo',
        icon: 'undo',
        shortcut: 'Mod-z',
        run: (editor) => editor.chain().focus().undo().run(),
        isEnabled: (editor) => editor.can().undo()
      },
      {
        name: 'redo',
        icon: 'redo',
        shortcut: 'Mod-Shift-z',
        run: (editor) => editor.chain().focus().redo().run(),
        isEnabled: (editor) => editor.can().redo()
      }
    ],
    [
      {
        name: 'bold',
        icon: 'bold',
        shortcut: 'Mod-b',
        run: (editor) => editor.chain().focus().toggleBold().run(),
        isActive: (editor) => editor.isActive('bold')
      },
      {
        name: 'italic',
        icon: 'italic',
        shortcut: 'Mod-i',
        run: (editor) => editor.chain().focus().toggleItalic().run(),
        isActive: (editor) => editor.isActive('italic')
      },
      {
        name: 'strike',
        icon: 'strike',
        shortcut: 'Mod-Shift-s',
        run: (editor) => editor.chain().focus().toggleStrike().run(),
        isActive: (editor) => editor.isActive('strike')
      }
    ],
    [
      {
        name: 'paragraph',
        icon: 'paragraph',
        shortcut: 'Mod-Alt-0',
        run: (editor) => editor.chain().focus().setParagraph().run(),
        isActive: (editor) => editor.isActive('paragraph')
      },
      {
        name: 'h1',
        icon: 'h1',
        shortcut: 'Mod-Alt-1',
        run: (editor) => editor.chain().focus().toggleHeading({ level: 1 }).run(),
        isActive: (editor) => editor.isActive('heading', { level: 1 })
      },
      {
        name: 'h2',
        icon: 'h2',
        shortcut: 'Mod-Alt-2',
        run: (editor) => editor.chain().focus().toggleHeading({ level: 2 }).run(),
        isActive: (editor) => editor.isActive('heading', { level: 2 })
      },
      {
        name: 'h3',
        icon: 'h3',
        shortcut: 'Mod-Alt-3',
        run: (editor) => editor.chain().focus().toggleHeading({ level: 3 }).run(),
        isActive: (editor) => editor.isActive('heading', { level: 3 })
      },
      {
        name: 'blockquote',
        icon: 'blockquote',
        shortcut: 'Mod-Shift-b',
        run: (editor) => editor.chain().focus().toggleBlockquote().run(),
        isActive: (editor) => editor.isActive('blockquote')
      }
    ],
    [
      {
        name: 'bulletlist',
        icon: 'bulletlist',
        shortcut: 'Mod-Shift-8',
        run: (editor) => editor.chain().focus().toggleBulletList().run(),
        isActive: (editor) => editor.isActive('bulletList')
      },
      {
        name: 'orderedList',
        icon: 'orderedlist',
        shortcut: 'Mod-Shift-7',
        run: (editor) => editor.chain().focus().toggleOrderedList().run(),
        isActive: (editor) => editor.isActive('orderedList')
      }
    ],
    [
      {
        name: 'horizontalRule',
        icon: 'minus',
        run: (editor) => editor.chain().focus().setHorizontalRule().run()
      },
      {
        name: 'clearFormatting',
        icon: 'eraser',
        run: (editor) => editor.chain().focus().unsetAllMarks().clearNodes().run()
      }
    ]
  ]

  const ACTIONS = ACTION_GROUPS.flat()

  interface ActionState {
    active: boolean
    enabled: boolean
  }

  let editor: Editor | null = $state(null)
  let editorElement: HTMLElement | undefined = $state()

  // ProseMirror owns the truth about what is toggled and what can still be
  // undone; this mirror is what carries it into the toolbar. Without it the
  // template reads the editor once and the buttons never move again.
  let actionState = $state<Record<string, ActionState>>({})

  function syncActionState(target: Editor | null) {
    const next: Record<string, ActionState> = {}
    if (target) {
      for (const action of ACTIONS) {
        next[action.name] = {
          active: action.isActive?.(target) ?? false,
          enabled: action.isEnabled?.(target) ?? true
        }
      }
    }
    actionState = next
  }

  // TipTap spells a shortcut the ProseMirror way; a tooltip spells it the way
  // this keyboard does, so the hint reads Mod-b as ⌘B on a Mac and Ctrl+B
  // anywhere else.
  const MODIFIER_LABELS: Record<string, [string, string]> = {
    Mod: ['Ctrl', '⌘'],
    Shift: ['Shift', '⇧'],
    Alt: ['Alt', '⌥']
  }

  let isMac = $state(false)

  function shortcutLabel(shortcut: string): string {
    const parts = shortcut.split('-').map((part) => {
      const labels = MODIFIER_LABELS[part]
      if (labels) return isMac ? labels[1] : labels[0]
      return part.toUpperCase()
    })
    return parts.join(isMac ? '' : '+')
  }

  function tooltip(action: EditorAction): string {
    const label = t(`editor.${action.name}`)
    return action.shortcut ? `${label} (${shortcutLabel(action.shortcut)})` : label
  }

  // Push an external change — another product opening in the drawer, a form
  // reset — into the editor. Guarded on inequality so the write-back that
  // typing performs never re-sets the document under the caret.
  $effect(() => {
    if (editor && modelValue !== editor.getHTML()) {
      editor.commands.setContent(modelValue, { emitUpdate: false })
    }
  })

  onMount(() => {
    isMac = /Mac|iPhone|iPad|iPod/.test(navigator.userAgent)
    if (!editorElement) return
    editor = new Editor({
      element: editorElement,
      extensions: [
        StarterKit,
        Placeholder.configure({
          placeholder: placeholder
        })
      ],
      content: modelValue,
      onUpdate: ({ editor }) => {
        const html = editor.getHTML()
        modelValue = html
        onupdateModelValue?.(html)
      },
      onCreate: ({ editor }) => syncActionState(editor),
      onTransaction: ({ editor }) => syncActionState(editor)
    })
    syncActionState(editor)
  })

  onDestroy(() => {
    if (editor) {
      editor.destroy()
      editor = null
    }
  })
</script>

<div class="editor">
  {#if editor}
    <div class="editor-toolbar" role="toolbar" aria-label={t('editor.toolbar')}>
      {#each ACTION_GROUPS as group, index (index)}
        {#if index > 0}
          <span class="editor-divider" aria-hidden="true"></span>
        {/if}
        {#each group as action (action.name)}
          {@const state = actionState[action.name]}
          <IconButton
            ico={action.icon}
            label={tooltip(action)}
            active={action.isActive ? (state?.active ?? false) : undefined}
            disabled={state ? !state.enabled : false}
            onclick={() => {
              if (editor) action.run(editor)
            }}
          />
        {/each}
      {/each}
    </div>
  {/if}
  <article class="editor-surface" bind:this={editorElement} {id}></article>
</div>

<style>
  @reference "tailwindcss";

  /*
   * The shell is the field: one border holding the toolbar and the writing
   * surface together, going green while it holds the caret — the same focus
   * treatment FormInput and FormTextarea carry.
   */
  .editor {
    @apply rounded border border-gray-300 bg-white focus-within:border-green-600 focus-within:ring-1 focus-within:ring-green-600;
  }

  /*
   * Pinned to the top of the drawer's scroll area, so a description long
   * enough to scroll away does not take its toolbar with it. The offset walks
   * back over the drawer's own padding — `p-6` in `Drawer` — because a toolbar
   * stopped at the padding edge leaves a band of scrolled text above it. The
   * radius is the shell's, less its 1px border.
   */
  .editor-toolbar {
    @apply sticky -top-6 z-10 flex flex-wrap items-center gap-0.5 rounded-t-[3px] border-b border-gray-200 bg-gray-50 px-1.5 py-1;
  }

  .editor-divider {
    @apply mx-1 h-5 w-px bg-gray-200;
  }

  .editor-surface {
    @apply block px-3 py-2.5 text-sm text-gray-800;
  }

  /*
   * The height belongs to the contenteditable itself, not to the shell around
   * it: an empty field is then a clickable writing area rather than a box
   * whose only live part is the first line.
   */
  :global(.editor-surface .tiptap) {
    @apply min-h-40;
  }

  .editor-toolbar :global(button:focus-visible) {
    @apply ring-1 ring-green-600 outline-none;
  }

  :global(.editor-surface .tiptap:focus) {
    outline: none;
  }
</style>
