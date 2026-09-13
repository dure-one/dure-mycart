<script lang="ts">
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  interface Props {
    kind?: 'loading' | 'empty' | 'error'
    message?: string
  }

  let { kind = 'loading', message = undefined }: Props = $props()

  const KIND_CLASSES: Record<string, string> = {
    loading: 'text-gray-900',
    empty: 'text-gray-500',
    error: 'text-red-600'
  }

  let colorClass = $derived(KIND_CLASSES[kind] ?? KIND_CLASSES.empty)
  let text = $derived(message ?? (kind === 'loading' ? t('common.loading') : ''))
</script>

<div class="py-8 text-center {colorClass}">{text}</div>
