import { get } from 'svelte/store'
import { DATE_FORMATS, getDateFormat } from '$lib/config/dateFormats'
import { mainSettingsStore } from '$lib/stores/main'

export function formatDate(
  dateValue: string | number | null | undefined,
  formatCode?: string
): string {
  if (!dateValue) return ''

  try {
    let date: Date
    if (typeof dateValue === 'number') {
      date = new Date(dateValue * 1000)
    } else {
      date = new Date(dateValue)
    }

    // Use provided format, or fall back to store setting, or default to EU
    const code = formatCode || get(mainSettingsStore).date_time_format || 'eu'
    const format = getDateFormat(code)

    if (!format) {
      // Fallback to ISO format
      return date.toLocaleDateString('en-US', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false
      })
    }

    return date.toLocaleString(format.locale || 'en-US', format.options)
  } catch (error) {
    return String(dateValue)
  }
}
