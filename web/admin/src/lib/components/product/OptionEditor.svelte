<script lang="ts">
  import { FormButton, FormInput, IconButton, SvgIcon } from '$lib/components'
  import type { ProductOption } from '$lib/types/models'
  import { translate } from '$lib/i18n'

  let t = $derived($translate)

  interface Props {
    option: ProductOption
    optionIndex: number
    onUpdate: (option: ProductOption) => void
    onDelete: () => void
    disabled?: boolean
  }

  let { option, optionIndex, onUpdate, onDelete, disabled = false }: Props = $props()

  let localOption = $state<ProductOption>({ ...option })

  function updateOption() {
    onUpdate(localOption)
  }

  function updateOptionName(event: Event) {
    const target = event.target as HTMLInputElement
    localOption.name = target.value
    updateOption()
  }

  function updateValueName(index: number, event: Event) {
    const target = event.target as HTMLInputElement
    localOption.values = localOption.values.map((v, i) =>
      i === index ? { ...v, value: target.value } : v
    )
    updateOption()
  }

  function addValue() {
    if (localOption.values.length >= 10) {
      return // Max 10 values per option
    }
    localOption.values = [
      ...localOption.values,
      { value: '', position: localOption.values.length }
    ]
    updateOption()
  }

  function deleteValue(index: number) {
    if (localOption.values.length <= 1) {
      return // Must have at least 1 value
    }
    localOption.values = localOption.values
      .filter((_, i) => i !== index)
      .map((v, i) => ({ ...v, position: i }))
    updateOption()
  }
</script>

<div class="mb-6 rounded-lg border border-gray-200 bg-gray-50 p-4">
  <div class="mb-4 flex items-center justify-between">
    <h3>{t('products.option')} {optionIndex + 1}</h3>
    <IconButton ico="trash" label={t('common.delete')} variant="danger" onclick={onDelete} {disabled} />
  </div>

  <div class="mb-4">
    <FormInput
      id="option-{optionIndex + 1}-name"
      label={t('products.optionNameNumbered', { index: optionIndex + 1 })}
      type="text"
      value={localOption.name}
      oninput={updateOptionName}
      placeholder={t('products.optionNamePlaceholder')}
      {disabled}
      required
    />
  </div>

  <div class="mb-2">
    {#each localOption.values as value, index (index)}
      <div class="mb-2 flex items-center gap-2">
        <div class="flex-1">
          <FormInput
            id="option-{optionIndex + 1}-value-{index + 1}"
            label={t('products.optionValueNumbered', { index: optionIndex + 1, value: index + 1 })}
            type="text"
            value={value.value}
            oninput={(e) => updateValueName(index, e)}
            placeholder={t('products.optionValueNumbered', { index: optionIndex + 1, value: index + 1 })}
            {disabled}
            required
          />
        </div>
        <IconButton
          ico="minus"
          label={t('common.delete')}
          variant="danger"
          onclick={() => deleteValue(index)}
          disabled={disabled || localOption.values.length <= 1}
        />
      </div>
    {/each}

    {#if localOption.values.length < 10}
      <FormButton
        type="button"
        variant="secondary"
        size="sm"
        onclick={addValue}
        {disabled}
      >
        <SvgIcon name="plus" className="mr-1 h-4 w-4" />
        {t('products.addValue')}
      </FormButton>
    {/if}
  </div>
</div>
