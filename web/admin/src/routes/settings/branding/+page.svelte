<script lang="ts">
  import { onMount } from 'svelte'
  import Main from '$lib/layouts/Main.svelte'
  import { FormButton, FormInput, FormUpload, PageHeader, PageState, Section } from '$lib/components'
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

  // The two marks are uploaded rather than typed, and each upload answers with
  // the group as it now stands — so the page takes its state from the response
  // rather than guessing at the name the server chose.
  function uploaded(kind: MarkKind, res: any) {
    if (!res?.success) return
    formData = { ...formData, ...res.result }
    showMessage(t(`branding.${kind}Uploaded`), 'connextSuccess')
  }

  async function remove(kind: MarkKind) {
    const res = await apiDelete(`/api/_/settings/branding/${kind}`)
    if (!res.success) {
      showMessage(res.message || t(`branding.${kind}RemoveFailed`), 'connextError')
      return
    }
    formData = { ...formData, ...res.result }
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

  // Both marks are drawn from the same block: a preview, the file that is in
  // use, and the two actions that change it.
  const marks: { kind: MarkKind; title: string; hint: string; ratio: string }[] = [
    {
      kind: 'logo',
      title: 'branding.logo',
      hint: 'branding.logoDesc',
      ratio: 'h-16 w-auto max-w-[16rem] object-contain'
    },
    {
      kind: 'favicon',
      title: 'branding.favicon',
      hint: 'branding.faviconDesc',
      ratio: 'h-16 w-16 object-contain'
    }
  ]
</script>

<Main>
  <PageHeader title={t('branding.title')} />

  {#if loading}
    <PageState kind="loading" />
  {:else}
    <Section title={t('branding.marks')}>
      <form onsubmit={(e) => { e.preventDefault(); handleSubmit() }} class="max-w-2xl space-y-6">
        {#each marks as mark (mark.kind)}
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="min-w-0">
              <h3>{t(mark.title)}</h3>
              <p class="text-sm text-gray-500">{t(mark.hint)}</p>
              {#if formData[mark.kind]}
                <p class="mt-2 text-xs text-gray-500">{formData[mark.kind]}</p>
              {/if}
            </div>

            <div class="flex shrink-0 items-center gap-3">
              {#if formData[mark.kind]}
                <!-- The storefront builds the same address from the same name;
                     this is the operator seeing what they uploaded. -->
                <img
                  src={`/uploads/${formData[mark.kind]}`}
                  alt=""
                  class={`shrink-0 border border-gray-200 bg-white p-1 ${mark.ratio}`}
                />
                <FormButton
                  type="button"
                  name={t('common.delete')}
                  variant="secondary"
                  onclick={() => remove(mark.kind)}
                />
              {/if}
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
            id="branding_tagline"
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
    </Section>
  {/if}
</Main>
