<script lang="ts">
  import { onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import { FormButton, FormInput, FormUpload, IconButton, PageHeader, PageState } from '$lib/components'
  import { showMessage } from '$lib/utils'
  import { apiDelete } from '$lib/utils/api'
  import { loadSettings, saveSettings } from '$lib/utils/settingsHelpers'
  import { translate } from '$lib/i18n'
  import type { Branding } from '$lib/types/models'

  // Reactive translation function
  let t = $derived($translate)

  const EMPTY: Branding = { logo: '', favicon: '', tagline: '' }

  let formData = $state<Branding>({ ...EMPTY })
  let loading = $state(true)

  onMount(async () => {
    // Merged over the empty group rather than assigned: the form binds its two
    // field values straight through, and a key the server did not send would
    // bind them to undefined.
    const loaded = await loadSettings<Branding>('branding', { ...EMPTY })
    formData = { ...EMPTY, ...loaded }
    loading = false
  })

  // The name the server chose for the stored file comes from the answer rather
  // than being guessed at here — but only that one field is taken from it.
  //
  // Both endpoints answer with the whole group, and the form is not the group:
  // a tagline typed and not yet saved is in the form, while the answer carries
  // the one still on file, which on a shop that has never saved it is empty.
  // Merging the answer in would drop what the operator had just typed — upload
  // a logo after typing a tagline and the tagline goes, with nothing on screen
  // saying so.
  function applyMark(kind: MarkKind, res: any): boolean {
    if (!res?.success) return false
    formData = { ...formData, [kind]: res.result?.[kind] ?? formData[kind] }
    return true
  }

  function uploaded(kind: MarkKind, res: any) {
    if (!applyMark(kind, res)) return
    showMessage(t(`branding.${kind}Uploaded`), 'connextSuccess')
  }

  async function remove(kind: MarkKind) {
    const res = await apiDelete(`/api/_/settings/branding/${kind}`)
    if (!applyMark(kind, res)) {
      showMessage(res.message || t(`branding.${kind}RemoveFailed`), 'connextError')
      return
    }
    showMessage(t(`branding.${kind}Removed`), 'connextSuccess')
  }

  async function handleSubmit() {
    // The marks ride along unchanged: the endpoint validates the whole group,
    // and one of them arriving empty here would clear it.
    await saveSettings('branding', {
      logo: formData.logo,
      favicon: formData.favicon,
      tagline: formData.tagline
    })
  }

  type MarkKind = 'logo' | 'favicon'

  // Both marks are drawn from the same block — a heading, what the file is for,
  // the mark in use and the drop target that replaces it — so the two read as
  // the same control and neither jumps when the other has something set. The
  // preview keeps the shape the file is used in: the storefront draws the logo
  // at its own aspect ratio and the favicon square.
  const marks: { kind: MarkKind; title: string; hint: string; preview: string }[] = [
    {
      kind: 'logo',
      title: 'branding.logo',
      hint: 'branding.logoDesc',
      preview: 'h-16 w-auto max-w-[16rem] object-contain'
    },
    {
      kind: 'favicon',
      title: 'branding.favicon',
      hint: 'branding.faviconDesc',
      preview: 'h-16 w-16 object-contain'
    }
  ]
</script>

<Main>
  <PageHeader title={t('branding.title')} />

  {#if loading}
    <PageState kind="loading" />
  {:else}
    <form onsubmit={(e) => { e.preventDefault(); handleSubmit() }} class="max-w-2xl space-y-6">
      {#each marks as mark (mark.kind)}
        <div>
          <h3>{t(mark.title)}</h3>
          <p class="text-sm text-gray-500">{t(mark.hint)}</p>

          {#if formData[mark.kind]}
            <!-- The storefront builds the same address from the same name;
                 this is the operator seeing what they uploaded, and the one
                 place the mark is taken back down again. -->
            <div class="relative mt-4 w-fit">
              <img
                src={`/uploads/${formData[mark.kind]}`}
                alt=""
                class={`border border-gray-200 bg-white p-1 ${mark.preview}`}
              />
              <div class="absolute end-1 top-1">
                <IconButton
                  ico="trash"
                  label={t('common.delete')}
                  variant="danger"
                  class="bg-white"
                  onclick={() => remove(mark.kind)}
                />
              </div>
            </div>
          {/if}

          <div class="mt-4">
            <FormUpload
              url={`/api/_/settings/branding/${mark.kind}`}
              accept="image/png,image/jpeg"
              multiple={false}
              onadded={(res: any) => uploaded(mark.kind, res)}
            />
          </div>
        </div>
      {/each}

      <div>
        <FormInput
          id="tagline"
          title={t('branding.tagline')}
          ico="pencil"
          bind:value={formData.tagline}
        />
        <p class="mt-2 text-sm text-gray-500">{t('branding.taglineDesc')}</p>
      </div>

      <div class="pt-4">
        <FormButton type="submit" name={t('common.save')} variant="primary" />
      </div>
    </form>
  {/if}
</Main>
