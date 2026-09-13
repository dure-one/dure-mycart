import { writable, derived, get } from 'svelte/store'
import en from './locales/en.json'
import zh from './locales/zh.json'
import ko from './locales/ko.json'
import fr from './locales/fr.json'
import es from './locales/es.json'
import de from './locales/de.json'
import it from './locales/it.json'
import be from './locales/be.json'

export type Locale = 'en' | 'zh' | 'ko' | 'fr' | 'es' | 'de' | 'it' | 'be'

const translations: Record<Locale, any> = {
  en,
  zh,
  ko,
  fr,
  es,
  de,
  it,
  be
}

// Order the pickers offer the locales in, and the name each locale calls
// itself — a language list reads the same whichever language is active.
const localeNames: Record<Locale, string> = {
  en: 'English',
  zh: '中文',
  ko: '한국어',
  fr: 'Français',
  es: 'Español',
  de: 'Deutsch',
  it: 'Italiano',
  be: 'Беларуская'
}

const localeOrder = Object.keys(localeNames) as Locale[]

const defaultLocale: Locale = 'en'

function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && value in localeNames
}

function syncDocumentLanguage(localeValue: Locale) {
  if (typeof document !== 'undefined') {
    document.documentElement.lang = localeValue
  }
}

// Store for current locale
function createLocaleStore() {
  const { subscribe, set, update } = writable<Locale>(defaultLocale)

  // Load locale from localStorage on initialization
  if (typeof window !== 'undefined') {
    const saved = localStorage.getItem('locale')
    if (isLocale(saved)) {
      set(saved)
      syncDocumentLanguage(saved)
    }
  }

  return {
    subscribe,
    set: (locale: Locale) => {
      set(locale)
      syncDocumentLanguage(locale)
      if (typeof window !== 'undefined') {
        localStorage.setItem('locale', locale)
      }
    },
    update
  }
}

export const locale = createLocaleStore()

// Helper function to get translation
function getTranslation(localeValue: Locale, key: string, params?: Record<string, string | number>): string {
  const keys = key.split('.')
  let value: any = translations[localeValue]

  for (const k of keys) {
    if (value && typeof value === 'object' && k in value) {
      value = value[k]
    } else {
      // Fallback to English if key not found
      value = translations.en
      for (const fallbackKey of keys) {
        if (value && typeof value === 'object' && fallbackKey in value) {
          value = value[fallbackKey]
        } else {
          return key // Return key if translation not found
        }
      }
      break
    }
  }

  if (typeof value !== 'string') {
    return key
  }

  // Replace parameters in string
  if (params) {
    return value.replace(/\{\{(\w+)\}\}/g, (match, paramKey) => {
      return params[paramKey]?.toString() || match
    })
  }

  return value
}

// Function to get translation by key (non-reactive, for use outside components)
export function t(key: string, params?: Record<string, string | number>): string {
  const currentLocale = get(locale)
  return getTranslation(currentLocale, key, params)
}

// Derived store for reactive access to translations
export const translate = derived(locale, (currentLocale) => {
  return (key: string, params?: Record<string, string | number>) => {
    return getTranslation(currentLocale, key, params)
  }
})

// List of available locales with their native names
export function getAvailableLocales(): Array<{ code: Locale; name: string }> {
  return localeOrder.map((code) => ({ code, name: localeNames[code] }))
}

// Derived store for available locales
export const availableLocales = derived(locale, () => getAvailableLocales())
