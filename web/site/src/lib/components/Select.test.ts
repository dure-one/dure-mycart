import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Select from './Select.svelte'

const options = [
  { value: 'en', label: 'English' },
  { value: 'zh', label: '中文' },
  { value: 'be', label: 'Беларуская' }
]

function setup(value = 'en') {
  const onChange = vi.fn()
  const { getByRole, queryByRole, getAllByRole } = render(Select, {
    props: { value, options, label: 'Language', onChange }
  })
  return { onChange, getByRole, queryByRole, getAllByRole }
}

describe('Select', () => {
  it('shows the current choice on a closed trigger', () => {
    const { getByRole, queryByRole } = setup('zh')

    expect(getByRole('combobox')).toHaveTextContent('中文')
    expect(getByRole('combobox')).toHaveAttribute('aria-expanded', 'false')
    expect(queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('opens on click and lists every option, marking the current one', async () => {
    const user = userEvent.setup()
    const { getByRole, getAllByRole } = setup('be')

    await user.click(getByRole('combobox'))

    expect(getByRole('combobox')).toHaveAttribute('aria-expanded', 'true')
    expect(getAllByRole('option').map((option) => option.textContent?.trim())).toEqual([
      'English',
      '中文',
      'Беларуская'
    ])
    expect(getByRole('option', { name: /Беларуская/ })).toHaveAttribute('aria-selected', 'true')
  })

  it('moves with the arrow keys and selects with Enter', async () => {
    const user = userEvent.setup()
    const { onChange, getByRole } = setup('en')

    const trigger = getByRole('combobox')
    trigger.focus()
    await user.keyboard('{ArrowDown}')
    expect(trigger).toHaveAttribute('aria-expanded', 'true')

    await user.keyboard('{ArrowDown}{Enter}')

    expect(onChange).toHaveBeenCalledWith('zh')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(trigger).toHaveFocus()
  })

  it('wraps to the last option when opened upwards', async () => {
    const user = userEvent.setup()
    const { onChange, getByRole } = setup('en')

    const trigger = getByRole('combobox')
    trigger.focus()
    await user.keyboard('{ArrowUp}{Enter}')

    expect(onChange).toHaveBeenCalledWith('be')
  })

  it('closes on Escape without changing the value', async () => {
    const user = userEvent.setup()
    const { onChange, getByRole, queryByRole } = setup('en')

    const trigger = getByRole('combobox')
    await user.click(trigger)
    await user.keyboard('{Escape}')

    expect(queryByRole('listbox')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
    expect(onChange).not.toHaveBeenCalled()
  })

  it('closes when the click lands outside', async () => {
    const user = userEvent.setup()
    const { getByRole, queryByRole } = setup('en')

    await user.click(getByRole('combobox'))
    await user.click(document.body)

    expect(queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('reports the option picked with the mouse', async () => {
    const user = userEvent.setup()
    const { onChange, getByRole } = setup('en')

    await user.click(getByRole('combobox'))
    await user.click(getByRole('option', { name: /中文/ }))

    expect(onChange).toHaveBeenCalledWith('zh')
  })
})
