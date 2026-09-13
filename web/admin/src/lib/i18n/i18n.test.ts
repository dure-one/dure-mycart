import { describe, it, expect } from 'vitest'
import { locale, t, getAvailableLocales, type Locale } from './index'
import en from './locales/en.json'
import zh from './locales/zh.json'
import ko from './locales/ko.json'
import fr from './locales/fr.json'
import es from './locales/es.json'
import de from './locales/de.json'
import itLocale from './locales/it.json'
import be from './locales/be.json'

const NATIVE_NAMES: Record<Locale, string> = {
  en: 'English',
  zh: '中文',
  ko: '한국어',
  fr: 'Français',
  es: 'Español',
  de: 'Deutsch',
  it: 'Italiano',
  be: 'Беларуская'
}

const bundles: Record<Locale, unknown> = { en, zh, ko, fr, es, de, it: itLocale, be }

// Every locale ships the complete English key set, so nothing falls back to
// English at runtime; a new key in en.json has to be translated everywhere.
const TRANSLATED_LOCALES = (Object.keys(NATIVE_NAMES) as Locale[]).filter((code) => code !== 'en')

function keyPaths(value: unknown, prefix = ''): string[] {
  if (value === null || typeof value !== 'object') return [prefix]
  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
    keyPaths(child, prefix ? `${prefix}.${key}` : key)
  )
}

describe('locale bundles', () => {
  it.each(TRANSLATED_LOCALES)('%s translates exactly the English key set', (code) => {
    expect(keyPaths(bundles[code]).sort()).toEqual(keyPaths(en).sort())
  })
})

describe('getAvailableLocales', () => {
  it('lists every locale under its own name', () => {
    const expected = Object.entries(NATIVE_NAMES).map(([code, name]) => ({ code, name }))
    expect(getAvailableLocales()).toEqual(expected)
  })
})

describe('t', () => {
  it.each(Object.keys(NATIVE_NAMES) as Locale[])('reads keys from the active locale (%s)', (code) => {
    locale.set(code)
    expect(t('common.save')).toBe((bundles[code] as typeof en).common.save)
    expect(t('common.save')).not.toBe('common.save')
  })

  it('returns the key itself when no locale defines it', () => {
    locale.set('en')
    expect(t('does.not.exist')).toBe('does.not.exist')
  })
})
