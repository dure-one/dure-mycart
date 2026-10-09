import { writable } from 'svelte/store'
import type { MainSettings } from '$lib/types/models'

export const mainSettingsStore = writable<MainSettings>({
  site_name: '',
  domain: '',
  email: '',
  date_time_format: 'eu'
})
