<script lang="ts">
  import FormButton from './form/Button.svelte'
  import ActionLink from './ActionLink.svelte'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  interface Props {
    onclose?: () => void
    /** Omit for a read-only drawer: the footer then offers Close alone. */
    submitLabel?: string
    /**
     * The trailing action of this drawer, opposite the submit/close pair.
     * Named for its common case, the destructive Delete; pair it with
     * `actionVariant="default"` for a trailing action that is not destructive,
     * such as the mail test letter.
     */
    ondelete?: () => void
    deleteLabel?: string
    actionVariant?: 'default' | 'danger'
  }

  let {
    onclose,
    submitLabel = undefined,
    ondelete = undefined,
    deleteLabel = undefined,
    actionVariant = 'danger'
  }: Props = $props()
</script>

<div class="flex items-center pt-8">
  <div class="flex flex-none items-center gap-2">
    {#if submitLabel}
      <FormButton type="submit" variant="primary" name={submitLabel} />
      <FormButton type="button" variant="secondary" name={t('common.close')} onclick={onclose} />
    {:else}
      <FormButton type="button" variant="primary" name={t('common.close')} onclick={onclose} />
    {/if}
  </div>
  <div class="grow"></div>
  {#if ondelete}
    <div class="flex-none">
      <ActionLink variant={actionVariant} name={deleteLabel ?? t('common.delete')} onclick={ondelete} />
    </div>
  {/if}
</div>
