<script lang="ts">
  import { DrawerFooter, DrawerHeader, FormInput, FormTextarea } from '$lib/components'
  import { loadData, saveData } from '$lib/utils/apiHelpers'
  import type { Page } from '$lib/types/models'
  import { translate } from '$lib/i18n'

  // Reactive translation function
  let t = $derived($translate)

  interface Props {
    page: Page
    onclose?: () => void
  }

  let { page, onclose }: Props = $props()

  let seoData = $state({
    title: '',
    keywords: '',
    description: ''
  })

  // Reactively load data when page.id changes
  $effect(() => {
    if (page?.id) {
      loadPage()
    }
  })

  async function loadPage() {
    if (!page?.id) return

    const pageData = await loadData<Page>(`/api/_/pages/${page.id}`, t('pages.failedToLoadPage'))
    if (pageData) {
      seoData = {
        title: pageData.seo?.title || '',
        keywords: pageData.seo?.keywords || '',
        description: pageData.seo?.description || ''
      }
    }
  }

  async function handleSubmit() {
    await saveData<Page>(
      `/api/_/pages/${page.id}`,
      { seo: seoData },
      true,
      t('pages.pageSaved'),
      t('pages.failedToSavePage')
    )
  }

  function close() {
    onclose?.()
  }
</script>

<div>
  <DrawerHeader title={t('pages.seo')} />

  <form onsubmit={(e) => { e.preventDefault(); handleSubmit() }}>
    <div class="flow-root">
      <dl class="mx-auto -my-3 mt-2 mb-0 space-y-4 text-sm">
        <FormInput id="seo-title" title={t('pages.seoTitle')} bind:value={seoData.title} ico="glob-alt" />
        <FormInput id="seo-keywords" title={t('pages.seoKeywords')} bind:value={seoData.keywords} ico="glob-alt" />
        <hr />
        <FormTextarea id="seo-description" title={t('pages.seoDescription')} bind:value={seoData.description} />
      </dl>
    </div>

    <DrawerFooter submitLabel={t('common.save')} onclose={close} />
  </form>
</div>
