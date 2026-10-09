export interface DateFormatConfig {
  code: string           // Format identifier
  name: string          // Display name
  pattern: string       // Intl.DateTimeFormat options serialized or custom pattern
  example: string       // Example output
  locale?: string       // Optional locale override
  options: Intl.DateTimeFormatOptions  // DateTimeFormat options
}

export const DATE_FORMATS: DateFormatConfig[] = [
  {
    code: 'iso',
    name: 'ISO 8601',
    pattern: 'YYYY-MM-DD HH:mm',
    example: '2026-09-30 01:11',
    options: { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }
  },
  {
    code: 'us',
    name: 'US',
    pattern: 'MM/DD/YYYY h:mm A',
    example: '09/30/2026 1:11 AM',
    locale: 'en-US',
    options: { year: 'numeric', month: '2-digit', day: '2-digit', hour: 'numeric', minute: '2-digit', hour12: true }
  },
  {
    code: 'uk',
    name: 'UK',
    pattern: 'DD/MM/YYYY HH:mm',
    example: '30/09/2026 01:11',
    locale: 'en-GB',
    options: { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }
  },
  {
    code: 'eu',
    name: 'EU',
    pattern: 'DD.MM.YYYY HH:mm',
    example: '30.09.2026 01:11',
    locale: 'de-DE',
    options: { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }
  },
  {
    code: 'jp',
    name: 'Japan',
    pattern: 'YYYY/MM/DD HH:mm',
    example: '2026/09/30 01:11',
    locale: 'ja-JP',
    options: { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }
  },
  {
    code: 'kr',
    name: 'Korea',
    pattern: 'YYYY.MM.DD HH:mm',
    example: '2026.09.30 01:11',
    locale: 'ko-KR',
    options: { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }
  },
  {
    code: 'cn',
    name: 'China',
    pattern: 'YYYY-MM-DD HH:mm',
    example: '2026-09-30 01:11',
    locale: 'zh-CN',
    options: { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }
  },
  {
    code: 'full-us',
    name: 'US (Full)',
    pattern: 'MMMM DD, YYYY h:mm A',
    example: 'September 30, 2026 1:11 AM',
    locale: 'en-US',
    options: { year: 'numeric', month: 'long', day: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true }
  },
  {
    code: 'full-eu',
    name: 'EU (Full)',
    pattern: 'DD MMMM YYYY HH:mm',
    example: '30 September 2026 01:11',
    locale: 'en-GB',
    options: { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }
  }
]

export function getDateFormat(code: string): DateFormatConfig | undefined {
  return DATE_FORMATS.find(f => f.code === code)
}
